// Package crypto 提供加密解密工具函数
package crypto

import (
	"crypto"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/hmac"
	"crypto/md5"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/sha512"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"math/big"
)

// ==================== 哈希函数 ====================

// MD5 计算MD5哈希
func MD5(data []byte) string {
	h := md5.New()
	h.Write(data)
	return hex.EncodeToString(h.Sum(nil))
}

// MD5String 计算字符串MD5
func MD5String(s string) string {
	return MD5([]byte(s))
}

// SHA256 计算SHA256哈希
func SHA256(data []byte) []byte {
	h := sha256.Sum256(data)
	return h[:]
}

// SHA256String 计算SHA256字符串
func SHA256String(data []byte) string {
	return hex.EncodeToString(SHA256(data))
}

// SHA256Base64 计算SHA256并返回Base64
func SHA256Base64(data []byte) string {
	return base64.StdEncoding.EncodeToString(SHA256(data))
}

// SHA512 计算SHA512哈希
func SHA512(data []byte) []byte {
	h := sha512.Sum512(data)
	return h[:]
}

// SHA512String 计算SHA512字符串
func SHA512String(data []byte) string {
	return hex.EncodeToString(SHA512(data))
}

// HMACSHA256 计算HMAC-SHA256
func HMACSHA256(key, data []byte) []byte {
	h := hmac.New(sha256.New, key)
	h.Write(data)
	return h.Sum(nil)
}

// HMACSHA256String 计算HMAC-SHA256字符串
func HMACSHA256String(key, data []byte) string {
	return hex.EncodeToString(HMACSHA256(key, data))
}

// HMACSHA256Base64 计算HMAC-SHA256并返回Base64
func HMACSHA256Base64(key, data []byte) string {
	return base64.StdEncoding.EncodeToString(HMACSHA256(key, data))
}

// ==================== AES加密 ====================

// AESKeySize AES密钥大小
const (
	AES128KeySize = 16
	AES192KeySize = 24
	AES256KeySize = 32
)

// GenerateAESKey 生成AES密钥
func GenerateAESKey(size int) ([]byte, error) {
	key := make([]byte, size)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return nil, err
	}
	return key, nil
}

// AESEncrypt AES加密 (CBC模式)
func AESEncrypt(plaintext, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	// PKCS7填充
	plaintext = pkcs7Padding(plaintext, block.BlockSize())

	ciphertext := make([]byte, aes.BlockSize+len(plaintext))
	iv := ciphertext[:aes.BlockSize]
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return nil, err
	}

	mode := cipher.NewCBCEncrypter(block, iv)
	mode.CryptBlocks(ciphertext[aes.BlockSize:], plaintext)

	return ciphertext, nil
}

// AESDecrypt AES解密 (CBC模式)
func AESDecrypt(ciphertext, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	if len(ciphertext) < aes.BlockSize {
		return nil, errors.New("ciphertext too short")
	}

	iv := ciphertext[:aes.BlockSize]
	ciphertext = ciphertext[aes.BlockSize:]

	if len(ciphertext)%block.BlockSize() != 0 {
		return nil, errors.New("ciphertext is not a multiple of the block size")
	}

	mode := cipher.NewCBCDecrypter(block, iv)
	mode.CryptBlocks(ciphertext, ciphertext)

	return pkcs7Unpadding(ciphertext)
}

// AESEncryptGCM AES加密 (GCM模式)
func AESEncryptGCM(plaintext, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	return ciphertext, nil
}

// AESDecryptGCM AES解密 (GCM模式)
func AESDecryptGCM(ciphertext, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, errors.New("ciphertext too short")
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	return gcm.Open(nil, nonce, ciphertext, nil)
}

// AESEncryptString AES加密字符串，返回Base64
func AESEncryptString(plaintext string, key []byte) (string, error) {
	ciphertext, err := AESEncryptGCM([]byte(plaintext), key)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// AESDecryptString AES解密Base64字符串
func AESDecryptString(ciphertext string, key []byte) (string, error) {
	data, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", err
	}
	plaintext, err := AESDecryptGCM(data, key)
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}

// pkcs7Padding PKCS7填充
func pkcs7Padding(data []byte, blockSize int) []byte {
	padding := blockSize - len(data)%blockSize
	padtext := make([]byte, padding)
	for i := range padtext {
		padtext[i] = byte(padding)
	}
	return append(data, padtext...)
}

