package global

type AppCode int

// 自定义业务码：StatusOK = 2000，区别于 http.StatusOK = 200
const (
	StatusOK                  AppCode = 2000 // 成功
	StatusBadRequest          AppCode = 4000 // 请求语法错误或无效参数
	StatusInternalServerError AppCode = 5000 // 服务器内部错误
	StatusUnauthorized        AppCode = 4010 // 未授权
	StatusForbidden           AppCode = 4230 // 状态禁止
	StatusTooManyRequests     AppCode = 4290 // 请求过于频繁
)

// JWT相关错误码
const (
	StatusTokenExpired   AppCode = 4011 // Token已过期
	StatusTokenMalformed AppCode = 4012 // Token格式错误
	StatusTokenInvalid   AppCode = 4013 // Token无效
	StatusUserNotFound   AppCode = 4014 // 用户不存在
	StatusUserFrozen     AppCode = 4015 // 用户被冻结
)

const (
	ErrBindDataFailed = "绑定数据失败"
)
