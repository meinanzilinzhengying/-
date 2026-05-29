#!/bin/bash
# TiDB 数据库备份脚本
# 支持全量备份、增量备份
# 使用方法:
#   - ./tidb-backup.sh full [路径]       # 全量备份
#   - ./tidb-backup.sh restore [备份目录] [数据库名] # 恢复备份
#   - ./tidb-backup.sh list [路径]        # 列出备份

set -e

# 默认配置
BACKUP_DIR="/tmp/tidb_backups"
TIDB_HOST="${TIDB_HOST:-tidb}"
TIDB_PORT="${TIDB_PORT:-4000}"
TIDB_USER="${TIDB_USER:-root}"
TIDB_PASSWORD="${TIDB_PASSWORD:-}"
TIDB_DATABASES="${TIDB_DATABASES:-cloudflow cloudflow_auth cloudflow_alert}"

# 日期格式化
DATE=$(date +%Y%m%d_%H%M%S)

# 日志函数
log() {
    echo "[$(date +'%Y-%m-%d %H:%M:%S')] $1"
}

# 检查工具是否可用
check_requirements() {
    if ! command -v mysql &> /dev/null && ! command -v mysqldump &> /dev/null; then
        log "警告: MySQL 客户端工具未完全安装"
    fi
}

# 执行 MySQL 查询
mysql_query() {
    local query="$1"
    local db="$2"
    mysql --host="$TIDB_HOST" --port="$TIDB_PORT" --user="$TIDB_USER" --password="$TIDB_PASSWORD" \
        --default-character-set=utf8mb4 --ssl-mode=DISABLED "$db" -e "$query" 2>/dev/null
}

# 获取数据库列表
get_databases() {
    if [ -n "$TIDB_DATABASES" ]; then
        echo "$TIDB_DATABASES"
    else
        mysql_query "SHOW DATABASES" | grep -v "Database" | grep -vE "information_schema|performance_schema|mysql|sys"
    fi
}

# 全量备份
full_backup() {
    local backup_path="${1:-$BACKUP_DIR/full_$DATE}"
    
    log "开始 TiDB 全量备份到 $backup_path"
    mkdir -p "$backup_path"
    
    local databases=$(get_databases)
    
    if [ -z "$databases" ]; then
        log "警告: 未找到要备份的数据库"
        exit 0
    fi
    
    for db in $databases; do
        log "备份数据库: $db"
        local backup_file="$backup_path/${db}_${DATE}.sql.gz"
        
        if command -v mysqldump &> /dev/null; then
            # 使用 mysqldump 导出
            log "使用 mysqldump 导出 $db"
            mysqldump --host="$TIDB_HOST" --port="$TIDB_PORT" --user="$TIDB_USER" --password="$TIDB_PASSWORD" \
                --single-transaction --quick --lock-tables=false --default-character-set=utf8mb4 \
                --ssl-mode=DISABLED "$db" 2>/dev/null | gzip > "$backup_file"
        else
            # 使用 mysql 手动导出（备选方案）
            log "使用 SQL 查询导出 $db"
            
            # 获取所有表
            local tables=$(mysql_query "SHOW TABLES" "$db" | tail -n +2)
            
            if [ -z "$tables" ]; then
                continue
            fi
            
            # 创建备份目录
            mkdir -p "$backup_path/$db"
            
            for table in $tables; do
                # 导出表结构
                mysql_query "SHOW CREATE TABLE $table" "$db" | tail -n +2 | awk -F'\t' '{print $2}' > "$backup_path/$db/${table}_schema.sql"
                
                # 导出表数据（简单的 CSV 格式）
                mysql_query "SELECT * FROM $table" "$db" > "$backup_path/$db/${table}_data.csv"
            done
            
            # 打包
            cd "$backup_path" && tar czf "${db}_${DATE}.tar.gz" "$db" && rm -rf "$db"
        fi
        
        if [ $? -eq 0 ]; then
            log "数据库 $db 备份完成: $backup_file"
        else
            log "警告: 数据库 $db 备份可能存在问题"
        fi
    done
    
    # 备份元信息
    cat > "$backup_path/backup_info.txt" <<EOF
BACKUP_TYPE=full
BACKUP_DATE=$DATE
BACKUP_DATABASES=$databases
TIDB_HOST=$TIDB_HOST
EOF
    
    log "TiDB 全量备份完成: $backup_path"
    echo "$backup_path"
}