// pkcs7Unpadding PKCS7去填充
func pkcs7Unpadding(data []byte) ([]byte, error) {
	length := len(data)
	if length == 0 {
		return nil, errors.New("invalid padding")
	}
	unpadding := int(data[length-1])
	if unpadding > length {
		return nil, errors.New("invalid padding")
	}
	return data[:(length - unpadding)], nil
}

// ==================== RSA加密 ====================

// RSAKeyPair RSA密钥对
type RSAKeyPair struct {
	PrivateKey *rsa.PrivateKey
	PublicKey  *rsa.PublicKey
}

// GenerateRSAKeyPair 生成RSA密钥对
func GenerateRSAKeyPair(bits int) (*RSAKeyPair, error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, bits)
	if err != nil {
		return nil, err
	}
	return &RSAKeyPair{
		PrivateKey: privateKey,
		PublicKey:  &privateKey.PublicKey,
	}, nil
}

// RSAEncrypt RSA加密
func RSAEncrypt(plaintext []byte, publicKey *rsa.PublicKey) ([]byte, error) {
	return rsa.EncryptOAEP(sha256.New(), rand.Reader, publicKey, plaintext, nil)
}

// RSADecrypt RSA解密
func RSADecrypt(ciphertext []byte, privateKey *rsa.PrivateKey) ([]byte, error) {
	return rsa.DecryptOAEP(sha256.New(), rand.Reader, privateKey, ciphertext, nil)
}

// RSASign RSA签名 (PSS)
func RSASign(data []byte, privateKey *rsa.PrivateKey) ([]byte, error) {
	hash := SHA256(data)
	return rsa.SignPSS(rand.Reader, privateKey, crypto.SHA256, hash, nil)
}

// RSAVerify RSA验签 (PSS)
func RSAVerify(data, signature []byte, publicKey *rsa.PublicKey) error {
	hash := SHA256(data)
	return rsa.VerifyPSS(publicKey, crypto.SHA256, hash, signature, nil)
}

// ExportRSAPublicKeyPEM 导出RSA公钥为PEM格式
func ExportRSAPublicKeyPEM(publicKey *rsa.PublicKey) ([]byte, error) {
	publicKeyBytes, err := x509.MarshalPKIXPublicKey(publicKey)
	if err != nil {
		return nil, err
	}
	publicKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PUBLIC KEY",
		Bytes: publicKeyBytes,
	})
	return publicKeyPEM, nil
}

// ExportRSAPrivateKeyPEM 导出RSA私钥为PEM格式
func ExportRSAPrivateKeyPEM(privateKey *rsa.PrivateKey) ([]byte, error) {
	privateKeyBytes := x509.MarshalPKCS1PrivateKey(privateKey)
	privateKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: privateKeyBytes,
	})
	return privateKeyPEM, nil
}

// ImportRSAPublicKeyPEM 从PEM导入RSA公钥
func ImportRSAPublicKeyPEM(pemData []byte) (*rsa.PublicKey, error) {
	block, _ := pem.Decode(pemData)
	if block == nil {
		return nil, errors.New("failed to parse PEM block")
	}

	publicKey, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, err
	}

	rsaPublicKey, ok := publicKey.(*rsa.PublicKey)
	if !ok {
		return nil, errors.New("not an RSA public key")
	}

	return rsaPublicKey, nil
}

// ImportRSAPrivateKeyPEM 从PEM导入RSA私钥
func ImportRSAPrivateKeyPEM(pemData []byte) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode(pemData)
	if block == nil {
		return nil, errors.New("failed to parse PEM block")
	}

	return x509.ParsePKCS1PrivateKey(block.Bytes)
}

// ==================== ECDSA ====================

// ECDSAKeyPair ECDSA密钥对
type ECDSAKeyPair struct {
	PrivateKey *ecdsa.PrivateKey
	PublicKey  *ecdsa.PublicKey
}

// GenerateECDSAKeyPair 生成ECDSA密钥对
func GenerateECDSAKeyPair(curve elliptic.Curve) (*ECDSAKeyPair, error) {
	privateKey, err := ecdsa.GenerateKey(curve, rand.Reader)
	if err != nil {
		return nil, err
	}
	return &ECDSAKeyPair{
		PrivateKey: privateKey,
		PublicKey:  &privateKey.PublicKey,
	}, nil
}

