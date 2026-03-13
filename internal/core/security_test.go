package core

import (
	"testing"

	"personal_assistant/global"
	"personal_assistant/internal/model/config"

	"go.uber.org/zap"
)

const (
	testSensitiveAESKeyBase64  = "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY="
	testSensitiveHashKeyBase64 = "ZmVkY2JhOTg3NjU0MzIxMGZlZGNiYTk4NzY1NDMyMTA="
)

func TestInitSensitiveDataCodecDisabled(t *testing.T) {
	originalConfig := global.Config
	originalCodec := global.SensitiveDataCodec
	originalLog := global.Log
	t.Cleanup(func() {
		global.Config = originalConfig
		global.SensitiveDataCodec = originalCodec
		global.Log = originalLog
	})

	global.Log = zap.NewNop()
	global.Config = &config.Config{
		Security: config.Security{
			SensitiveData: config.SensitiveData{
				Enabled: false,
			},
		},
	}
	global.SensitiveDataCodec = nil

	if err := InitSensitiveDataCodec(); err != nil {
		t.Fatalf("init codec failed: %v", err)
	}
	if global.SensitiveDataCodec != nil {
		t.Fatalf("codec should stay nil when disabled")
	}
}

func TestInitSensitiveDataCodecEnabled(t *testing.T) {
	originalConfig := global.Config
	originalCodec := global.SensitiveDataCodec
	originalLog := global.Log
	t.Cleanup(func() {
		global.Config = originalConfig
		global.SensitiveDataCodec = originalCodec
		global.Log = originalLog
	})

	global.Log = zap.NewNop()
	global.Config = &config.Config{
		Security: config.Security{
			SensitiveData: config.SensitiveData{
				Enabled:       true,
				CipherPrefix:  "custom:v2:",
				AESKeyBase64:  testSensitiveAESKeyBase64,
				HashKeyBase64: testSensitiveHashKeyBase64,
			},
		},
	}
	global.SensitiveDataCodec = nil

	if err := InitSensitiveDataCodec(); err != nil {
		t.Fatalf("init codec failed: %v", err)
	}
	if global.SensitiveDataCodec == nil {
		t.Fatalf("codec should be initialized")
	}
	if global.SensitiveDataCodec.CipherPrefix() != "custom:v2:" {
		t.Fatalf("unexpected cipher prefix: %s", global.SensitiveDataCodec.CipherPrefix())
	}
}

func TestInitSensitiveDataCodecInvalidKey(t *testing.T) {
	originalConfig := global.Config
	originalCodec := global.SensitiveDataCodec
	originalLog := global.Log
	t.Cleanup(func() {
		global.Config = originalConfig
		global.SensitiveDataCodec = originalCodec
		global.Log = originalLog
	})

	global.Log = zap.NewNop()
	global.Config = &config.Config{
		Security: config.Security{
			SensitiveData: config.SensitiveData{
				Enabled:       true,
				AESKeyBase64:  "invalid",
				HashKeyBase64: testSensitiveHashKeyBase64,
			},
		},
	}
	global.SensitiveDataCodec = nil

	if err := InitSensitiveDataCodec(); err == nil {
		t.Fatalf("expected init failure")
	}
	if global.SensitiveDataCodec != nil {
		t.Fatalf("codec should stay nil on failure")
	}
}
