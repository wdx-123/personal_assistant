package sensitivedata

import (
	"strings"
	"testing"
)

const (
	testAESKeyBase64  = "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY="
	testHashKeyBase64 = "ZmVkY2JhOTg3NjU0MzIxMGZlZGNiYTk4NzY1NDMyMTA="
)

func newTestCodec(t *testing.T) *Codec {
	t.Helper()

	codec, err := New(Options{
		AESKeyBase64:  testAESKeyBase64,
		HashKeyBase64: testHashKeyBase64,
	})
	if err != nil {
		t.Fatalf("new codec failed: %v", err)
	}
	return codec
}

func TestEncryptDecryptRoundTrip(t *testing.T) {
	codec := newTestCodec(t)

	ciphertext, err := codec.Encrypt("profile.real_name", "张三")
	if err != nil {
		t.Fatalf("encrypt failed: %v", err)
	}
	if !codec.IsEncrypted(ciphertext) {
		t.Fatalf("ciphertext prefix missing: %s", ciphertext)
	}

	plaintext, err := codec.Decrypt("profile.real_name", ciphertext)
	if err != nil {
		t.Fatalf("decrypt failed: %v", err)
	}
	if plaintext != "张三" {
		t.Fatalf("unexpected plaintext: %s", plaintext)
	}
}

func TestEncryptUsesRandomNonce(t *testing.T) {
	codec := newTestCodec(t)

	c1, err := codec.Encrypt("user.identifier", "20230001")
	if err != nil {
		t.Fatalf("encrypt #1 failed: %v", err)
	}
	c2, err := codec.Encrypt("user.identifier", "20230001")
	if err != nil {
		t.Fatalf("encrypt #2 failed: %v", err)
	}
	if c1 == c2 {
		t.Fatalf("ciphertext should be different for same plaintext")
	}
}

func TestDecryptRejectsPlaintext(t *testing.T) {
	codec := newTestCodec(t)

	_, err := codec.Decrypt("user.identifier", "20230001")
	if err == nil {
		t.Fatalf("decrypt plaintext should fail")
	}
}

func TestDecryptRejectsMismatchedScope(t *testing.T) {
	codec := newTestCodec(t)

	ciphertext, err := codec.Encrypt("user.real_name", "张三")
	if err != nil {
		t.Fatalf("encrypt failed: %v", err)
	}

	_, err = codec.Decrypt("user.nickname", ciphertext)
	if err == nil {
		t.Fatalf("decrypt with mismatched scope should fail")
	}
}

func TestDecryptRejectsInvalidBase64Payload(t *testing.T) {
	codec := newTestCodec(t)

	_, err := codec.Decrypt("user.real_name", codec.CipherPrefix()+"%%%")
	if err == nil {
		t.Fatalf("decrypt invalid payload should fail")
	}
}

func TestDecryptRejectsShortPayload(t *testing.T) {
	codec := newTestCodec(t)

	_, err := codec.Decrypt("user.real_name", codec.CipherPrefix()+"YQ==")
	if err == nil {
		t.Fatalf("decrypt short payload should fail")
	}
}

func TestHashIndex(t *testing.T) {
	codec := newTestCodec(t)

	h1, err := codec.HashIndex("student", "华中科技大学", "20230001")
	if err != nil {
		t.Fatalf("hash #1 failed: %v", err)
	}
	h2, err := codec.HashIndex("student", "华中科技大学", "20230001")
	if err != nil {
		t.Fatalf("hash #2 failed: %v", err)
	}
	if h1 != h2 {
		t.Fatalf("same input must produce same hash")
	}
	if len(h1) != 64 {
		t.Fatalf("unexpected hash length: %d", len(h1))
	}

	h3, err := codec.HashIndex("student", "中山大学", "20230001")
	if err != nil {
		t.Fatalf("hash #3 failed: %v", err)
	}
	if h1 == h3 {
		t.Fatalf("different parts should produce different hash")
	}

	h4, err := codec.HashIndex("student_archive", "华中科技大学", "20230001")
	if err != nil {
		t.Fatalf("hash #4 failed: %v", err)
	}
	if h1 == h4 {
		t.Fatalf("different namespace should produce different hash")
	}
}

func TestNewRejectsInvalidKeys(t *testing.T) {
	testCases := []struct {
		name string
		opts Options
	}{
		{
			name: "missing aes key",
			opts: Options{HashKeyBase64: testHashKeyBase64},
		},
		{
			name: "missing hash key",
			opts: Options{AESKeyBase64: testAESKeyBase64},
		},
		{
			name: "invalid aes base64",
			opts: Options{
				AESKeyBase64:  "%%%invalid%%%",
				HashKeyBase64: testHashKeyBase64,
			},
		},
		{
			name: "invalid hash base64",
			opts: Options{
				AESKeyBase64:  testAESKeyBase64,
				HashKeyBase64: "%%%invalid%%%",
			},
		},
		{
			name: "wrong aes length",
			opts: Options{
				AESKeyBase64:  "c2hvcnQ=",
				HashKeyBase64: testHashKeyBase64,
			},
		},
		{
			name: "wrong hash length",
			opts: Options{
				AESKeyBase64:  testAESKeyBase64,
				HashKeyBase64: "c2hvcnQ=",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := New(tc.opts)
			if err == nil {
				t.Fatalf("expected error")
			}
		})
	}
}

func TestIsEncryptedUsesConfiguredPrefix(t *testing.T) {
	codec, err := New(Options{
		AESKeyBase64:  testAESKeyBase64,
		HashKeyBase64: testHashKeyBase64,
		CipherPrefix:  "custom:v2:",
	})
	if err != nil {
		t.Fatalf("new codec failed: %v", err)
	}

	ciphertext, err := codec.Encrypt("user.real_name", "张三")
	if err != nil {
		t.Fatalf("encrypt failed: %v", err)
	}
	if !strings.HasPrefix(ciphertext, "custom:v2:") {
		t.Fatalf("unexpected prefix: %s", ciphertext)
	}
	if !codec.IsEncrypted(ciphertext) {
		t.Fatalf("ciphertext should be detected")
	}
	if codec.IsEncrypted("enc:v1:anything") {
		t.Fatalf("old prefix should not be treated as encrypted for this codec")
	}
}

func TestEncryptRejectsEmptyScope(t *testing.T) {
	codec := newTestCodec(t)

	_, err := codec.Encrypt("   ", "张三")
	if err == nil {
		t.Fatalf("empty scope should fail")
	}
}

func TestHashIndexRejectsEmptyNamespace(t *testing.T) {
	codec := newTestCodec(t)

	_, err := codec.HashIndex("   ", "part")
	if err == nil {
		t.Fatalf("empty namespace should fail")
	}
}