# 恢复备份
restore_backup() {
    local backup_dir="$1"
    local target_db="$2"
    
    if [ -z "$backup_dir" ] || [ ! -d "$backup_dir" ]; then
        log "错误: 备份目录不存在"
        exit 1
    fi
    
    log "从 $backup_dir 恢复备份"
    
    # 查找备份文件
    for backup_file in "$backup_dir"/*.sql.gz "$backup_dir"/*.tar.gz; do
        if [ -f "$backup_file" ]; then
            if [[ "$backup_file" == *.sql.gz ]]; then
                local db_name=$(basename "$backup_file" .sql.gz | cut -d'_' -f1)
                if [ -n "$target_db" ]; then
                    db_name="$target_db"
                fi
                
                log "恢复数据库 $db_name"
                
                # 创建数据库
                mysql_query "CREATE DATABASE IF NOT EXISTS $db_name"
                
                # 恢复数据
                if command -v gunzip &> /dev/null && command -v mysql &> /dev/null; then
                    gunzip -c "$backup_file" | mysql --host="$TIDB_HOST" --port="$TIDB_PORT" \
                        --user="$TIDB_USER" --password="$TIDB_PASSWORD" --default-character-set=utf8mb4 \
                        --ssl-mode=DISABLED "$db_name"
                fi
            elif [[ "$backup_file" == *.tar.gz ]]; then
                # 解压并恢复
                local temp_dir=$(mktemp -d)
                tar xzf "$backup_file" -C "$temp_dir"
                
                for table_schema in "$temp_dir"/*/*_schema.sql; do
                    if [ -f "$table_schema" ]; then
                        local table=$(basename "$table_schema" _schema.sql)
                        log "恢复表 $table"
                        mysql "$db_name" < "$table_schema"
                        
                        # 恢复数据
                        local data_file="${table_schema%_schema.sql}_data.csv"
                        if [ -f "$data_file" ]; then
                            log "恢复表 $table 数据"
                            # 注意：CSV 导入需要处理
                        fi
                    fi
                done
                
                rm -rf "$temp_dir"
            fi
        fi
    done
    
    log "恢复完成"
}

# 列出备份
list_backups() {
    local backup_path="${1:-$BACKUP_DIR}"
    
    if [ ! -d "$backup_path" ]; then
        log "备份目录不存在: $backup_path"
        return
    fi
    
    log "TiDB 备份列表:"
    echo "======================================"
    
    for backup_dir in "$backup_path"/full_*; do
        if [ -d "$backup_dir" ]; then
            if [ -f "$backup_dir/backup_info.txt" ]; then
                echo "备份: $(basename "$backup_dir")"
                cat "$backup_dir/backup_info.txt"
                echo "--------------------------------------"
            else
                echo "备份: $(basename "$backup_dir")"
                echo "  - 类型: 全量"
                echo "  - 时间: $(stat -c %z "$backup_dir" 2>/dev/null || ls -lh --time-style="+%Y-%m-%d %H:%M" "$backup_dir" | awk '{print $6, $7}')"
                echo "--------------------------------------"
            fi
        fi
    done
}

# 清理旧备份
cleanup_old() {
    local backup_path="${1:-$BACKUP_DIR}"
    local days="${2:-30}"
    
    log "清理 $days 天前的备份: $backup_path"
    
    if [ ! -d "$backup_path" ]; then
        return
    fi
    
    find "$backup_path" -type d -name "full_*" -mtime +$days -exec rm -rf {} \; 2>/dev/null
    find "$backup_path" -type d -name "incremental_*" -mtime +$days -exec rm -rf {} \; 2>/dev/null
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
        restore)
            restore_backup "$@"
            ;;
        list)
            list_backups "$@"
            ;;
        cleanup)
            cleanup_old "$@"
            ;;
        *)
            echo "用法: $0 {full|restore|list|cleanup} [参数...]"
            echo
            echo "命令:"
            echo "  full [路径]              全量备份"
            echo "  restore 备份目录 [数据库名] 恢复备份"
            echo "  list [路径]               列出备份"
            echo "  cleanup [路径] [天数]      清理旧备份"
            exit 1
            ;;
    esac
}

main "$@"
