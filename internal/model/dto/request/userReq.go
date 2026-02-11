package request

// RegisterReq 注册
type RegisterReq struct {
	Username  string `json:"username" binding:"required,max=20"`
	Password  string `json:"password" binding:"required,min=8,max=16"`
	Phone     string `json:"phone" binding:"required,len=11"`
	Captcha   string `json:"captcha" binding:"required,len=6"`
	CaptchaID string `json:"captcha_id" binding:"required"`
	OrgID     uint   `json:"org_id" binding:"required"`
}

// LoginReq 登录
type LoginReq struct {
	Phone     string `json:"phone" binding:"required,len=11"`
	Password  string `json:"password" binding:"required,min=8,max=16"`
	Captcha   string `json:"captcha" binding:"required,len=6"`
	CaptchaID string `json:"captcha_id" binding:"required"`
}

// UpdateProfileReq 更新个人资料
type UpdateProfileReq struct {
	Username  *string `json:"username" binding:"omitempty,max=20"`
	Signature *string `json:"signature" binding:"omitempty,max=100"`
	Avatar    *string `json:"avatar" binding:"omitempty,url"`
	AvatarID  *uint   `json:"avatar_id" binding:"omitempty"`
}

// ChangePhoneReq 换绑手机号
type ChangePhoneReq struct {
	Password  string `json:"password" binding:"required,min=8,max=16"`
	NewPhone  string `json:"new_phone" binding:"required,len=11"`
	Captcha   string `json:"captcha" binding:"required,len=6"`
	CaptchaID string `json:"captcha_id" binding:"required"`
}

// ChangePasswordReq 修改登录密码
type ChangePasswordReq struct {
	OldPassword string `json:"old_password" binding:"required,min=8,max=16"`
	NewPassword string `json:"new_password" binding:"required,min=8,max=16"`
}

// UserListReq 用户列表请求，支持分页、组织过滤和关键词搜索
// Page 和 PageSize 为可选参数，未提供时由 Service 层设置默认值（page=1, page_size=10）
type UserListReq struct {
	Page     int    `form:"page" binding:"omitempty,min=1"`
	PageSize int    `form:"page_size" binding:"omitempty,min=1,max=100"`
	OrgID    uint   `form:"org_id" binding:"omitempty"`
	Keyword  string `form:"keyword" binding:"omitempty,max=50"`
}

// AssignUserRoleReq 分配用户角色请求（全量替换）
// 将指定用户在指定组织下的角色替换为 RoleIDs 中的角色列表
type AssignUserRoleReq struct {
	UserID  uint   `json:"user_id" binding:"required"`
	OrgID   uint   `json:"org_id" binding:"required"`
	RoleIDs []uint `json:"role_ids" binding:"required"`
}
