package consts

// Category 图片类别（底层为 int，JSON 序列化/反序列化均使用数值）
// form 绑定与 JSON 绑定行为一致，前端统一使用 int 值
type Category int

const (
	CatNull         Category = iota // 0 - 未使用
	CatSystem                       // 1 - 系统
	CatCarousel                     // 2 - 轮播
	CatCover                        // 3 - 封面
	CatIllustration                 // 4 - 插图
	CatAdImage                      // 5 - 广告
	CatLogo                         // 6 - Logo
	CatAvatar                       // 7 - 头像
)

// String 返回 Category 的中文可读标签
func (c Category) String() string {
	switch c {
	case CatNull:
		return "未使用"
	case CatSystem:
		return "系统"
	case CatCarousel:
		return "轮播"
	case CatCover:
		return "封面"
	case CatIllustration:
		return "插图"
	case CatAdImage:
		return "广告"
	case CatLogo:
		return "Logo"
	case CatAvatar:
		return "头像"
	default:
		return "未知类别"
	}
}
