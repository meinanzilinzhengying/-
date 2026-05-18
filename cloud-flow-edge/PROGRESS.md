# cloud-flow 全套平台 — 进展报告

> 更新时间：2026-04-12 19:50 GMT+8

## 架构

```
探针 (Agent) ──gRPC──► 边缘节点 (Edge) ──gRPC──► 中心服务 (Center)
  │                       │                        │
  ├ cloud-flow-agent      ├ cloud-flow-edge        ├ cloud-flow-center
  ├ 采集 CPU/内存/网络     ├ 探针管理+数据缓冲       ├ 数据存储+清理
  └ 注册+心跳+批量上报     └ 批量转发+TLS+Metrics   └ 文件存储+过期清理
```

## 项目结构

```
workspace/
├── build_package.sh                   # 一键构建打包脚本
├── dist/                              # 部署包输出目录
│   ├── cloud-flow-center-1.0.0-linux-amd64.tar.gz  (4.1M)
│   ├── cloud-flow-edge-1.0.0-linux-amd64.tar.gz    (4.8M)
│   └── cloud-flow-agent-1.0.0-linux-amd64.tar.gz   (4.1M)
├── cloud-flow-server/                 # 中心服务
│   ├── cmd/main.go
│   ├── internal/config/
│   ├── internal/grpcserver/           # 实现 CenterServiceServer
│   ├── internal/storage/              # 文件存储 + 过期清理
│   ├── configs/config.yaml
│   └── proto/ → cloud-flow-edge/proto (replace)
├── cloud-flow-edge/                   # 边缘节点
│   ├── cmd/main.go
│   ├── internal/config/               # TLS 配置
│   ├── internal/forwarder/            # 缓冲 + 批量转发
│   ├── internal/grpcclient/           # TLS + 重连熔断
│   ├── internal/grpcserver/           # TLS 服务端
│   ├── internal/probemgr/             # 探针管理
│   ├── pkg/metrics/                   # Prometheus
│   └── proto/                         # 共享 proto（含 go.mod）
└── cloud-flow-agent/                  # 探针
    ├── cmd/main.go
    ├── internal/config/
    ├── internal/collector/            # /proc 文件系统采集（零依赖）
    ├── internal/grpcclient/
    └── configs/config.yaml
```

## 二进制防反编译措施

| 措施 | 说明 |
|------|------|
| `-ldflags="-s"` | 去除符号表（函数名、行号） |
| `-ldflags="-w"` | 去除 DWARF 调试信息 |
| `-trimpath` | 去除编译路径泄露 |
| `strip` | 进一步去除多余段 |
| 静态链接 | 无外部 .so 依赖 |
| Go 编译特性 | Go 二进制天然比 C 更难逆向 |

## 部署方式

每个部署包内含：`bin/二进制` + `configs/config.yaml` + `install.sh` + `README.md`

```bash
# 1. 上传到目标服务器
scp cloud-flow-edge-1.0.0-linux-amd64.tar.gz user@server:/tmp/

# 2. 解压
tar xzf cloud-flow-edge-1.0.0-linux-amd64.tar.gz

# 3. 一键安装（默认 /opt/cloud-flow）
sudo bash cloud-flow-edge-1.0.0-linux-amd64/install.sh

# 4. 编辑配置
vi /opt/cloud-flow/cloud-flow-edge/configs/config.yaml

# 5. 启动
sudo systemctl start cloud-flow-edge
sudo systemctl enable cloud-flow-edge
```

install.sh 自动完成：
- 创建 `/opt/cloud-flow/<组件>/` 目录结构
- 复制二进制 + 配置（已有配置不覆盖，新配置保存为 .default）
- 生成 systemd 服务文件
- `systemctl daemon-reload`

## 测试结果

| 组件 | 编译 | 测试 | 覆盖率 |
|------|------|------|--------|
| cloud-flow-center | ✅ 11MB | — | — |
| cloud-flow-edge | ✅ 13MB | 16/16 PASS | 86.5% |
| cloud-flow-agent | ✅ 11MB | — | — |

## 构建命令

```bash
# 一键构建打包（版本号可选，默认 1.0.0）
bash build_package.sh 1.0.0

# 输出 3 个部署包到 dist/
```
