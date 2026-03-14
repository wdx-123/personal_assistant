package entity

import "time"

// LanqiaoUserDetail 保存蓝桥账号凭据与蓝桥特有的提交统计快照。
// 注意：通过题总数不以这里的统计字段为真相，而是以 LanqiaoUserQuestion 的去重数量为准。
type LanqiaoUserDetail struct {
	MODEL
	CredentialHash       string     `json:"-" gorm:"type:char(64);not null;uniqueIndex;comment:'手机号+密码联合哈希'"`          // 用于识别蓝桥账号唯一性
	PhoneCipher          string     `json:"-" gorm:"type:varchar(255);not null;comment:'加密后的手机号'"`                     // 手机号密文
	PasswordCipher       string     `json:"-" gorm:"type:varchar(255);not null;comment:'加密后的密码'"`                      // 密码密文
	MaskedPhone          string     `json:"masked_phone" gorm:"type:varchar(32);not null;default:'';comment:'脱敏手机号'"`  // 用于展示，不暴露原始手机号
	SubmitSuccessCount   int        `json:"submit_success_count" gorm:"not null;default:0;comment:'提交成功次数'"`           // sync_num=-1 的统计口径
	SubmitFailedCount    int        `json:"submit_failed_count" gorm:"not null;default:0;comment:'提交失败次数'"`            // sync_num=-1 的统计口径
	SubmitStatsUpdatedAt *time.Time `json:"submit_stats_updated_at,omitempty" gorm:"type:datetime;comment:'提交统计更新时间'"` // 最近一次 -1 统计刷新时间
	LastBindAt           *time.Time `json:"last_bind_at,omitempty" gorm:"type:datetime;comment:'上次绑定时间'"`              // 首次/重新绑定时间
	LastSyncAt           *time.Time `json:"last_sync_at,omitempty" gorm:"type:datetime;comment:'最近一次题目同步时间'"`          // 最近一次 0 或 >0 明细同步时间

	UserID uint `json:"user_id" gorm:"not null;index;comment:'所属用户ID(外键)'"` // 所属用户ID
	User   User `json:"-" gorm:"foreignKey:UserID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
}

// LanqiaoUserQuestion 保存用户已经通过的蓝桥题目事实。
// 一道题对同一用户只保留一条“首次通过”记录，作为通过题总数与曲线的真实来源。
type LanqiaoUserQuestion struct {
	MODEL
	LanqiaoUserDetailID uint      `json:"lanqiao_user_detail_id" gorm:"not null;index;uniqueIndex:idx_lanqiao_user_question;comment:'蓝桥账号详情ID(外键)'"` // 蓝桥账号详情ID
	LanqiaoQuestionID   uint      `json:"lanqiao_question_id" gorm:"not null;index;uniqueIndex:idx_lanqiao_user_question;comment:'蓝桥题库题目ID(外键)'"`    // 蓝桥题库题目ID
	SolvedAt            time.Time `json:"solved_at" gorm:"type:datetime;not null;index;comment:'首次通过时间'"`                                            // 题目首次通过时间

	LanqiaoUserDetail LanqiaoUserDetail   `json:"-" gorm:"foreignKey:LanqiaoUserDetailID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	LanqiaoQuestion   LanqiaoQuestionBank `json:"-" gorm:"foreignKey:LanqiaoQuestionID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
}

// LanqiaoQuestionBank 保存蓝桥题目元信息。
type LanqiaoQuestionBank struct {
	MODEL
	ProblemID int    `json:"problem_id" gorm:"not null;uniqueIndex;comment:'蓝桥题目唯一ID'"` // 蓝桥题目ID
	Title     string `json:"title" gorm:"type:varchar(255);not null;comment:'题目标题'"`    // 蓝桥题目标题
}
