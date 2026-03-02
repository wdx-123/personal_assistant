package qiniu

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"github.com/qiniu/go-sdk/v7/auth"
	qstorage "github.com/qiniu/go-sdk/v7/storage"

	"personal_assistant/global"
	"personal_assistant/pkg/storage"
)

// Driver 七牛云存储驱动，并发安全
type Driver struct{}

// New 创建并返回一个七牛云存储驱动实例
func New() *Driver { return &Driver{} }

// Name 返回驱动名称
func (d *Driver) Name() string { return "qiniu" }

// Delete 删除七牛云对象，返回 612（资源不存在）视为成功（幂等）
func (d *Driver) Delete(_ context.Context, key string) error {
	cfg := sdkConfig()
	mac, bucket, err := credentials()
	if err != nil {
		return err
	}
	bm := qstorage.NewBucketManager(mac, cfg)
	if err := bm.Delete(bucket, key); err != nil {
		// 七牛错误码 612 表示资源不存在，视为成功
		if strings.Contains(err.Error(), "612") {
			return nil
		}
		return err
	}
	return nil
}

// Upload 使用七牛 Resumable V2 接口进行分片上传，支持 io.Reader 流式上传。
// 上传前将数据缓冲到内存，以支持失败后重试时 Seek 回起点。
// 返回 StorageObject，URL 使用配置的自定义域名。
func (d *Driver) Upload(
	ctx context.Context,
	r io.Reader,
	filename string,
) (storage.StorageObject, error) {
	// 1. 配置与凭证
	cfg := sdkConfig()
	mac, bucket, err := credentials()
	if err != nil {
		return storage.StorageObject{}, err
	}

	// 2. 生成对象键：[key_prefix/]<yyyyMMdd>/<rand32hex><ext>
	key := generateObjectKey(filename)
	if kp := strings.Trim(global.Config.Storage.Qiniu.KeyPrefix, "/"); kp != "" {
		key = kp + "/" + key
	}

	// 3. 上传凭证
	putPolicy := qstorage.PutPolicy{Scope: bucket}
	upToken := putPolicy.UploadToken(mac)

	// 4. 将流缓冲到内存，使其可重复读取（重试时需要 Seek 回起点）
	buf, err := io.ReadAll(r)
	if err != nil {
		return storage.StorageObject{}, fmt.Errorf("qiniu read upload data: %w", err)
	}
	reader := bytes.NewReader(buf)

	// 5. 分片上传（带重试，每次重试前 Seek 回起点）
	ru := qstorage.NewResumeUploaderV2(cfg)
	var putRet qstorage.PutRet
	uploadErr := storage.DoWithBackoff(
		ctx,
		3,                    // 最多重试 3 次
		300*time.Millisecond, // 基础延迟
		200*time.Millisecond, // 随机抖动
		func() error {
			if _, err := reader.Seek(0, io.SeekStart); err != nil {
				return err
			}
			return ru.PutWithoutSize(ctx, &putRet, upToken, key, reader, &qstorage.RputV2Extra{})
		},
	)
	if uploadErr != nil {
		return storage.StorageObject{}, fmt.Errorf("qiniu upload: %w", uploadErr)
	}

	// 6. 拼接访问 URL
	domain := strings.TrimSuffix(global.Config.Storage.Qiniu.Domain, "/")
	if domain == "" {
		return storage.StorageObject{}, fmt.Errorf("qiniu domain not configured")
	}
	if !strings.HasPrefix(domain, "http://") && !strings.HasPrefix(domain, "https://") {
		domain = "http://" + domain
	}
	fullURL := domain + "/" + key

	return storage.StorageObject{
		Key:  key,
		URL:  fullURL,
		Size: int64(len(buf)),
		Name: filename,
	}, nil
}

// sdkConfig 构建七牛 SDK 配置
func sdkConfig() *qstorage.Config {
	useHTTPS := strings.HasPrefix(global.Config.Storage.Qiniu.Domain, "https://")
	return &qstorage.Config{
		UseHTTPS: useHTTPS,
	}
}

// credentials 从全局配置读取七牛凭证
func credentials() (*auth.Credentials, string, error) {
	q := global.Config.Storage.Qiniu
	ak := strings.TrimSpace(q.AccessKey)
	sk := strings.TrimSpace(q.SecretKey)
	if ak == "" || sk == "" {
		return nil, "", fmt.Errorf("qiniu credentials not configured: set access_key and secret_key")
	}
	if q.Bucket == "" {
		return nil, "", fmt.Errorf("qiniu bucket not configured")
	}
	return auth.New(ak, sk), q.Bucket, nil
}

// generateObjectKey 生成符合规范的对象键：<yyyyMMdd>/<rand32hex><ext>
func generateObjectKey(filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))
	if ext == "" {
		ext = ".bin"
	}
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		// 降级：使用时间戳
		fallback := time.Now().UnixNano()
		b = []byte(fmt.Sprintf("%x", fallback))
	}
	randHex := hex.EncodeToString(b)
	dateDir := time.Now().Format("20060102")
	return filepath.ToSlash(filepath.Join(dateDir, randHex+ext))
}
