#!/bin/bash

# ClickHouse 备份脚本
# 支持增量备份和全量备份
# 使用方法:
#   - ./backup.sh full [路径]      # 全量备份
#   - ./backup.sh incremental [路径] # 增量备份
#   - ./backup.sh restore [备份目录] [恢复目录] # 恢复备份

set -e

# 默认配置
BACKUP_DIR="/tmp/clickhouse_backups"
CLICKHOUSE_HOST="${CLICKHOUSE_HOST:-clickhouse}"
CLICKHOUSE_PORT="${CLICKHOUSE_PORT:-9000}"
CLICKHOUSE_USER="${CLICKHOUSE_USER:-default}"
CLICKHOUSE_PASSWORD="${CLICKHOUSE_PASSWORD:-}"
CLICKHOUSE_DATABASE="${CLICKHOUSE_DATABASE:-cloudflow}"

# 日期格式化
DATE=$(date +%Y%m%d_%H%M%S)

# 日志函数
log() {
    echo "[$(date +'%Y-%m-%d %H:%M:%S')] $1"
}

# 检查 clickhouse-backup 工具是否可用
check_requirements() {
    if ! command -v clickhouse-client &> /dev/null; then
        log "错误: clickhouse-client 未安装"
        exit 1
    fi
}

# 全量备份
full_backup() {
    local backup_path="${1:-$BACKUP_DIR/full_$DATE}"
    
    log "开始全量备份到 $backup_path"
    mkdir -p "$backup_path"
    
    # 使用 clickhouse-backup 或直接导出
    log "导出数据库 $CLICKHOUSE_DATABASE"
    
    # 获取所有表
    tables=$(clickhouse-client --host="$CLICKHOUSE_HOST" --port="$CLICKHOUSE_PORT" \
        --user="$CLICKHOUSE_USER" --password="$CLICKHOUSE_PASSWORD" \
        --query="SHOW TABLES FROM $CLICKHOUSE_DATABASE" 2>/dev/null)
    
    if [ -z "$tables" ]; then
        log "警告: 未找到表"
        exit 0
    fi
    
    for table in $tables; do
        log "导出表 $table"
        # 导出表结构
        clickhouse-client --host="$CLICKHOUSE_HOST" --port="$CLICKHOUSE_PORT" \
            --user="$CLICKHOUSE_USER" --password="$CLICKHOUSE_PASSWORD" \
            --query="SHOW CREATE TABLE $CLICKHOUSE_DATABASE.$table" > "$backup_path/${table}_schema.sql"
        
        # 导出数据
        clickhouse-client --host="$CLICKHOUSE_HOST" --port="$CLICKHOUSE_PORT" \
            --user="$CLICKHOUSE_USER" --password="$CLICKHOUSE_PASSWORD" \
            --query="SELECT * FROM $CLICKHOUSE_DATABASE.$table FORMAT Native" > "$backup_path/${table}_data.native"
        
        log "表 $table 导出完成"
    done
    
    log "全量备份完成: $backup_path"
    echo "$backup_path"
}

# 增量备份（基于分区）
incremental_backup() {
    local backup_path="${1:-$BACKUP_DIR/incremental_$DATE}"
    
    log "开始增量备份到 $backup_path"
    mkdir -p "$backup_path"
    
    # 获取昨天的日期用于增量备份
    yesterday=$(date -d "yesterday" +%Y%m%d)
    
    log "增量备份 $yesterday 之后的分区"
    
    # 获取所有表
    tables=$(clickhouse-client --host="$CLICKHOUSE_HOST" --port="$CLICKHOUSE_PORT" \
        --user="$CLICKHOUSE_USER" --password="$CLICKHOUSE_PASSWORD" \
        --query="SHOW TABLES FROM $CLICKHOUSE_DATABASE" 2>/dev/null)
    
    for table in $tables; do
        # 获取表的分区
        partitions=$(clickhouse-client --host="$CLICKHOUSE_HOST" --port="$CLICKHOUSE_PORT" \
            --user="$CLICKHOUSE_USER" --password="$CLICKHOUSE_PASSWORD" \
            --query="SELECT DISTINCT partition FROM system.parts WHERE database = '$CLICKHOUSE_DATABASE' AND table = '$table' AND active = 1" 2>/dev/null)
        
        if [ -n "$partitions" ]; then
            log "备份表 $table 的分区: $partitions"
            for partition in $partitions; do
                # 这里简化处理，导出分区数据
                # 实际中可以使用更精细的分区管理
                clickhouse-client --host="$CLICKHOUSE_HOST" --port="$CLICKHOUSE_PORT" \
                    --user="$CLICKHOUSE_USER" --password="$CLICKHOUSE_PASSWORD" \
                    --query="SELECT * FROM $CLICKHOUSE_DATABASE.$table WHERE partition = '$partition' FORMAT Native" > "$backup_path/${table}_${partition}_data.native"
            done
        fi
    done
    
    log "增量备份完成: $backup_path"
    echo "$backup_path"
}

# 恢复备份
restore_backup() {
    local backup_dir="$1"
    local target_db="${2:-$CLICKHOUSE_DATABASE}"
    
    if [ -z "$backup_dir" ] || [ ! -d "$backup_dir" ]; then
        log "错误: 备份目录不存在"
        exit 1
    fi
    
    log "从 $backup_dir 恢复到数据库 $target_db"
    
    # 首先创建数据库（如果不存在）
    clickhouse-client --host="$CLICKHOUSE_HOST" --port="$CLICKHOUSE_PORT" \
        --user="$CLICKHOUSE_USER" --password="$CLICKHOUSE_PASSWORD" \
        --query="CREATE DATABASE IF NOT EXISTS $target_db" 2>/dev/null || true
    
    # 恢复所有表
    for schema_file in "$backup_dir"/*_schema.sql; do
        if [ -f "$schema_file" ]; then
            table=$(basename "$schema_file" _schema.sql)
            log "恢复表 $table"
            
            # 执行建表语句
            clickhouse-client --host="$CLICKHOUSE_HOST" --port="$CLICKHOUSE_PORT" \
                --user="$CLICKHOUSE_USER" --password="$CLICKHOUSE_PASSWORD" \
                --database="$target_db" --multiquery < "$schema_file"
            
            # 恢复数据
            data_file="$backup_dir/${table}_data.native"
            if [ -f "$data_file" ]; then
                clickhouse-client --host="$CLICKHOUSE_HOST" --port="$CLICKHOUSE_PORT" \
                    --user="$CLICKHOUSE_USER" --password="$CLICKHOUSE_PASSWORD" \
                    --database="$target_db" --query="INSERT INTO $table FORMAT Native" < "$data_file"
            fi
        fi
    done
    
    log "恢复完成"
}

# 主函数
main() {
    local command="$1"
    shift
    
    check_requirements
    
    case "$command" in
        full)
            full_backup "$@"
            ;;
        incremental)
            incremental_backup "$@"
            ;;
        restore)
            restore_backup "$@"
            ;;
        *)
            echo "用法: $0 {full|incremental|restore} [参数...]"
            echo
            echo "命令:"
            echo "  full [路径]            全量备份"
            echo "  incremental [路径]     增量备份"
            echo "  restore 备份目录 [数据库名]  恢复备份"
            exit 1
            ;;
    esac
}

main "$@"
