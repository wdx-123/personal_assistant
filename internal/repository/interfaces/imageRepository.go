package interfaces

import (
	"context"
	"personal_assistant/internal/model/consts"
	"personal_assistant/internal/model/entity"
)

// ImageRepository 图片仓储接口 — 仅负责数据库交互，不涉及文件系统操作
type ImageRepository interface {
	// Create 创建单条图片记录
	Create(ctx context.Context, image *entity.Image) error
	// BatchCreate 批量创建图片记录
	BatchCreate(ctx context.Context, images []*entity.Image) error
	// GetByID 根据 ID 获取图片
	GetByID(ctx context.Context, id uint) (*entity.Image, error)
	// GetByIDs 根据 ID 列表批量获取图片
	GetByIDs(ctx context.Context, ids []uint) ([]entity.Image, error)
	// Delete 根据 ID 列表软删除图片记录
	Delete(ctx context.Context, ids []uint) error
	// List 分页查询图片列表，category 为 nil 时不过滤
	List(ctx context.Context, category *consts.Category, offset, limit int) ([]entity.Image, int64, error)
	// GetByFileHash 根据文件哈希和大小查找已有记录（用于秒传去重）
	GetByFileHash(ctx context.Context, hash string, size int64) (*entity.Image, error)
	// SumSizeByUploader 统计指定用户已使用的存储空间（字节），用于配额检查
	SumSizeByUploader(ctx context.Context, uploaderID uint) (int64, error)
	// FindOrphanKeys 查找孤儿存储键：已被软删除且无活跃引用的 key
	// 返回 (key, driver) 对，供清理任务确定用哪个驱动删除物理文件
	FindOrphanKeys(ctx context.Context) (keys []string, drivers []string, err error)
	// HardDeleteByKeys 物理删除指定 key 的所有已软删除记录（清理完物理文件后调用）
	HardDeleteByKeys(ctx context.Context, keys []string) error

	// UpdateCategoryByID 根据 ID 更新图片分类
	UpdateCategoryByID(ctx context.Context, id uint, category consts.Category) error

	// WithTx 启用事务
	WithTx(tx any) ImageRepository
}
