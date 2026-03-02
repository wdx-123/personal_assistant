package entity

import (
	"time"

	"gorm.io/gorm"
)

// MODEL 基础模型结构体 - 包含所有实体表的公共字段，提供统一的主键、时间戳和软删除功能
type MODEL struct {
	ID        uint           `json:"id" gorm:"primarykey;comment:'主键ID'"`                         // 数据库主键，自增长整型
	CreatedAt time.Time      `json:"created_at" gorm:"type:datetime;not null;comment:'记录创建时间'"`   // 记录创建时间，GORM自动管理
	UpdatedAt time.Time      `json:"updated_at" gorm:"type:datetime;not null;comment:'记录最后更新时间'"` // 记录最后更新时间，GORM自动管理
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index;comment:'软删除时间'"`                              // 软删除时间戳，非空表示已删除，不在JSON中返回
}
