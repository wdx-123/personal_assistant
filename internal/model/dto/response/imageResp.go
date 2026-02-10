package response

import "personal_assistant/internal/model/consts"

// ImageItem 单张图片响应信息
type ImageItem struct {
	ID            uint             `json:"id"`             // 图片 ID
	URL           string           `json:"url"`            // 访问 URL
	Name          string           `json:"name"`           // 原始文件名
	Type          string           `json:"type"`           // 文件扩展名
	Size          int64            `json:"size"`           // 文件大小（字节）
	Category      consts.Category  `json:"category"`       // 图片分类（int，与请求参数一致）
	CategoryLabel string           `json:"category_label"` // 图片分类中文标签（便于前端直接展示）
}
