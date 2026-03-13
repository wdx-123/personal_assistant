package core

import (
	"fmt"
	"strings"

	"personal_assistant/global"
	sensitivedata "personal_assistant/pkg/security/sensitivedata"

	"go.uber.org/zap"
)

// InitSensitiveDataCodec 初始化敏感数据编解码器并注册到全局变量。
func InitSensitiveDataCodec() error {
	if global.Config == nil {
		return fmt.Errorf("global config is nil")
	}

	cfg := global.Config.Security.SensitiveData
	if !cfg.Enabled {
		global.SensitiveDataCodec = nil
		if global.Log != nil {
			global.Log.Info("敏感数据编解码器未启用",
				zap.Bool("enabled", false),
				zap.String("cipher_prefix", resolveSensitiveDataCipherPrefix(cfg.CipherPrefix)))
		}
		return nil
	}

	// 创建敏感数据编解码器实例
	codec, err := sensitivedata.New(sensitivedata.Options{
		AESKeyBase64:  cfg.AESKeyBase64,
		HashKeyBase64: cfg.HashKeyBase64,
		CipherPrefix:  cfg.CipherPrefix,
	})
	if err != nil {
		if global.Log != nil {
			global.Log.Error("敏感数据编解码器初始化失败",
				zap.Bool("enabled", true),
				zap.String("cipher_prefix", resolveSensitiveDataCipherPrefix(cfg.CipherPrefix)),
				zap.Error(err))
		}
		return err
	}

	global.SensitiveDataCodec = codec
	if global.Log != nil {
		global.Log.Info("敏感数据编解码器初始化完成",
			zap.Bool("enabled", true),
			zap.String("cipher_prefix", codec.CipherPrefix()))
	}
	return nil
}

// resolveSensitiveDataCipherPrefix 解析并返回敏感数据密文前缀，默认为 "enc:v1:"。
func resolveSensitiveDataCipherPrefix(prefix string) string {
	prefix = strings.TrimSpace(prefix)
	if prefix == "" {
		return "enc:v1:"
	}
	return prefix
}
