package entity

type LeetcodeUserDetail struct {
	MODEL
	UserSlug   string `json:"user_slug" gorm:"type:varchar(64);index"`
	RealName   string `json:"real_name" gorm:"type:varchar(64)"`
	UserAvatar string `json:"user_avatar" gorm:"type:varchar(255)"`

	EasyNumber   int `json:"easy_number" gorm:"not null;default:0"`
	MediumNumber int `json:"medium_number" gorm:"not null;default:0"`
	HardNumber   int `json:"hard_number" gorm:"not null;default:0"`
	TotalNumber  int `json:"total_number" gorm:"not null;default:0"`

	UserID uint `json:"user_id" gorm:"not null;index;comment:'所属用户ID(外键)'"`
	User   User `json:"-" gorm:"foreignKey:UserID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
}

type LeetcodeUserQuestion struct {
	MODEL
	LeetcodeUserDetailID uint `json:"leetcode_user_detail_id" gorm:"not null;index;comment:'力扣账号详情ID(外键)'"`
	LeetcodeQuestionID   uint `json:"leetcode_question_id" gorm:"not null;index;comment:'力扣题库题目ID(外键)'"`

	LeetcodeUserDetail LeetcodeUserDetail   `json:"-" gorm:"foreignKey:LeetcodeUserDetailID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	LeetcodeQuestion   LeetcodeQuestionBank `json:"-" gorm:"foreignKey:LeetcodeQuestionID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT;"`
}

type LeetcodeQuestionBank struct {
	MODEL
	Leetcode uint   `json:"leetcode" gorm:"not null;index"`
	Title    string `json:"title" gorm:"type:varchar(255);not null"`
}