// ECDSASign ECDSA签名
func ECDSASign(data []byte, privateKey *ecdsa.PrivateKey) (r, s []byte, err error) {
	hash := SHA256(data)
	rBig, sBig, err := ecdsa.Sign(rand.Reader, privateKey, hash)
	if err != nil {
		return nil, nil, err
	}
	return rBig.Bytes(), sBig.Bytes(), nil
}

// ECDSAVerify ECDSA验签
func ECDSAVerify(data, r, s []byte, publicKey *ecdsa.PublicKey) bool {
	hash := SHA256(data)
	rBig := new(big.Int).SetBytes(r)
	sBig := new(big.Int).SetBytes(s)
	return ecdsa.Verify(publicKey, hash, rBig, sBig)
}

// ==================== Ed25519 ====================

// GenerateEd25519KeyPair 生成Ed25519密钥对
func GenerateEd25519KeyPair() (ed25519.PublicKey, ed25519.PrivateKey, error) {
	return ed25519.GenerateKey(rand.Reader)
}

// Ed25519Sign Ed25519签名
func Ed25519Sign(message []byte, privateKey ed25519.PrivateKey) []byte {
	return ed25519.Sign(privateKey, message)
}

// Ed25519Verify Ed25519验签
func Ed25519Verify(message, sig []byte, publicKey ed25519.PublicKey) bool {
	return ed25519.Verify(publicKey, message, sig)
}

// ==================== 随机数 ====================

// GenerateRandomBytes 生成随机字节
func GenerateRandomBytes(length int) ([]byte, error) {
	bytes := make([]byte, length)
	if _, err := io.ReadFull(rand.Reader, bytes); err != nil {
		return nil, err
	}
	return bytes, nil
}

// GenerateRandomString 生成随机字符串
func GenerateRandomString(length int) (string, error) {
	bytes, err := GenerateRandomBytes(length)
	if err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(bytes)[:length], nil
}

// GenerateNonce 生成随机nonce
func GenerateNonce() (string, error) {
	bytes, err := GenerateRandomBytes(16)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// ==================== Base64 ====================

// Base64Encode Base64编码
func Base64Encode(data []byte) string {
	return base64.StdEncoding.EncodeToString(data)
}

// Base64Decode Base64解码
func Base64Decode(s string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(s)
}

// Base64URLEncode URL安全的Base64编码
func Base64URLEncode(data []byte) string {
	return base64.URLEncoding.EncodeToString(data)
}

// Base64URLDecode URL安全的Base64解码
func Base64URLDecode(s string) ([]byte, error) {
	return base64.URLEncoding.DecodeString(s)
}

// ==================== 便捷函数 ====================

// GenerateSecureToken 生成安全令牌
func GenerateSecureToken(length int) (string, error) {
	bytes, err := GenerateRandomBytes(length)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// HashPassword 密码哈希 (简单实现，生产环境建议使用 bcrypt)
func HashPassword(password string, salt []byte) string {
	return SHA256String(append([]byte(password), salt...))
}

// VerifyPassword 验证密码
func VerifyPassword(password string, salt []byte, hash string) bool {
	return HashPassword(password, salt) == hash
}

// EncryptWithPassword 使用密码加密
func EncryptWithPassword(plaintext, password string) (string, error) {
	key := SHA256([]byte(password))[:AES256KeySize]
	return AESEncryptString(plaintext, key)
}

// DecryptWithPassword 使用密码解密
func DecryptWithPassword(ciphertext, password string) (string, error) {
	key := SHA256([]byte(password))[:AES256KeySize]
	return AESDecryptString(ciphertext, key)
}

// MustGenerateAESKey 生成AES密钥，失败时panic
func MustGenerateAESKey(size int) []byte {
	key, err := GenerateAESKey(size)
	if err != nil {
		panic(fmt.Sprintf("failed to generate AES key: %v", err))
	}
	return key
}

// MustGenerateRandomString 生成随机字符串，失败时panic
func MustGenerateRandomString(length int) string {
	s, err := GenerateRandomString(length)
	if err != nil {
		panic(fmt.Sprintf("failed to generate random string: %v", err))
	}
	return s
}
