#!/bin/bash
# TLS 证书生成脚本 - 用于开发和测试
# 生产环境应使用正式的 PKI 和证书管理

set -e

CERT_DIR="${1:-./certs}"
mkdir -p "$CERT_DIR"

echo "生成 CloudFlow TLS 证书..."

# 生成 CA 私钥
openssl genrsa -out "$CERT_DIR/ca.key" 4096

# 生成 CA 证书
openssl req -x509 -new -nodes -key "$CERT_DIR/ca.key" \
  -sha256 -days 3650 \
  -out "$CERT_DIR/ca.crt" \
  -subj "/CN=CloudFlow CA/O=CloudFlow"

# 生成服务器私钥
openssl genrsa -out "$CERT_DIR/server.key" 4096

# 生成服务器证书请求
openssl req -new -key "$CERT_DIR/server.key" \
  -out "$CERT_DIR/server.csr" \
  -subj "/CN=localhost/O=CloudFlow"

# 生成服务器证书（使用 SAN 支持 localhost）
cat > "$CERT_DIR/server_ext.cnf" << EOF
[v3_req]
keyUsage = keyEncipherment, dataEncipherment
extendedKeyUsage = serverAuth
subjectAltName = @alt_names
[alt_names]
DNS.1 = localhost
DNS.2 = control-plane
DNS.3 = data-plane
DNS.4 = auth-service
DNS.5 = tenant-service
DNS.6 = query-service
IP.1 = 127.0.0.1
EOF

openssl x509 -req -in "$CERT_DIR/server.csr" \
  -CA "$CERT_DIR/ca.crt" -CAkey "$CERT_DIR/ca.key" \
  -CAcreateserial \
  -out "$CERT_DIR/server.crt" \
  -days 365 -sha256 \
  -extfile "$CERT_DIR/server_ext.cnf" -extensions v3_req

# 生成客户端私钥
openssl genrsa -out "$CERT_DIR/client.key" 4096

# 生成客户端证书请求
openssl req -new -key "$CERT_DIR/client.key" \
  -out "$CERT_DIR/client.csr" \
  -subj "/CN=client/O=CloudFlow"

# 生成客户端证书
openssl x509 -req -in "$CERT_DIR/client.csr" \
  -CA "$CERT_DIR/ca.crt" -CAkey "$CERT_DIR/ca.key" \
  -CAcreateserial \
  -out "$CERT_DIR/client.crt" \
  -days 365 -sha256

# 清理临时文件
rm -f "$CERT_DIR/server.csr" "$CERT_DIR/client.csr" "$CERT_DIR/server_ext.cnf"

# 设置权限
chmod 600 "$CERT_DIR"/*.key
chmod 644 "$CERT_DIR"/*.crt

echo ""
echo "证书已生成在 $CERT_DIR 目录:"
ls -la "$CERT_DIR"
echo ""
echo "证书文件:"
echo "  - ca.crt:     CA 证书（用于验证其他证书）"
echo "  - server.crt: 服务器证书"
echo "  - server.key: 服务器私钥"
echo "  - client.crt: 客户端证书（用于 mTLS）"
echo "  - client.key: 客户端私钥"
echo ""
echo "使用说明:"
echo "1. 启用 TLS: 设置 TLS_ENABLED=true"
echo "2. mTLS: 设置 TLS_CLIENT_AUTH=true"
echo "3. 开发跳过验证: 设置 TLS_INSECURE_SKIP=true"
