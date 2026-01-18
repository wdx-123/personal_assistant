package entity

type LuoguUserDetail struct {
	MODEL
	Identification string `json:"identification" gorm:"type:varchar(64);not null;index"`
	RealName       string `json:"real_name" gorm:"type:varchar(64)"`
	UserAvatar     string `json:"user_avatar" gorm:"type:varchar(255)"`
	PassedNumber   int    `json:"passed_number" gorm:"not null;default:0"`

	UserID uint `json:"user_id" gorm:"not null;index;comment:'所属用户ID(外键)'"`
	User   User `json:"-" gorm:"foreignKey:UserID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
}

type LuoguUserQuestion struct {
	MODEL
	LuoguUserDetailID uint `json:"luogu_user_detail_id" gorm:"not null;index;comment:'洛谷账号详情ID(外键)'"`
	LuoguQuestionID   uint `json:"luogu_question_id" gorm:"not null;index;comment:'洛谷题库题目ID(外键)'"`

	LuoguUserDetail LuoguUserDetail   `json:"-" gorm:"foreignKey:LuoguUserDetailID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	LuoguQuestion   LuoguQuestionBank `json:"-" gorm:"foreignKey:LuoguQuestionID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT;"`
}

type LuoguQuestionBank struct {
	MODEL
	Luogu      uint   `json:"luogu" gorm:"not null;index"`
	Pid        string `json:"pid" gorm:"type:varchar(64);index"`
	Title      string `json:"title" gorm:"type:varchar(255);not null"`
	Difficulty string `json:"difficulty" gorm:"type:varchar(32);default:''"`
	Type       string `json:"type" gorm:"type:varchar(64);default:''"`
}
