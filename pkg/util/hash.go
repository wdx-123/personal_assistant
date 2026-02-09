package util

import (
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"io"

	"golang.org/x/crypto/bcrypt"
)

// ==================== 文件哈希（图片去重等场景统一使用） ====================

// FileHashAlgo 文件内容哈希算法标识，升级算法时只需改此处
const FileHashAlgo = "sha256"

// FileHashBytes 计算字节切片的文件哈希（适用于内存中已有完整数据的场景）
func FileHashBytes(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

// FileHashReader 从 io.Reader 流式计算文件哈希（适用于大文件，避免一次性读入内存）
func FileHashReader(r io.Reader) (string, error) {
	h := sha256.New()
	if _, err := io.Copy(h, r); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// ==================== 密码哈希 ====================

// BcryptHash 使用 bcrypt 对密码进行加密
func BcryptHash(password string) string {
	bytes, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes)
}

// BcryptCheck 对比明文密码和数据库的哈希值
func BcryptCheck(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

// ==================== 通用 MD5（兼容旧逻辑） ====================

func MD5V(str []byte, b ...byte) string {
	h := md5.New()
	h.Write(str)
	return hex.EncodeToString(h.Sum(b))
}
