package storage

import (
	"context"
	"io"
)

// StorageObject 统一描述一次存储操作的结果与元信息。
// 约定与语义：
//   - Key：驱动内部的"对象名/相对路径"，不包含业务前缀。
//     local 为 <date>/<rand>.ext；qiniu 为桶内对象键。
//   - URL：可直接用于前端访问的完整地址。
//     local 使用 <baseURL>/<key>；qiniu 使用 <domain>/<key>。
//   - Size/Type/Name：可选元信息，供业务使用，驱动按需填充。
type StorageObject struct {
	Key  string // 存储键（相对路径或对象名），不含任何静态前缀
	URL  string // 可访问地址：local 使用相对路径前缀，qiniu 使用完整域名
	Size int64  // 对象大小（字节），未知时为 0
	Type string // MIME 类型或业务自定义类型，未知时为空
	Name string // 原始文件名或业务命名，便于记录与排查
}

// Driver 统一的存储驱动接口，实现需保证并发安全。
// 调用约定：
//   - Delete(ctx, key)：key 为存储键（不含前缀）。资源不存在时必须视为成功（幂等）。
//   - Upload(ctx, r, filename)：r 为流式数据源，filename 用作对象名/建议名。
//     实现应尽量避免整文件缓冲。
//   - Name()：返回驱动名（如 "local"、"qiniu"），用于日志与分支控制。
type Driver interface {
	// Name 返回驱动名称，用于注册与选择
	Name() string
	// Delete 删除存储对象，资源不存在视为成功
	Delete(ctx context.Context, key string) error
	// Upload 上传文件流，返回存储对象信息
	Upload(ctx context.Context, r io.Reader, filename string) (StorageObject, error)
}
