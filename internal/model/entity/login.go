package entity

// Login 登录日志表 - 记录用户登录行为和设备信息，用于安全审计和统计分析
type Login struct {
	MODEL
	UserID      uint   `json:"user_id" gorm:"type:bigint unsigned;not null;index;comment:'关联用户ID'"`        // 关联的用户ID，外键引用users表
	User        User   `json:"user" gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE;comment:'关联用户信息'"` // 关联的用户实体，级联删除
	LoginMethod string `json:"login_method" gorm:"type:varchar(20);not null;index;comment:'登录方式'"`         // 登录方式：password/oauth/sms等
	IP          string `json:"ip" gorm:"type:varchar(45);not null;index;comment:'登录IP地址'"`                 // 用户登录的IP地址（支持IPv6）
	Address     string `json:"address" gorm:"type:varchar(200);default:'';comment:'登录地理位置'"`               // 根据IP解析的地理位置信息
	OS          string `json:"os" gorm:"type:varchar(50);default:'';comment:'操作系统信息'"`                     // 用户设备的操作系统
	DeviceInfo  string `json:"device_info" gorm:"type:varchar(200);default:'';comment:'设备详细信息'"`           // 设备的详细信息（型号、版本等）
	BrowserInfo string `json:"browser_info" gorm:"type:varchar(200);default:'';comment:'浏览器信息'"`           // 浏览器类型和版本信息
	Status      int    `json:"status" gorm:"type:tinyint;not null;default:1;index;comment:'登录状态'"`         // 登录状态：1成功，0失败，2异常
}
