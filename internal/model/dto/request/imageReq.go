package request

import "personal_assistant/internal/model/consts"

// UploadImageReq 图片上传请求参数（multipart/form-data 表单字段）
type UploadImageReq struct {
	// Category 图片分类（可选，默认为 0/Null）
	Category consts.Category `form:"category"`
	// Driver 指定存储驱动（可选，如 "local"/"qiniu"），为空则使用当前配置的默认驱动
	Driver string `form:"driver"`
}

// DeleteImageReq 批量删除图片请求
type DeleteImageReq struct {
	// IDs 要删除的图片 ID 列表
	IDs []uint `json:"ids" binding:"required,min=1"`
}

// ListImageReq 图片列表查询请求
type ListImageReq struct {
	// Page 页码（默认 1）
	Page int `form:"page" binding:"omitempty,min=1"`
	// PageSize 每页数量（默认 10，最大 100）
	PageSize int `form:"page_size" binding:"omitempty,min=1,max=100"`
	// Category 按分类过滤（可选）
	Category *consts.Category `form:"category" binding:"omitempty"`
}
