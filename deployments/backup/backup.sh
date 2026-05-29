#!/bin/bash
# CloudFlow 统一备份管理脚本
# 支持 TiDB 和 ClickHouse 的全量/增量备份、定时备份、恢复
# 使用方法:
#   - ./backup.sh all                    # 备份所有数据库
#   - ./backup.sh tidb [full|list]      # TiDB 备份
#   - ./backup.sh clickhouse [full|list] # ClickHouse 备份
#   - ./backup.sh schedule               # 显示定时任务配置
#   - ./backup.sh help                   # 显示帮助

set -eo pipefail

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 配置
BACKUP_ROOT="/data/cloudflow_backups"
TIDB_BACKUP_DIR="$BACKUP_ROOT/tidb"
CLICKHOUSE_BACKUP_DIR="$BACKUP_ROOT/clickhouse"
LOG_DIR="$BACKUP_ROOT/logs"

# 服务配置
TIDB_HOST="${TIDB_HOST:-tidb}"
TIDB_PORT="${TIDB_PORT:-4000}"
TIDB_USER="${TIDB_USER:-root}"
TIDB_PASSWORD="${TIDB_PASSWORD:-}"

CLICKHOUSE_HOST="${CLICKHOUSE_HOST:-clickhouse}"
CLICKHOUSE_PORT="${CLICKHOUSE_PORT:-9000}"
CLICKHOUSE_USER="${CLICKHOUSE_USER:-default}"
CLICKHOUSE_PASSWORD="${CLICKHOUSE_PASSWORD:-}"

# 备份保留策略
FULL_BACKUP_RETENTION_DAYS="${FULL_BACKUP_RETENTION_DAYS:-30}"
INCREMENTAL_BACKUP_RETENTION_DAYS="${INCREMENTAL_BACKUP_RETENTION_DAYS:-7}"

# 脚本目录
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TIDB_BACKUP_SCRIPT="$SCRIPT_DIR/../tidb/backup.sh"
CLICKHOUSE_BACKUP_SCRIPT="$SCRIPT_DIR/../clickhouse/backup.sh"

# 日期格式化
DATE=$(date +%Y%m%d_%H%M%S)
DATE_ONLY=$(date +%Y%m%d)

# 日志函数
log() {
    local level="$1"
    local message="$2"
    local timestamp=$(date +'%Y-%m-%d %H:%M:%S')
    
    case "$level" in
        INFO)
            echo -e "${BLUE}[INFO]${NC} [$timestamp] $message"
            ;;
        WARN)
            echo -e "${YELLOW}[WARN]${NC} [$timestamp] $message"
            ;;
        ERROR)
            echo -e "${RED}[ERROR]${NC} [$timestamp] $message"
            ;;
        SUCCESS)
            echo -e "${GREEN}[SUCCESS]${NC} [$timestamp] $message"
            ;;
        *)
            echo "[$timestamp] $message"
            ;;
    esac
    
    # 写入日志文件
    echo "[$level] [$timestamp] $message" >> "$LOG_DIR/backup_$(date +%Y%m%d).log" 2>/dev/null
}

# 初始化目录
init_dirs() {
    mkdir -p "$BACKUP_ROOT" "$TIDB_BACKUP_DIR" "$CLICKHOUSE_BACKUP_DIR" "$LOG_DIR"
}

# 检查脚本
check_scripts() {
    if [ ! -f "$TIDB_BACKUP_SCRIPT" ]; then
        log "WARN" "TiDB 备份脚本不存在: $TIDB_BACKUP_SCRIPT"
    fi
    if [ ! -f "$CLICKHOUSE_BACKUP_SCRIPT" ]; then
        log "WARN" "ClickHouse 备份脚本不存在: $CLICKHOUSE_BACKUP_SCRIPT"
    fi
}

# TiDB 全量备份
backup_tidb_full() {
    log "INFO" "开始 TiDB 全量备份"
    
    if [ ! -f "$TIDB_BACKUP_SCRIPT" ]; then
        log "ERROR" "TiDB 备份脚本不存在"
        return 1
    fi
    
    chmod +x "$TIDB_BACKUP_SCRIPT"
    export TIDB_HOST TIDB_PORT TIDB_USER TIDB_PASSWORD
    export BACKUP_DIR="$TIDB_BACKUP_DIR"
    
    local backup_result=$("$TIDB_BACKUP_SCRIPT" full "$TIDB_BACKUP_DIR/full_$DATE")
    
    if [ $? -eq 0 ]; then
        log "SUCCESS" "TiDB 全量备份完成: $backup_result"
    else
        log "ERROR" "TiDB 全量备份失败"
        return 1
    fi
}

