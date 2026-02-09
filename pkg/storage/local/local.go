package local

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"personal_assistant/global"
	"personal_assistant/pkg/storage"
)

// Driver 本地文件存储驱动，并发安全
type Driver struct{}

// New 创建并返回一个本地存储驱动实例
func New() *Driver { return &Driver{} }

// Name 返回驱动名称
func (d *Driver) Name() string { return "local" }

// Delete 删除本地文件，资源不存在视为成功（幂等）
func (d *Driver) Delete(_ context.Context, key string) error {
	root := global.Config.Static.Path
	realPath := filepath.Join(root, key)
	if err := os.Remove(realPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	return nil
}

// Upload 将流式数据写入本地文件系统，返回存储对象信息
// 支持 Context 取消：大文件写入过程中会检查 ctx.Done()，请求取消时及时中断并清理临时文件
func (d *Driver) Upload(ctx context.Context, r io.Reader, filename string) (storage.StorageObject, error) {
	// 1. 生成唯一存储键
	key, err := generateLocalKey(filename)
	if err != nil {
		return storage.StorageObject{}, fmt.Errorf("generate local key: %w", err)
	}

	// 2. 确保目录存在并写入文件
	root := global.Config.Static.Path
	realPath := filepath.Join(root, key)
	if mkErr := os.MkdirAll(filepath.Dir(realPath), 0755); mkErr != nil {
		return storage.StorageObject{}, fmt.Errorf("create directory: %w", mkErr)
	}

	f, err := os.Create(realPath)
	if err != nil {
		return storage.StorageObject{}, fmt.Errorf("create file: %w", err)
	}
	defer func() { _ = f.Close() }()

	n, err := copyWithContext(ctx, f, r)
	if err != nil {
		// 写入被取消或失败时，清理不完整的文件
		_ = os.Remove(realPath)
		return storage.StorageObject{}, fmt.Errorf("write file: %w", err)
	}

	// 3. 构建访问 URL：static.prefix + / + key
	//    例：/images/20260207/abc123.png
	prefix := strings.TrimSuffix(global.Config.Static.Prefix, "/")
	url := prefix + "/" + strings.TrimLeft(key, "/")

	return storage.StorageObject{
		Key:  key,
		URL:  url,
		Size: n,
		Name: filename,
	}, nil
}

// copyWithContext 分块拷贝，每块之间检查 Context 取消信号
// 32KB 分块大小在内存和响应性之间取得平衡
func copyWithContext(ctx context.Context, dst io.Writer, src io.Reader) (int64, error) {
	buf := make([]byte, 32*1024)
	var written int64
	for {
		select {
		case <-ctx.Done():
			return written, ctx.Err()
		default:
		}
		nr, readErr := src.Read(buf)
		if nr > 0 {
			nw, writeErr := dst.Write(buf[:nr])
			written += int64(nw)
			if writeErr != nil {
				return written, writeErr
			}
		}
		if readErr != nil {
			if readErr == io.EOF {
				return written, nil
			}
			return written, readErr
		}
	}
}

// generateLocalKey 生成唯一键：[key_prefix/]<yyyyMMdd>/<rand32hex><ext>
func generateLocalKey(filename string) (string, error) {
	ext := filepath.Ext(filename)
	if ext == "" {
		ext = ".bin"
	}
	ext = strings.ToLower(ext)

	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	randHex := hex.EncodeToString(b)
	dateDir := time.Now().Format("20060102")
	key := filepath.ToSlash(filepath.Join(dateDir, randHex+ext))

	if kp := strings.Trim(global.Config.Storage.Local.KeyPrefix, "/"); kp != "" {
		key = kp + "/" + key
	}
	return key, nil
}
