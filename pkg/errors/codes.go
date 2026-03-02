package errors

// BizCode 业务错误码类型
// 用于 Service 层定义业务错误，Controller 层直接返回给前端
type BizCode int

// 错误码规范：
// - 0: 成功
// - 1xxxx: 通用错误
// - 2xxxx: 用户模块
// - 3xxxx: 组织与权限模块（组织/角色/菜单/API）
// - 4xxxx: OJ模块

const (
	// ==================== 成功 ====================

	CodeSuccess BizCode = 0

	// ==================== 通用错误 1xxxx ====================

	CodeUnknown         BizCode = 10000 // 未知错误
	CodeInvalidParams   BizCode = 10001 // 参数错误
	CodeBindFailed      BizCode = 10002 // 参数绑定失败
	CodeValidateFailed  BizCode = 10003 // 参数校验失败
	CodeInternalError   BizCode = 10004 // 服务器内部错误
	CodeDBError         BizCode = 10005 // 数据库错误
	CodeRedisError      BizCode = 10006 // Redis错误
	CodeThirdPartyError BizCode = 10007 // 第三方服务错误
	CodeTooManyRequests BizCode = 10008 // 请求过于频繁

	// ==================== 认证相关 11xxx ====================

	CodeUnauthorized     BizCode = 11000 // 未授权
	CodeTokenExpired     BizCode = 11001 // Token已过期
	CodeTokenMalformed   BizCode = 11002 // Token格式错误
	CodeTokenInvalid     BizCode = 11003 // Token无效
	CodeTokenBlacklisted BizCode = 11004 // Token已被加入黑名单
	CodeLoginRequired    BizCode = 11005 // 需要登录
	CodePermissionDenied BizCode = 11006 // 权限不足

	// ==================== 用户模块 2xxxx ====================

	CodeUserNotFound      BizCode = 20001 // 用户不存在
	CodeUserAlreadyExists BizCode = 20002 // 用户已存在
	CodePasswordError     BizCode = 20003 // 密码错误
	CodeUserFrozen        BizCode = 20004 // 用户已被冻结
	CodeUserDisabled      BizCode = 20005 // 用户已被禁用
	CodePhoneAlreadyUsed  BizCode = 20006 // 手机号已被使用
	CodeEmailAlreadyUsed  BizCode = 20007 // 邮箱已被使用
	CodeCaptchaError      BizCode = 20008 // 验证码错误
	CodeCaptchaExpired    BizCode = 20009 // 验证码已过期
	CodeEmailSendFailed   BizCode = 20010 // 邮件发送失败

	// ==================== 组织与权限模块 3xxxx ====================

	CodeOrgNotFound       BizCode = 30001 // 组织不存在
	CodeOrgAlreadyExists  BizCode = 30002 // 组织已存在
	CodeOrgNameDuplicate  BizCode = 30003 // 组织名称重复
	CodeNotOrgMember      BizCode = 30004 // 非组织成员
	CodeOrgHasMembers     BizCode = 30005 // 组织下有成员，无法删除
	CodeOrgOwnerOnly      BizCode = 30006 // 仅组织所有者可操作
	CodeRoleNotFound      BizCode = 30101 // 角色不存在
	CodeRoleAlreadyExists BizCode = 30102 // 角色已存在
	CodeMenuNotFound      BizCode = 30201 // 菜单不存在
	CodeMenuCodeDuplicate BizCode = 30202 // 菜单code重复
	CodeMenuHasChildren   BizCode = 30203 // 菜单存在子菜单，无法删除
	CodeAPINotFound       BizCode = 30301 // API不存在
	CodeAPIAlreadyExists  BizCode = 30302 // API已存在（path+method重复）

	// ==================== OJ模块 4xxxx ====================

	CodeOJAccountNotBound   BizCode = 40001 // OJ账号未绑定
	CodeOJAccountBound      BizCode = 40002 // OJ账号已绑定
	CodeOJPlatformInvalid   BizCode = 40003 // OJ平台无效
	CodeOJIdentifierInvalid BizCode = 40004 // OJ账号标识无效
	CodeOJBindCoolDown      BizCode = 40005 // 绑定冷却中
	CodeOJSyncFailed        BizCode = 40006 // OJ数据同步失败
)

// codeMessages 错误码与默认消息的映射
var codeMessages = map[BizCode]string{
	CodeSuccess: "操作成功",

	// 通用错误
	CodeUnknown:         "未知错误",
	CodeInvalidParams:   "参数错误",
	CodeBindFailed:      "参数绑定失败",
	CodeValidateFailed:  "参数校验失败",
	CodeInternalError:   "服务器内部错误",
	CodeDBError:         "数据库错误",
	CodeRedisError:      "缓存服务错误",
	CodeThirdPartyError: "第三方服务错误",
	CodeTooManyRequests: "请求过于频繁，请稍后再试",

	// 认证相关
	CodeUnauthorized:     "未授权访问",
	CodeTokenExpired:     "登录已过期，请重新登录",
	CodeTokenMalformed:   "Token格式错误",
	CodeTokenInvalid:     "Token无效",
	CodeTokenBlacklisted: "Token已失效，请重新登录",
	CodeLoginRequired:    "请先登录",
	CodePermissionDenied: "权限不足",

	// 用户模块
	CodeUserNotFound:      "用户不存在",
	CodeUserAlreadyExists: "用户已存在",
	CodePasswordError:     "用户名或密码错误",
	CodeUserFrozen:        "用户已被冻结",
	CodeUserDisabled:      "用户已被禁用",
	CodePhoneAlreadyUsed:  "手机号已被使用",
	CodeEmailAlreadyUsed:  "邮箱已被使用",
	CodeCaptchaError:      "验证码错误",
	CodeCaptchaExpired:    "验证码已过期",
	CodeEmailSendFailed:   "邮件发送失败",

	// 组织与权限
	CodeOrgNotFound:       "组织不存在",
	CodeOrgAlreadyExists:  "组织已存在",
	CodeOrgNameDuplicate:  "组织名称已存在",
	CodeNotOrgMember:      "您不是该组织成员",
	CodeOrgHasMembers:     "组织下还有成员，无法删除",
	CodeOrgOwnerOnly:      "仅组织所有者可操作",
	CodeRoleNotFound:      "角色不存在",
	CodeRoleAlreadyExists: "角色已存在",
	CodeMenuNotFound:      "菜单不存在",
	CodeMenuCodeDuplicate: "菜单权限标识已存在",
	CodeMenuHasChildren:   "该菜单下存在子菜单，无法删除",
	CodeAPINotFound:       "API不存在",
	CodeAPIAlreadyExists:  "API已存在（路径与方法组合重复）",

	// OJ模块
	CodeOJAccountNotBound:   "OJ账号未绑定",
	CodeOJAccountBound:      "OJ账号已绑定",
	CodeOJPlatformInvalid:   "不支持的OJ平台",
	CodeOJIdentifierInvalid: "OJ账号标识无效",
	CodeOJBindCoolDown:      "操作过于频繁，请稍后再试",
	CodeOJSyncFailed:        "OJ数据同步失败",
}

// Message 获取错误码对应的默认消息
func (c BizCode) Message() string {
	if msg, ok := codeMessages[c]; ok {
		return msg
	}
	return "未知错误"
}

// Int 转换为 int 类型
func (c BizCode) Int() int {
	return int(c)
}
