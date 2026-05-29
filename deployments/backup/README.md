# CloudFlow 备份与恢复手册

## 概述

本文档介绍 CloudFlow 系统的数据备份与恢复方案，包括 TiDB（结构化数据）和 ClickHouse（时序数据）的完整备份策略。

## 目录

1. [备份架构](#备份架构)
2. [快速开始](#快速开始)
3. [TiDB 备份](#tidb-备份)
4. [ClickHouse 备份](#clickhouse-备份)
5. [定时备份策略](#定时备份策略)
6. [数据恢复](#数据恢复)
7. [故障排查](#故障排查)

---

## 备份架构

### 备份服务部署

```
cloudflow/
├── deployments/
│   ├── backup/
│   │   └── backup.sh          # 统一备份管理脚本
│   ├── tidb/
│   │   └── backup.sh         # TiDB 备份脚本
│   ├── clickhouse/
│   │   └── backup.sh         # ClickHouse 备份脚本
│   └── docker-compose.yml    # Docker 部署配置
└── data/
    └── cloudflow_backups/     # 备份存储目录
        ├── tidb/
        └── clickhouse/
        └── logs/
```

### 备份服务容器

```
┌─────────────────┐
│ Backup Service  │
└────────┬────────┘
         │
    ┌────┴────┐
    │         │
    ▼         ▼
┌──────┐  ┌──────────┐
│ TiDB │  │ClickHouse│
└──────┘  └──────────┘
```

---

## 快速开始

### 1. 创建备份目录

```bash
# 创建备份目录结构
mkdir -p /data/cloudflow_backups/{tidb,clickhouse,logs}
```

### 2. 执行首次备份

```bash
# 进入备份目录
cd /workspace/deployments/backup

# 所有服务完整备份
chmod +x backup.sh
./backup.sh all

# 或者单独备份
./backup.sh tidb full
./backup.sh clickhouse full
```

### 3. 查看备份

```bash
./backup.sh list
```

---

## TiDB 备份

### TiDB 备份命令

```bash
cd /workspace/deployments/tidb
chmod +x backup.sh

# 全量备份
./backup.sh full

# 列出备份
./backup.sh list

# 清理旧备份
./backup.sh cleanup
```

### 环境变量配置

| 变量 | 默认值 | 说明 |
|------|--------|------|
| TIDB_HOST | tidb | TiDB 主机 |
| TIDB_PORT | 4000 | TiDB 端口 |
| TIDB_USER | root | TiDB 用户 |
| TIDB_PASSWORD | - | TiDB 密码 |
| TIDB_DATABASES | cloudflow cloudflow_auth cloudflow_alert | 要备份的数据库列表 |

### 备份内容

- cloudflow (业务数据)
- cloudflow_auth (认证数据)
- cloudflow_alert (告警数据)

### 备份文件

```
/tmp/tidb_backups/
└── full_20240529_123456/
    ├── cloudflow_20240529_123456.sql.gz
    ├── cloudflow_auth_20240529_123456.sql.gz
    ├── cloudflow_alert_20240529_123456.sql.gz
    └── backup_info.txt
```

---

## ClickHouse 备份

### ClickHouse 备份命令

```bash
cd /workspace/deployments/clickhouse
chmod +x backup.sh

# 全量备份
./backup.sh full

# 增量备份
./backup.sh incremental
```

### 环境变量配置

| 变量 | 默认值 | 说明 |
|------|--------|------|
| CLICKHOUSE_HOST | clickhouse | ClickHouse 主机 |
| CLICKHOUSE_PORT | 9000 | ClickHouse 端口 |
| CLICKHOUSE_USER | default | ClickHouse 用户 |
| CLICKHOUSE_PASSWORD | - | ClickHouse 密码 |
| CLICKHOUSE_DATABASE | cloudflow | ClickHouse 数据库名 |

---

## 定时备份策略

### 1. 配置定时任务

```bash
# 编辑 crontab
crontab -e
```

### 2. 添加备份任务

```bash
# CloudFlow 备份任务
# TiDB 每日 2:00 全量备份
0 2 * * * /workspace/deployments/backup/backup.sh tidb >> /data/cloudflow_backups/logs/tidb_backup.log 2>&1

# ClickHouse 每日 3:00 全量备份
0 3 * * * /workspace/deployments/backup/backup.sh clickhouse >> /data/cloudflow_backups/logs/clickhouse_backup.log 2>&1

# 所有服务每周日 4:00 完整备份
0 4 * * 0 /workspace/deployments/backup/backup.sh all >> /data/cloudflow_backups/logs/backup.log 2>&1

# 清理 30 天前的旧备份 每周一 5:00
0 5 * * 1 /workspace/deployments/backup/backup.sh cleanup >> /data/cloudflow_backups/logs/cleanup.log 2>&1
```

### 3. 查看当前任务

```bash
crontab -l
```

### 4. 查看定时任务建议

使用统一脚本查看定时任务配置：

```bash
./backup.sh schedule
```

---

## 数据恢复

### TiDB 恢复步骤

#### 1. 查找可用备份

```bash
cd /workspace/deployments/tidb
./backup.sh list
```

#### 2. 执行恢复

```bash
# 恢复指定备份
./backup.sh restore /path/to/backup/directory

# 或者恢复到指定数据库
./backup.sh restore /path/to/backup/directory cloudflow
```

### ClickHouse 恢复

#### 1. 查找可用备份

```bash
cd /workspace/deployments/clickhouse
ls -lh /tmp/clickhouse_backups
```

#### 2. 执行恢复

```bash
./backup.sh restore /path/to/backup/directory
```

### 使用 Docker 容器内恢复

如果在 Docker 容器内执行恢复：

```bash
# 进入 TiDB 容器
docker-compose exec tidb bash

# 或者使用工具
docker-compose exec tidb sh -c "mysql -uroot -p..."
```

### 完整的恢复命令总结

| 场景 | 命令 |
|------|------|
| TiDB 完整恢复 | `./backup.sh restore <备份目录>` |
| ClickHouse 完整恢复 | `./clickhouse/backup.sh restore <备份目录>` |
| 使用统一脚本恢复 | `./backup.sh restore tidb <目录` |

---

## 备份保留策略

| 备份类型 | 保留天数 | 说明 |
|----------|----------|------|
| 全量备份 | 30 天 | 完整数据库备份 |
| 增量备份 | 7 天 | 分区级别的增量 |

---

## 故障排查

### 1. 备份失败排查

```bash
# 查看备份日志
tail -f /data/cloudflow_backups/logs/backup_*.log

# 检查服务状态
docker-compose ps tidb clickhouse
```

### 2. 连接失败

```bash
# 检查连接测试
mysql -h 127.0.0.1 -P 4000 -u root
clickhouse-client --host 127.0.0.1 --port 9000
```

### 3. 空间不足

```bash
# 检查磁盘空间
df -h

# 清理旧备份
./backup.sh cleanup
```

---

## 快速参考

```bash
# 备份所有数据
./backup.sh all

# 查看备份状态
./backup.sh list

# 设置定时任务
crontab -e

# 查看定时任务
crontab -l
```