# ClickHouse 全量备份
backup_clickhouse_full() {
    log "INFO" "开始 ClickHouse 全量备份"
    
    if [ ! -f "$CLICKHOUSE_BACKUP_SCRIPT" ]; then
        log "ERROR" "ClickHouse 备份脚本不存在"
        return 1
    fi
    
    chmod +x "$CLICKHOUSE_BACKUP_SCRIPT"
    export CLICKHOUSE_HOST CLICKHOUSE_PORT CLICKHOUSE_USER CLICKHOUSE_PASSWORD
    export BACKUP_DIR="$CLICKHOUSE_BACKUP_DIR"
    
    local backup_result=$("$CLICKHOUSE_BACKUP_SCRIPT" full "$CLICKHOUSE_BACKUP_DIR/full_$DATE")
    
    if [ $? -eq 0 ]; then
        log "SUCCESS" "ClickHouse 全量备份完成: $backup_result"
    else
        log "ERROR" "ClickHouse 全量备份失败"
        return 1
    fi
}

# 备份所有服务
backup_all() {
    log "INFO" "开始所有服务的完整备份"
    
    local failures=0
    
    # TiDB 备份
    if ! backup_tidb_full; then
        failures=$((failures + 1))
    fi
    
    # ClickHouse 备份
    if ! backup_clickhouse_full; then
        failures=$((failures + 1))
    fi
    
    # 清理旧备份
    cleanup_old_backups
    
    if [ $failures -eq 0 ]; then
        log "SUCCESS" "所有服务备份完成"
    else
        log "WARN" "备份完成，存在 $failures 个失败"
        return 1
    fi
}

# 列出备份
list_backups() {
    echo "======================================"
    echo "CloudFlow 备份列表"
    echo "======================================"
    echo
    
    echo "TiDB 备份:"
    if [ -d "$TIDB_BACKUP_DIR" ]; then
        ls -lh "$TIDB_BACKUP_DIR" 2>/dev/null || echo "  暂无备份"
    else
        echo "  暂无备份"
    fi
    
    echo
    echo "ClickHouse 备份:"
    if [ -d "$CLICKHOUSE_BACKUP_DIR" ]; then
        ls -lh "$CLICKHOUSE_BACKUP_DIR" 2>/dev/null || echo "  暂无备份"
    else
        echo "  暂无备份"
    fi
}

# 清理旧备份
cleanup_old_backups() {
    log "INFO" "清理 $FULL_BACKUP_RETENTION_DAYS 天前的旧备份"
    
    # 清理 TiDB
    if [ -d "$TIDB_BACKUP_DIR" ]; then
        find "$TIDB_BACKUP_DIR" -type d -name "full_*" -mtime +$FULL_BACKUP_RETENTION_DAYS -exec rm -rf {} \; 2>/dev/null
        find "$TIDB_BACKUP_DIR" -type d -name "incremental_*" -mtime +$INCREMENTAL_BACKUP_RETENTION_DAYS -exec rm -rf {} \; 2>/dev/null
    fi
    
    # 清理 ClickHouse
    if [ -d "$CLICKHOUSE_BACKUP_DIR" ]; then
        find "$CLICKHOUSE_BACKUP_DIR" -type d -name "full_*" -mtime +$FULL_BACKUP_RETENTION_DAYS -exec rm -rf {} \; 2>/dev/null
        find "$CLICKHOUSE_BACKUP_DIR" -type d -name "incremental_*" -mtime +$INCREMENTAL_BACKUP_RETENTION_DAYS -exec rm -rf {} \; 2>/dev/null
    fi
    
    # 清理日志
    if [ -d "$LOG_DIR" ]; then
        find "$LOG_DIR" -name "backup_*.log" -mtime +14 -delete 2>/dev/null
    fi
    
    log "SUCCESS" "旧备份清理完成"
}

# 显示定时任务配置
show_schedule() {
    echo "======================================"
    echo "CloudFlow 定时备份建议配置"
    echo "======================================"
    echo
    echo "编辑定时任务:"
    echo "  crontab -e"
    echo
    echo "添加以下内容:"
    echo
    echo "# TiDB 每日 2:00 全量备份"
    echo "0 2 * * * $SCRIPT_DIR/backup.sh tidb >> $LOG_DIR/tidb_backup.log 2>&1"
    echo
    echo "# ClickHouse 每日 3:00 全量备份"
    echo "0 3 * * * $SCRIPT_DIR/backup.sh clickhouse >> $LOG_DIR/clickhouse_backup.log 2>&1"
    echo
    echo "# 所有服务每周日 4:00 完整备份"
    echo "0 4 * * 0 $SCRIPT_DIR/backup.sh all >> $LOG_DIR/backup.log 2>&1"
    echo
    echo "# 清理 30 天前的旧备份 每周一 5:00"
    echo "0 5 * * 1 $SCRIPT_DIR/backup.sh cleanup >> $LOG_DIR/cleanup.log 2>&1"
    echo
    echo "查看当前定时任务:"
    echo "  crontab -l"
    echo
    echo "======================================"
}

