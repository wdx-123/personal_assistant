package system

import (
	"context"
	"errors"

	"personal_assistant/internal/model/consts"
	"personal_assistant/internal/model/entity"
	"personal_assistant/internal/repository/interfaces"

	"gorm.io/gorm"
)

// imageRepository 图片仓储实现 — 纯 DB 操作
type imageRepository struct {
	db *gorm.DB
}

// NewImageRepository 创建图片仓储实例
func NewImageRepository(db *gorm.DB) interfaces.ImageRepository {
	return &imageRepository{db: db}
}

// Create 创建单条图片记录
func (r *imageRepository) Create(ctx context.Context, image *entity.Image) error {
	return r.db.WithContext(ctx).Create(image).Error
}

// BatchCreate 批量创建图片记录
func (r *imageRepository) BatchCreate(ctx context.Context, images []*entity.Image) error {
	if len(images) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Create(&images).Error
}

// GetByID 根据 ID 获取图片
func (r *imageRepository) GetByID(ctx context.Context, id uint) (*entity.Image, error) {
	var image entity.Image
	err := r.db.WithContext(ctx).First(&image, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &image, nil
}

// UpdateCategoryByID 根据 ID 更新图片分类
func (r *imageRepository) UpdateCategoryByID(ctx context.Context, id uint, category consts.Category) error {
	return r.db.WithContext(ctx).
		Model(&entity.Image{}).
		Where("id = ?", id).
		Update("category", category).Error
}

// WithTx 启用事务
func (r *imageRepository) WithTx(tx any) interfaces.ImageRepository {
	if transaction, ok := tx.(*gorm.DB); ok {
		return &imageRepository{db: transaction}
	}
	return r
}

// GetByIDs 根据 ID 列表批量获取图片
func (r *imageRepository) GetByIDs(ctx context.Context, ids []uint) ([]entity.Image, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var images []entity.Image
	err := r.db.WithContext(ctx).Where("id IN ?", ids).Find(&images).Error
	if err != nil {
		return nil, err
	}
	return images, nil
}

// Delete 根据 ID 列表软删除图片记录
func (r *imageRepository) Delete(ctx context.Context, ids []uint) error {
	if len(ids) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Where("id IN ?", ids).Delete(&entity.Image{}).Error
}

// List 分页查询图片列表
func (r *imageRepository) List(ctx context.Context, category *consts.Category, offset, limit int) ([]entity.Image, int64, error) {
	var images []entity.Image
	var total int64

	query := r.db.WithContext(ctx).Model(&entity.Image{})
	if category != nil {
		query = query.Where("category = ?", *category)
	}

	// 先查总数
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 再查分页数据
	if err := query.Order("id DESC").Offset(offset).Limit(limit).Find(&images).Error; err != nil {
		return nil, 0, err
	}
	return images, total, nil
}

// FindOrphanKeys 查找孤儿存储键：已被软删除且无活跃引用的 key
// 返回 (key, driver) 对，供定时清理任务确定用哪个驱动删除物理文件
func (r *imageRepository) FindOrphanKeys(ctx context.Context) (keys []string, drivers []string, err error) {
	type row struct {
		Key    string
		Driver string
	}
	var rows []row
	// 子查询排除仍有活跃引用的 key
	err = r.db.WithContext(ctx).
		Unscoped().
		Model(&entity.Image{}).
		Select("DISTINCT `key`, driver").
		Where("deleted_at IS NOT NULL").
		Where("NOT EXISTS (SELECT 1 FROM images a WHERE a.`key` = images.`key` AND a.deleted_at IS NULL)").
		Scan(&rows).Error
	if err != nil {
		return nil, nil, err
	}
	keys = make([]string, len(rows))
	drivers = make([]string, len(rows))
	for i, r := range rows {
		keys[i] = r.Key
		drivers[i] = r.Driver
	}
	return keys, drivers, nil
}

// HardDeleteByKeys 物理删除指定 key 的所有已软删除记录（清理完物理文件后调用）
func (r *imageRepository) HardDeleteByKeys(ctx context.Context, keys []string) error {
	if len(keys) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).
		Unscoped().
		Where("`key` IN ? AND deleted_at IS NOT NULL", keys).
		Delete(&entity.Image{}).Error
}

// SumSizeByUploader 统计指定用户已使用的存储空间（字节）
// 用于上传前的配额检查，COALESCE 兜底确保用户无记录时返回 0 而非 NULL
func (r *imageRepository) SumSizeByUploader(ctx context.Context, uploaderID uint) (int64, error) {
	var total int64
	err := r.db.WithContext(ctx).Model(&entity.Image{}).
		Where("uploader_id = ?", uploaderID).
		Select("COALESCE(SUM(size), 0)").Scan(&total).Error
	return total, err
}

// GetByFileHash 根据文件哈希和大小查找已有记录（秒传去重）
func (r *imageRepository) GetByFileHash(ctx context.Context, hash string, size int64) (*entity.Image, error) {
	var image entity.Image
	err := r.db.WithContext(ctx).
		Where("file_hash = ? AND size = ?", hash, size).
		First(&image).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &image, nil
}
