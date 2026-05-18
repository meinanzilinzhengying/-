// SPDX-License-Identifier: GPL-2.0
#include <linux/bpf.h>
#include <linux/if_ether.h>
#include <linux/ip.h>
#include <linux/tcp.h>
#include <linux/udp.h>

#include <bpf/bpf_endian.h>
#include <bpf/bpf_helpers.h>

// 网络流量数据结构
typedef struct {
    __be32 dst_ip;
    __be16 dst_port;
    __u8 protocol;
    __u64 bytes;
    __u64 packets;
    __u64 timestamp;
} __attribute__((packed)) network_data_t;

// flow key 结构体 - 显式 packed 确保与 Go 端 bpfKeySize=12 一致
typedef struct __attribute__((packed)) {
    __be32 src_ip;
    __be32 dst_ip;
    __be16 src_port;
    __be16 dst_port;
} flow_key_t;

// 哈希表，用于存储网络流量数据
struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(max_entries, 10240);
    __type(key, flow_key_t);
    __type(value, network_data_t);
} network_map SEC("maps");

// TC 程序，用于采集网络流量
SEC("tc")
int tc_prog(struct __sk_buff *skb) {
    void *data = (void *)(long)skb->data;
    void *data_end = (void *)(long)skb->data_end;
    struct ethhdr *eth = data;
    struct iphdr *ip;
    struct tcphdr *tcp;
    struct udphdr *udp;
    __be32 src_ip, dst_ip;
    __be16 src_port, dst_port;
    __u8 protocol;
    __u32 data_len;

    // 检查以太网头部
    if (data + sizeof(*eth) > data_end) {
        return BPF_OK;
    }

    // 检查是否为 IP 数据包
    if (eth->h_proto != bpf_htons(ETH_P_IP)) {
        return BPF_OK;
    }

    // 解析 IP 头部
    ip = data + sizeof(*eth);
    if ((void *)ip + sizeof(*ip) > data_end) {
        return BPF_OK;
    }

    // 检查 IP 分片
    // 只处理首片（fragment offset = 0），后续分片没有传输层头部
    if (ip->frag_off & htons(0x1FFF)) {
        return BPF_OK;
    }

    src_ip = ip->saddr;
    dst_ip = ip->daddr;
    protocol = ip->protocol;
    data_len = ntohs(ip->tot_len) - (ip->ihl * 4);

    // 解析传输层头部
    if (protocol == IPPROTO_TCP) {
        tcp = (void *)ip + (ip->ihl * 4);
        if ((void *)tcp + sizeof(*tcp) > data_end) {
            return BPF_OK;
        }
        src_port = tcp->source;
        dst_port = tcp->dest;
    } else if (protocol == IPPROTO_UDP) {
        udp = (void *)ip + (ip->ihl * 4);
        if ((void *)udp + sizeof(*udp) > data_end) {
            return BPF_OK;
        }
        src_port = udp->source;
        dst_port = udp->dest;
    } else if (protocol == IPPROTO_ICMP) {
        // ICMP 协议，使用固定端口 0
        src_port = 0;
        dst_port = 0;
    } else {
        // 只处理 TCP、UDP 和 ICMP
        return BPF_OK;
    }

    // 构造 key
    flow_key_t key = {
        .src_ip = src_ip,
        .dst_ip = dst_ip,
        .src_port = src_port,
        .dst_port = dst_port,
    };

    // 查找现有数据
    network_data_t *value = bpf_map_lookup_elem(&network_map, &key);
    if (value) {
        // 更新现有数据
        value->bytes += data_len;
        value->packets += 1;
        value->timestamp = bpf_ktime_get_ns() / 1000000;
    } else {
        // 创建新数据
        network_data_t new_value = {
            .dst_ip = dst_ip,
            .dst_port = dst_port,
            .protocol = protocol,
            .bytes = data_len,
            .packets = 1,
            .timestamp = bpf_ktime_get_ns() / 1000000,
        };
        bpf_map_update_elem(&network_map, &key, &new_value, BPF_NOEXIST);
    }

    return BPF_OK;
}

char _license[] SEC("license") = "GPL";