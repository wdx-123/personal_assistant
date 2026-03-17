package entity

import "time"

// 多对多模型，并且通过级联！
// 在删除leetcodeUserDetail/LeetcodeQuestionBank时，会级联删除leetcodeUserQuestion

type LeetcodeUserDetail struct {
	MODEL
	UserSlug   string `json:"user_slug" gorm:"type:varchar(64);not null;index"`
	RealName   string `json:"real_name" gorm:"type:varchar(64)"`
	UserAvatar string `json:"user_avatar" gorm:"type:varchar(255)"`

	EasyNumber   int `json:"easy_number" gorm:"not null;default:0"`
	MediumNumber int `json:"medium_number" gorm:"not null;default:0"`
	HardNumber   int `json:"hard_number" gorm:"not null;default:0"`
	TotalNumber  int `json:"total_number" gorm:"not null;default:0"`

	LastBindAt *time.Time `json:"last_bind_at" gorm:"comment:'上次绑定时间'"`

	UserID uint `json:"user_id" gorm:"not null;index;comment:'所属用户ID(外键)'"`
	User   User `json:"-" gorm:"foreignKey:UserID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
}

type LeetcodeUserQuestion struct {
	MODEL
	LeetcodeUserDetailID uint `json:"leetcode_user_detail_id" gorm:"not null;index;uniqueIndex:idx_user_question;comment:'力扣账号详情ID(外键)'"`
	LeetcodeQuestionID   uint `json:"leetcode_question_id" gorm:"not null;index;uniqueIndex:idx_user_question;comment:'力扣题库题目ID(外键)'"`

	LeetcodeUserDetail LeetcodeUserDetail   `json:"-" gorm:"foreignKey:LeetcodeUserDetailID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	LeetcodeQuestion   LeetcodeQuestionBank `json:"-" gorm:"foreignKey:LeetcodeQuestionID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
}

type LeetcodeQuestionBank struct {
	MODEL
	TitleSlug        string     `json:"title_slug" gorm:"type:varchar(255);uniqueIndex;comment:'题目英文唯一标识(Slug)'"`
	Title            string     `json:"title" gorm:"type:varchar(255);not null"`
	SourceStatus     int8       `json:"source_status" gorm:"type:tinyint;not null;default:1;index;comment:'来源状态:1 verified,2 pending,3 invalid'"`
	SourceType       string     `json:"source_type" gorm:"type:varchar(16);not null;default:'sync';comment:'来源类型 sync|manual'"`
	LastVerifiedAt   *time.Time `json:"last_verified_at,omitempty" gorm:"type:datetime;comment:'最近一次校验时间'"`
	VerifyFailReason string     `json:"verify_fail_reason" gorm:"type:varchar(255);not null;default:'';comment:'最近一次校验失败原因'"`
}
