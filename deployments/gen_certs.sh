#!/bin/bash

# 生成 CloudFlow 微服务 TLS 证书脚本
#
# 生成 CA 证书和各服务证书，用于双向 TLS (mTLS) 通信
#
# 用法:
#   ./gen_certs.sh        # 使用默认配置生成
#   ./gen_certs.sh /path/to/certs  # 指定输出目录

set -e

# 输出目录
OUTPUT_DIR=${1:-"deployments/certs"}
mkdir -p "${OUTPUT_DIR}"
cd "${OUTPUT_DIR}" || exit 1

# 服务列表
SERVICES=("auth-service" "data-plane" "api-gateway" "clickhouse-service")

# 有效期（天）
VALID_DAYS=3650

# 清理旧证书
echo "清理旧证书..."
rm -f *.pem *.srl *.key *.crt

echo "========================================="
echo "  CloudFlow 微服务 TLS 证书生成"
echo "========================================="

# 1. 生成 CA 私钥和证书
echo ""
echo "1. 生成 CA 根证书..."
openssl genrsa -out ca.key 4096 2>/dev/null
openssl req -new -x509 -days ${VALID_DAYS} -key ca.key -out ca.crt -subj "/C=CN/ST=Beijing/L=Beijing/O=CloudFlow/OU=CA/CN=cloudflow-ca" 2>/dev/null
echo "   CA 证书生成完成: ca.crt"

# 2. 为每个服务生成证书
echo ""
echo "2. 生成服务证书..."
for service in "${SERVICES[@]}"; do
    echo ""
    echo "   生成 ${service} 证书..."
    
    # 生成服务私钥
    openssl genrsa -out "${service}.key" 2048 2>/dev/null
    
    # 生成 CSR
    openssl req -new -key "${service}.key" -out "${service}.csr" -subj "/C=CN/ST=Beijing/L=Beijing/O=CloudFlow/OU=Services/CN=${service}" 2>/dev/null
    
    # 生成扩展配置
    cat > "${service}.ext" <<EOF
authorityKeyIdentifier=keyid,issuer
basicConstraints=CA:FALSE
keyUsage = digitalSignature, nonRepudiation, keyEncipherment, dataEncipherment
extendedKeyUsage = serverAuth, clientAuth
subjectAltName = DNS:${service}, DNS:${service}.default.svc.cluster.local, DNS:localhost, IP:127.0.0.1
EOF
    
    # 用 CA 签署证书
    openssl x509 -req -in "${service}.csr" -CA ca.crt -CAkey ca.key -CAcreateserial -out "${service}.crt" -days ${VALID_DAYS} -extfile "${service}.ext" 2>/dev/null
    
    # 清理临时文件
    rm -f "${service}.csr" "${service}.ext"
    
    echo "   ${service} 证书生成完成: ${service}.crt, ${service}.key"
done

# 3. 生成客户端证书（用于测试）
echo ""
echo "3. 生成客户端测试证书..."
openssl genrsa -out client.key 2048 2>/dev/null
openssl req -new -key client.key -out client.csr -subj "/C=CN/ST=Beijing/L=Beijing/O=CloudFlow/OU=Client/CN=client" 2>/dev/null
cat > client.ext <<EOF
authorityKeyIdentifier=keyid,issuer
basicConstraints=CA:FALSE
keyUsage = digitalSignature, nonRepudiation, keyEncipherment
extendedKeyUsage = clientAuth
EOF
openssl x509 -req -in client.csr -CA ca.crt -CAkey ca.key -CAcreateserial -out client.crt -days ${VALID_DAYS} -extfile client.ext 2>/dev/null
rm -f client.csr client.ext
echo "   客户端证书生成完成"

echo ""
echo "========================================="
echo "  证书生成完成！"
echo "  输出目录: ${OUTPUT_DIR}"
echo "========================================="
echo ""
echo "使用方式："
echo "  - ca.crt: CA 根证书（用于验证）"
echo "  - <service>.crt: 服务端/客户端证书"
echo "  - <service>.key: 服务端/客户端私钥"
echo ""
echo "Docker Compose 挂载示例："
echo "  volumes:"
echo "    - ./certs:/certs"
echo ""
