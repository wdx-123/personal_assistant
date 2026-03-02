package config

// Website 个人网站配置结构体
type Website struct {
	Logo                 string `json:"logo" yaml:"logo"`                                     // 网站小图标/Logo，用于浏览器标签页显示
	FullLogo             string `json:"full_logo" yaml:"full_logo"`                           // 网站完整Logo，用于页面头部显示
	Title                string `json:"title" yaml:"title"`                                   // 网站标题，显示在浏览器标题栏
	Slogan               string `json:"slogan" yaml:"slogan"`                                 // 网站中文标语/口号
	SloganEn             string `json:"slogan_en" yaml:"slogan_en"`                           // 网站英文标语/口号
	Description          string `json:"description" yaml:"description"`                       // 网站描述，用于SEO和页面介绍
	Version              string `json:"version" yaml:"version"`                               // 网站版本号
	CreatedAt            string `json:"created_at" yaml:"created_at"`                         // 网站创建日期
	IcpFiling            string `json:"icp_filing" yaml:"icp_filing"`                         // ICP备案号，中国大陆网站必需
	PublicSecurityFiling string `json:"public_security_filing" yaml:"public_security_filing"` // 公安备案号，中国大陆网站必需
	BilibiliUrl          string `json:"bilibili_url" yaml:"bilibili_url"`                     // B站个人主页链接
	GiteeUrl             string `json:"gitee_url" yaml:"gitee_url"`                           // Gitee个人主页链接
	GithubUrl            string `json:"github_url" yaml:"github_url"`                         // GitHub个人主页链接
	BlogUrl              string `json:"blog_url" yaml:"blog_url"`                             // 个人博客网站链接
	Name                 string `json:"name" yaml:"name"`                                     // 博主昵称/姓名
	Job                  string `json:"job" yaml:"job"`                                       // 博主职业/工作
	Address              string `json:"address" yaml:"address"`                               // 博主所在地址
	Email                string `json:"email" yaml:"email"`                                   // 博主联系邮箱
	QqImage              string `json:"qq_image" yaml:"qq_image"`                             // QQ二维码图片路径
	WechatImage          string `json:"wechat_image" yaml:"wechat_image"`                     // 微信二维码图片路径
}
