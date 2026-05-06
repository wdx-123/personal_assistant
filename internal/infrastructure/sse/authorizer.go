package sse

import (
	"context"
	"errors"
)

var (
	// ErrQueryTokenNotAllowed 表示当前 SSE 接入策略不允许从 query string 读取 token。
	// 这样做是为了避免令牌进入访问日志、浏览器历史或代理缓存，降低泄露风险。
	ErrQueryTokenNotAllowed = errors.New("query token is not allowed for SSE")

	// ErrForbiddenChannel 表示当前主体无权订阅目标 channel。
	// 该错误预留给更严格的授权器实现使用，AllowAllAuthorizer 本身不会主动返回它。
	ErrForbiddenChannel = errors.New("channel subscription forbidden")
)

// AllowAllAuthorizer 提供一套最宽松的授权实现。
// 它的职责不是做业务鉴权，而是在缺省场景下兜底保证 SSE 基础设施可以跑通。
// 注意：
//   - 这里仍然会拒绝 query token，因为这是基础接入安全底线。
//   - 其余字段只做透传，不在这一层引入业务判断。
type AllowAllAuthorizer struct{}

// AuthorizeConnect 负责在连接建立前生成 Principal。
// 参数：
//   - ctx：连接建立阶段的上下文，保留给上层实现接入审计、超时控制或链路透传。
//   - req：客户端发起连接时携带的连接元信息。
//
// 返回值：
//   - *Principal：后续连接注册和事件过滤阶段都会复用的身份快照。
//   - error：当接入方式不被允许时返回错误。
//
// 核心流程：
//  1. 先拒绝 query token，避免把认证信息暴露到 URL。
//  2. 再把请求中的主体信息封装成 Principal，供后续链路共享。
//
// 注意事项：
//   - 这里不校验 subject 与 tenant 的真实性；根据上下文推测，真实鉴权会由更具体的 Authorizer 接管。
func (a *AllowAllAuthorizer) AuthorizeConnect(ctx context.Context, req ConnectRequest) (*Principal, error) {
	_ = ctx

	// 先执行基础安全校验，避免后续链路在无意中接受高风险的 token 传输方式。
	if req.QueryToken != "" {
		return nil, ErrQueryTokenNotAllowed
	}

	// 这里直接构造 Principal，是为了让后续 Broker 与 FilterEvent 可以拿到统一的主体快照。
	return &Principal{
		SubjectID: req.SubjectID,
		TenantID:  req.TenantID,
		Origin:    req.Origin,
	}, nil
}

// AuthorizeSubscribe 负责校验主体是否允许订阅目标 channel。
// 参数：
//   - ctx：订阅阶段上下文。
//   - principal：连接阶段已经确认的主体信息。
//   - channel：即将订阅的目标频道。
//
// 返回值：
//   - error：返回 nil 表示允许订阅。
//
// 核心流程：
//  1. 当前实现仅保留方法位点，不做业务授权。
//  2. 统一返回 nil，让调用方可以使用一套固定接口而不关心具体实现。
//
// 注意事项：
//   - 之所以保留空实现，而不是让调用方直接跳过，是为了后续替换为真实授权器时不改调用链。
func (a *AllowAllAuthorizer) AuthorizeSubscribe(ctx context.Context, principal *Principal, channel string) error {
	_ = ctx
	_ = principal
	_ = channel
	return nil
}

// FilterEvent 负责在事件写出前按主体做最终过滤。
// 参数：
//   - ctx：事件发送阶段上下文。
//   - principal：当前连接对应的主体快照。
//   - evt：待发送事件。
//
// 返回值：
//   - *StreamEvent：允许发送的事件；返回 nil 表示该事件应被丢弃。
//   - error：过滤阶段出现的异常。
//
// 核心流程：
//  1. 当前实现不改写事件内容。
//  2. 直接返回原始事件，保持链路最小干预。
//
// 注意事项：
//   - 如果后续要按租户、组织或字段级权限裁剪事件，应优先在这里做，而不是散落在各个调用方。
func (a *AllowAllAuthorizer) FilterEvent(ctx context.Context, principal *Principal, evt *StreamEvent) (*StreamEvent, error) {
	_ = ctx
	_ = principal
	return evt, nil
}
