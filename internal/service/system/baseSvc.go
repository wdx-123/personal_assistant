package system

import (
	"errors"
	"fmt"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/mojocn/base64Captcha"
	"go.uber.org/zap"
	"personal_assistant/global"
	"personal_assistant/internal/model/dto/request"
	"personal_assistant/pkg/util"
	"time"
)

type BaseService struct {
}

// NewBaseService 返回实例
func NewBaseService() *BaseService {
	return &BaseService{}
}

func (b *BaseService) GetCaptcha(store base64Captcha.Store) (string, string, error) {
	// 创建数字验证码的驱动
	driver := base64Captcha.NewDriverDigit(
		global.Config.Captcha.Height,
		global.Config.Captcha.Width,
		global.Config.Captcha.Length,
		global.Config.Captcha.MaxSkew,
		global.Config.Captcha.DotCount,
	)

	// 创建验证码对象
	captcha := base64Captcha.NewCaptcha(driver, store)

	// 生成验证码
	id, b64s, _, err := captcha.Generate()

	return id, b64s, err
}

func (b *BaseService) VerifyAndSendEmailCode(
	ctx *gin.Context,
	store base64Captcha.Store,
	req *request.SendEmailVerificationCodeReq,
) error {
	if store.Verify(req.CaptchaID, req.Captcha, true) { // 调用的是上方存在的session
		//                        ↑             ↑            ↑
		//   			        验证码ID      用户输入      验证后清除
		err := b.SendEmailVerificationCode(ctx, req.Email)
		if err != nil {
			global.Log.Error("发送邮箱失败", zap.Error(err))
			return fmt.Errorf("发送邮箱失败：%w", err)
		}
		return nil
	}
	global.Log.Error("Verify校验CaptchaID与Captcha错误")
	return errors.New("邮箱验证码校验错误")
}

func (b *BaseService) SendEmailVerificationCode(ctx *gin.Context, to string) error {
	verificationCode := util.GenerateVerificationCode(6)
	expireTime := time.Now().Add(5 * time.Minute).Unix()

	// 将验证码、验证邮箱、过期时间存入会话中
	session := sessions.Default(ctx)
	session.Set("verification_code", verificationCode)
	session.Set("email", to)
	session.Set("expire_time", expireTime)
	err := session.Save()
	if err != nil {
		global.Log.Error("保存session错误：", zap.Error(err))
	}

	subject := "您的邮箱验证码"
	body := `亲爱的用户[` + to + `]，<br/>
<br/>
感谢您注册` + global.Config.Website.Name + `的个人博客！为了确保您的邮箱安全，请使用以下验证码进行验证：<br/>
<br/>
验证码：[<font color="blue"><u>` + verificationCode + `</u></font>]<br/>
该验证码在 5 分钟内有效，请尽快使用。<br/>
<br/>
如果您没有请求此验证码，请忽略此邮件。
<br/>
如有任何疑问，请联系我们的支持团队：<br/>
邮箱：` + global.Config.Email.From + `<br/>
<br/>
祝好，<br/>` +
		global.Config.Website.Title + `<br/>
<br/>`

	_ = util.Email(to, subject, body)

	return nil
}
