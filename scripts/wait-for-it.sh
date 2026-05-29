#!/bin/bash
# wait-for-it.sh - 等待服务就绪的脚本
# 使用: wait-for-it.sh host:port [-t timeout] [-- command args]
#
# 示例:
#   ./wait-for-it.sh mysql:3306 -- echo "MySQL is ready!"
#   ./wait-for-it.sh -t 60 redis:6379 -- ./start-app.sh

set -e

TIMEOUT=30
QUIET=0
HOST=""
PORT=""

usage() {
    echo "Usage: $0 host:port [-t timeout] [-- command args]"
    echo ""
    echo "Example:"
    echo "  $0 mysql:3306 -t 60 -- echo 'MySQL is ready'"
    echo "  $0 -q -t 30 redis:6379 -- ./start-app.sh"
    exit 1
}

# 解析参数
while [[ $# -gt 0 ]]; do
    case "$1" in
        *:* )
        HOSTPORT=$1
        shift
        ;;
        -t )
        TIMEOUT=$2
        shift 2
        ;;
        -q )
        QUIET=1
        shift
        ;;
        -- )
        shift
        break
        ;;
        --help )
        usage
        ;;
        *)
        break
        ;;
    esac
done

if [ -z "$HOSTPORT" ]; then
    usage
fi

# 分离主机和端口
HOST=${HOSTPORT%%:*}
PORT=${HOSTPORT##*:}

# 验证端口是数字
if ! [[ $PORT =~ ^[0-9]+$ ]]; then
    echo "Error: '$PORT' is not a valid port number" >&2
    exit 1
fi

# 超时时间转换为秒
TIMEOUT_END=$((SECONDS + TIMEOUT))

if [ "$QUIET" -eq 0 ]; then
    echo "Waiting for $HOST:$PORT (timeout: ${TIMEOUT}s)..."
fi

# 等待函数
wait_for_service() {
    local host=$1
    local port=$2
    local timeout_end=$3
    
    while true; do
        if [ $SECONDS -gt $timeout_end ]; then
            echo "Timeout waiting for $host:$port" >&2
            return 1
        fi
        
        # 尝试连接
        if command -v nc > /dev/null 2>&1; then
            if nc -z "$host" "$port"; then
                return 0
            fi
        elif command -v curl > /dev/null 2>&1; then
            if curl -s -o /dev/null -w "%{http_code}" "http://$host:$port" > /dev/null 2>&1 || 
               curl -s -o /dev/null -w "%{http_code}" "https://$host:$port" > /dev/null 2>&1; then
                return 0
            fi
        else
            # 纯 bash 网络检查 (如果可用)
            if exec 3<>/dev/tcp/"$host"/"$port"; then
                exec 3>&-
                exec 3<&-
                return 0
            fi
        fi
        
        sleep 1
    done
}

if wait_for_service "$HOST" "$PORT" "$TIMEOUT_END"; then
    if [ "$QUIET" -eq 0 ]; then
        echo "$HOST:$PORT is available!"
    fi
else
    exit 1
fi

# 如果有命令要执行，执行它
if [ $# -gt 0 ]; then
    exec "$@"
fi
