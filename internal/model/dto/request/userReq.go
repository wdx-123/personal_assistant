package request

// RegisterReq 注册
type RegisterReq struct {
	Username  string `json:"username" binding:"required,max=20"`
	Password  string `json:"password" binding:"required,min=8,max=16"`
	Phone     string `json:"phone" binding:"required,len=11"`
	Captcha   string `json:"captcha" binding:"required,len=6"`
	CaptchaID string `json:"captcha_id" binding:"required"`
	OrgID     uint   `json:"org_id"` // 可选：加入的组织ID
}

// LoginReq 登录
type LoginReq struct {
	Phone     string `json:"phone" binding:"required,len=11"`
	Password  string `json:"password" binding:"required,min=8,max=16"`
	Captcha   string `json:"captcha" binding:"required,len=6"`
	CaptchaID string `json:"captcha_id" binding:"required"`
}
