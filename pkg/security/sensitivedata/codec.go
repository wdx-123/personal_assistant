package sensitivedata

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"strings"
)

const defaultCipherPrefix = "enc:v1:"

// Options 定义敏感数据编解码器的初始化参数。
type Options struct {
	AESKeyBase64  string
	HashKeyBase64 string
	CipherPrefix  string
}

// Codec 提供通用敏感数据加解密与索引哈希能力。
type Codec struct {
	gcm          cipher.AEAD
	hashKey      []byte
	cipherPrefix string
}

// New 创建敏感数据编解码器。
// AESKeyBase64 与 HashKeyBase64 必须是 base64 编码后的 32 字节密钥。
func New(opts Options) (*Codec, error) {
	aesKey, err := decodeBase64Key("aes_key_base64", opts.AESKeyBase64)
	if err != nil {
		return nil, err
	}
	hashKey, err := decodeBase64Key("hash_key_base64", opts.HashKeyBase64)
	if err != nil {
		return nil, err
	}

	block, err := aes.NewCipher(aesKey)
	if err != nil {
		return nil, fmt.Errorf("init aes cipher failed: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("init gcm failed: %w", err)
	}

	return &Codec{
		gcm:          gcm,
		hashKey:      hashKey,
		cipherPrefix: resolveCipherPrefix(opts.CipherPrefix),
	}, nil
}

// CipherPrefix 返回当前编解码器使用的密文前缀。
func (c *Codec) CipherPrefix() string {
	if c == nil {
		return resolveCipherPrefix("")
	}
	return c.cipherPrefix
}

// IsEncrypted 判断值是否为当前编解码器支持的密文格式。
func (c *Codec) IsEncrypted(value string) bool {
	return strings.HasPrefix(value, c.CipherPrefix())
}

// Encrypt 使用 AES-256-GCM 对明文进行加密。
// scope 会参与 AAD 构建，避免不同业务作用域之间误解密。
func (c *Codec) Encrypt(scope, plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}
	if c == nil {
		return "", fmt.Errorf("codec is nil")
	}

	aad, err := buildScopeAAD(scope)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, c.gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("generate nonce failed: %w", err)
	}

	ciphertext := c.gcm.Seal(nil, nonce, []byte(plaintext), aad)
	raw := append(nonce, ciphertext...)

	return c.CipherPrefix() + base64.StdEncoding.EncodeToString(raw), nil
}

// Decrypt 使用 AES-256-GCM 对密文进行解密。
func (c *Codec) Decrypt(scope, value string) (string, error) {
	if value == "" {
		return "", nil
	}
	if c == nil {
		return "", fmt.Errorf("codec is nil")
	}
	if !c.IsEncrypted(value) {
		return "", fmt.Errorf("invalid ciphertext format")
	}

	aad, err := buildScopeAAD(scope)
	if err != nil {
		return "", err
	}

	encoded := strings.TrimPrefix(value, c.CipherPrefix())
	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("decode ciphertext failed: %w", err)
	}

	nonceSize := c.gcm.NonceSize()
	if len(raw) <= nonceSize {
		return "", fmt.Errorf("invalid ciphertext payload")
	}

	nonce := raw[:nonceSize]
	ciphertext := raw[nonceSize:]
	plain, err := c.gcm.Open(nil, nonce, ciphertext, aad)
	if err != nil {
		return "", fmt.Errorf("gcm decrypt failed: %w", err)
	}

	return string(plain), nil
}

// HashIndex 使用 HMAC-SHA256 为命名空间和索引片段生成稳定索引值。
func (c *Codec) HashIndex(namespace string, parts ...string) (string, error) {
	if c == nil {
		return "", fmt.Errorf("codec is nil")
	}

	namespace = strings.TrimSpace(namespace)
	if namespace == "" {
		return "", fmt.Errorf("namespace is required")
	}

	mac := hmac.New(sha256.New, c.hashKey)
	writeHashSegment(mac, namespace)
	for _, part := range parts {
		writeHashSegment(mac, part)
	}

	return hex.EncodeToString(mac.Sum(nil)), nil
}

func resolveCipherPrefix(prefix string) string {
	prefix = strings.TrimSpace(prefix)
	if prefix == "" {
		return defaultCipherPrefix
	}
	return prefix
}

func decodeBase64Key(name, value string) ([]byte, error) {
	raw := strings.TrimSpace(value)
	if raw == "" {
		return nil, fmt.Errorf("%s is required", name)
	}

	key, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		key, err = base64.RawStdEncoding.DecodeString(raw)
		if err != nil {
			return nil, fmt.Errorf("%s must be valid base64: %w", name, err)
		}
	}

	if len(key) != 32 {
		return nil, fmt.Errorf("%s must decode to 32 bytes, got %d", name, len(key))
	}
	return key, nil
}

func buildScopeAAD(scope string) ([]byte, error) {
	scope = strings.TrimSpace(scope)
	if scope == "" {
		return nil, fmt.Errorf("scope is required")
	}
	return []byte("scope:" + scope + ":v1"), nil
}

func writeHashSegment(mac interface{ Write([]byte) (int, error) }, value string) {
	var lengthBuf [4]byte
	binary.BigEndian.PutUint32(lengthBuf[:], uint32(len(value)))
	_, _ = mac.Write(lengthBuf[:])
	_, _ = mac.Write([]byte(value))
}
