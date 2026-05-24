/*
 * VXLAN 解封装 eBPF 程序
 * 
 * 功能：
 * - 解析 VXLAN 封装流量（UDP端口4789）
 * - 提取内层五元组（源/目的IP、端口、协议）
 * - 提取 VNI（Virtual Network Identifier）
 * - 支持华为云 VXLAN 格式
 * - 将解封装后的流量信息写入 BPF Map
 */

#include "vmlinux.h"
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_endian.h>

#define ETH_P_IP    0x0800
#define ETH_P_IPV6  0x86DD
#define IPPROTO_UDP 17
#define VXLAN_PORT  4789

#define BPF_OK      0

/* VXLAN 头结构 */
struct vxlanhdr {
    __u32 vx_flags;    /* Flags (8 bits) + Reserved (24 bits) */
    __u32 vx_vni;      /* VNI (24 bits) + Reserved (8 bits) */
};

/* 内层五元组信息 */
struct inner_flow_key {
    __u32 outer_src_ip;    /* 外层源IP */
    __u32 outer_dst_ip;    /* 外层目的IP */
    __u32 inner_src_ip;    /* 内层源IP */
    __u32 inner_dst_ip;    /* 内层目的IP */
    __u16 outer_src_port;  /* 外层源端口 */
    __u16 outer_dst_port;  /* 外层目的端口 */
    __u16 inner_src_port;  /* 内层源端口 */
    __u16 inner_dst_port;  /* 内层目的端口 */
    __u8  inner_protocol;  /* 内层协议 */
    __u8  inner_ip_version;/* 内层IP版本(4/6) */
    __u32 vni;             /* VXLAN VNI */
};

/* 解封装后的流量统计 */
struct decap_flow_stats {
    __u64 packets;
    __u64 bytes;
    __u64 inner_bytes;     /* 内层字节数 */
    __u64 ts_first;
    __u64 ts_last;
};

/* 内层流量信息（用于上报） */
struct inner_flow_info {
    __u32 inner_src_ip;
    __u32 inner_dst_ip;
    __u16 inner_src_port;
    __u16 inner_dst_port;
    __u8  inner_protocol;
    __u8  inner_ip_version;
    __u32 vni;
    __u64 outer_packet_size;
    __u64 inner_packet_size;
    __u64 timestamp;
};

/* BPF Maps */
struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(max_entries, 100000);
    __type(key, struct inner_flow_key);
    __type(value, struct decap_flow_stats);
} vxlan_flow_map SEC(".maps");

/* 解封装后的流量队列（发送到用户态） */
struct {
    __uint(type, BPF_MAP_TYPE_RINGBUF);
    __uint(max_entries, 1 << 24);  /* 16MB */
} inner_flow_events SEC(".maps");

/* Ethernet Header */
struct ethhdr {
    __u8  h_dest[6];
    __u8  h_source[6];
    __u16 h_proto;
};

/* IP Header */
struct iphdr {
    __u8  ihl:4, version:4;
    __u8  tos;
    __u16 tot_len;
    __u16 id;
    __u16 frag_off;
    __u8  ttl;
    __u8  protocol;
    __u16 check;
    __u32 saddr;
    __u32 daddr;
};

/* IPv6 Header */
struct ipv6hdr {
    __u32 flow_lbl:20, priority:8, version:4;
    __u16 payload_len;
    __u8  nexthdr;
    __u8  hop_limit;
    __u8  saddr[16];
    __u8  daddr[16];
};

/* UDP Header */
struct udphdr {
    __u16 source;
    __u16 dest;
    __u16 len;
    __u16 check;
};

/* TCP Header */
struct tcphdr {
    __u16 source;
    __u16 dest;
    __u32 seq;
    __u32 ack_seq;
    __u16 res1:4, doff:4, fin:1, syn:1, rst:1, psh:1, ack:1, urg:1, ece:1, cwr:1;
    __u16 window;
    __u16 check;
    __u16 urg_ptr;
};

/*
 * VXLAN 解封装主程序
 * 
 * 挂载方式：TC 或 Socket Filter
 * 输入：原始网络包（包含 VXLAN 封装）
 * 输出：内层五元组信息写入 Map
 */
