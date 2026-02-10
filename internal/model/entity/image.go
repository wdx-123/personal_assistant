package entity

import "personal_assistant/internal/model/consts"

// Image 图片表 — 记录所有上传图片的元信息与存储位置
type Image struct {
	MODEL

	// ===== 文件基础信息 =====

	// Name 原始文件名（上传时的文件名）
	Name string `json:"name" gorm:"type:varchar(255);not null;comment:'原始文件名'"`
	// Type 文件扩展名（如 .png、.jpg）
	Type string `json:"type" gorm:"type:varchar(32);not null;comment:'文件扩展名'"`
	// Size 文件大小（字节）
	Size int64 `json:"size" gorm:"not null;default:0;comment:'文件大小(字节)'"`

	// ===== 存储定位 =====

	// Driver 存储驱动名称（local / qiniu）
	Driver string `json:"driver" gorm:"type:varchar(16);not null;default:'local';comment:'存储驱动'"`
	// Key 存储键（驱动内的相对路径/对象名，不含域名前缀）
	Key string `json:"key" gorm:"type:varchar(512);not null;comment:'存储键'"`
	// URL 可直接访问的完整 URL
	URL string `json:"url" gorm:"type:varchar(1024);not null;comment:'访问URL'"`

	// ===== 业务归属 =====

	// Category 图片分类（系统/轮播/封面/插图/广告/Logo 等）
	Category consts.Category `json:"category" gorm:"type:tinyint;not null;default:0;index;comment:'图片分类'"`
	// UploaderID 上传者用户 ID
	UploaderID uint `json:"uploader_id" gorm:"index;comment:'上传者用户ID'"`
	// OrgID 所属组织 ID（可为空，表示个人上传）
	OrgID *uint `json:"org_id,omitempty" gorm:"index;comment:'所属组织ID'"`

	// ===== 去重与校验 =====

	// FileHash 文件内容哈希（SHA-256 64 位十六进制），用于秒传去重
	FileHash string `json:"file_hash,omitempty" gorm:"type:varchar(64);index:idx_images_file_hash;comment:'文件SHA256哈希'"`
	// HashAlgo 哈希算法标识（默认 sha256），便于未来升级算法时区分
	HashAlgo string `json:"hash_algo,omitempty" gorm:"type:varchar(16);default:'sha256';comment:'哈希算法'"`
}

// TableName 指定表名
func (Image) TableName() string {
	return "images"
}
