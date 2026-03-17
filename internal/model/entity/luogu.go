package entity

import "time"

// 多对多模型，并且通过级联！
// 并且在删除LuoguUserDetail/LuoguQuestionBank时，会级联删除LuoguUserQuestion

type LuoguUserDetail struct {
	MODEL
	Identification string `json:"identification" gorm:"type:varchar(64);not null;index"`
	RealName       string `json:"real_name" gorm:"type:varchar(64)"`
	UserAvatar     string `json:"user_avatar" gorm:"type:varchar(255)"`
	PassedNumber   int    `json:"passed_number" gorm:"not null;default:0"`

	LastBindAt *time.Time `json:"last_bind_at" gorm:"comment:'上次绑定时间'"`

	UserID uint `json:"user_id" gorm:"not null;index;comment:'所属用户ID(外键)'"`
	User   User `json:"-" gorm:"foreignKey:UserID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
}

type LuoguUserQuestion struct {
	MODEL
	LuoguUserDetailID uint `json:"luogu_user_detail_id" gorm:"not null;index;uniqueIndex:idx_user_question;comment:'洛谷账号详情ID(外键)'"`
	LuoguQuestionID   uint `json:"luogu_question_id" gorm:"not null;index;uniqueIndex:idx_user_question;comment:'洛谷题库题目ID(外键)'"`

	LuoguUserDetail LuoguUserDetail   `json:"-" gorm:"foreignKey:LuoguUserDetailID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	LuoguQuestion   LuoguQuestionBank `json:"-" gorm:"foreignKey:LuoguQuestionID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
}

type LuoguQuestionBank struct {
	MODEL
	Pid              string     `json:"pid" gorm:"type:varchar(64);uniqueIndex;comment:'题目唯一标识(如P1001)'"`
	Title            string     `json:"title" gorm:"type:varchar(255);not null"`
	Difficulty       string     `json:"difficulty" gorm:"type:varchar(32);default:''"`
	Type             string     `json:"type" gorm:"type:varchar(64);default:''"`
	SourceStatus     int8       `json:"source_status" gorm:"type:tinyint;not null;default:1;index;comment:'来源状态:1 verified,2 pending,3 invalid'"`
	SourceType       string     `json:"source_type" gorm:"type:varchar(16);not null;default:'sync';comment:'来源类型 sync|manual'"`
	LastVerifiedAt   *time.Time `json:"last_verified_at,omitempty" gorm:"type:datetime;comment:'最近一次校验时间'"`
	VerifyFailReason string     `json:"verify_fail_reason" gorm:"type:varchar(255);not null;default:'';comment:'最近一次校验失败原因'"`
}