SEC("tc")
int vxlan_decap(struct __sk_buff *skb)
{
    void *data = (void *)(long)skb->data;
    void *data_end = (void *)(long)skb->data_end;
    
    /* 解析外层 Ethernet 头 */
    struct ethhdr *eth = data;
    if ((void *)(eth + 1) > data_end)
        return BPF_OK;
    
    __u16 eth_proto = bpf_ntohs(eth->h_proto);
    if (eth_proto != ETH_P_IP)
        return BPF_OK;  /* 仅处理 IPv4 */
    
    /* 解析外层 IP 头 */
    struct iphdr *outer_ip = (struct iphdr *)(eth + 1);
    if ((void *)(outer_ip + 1) > data_end)
        return BPF_OK;
    
    if (outer_ip->protocol != IPPROTO_UDP)
        return BPF_OK;  /* VXLAN 使用 UDP */
    
    /* 解析外层 UDP 头 */
    struct udphdr *outer_udp = (struct udphdr *)((void *)outer_ip + (outer_ip->ihl * 4));
    if ((void *)(outer_udp + 1) > data_end)
        return BPF_OK;
    
    /* 检查是否为 VXLAN 端口 (4789) */
    if (bpf_ntohs(outer_udp->dest) != VXLAN_PORT &&
        bpf_ntohs(outer_udp->source) != VXLAN_PORT)
        return BPF_OK;
    
    /* 解析 VXLAN 头 */
    struct vxlanhdr *vxlan = (struct vxlanhdr *)(outer_udp + 1);
    if ((void *)(vxlan + 1) > data_end)
        return BPF_OK;
    
    /* 提取 VNI（低24位） */
    __u32 vni = bpf_ntohl(vxlan->vx_vni) >> 8;
    
    /* 解析内层 Ethernet 头 */
    struct ethhdr *inner_eth = (struct ethhdr *)(vxlan + 1);
    if ((void *)(inner_eth + 1) > data_end)
        return BPF_OK;
    
    __u16 inner_eth_proto = bpf_ntohs(inner_eth->h_proto);
    
    /* 构建内层五元组键 */
    struct inner_flow_key key = {};
    key.outer_src_ip = outer_ip->saddr;
    key.outer_dst_ip = outer_ip->daddr;
    key.outer_src_port = bpf_ntohs(outer_udp->source);
    key.outer_dst_port = bpf_ntohs(outer_udp->dest);
    key.vni = vni;
    
    /* 解析内层 IP 头 */
    if (inner_eth_proto == ETH_P_IP) {
        struct iphdr *inner_ip = (struct iphdr *)(inner_eth + 1);
        if ((void *)(inner_ip + 1) > data_end)
            return BPF_OK;
        
        key.inner_src_ip = inner_ip->saddr;
        key.inner_dst_ip = inner_ip->daddr;
        key.inner_protocol = inner_ip->protocol;
        key.inner_ip_version = 4;
        
        /* 解析内层端口 */
        void *inner_l4 = (void *)inner_ip + (inner_ip->ihl * 4);
        if (inner_ip->protocol == 6) {  /* TCP */
            struct tcphdr *inner_tcp = inner_l4;
            if ((void *)(inner_tcp + 1) > data_end)
                return BPF_OK;
            key.inner_src_port = bpf_ntohs(inner_tcp->source);
            key.inner_dst_port = bpf_ntohs(inner_tcp->dest);
        } else if (inner_ip->protocol == 17) {  /* UDP */
            struct udphdr *inner_udp = inner_l4;
            if ((void *)(inner_udp + 1) > data_end)
                return BPF_OK;
            key.inner_src_port = bpf_ntohs(inner_udp->source);
            key.inner_dst_port = bpf_ntohs(inner_udp->dest);
        }
    } else if (inner_eth_proto == ETH_P_IPV6) {
        /* IPv6 处理 */
        struct ipv6hdr *inner_ip6 = (struct ipv6hdr *)(inner_eth + 1);
        if ((void *)(inner_ip6 + 1) > data_end)
            return BPF_OK;
        
        key.inner_ip_version = 6;
        key.inner_protocol = inner_ip6->nexthdr;
        /* IPv6 地址暂不处理，标记为 0 */
    } else {
        return BPF_OK;  /* 非 IP 协议 */
    }
    
    /* 更新流量统计 */
    struct decap_flow_stats *stats = bpf_map_lookup_elem(&vxlan_flow_map, &key);
    if (stats) {
        stats->packets += 1;
        stats->bytes += skb->len;
        stats->inner_bytes += skb->len - ((void *)inner_eth - data);
        stats->ts_last = bpf_ktime_get_ns();
    } else {
        struct decap_flow_stats new_stats = {};
        new_stats.packets = 1;
        new_stats.bytes = skb->len;
        new_stats.inner_bytes = skb->len - ((void *)inner_eth - data);
        new_stats.ts_first = bpf_ktime_get_ns();
        new_stats.ts_last = new_stats.ts_first;
        bpf_map_update_elem(&vxlan_flow_map, &key, &new_stats, BPF_ANY);
    }
    
    /* 发送内层流量信息到用户态（用于 TAP 镜像） */
    struct inner_flow_info *event;
    event = bpf_ringbuf_reserve(&inner_flow_events, sizeof(*event), 0);
    if (event) {
        event->inner_src_ip = key.inner_src_ip;
        event->inner_dst_ip = key.inner_dst_ip;
        event->inner_src_port = key.inner_src_port;
        event->inner_dst_port = key.inner_dst_port;
        event->inner_protocol = key.inner_protocol;
        event->inner_ip_version = key.inner_ip_version;
        event->vni = vni;
        event->outer_packet_size = skb->len;
        event->inner_packet_size = skb->len - ((void *)inner_eth - data);
        event->timestamp = bpf_ktime_get_ns();
        bpf_ringbuf_submit(event, 0);
    }
    
    return BPF_OK;
}

/*
 * 轻量级 VXLAN 解析（仅提取 VNI 和内层 IP）
 * 用于高吞吐场景
 */
SEC("socket")
int vxlan_parse_light(struct __sk_buff *skb)
{
    void *data = (void *)(long)skb->data;
    void *data_end = (void *)(long)skb->data_end;
    
    /* 快速跳过非 VXLAN 流量 */
    struct ethhdr *eth = data;
    if ((void *)(eth + 1) > data_end)
        return BPF_OK;
    
    if (bpf_ntohs(eth->h_proto) != ETH_P_IP)
        return BPF_OK;
    
    struct iphdr *ip = (struct iphdr *)(eth + 1);
    if ((void *)(ip + 1) > data_end || ip->protocol != IPPROTO_UDP)
        return BPF_OK;
    
    struct udphdr *udp = (struct udphdr *)((void *)ip + (ip->ihl * 4));
    if ((void *)(udp + 1) > data_end)
        return BPF_OK;
    
    if (bpf_ntohs(udp->dest) != VXLAN_PORT)
        return BPF_OK;
    
    /* 标记为 VXLAN 流量，允许继续处理 */
    return BPF_OK;
}

char _license[] SEC("license") = "GPL";
