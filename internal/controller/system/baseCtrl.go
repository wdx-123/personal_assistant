package system

import (
	"fmt"

	"go.uber.org/zap"

	"personal_assistant/global"
	"personal_assistant/internal/model/dto/request"
	resp "personal_assistant/internal/model/dto/response"
	serviceSystem "personal_assistant/internal/service/system"
	"personal_assistant/pkg/response"

	"github.com/gin-gonic/gin"
	"github.com/mojocn/base64Captcha"
)

type BaseCtrl struct {
	baseService *serviceSystem.BaseService
}

// 用来存储共享验证码
var store = base64Captcha.DefaultMemStore

// Captcha 生成数字验证码
func (b *BaseCtrl) Captcha(ctx *gin.Context) {
	helper := response.NewAPIHelper(ctx, "Captcha")

	// 调用服务层生成验证码，传递store
	id, b64s, err := b.baseService.GetCaptcha(store)
	if err != nil {
		helper.CommonError("Failed to generate captcha", global.StatusInternalServerError, err)
		response.NewResponse[resp.Captcha, resp.Captcha](ctx).
			SetCode(global.StatusInternalServerError).Failed("Failed to generate captcha", nil)
		return
	}

	// 成功响应
	response.NewResponse[resp.Captcha, resp.Captcha](ctx).
		SetCode(global.StatusOK).Success("验证码生成成功", resp.Captcha{
		CaptchaID: id,
		PicPath:   b64s,
	})
}

// SendEmailVerificationCode 发送邮箱验证码
func (b *BaseCtrl) SendEmailVerificationCode(ctx *gin.Context) {
	var req request.SendEmailVerificationCodeReq
	err := ctx.ShouldBindJSON(&req)
	if err != nil {
		global.Log.Error("绑定数据错误", zap.Error(err))
		response.NewResponse[any, any](ctx).SetCode(global.StatusBadRequest).
			Failed(fmt.Sprintf("绑定数据错误: %v", err), nil)
		return
	}
	err = b.baseService.VerifyAndSendEmailCode(ctx, store, &req)
	if err != nil {
		global.Log.Error("验证码校验错误", zap.Error(err))
		response.NewResponse[any, any](ctx).SetCode(global.StatusBadRequest).
			Failed(fmt.Sprintf("验证码校验错误: %v", err), nil)
		return
	}
	response.NewResponse[any, any](ctx).SetCode(global.StatusOK).
		Success("已成功发送邮件", nil)
}
