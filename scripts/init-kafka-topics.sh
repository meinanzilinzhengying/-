#!/bin/bash
# P1: Kafka Topic 初始化脚本
# 自动创建 CloudFlow 所需的所有 Topic 和对应的 DLQ topic
# 用法: ./init-kafka-topics.sh [kafka_broker:port]

set -e

KAFKA_BROKER=${1:-"localhost:9092"}
REPLICATION_FACTOR=${2:-1}
PARTITIONS=${3:-12}

echo "================================================"
echo "CloudFlow Kafka Topic 初始化脚本"
echo "================================================"
echo "Broker: $KAFKA_BROKER"
echo "Partitions: $PARTITIONS"
echo "Replication Factor: $REPLICATION_FACTOR"
echo ""

# 检查 kafka-topics 命令
if ! command -v kafka-topics &> /dev/null; then
    echo "警告: kafka-topics 命令未找到，尝试使用 Docker..."
    KAFKA_CMD="docker exec -it kafka kafka-topics --bootstrap-server $KAFKA_BROKER"
else
    KAFKA_CMD="kafka-topics --bootstrap-server $KAFKA_BROKER"
fi

# 定义 Topic 列表和配置
# 格式: topic_name:retention_ms:compression_type
declare -A TOPICS=(
    ["flow.raw"]="604800000:snappy"      # 7天
    ["flow.l4"]="604800000:snappy"       # 7天
    ["flow.l7"]="604800000:snappy"       # 7天
    ["metrics"]="259200000:snappy"       # 3天
    ["traces"]="604800000:snappy"        # 7天
    ["logs"]="259200000:snappy"          # 3天
    ["profiling"]="604800000:snappy"     # 7天
    ["topology"]="259200000:snappy"      # 3天
    ["alerts"]="604800000:snappy"        # 7天
)

# DLQ Topic 配置（死信队列）
DLQ_RETENTION="1209600000"  # 14天

echo "创建主 Topic..."
echo "------------------------------------------------"

for topic in "${!TOPICS[@]}"; do
    config="${TOPICS[$topic]}"
    retention=$(echo $config | cut -d: -f1)
    compression=$(echo $config | cut -d: -f2)
    
    echo -n "创建 topic: $topic ... "
    
    if $KAFKA_CMD --list --topic "$topic" 2>/dev/null | grep -q "^${topic}$"; then
        echo "已存在，跳过"
        continue
    fi
    
    $KAFKA_CMD --create \
        --topic "$topic" \
        --partitions $PARTITIONS \
        --replication-factor $REPLICATION_FACTOR \
        --config retention.ms=$retention \
        --config compression.type=$compression \
        --config min.insync.replicas=1 \
        --config cleanup.policy=delete 2>/dev/null
    
    if [ $? -eq 0 ]; then
        echo "成功"
    else
        echo "失败"
    fi
done

echo ""
echo "创建 DLQ (Dead Letter Queue) Topic..."
echo "------------------------------------------------"

for topic in "${!TOPICS[@]}"; do
    dlq_topic="${topic}.dlq"
    echo -n "创建 DLQ topic: $dlq_topic ... "
    
    if $KAFKA_CMD --list --topic "$dlq_topic" 2>/dev/null | grep -q "^${dlq_topic}$"; then
        echo "已存在，跳过"
        continue
    fi
    
    $KAFKA_CMD --create \
        --topic "$dlq_topic" \
        --partitions $PARTITIONS \
        --replication-factor $REPLICATION_FACTOR \
        --config retention.ms=$DLQ_RETENTION \
        --config compression.type=snappy \
        --config min.insync.replicas=1 \
        --config cleanup.policy=delete 2>/dev/null
    
    if [ $? -eq 0 ]; then
        echo "成功"
    else
        echo "失败"
    fi
done

echo ""
echo "验证所有 Topic..."
echo "------------------------------------------------"
$KAFKA_CMD --list 2>/dev/null | grep -E "^flow\.|^metrics|^traces|^logs|^profiling|^topology|^alerts" | sort

echo ""
echo "================================================"
echo "Kafka Topic 初始化完成"
echo "================================================"