# 生成恢复脚本
generate_restore_scripts() {
    log "INFO" "生成恢复脚本"
    
    # TiDB 恢复快速脚本
    cat > "$BACKUP_ROOT/restore-tidb.sh" <<EOF
#!/bin/bash
# TiDB 快速恢复脚本
echo "TiDB 恢复使用方法:"
echo "1. 查看可用备份: $TIDB_BACKUP_SCRIPT list"
echo "2. 恢复指定备份: $TIDB_BACKUP_SCRIPT restore <备份目录>"
echo ""
echo "或使用统一备份管理脚本:"
echo "  $0 restore tidb <备份目录>"
EOF
    chmod +x "$BACKUP_ROOT/restore-tidb.sh"
    
    # ClickHouse 恢复快速脚本
    cat > "$BACKUP_ROOT/restore-clickhouse.sh" <<EOF
#!/bin/bash
# ClickHouse 快速恢复脚本
echo "ClickHouse 恢复使用方法:"
echo "1. 查看可用备份: $CLICKHOUSE_BACKUP_SCRIPT list"
echo "2. 恢复指定备份: $CLICKHOUSE_BACKUP_SCRIPT restore <备份目录>"
echo ""
echo "或使用统一备份管理脚本:"
echo "  $0 restore clickhouse <备份目录>"
EOF
    chmod +x "$BACKUP_ROOT/restore-clickhouse.sh"
    
    log "SUCCESS" "恢复脚本已生成: $BACKUP_ROOT/"
}

# 显示帮助
show_help() {
    echo "======================================"
    echo "CloudFlow 统一备份管理脚本"
    echo "======================================"
    echo
    echo "用法: $0 <命令> [参数...]"
    echo
    echo "命令:"
    echo "  all                    备份所有服务"
    echo "  tidb [full|list|restore] TiDB 备份操作"
    echo "  clickhouse [full|list|restore] ClickHouse 备份操作"
    echo "  list                   列出所有备份"
    echo "  cleanup                清理旧备份"
    echo "  schedule               显示定时任务配置"
    echo "  help                   显示此帮助信息"
    echo
    echo "示例:"
    echo "  $0 all                 # 完整备份所有服务"
    echo "  $0 tidb full           # TiDB 全量备份"
    echo "  $0 clickhouse full     # ClickHouse 全量备份"
    echo "  $0 list                # 查看备份"
    echo
    echo "环境变量:"
    echo "  BACKUP_ROOT            备份根目录 (默认: $BACKUP_ROOT)"
    echo "  TIDB_HOST              TiDB 主机 (默认: $TIDB_HOST)"
    echo "  CLICKHOUSE_HOST        ClickHouse 主机 (默认: $CLICKHOUSE_HOST)"
    echo "  FULL_BACKUP_RETENTION_DAYS  全量备份保留天数 (默认: $FULL_BACKUP_RETENTION_DAYS)"
    echo
}

# 主函数
main() {
    local command="$1"
    shift
    
    init_dirs
    check_scripts
    
    case "$command" in
        all)
            backup_all "$@"
            ;;
        tidb)
            local subcmd="$1"
            shift
            case "$subcmd" in
                full)
                    backup_tidb_full "$@"
                    ;;
                list)
                    if [ -f "$TIDB_BACKUP_SCRIPT" ]; then
                        "$TIDB_BACKUP_SCRIPT" list "$TIDB_BACKUP_DIR"
                    fi
                    ;;
                restore)
                    if [ -f "$TIDB_BACKUP_SCRIPT" ]; then
                        export BACKUP_DIR="$TIDB_BACKUP_DIR"
                        "$TIDB_BACKUP_SCRIPT" restore "$@"
                    fi
                    ;;
                *)
                    backup_tidb_full "$@"
                    ;;
            esac
            ;;
        clickhouse)
            local subcmd="$1"
            shift
            case "$subcmd" in
                full)
                    backup_clickhouse_full "$@"
                    ;;
                list)
                    if [ -f "$CLICKHOUSE_BACKUP_SCRIPT" ]; then
                        echo "ClickHouse 备份列表:"
                        ls -lh "$CLICKHOUSE_BACKUP_DIR" 2>/dev/null
                    fi
                    ;;
                restore)
                    if [ -f "$CLICKHOUSE_BACKUP_SCRIPT" ]; then
                        export BACKUP_DIR="$CLICKHOUSE_BACKUP_DIR"
                        "$CLICKHOUSE_BACKUP_SCRIPT" restore "$@"
                    fi
                    ;;
                *)
                    backup_clickhouse_full "$@"
                    ;;
            esac
            ;;
        list)
            list_backups "$@"
            ;;
        cleanup)
            cleanup_old_backups "$@"
            ;;
        schedule)
            show_schedule "$@"
            ;;
        help|--help|-h)
            show_help
            ;;
        *)
            log "ERROR" "未知命令: $command"
            show_help
            exit 1
            ;;
    esac
}

main "$@"
