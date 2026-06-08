package eino

import (
	"context"
	"time"

	einomodel "github.com/cloudwego/eino/components/model"
)

// ChatModelFactory 允许调用方自定义底层 ChatModel 构造逻辑。
// 未注入时仍走 provider 分支创建默认模型。
type ChatModelFactory func(ctx context.Context, cfg Options) (einomodel.BaseChatModel, error)

// Options 描述 Eino 基础流式 runtime 的初始化配置。
// 作用：把外部传入的 AI 运行参数收拢成一个统一配置对象，供 NewRuntime 之类的构造函数使用。
type Options struct {
	// Provider 表示底层使用的模型提供商。
	// 例如：openai、qwen、ark 等。
	Provider string

	// APIKey 表示访问模型服务所需的鉴权密钥。
	// 一般用于请求大模型平台时进行身份认证。
	APIKey string

	// BaseURL 表示模型服务的基础请求地址。
	// 常用于兼容 OpenAI 协议的第三方平台，或自定义代理地址。
	BaseURL string

	// Model 表示本次运行时默认使用的模型名称。
	// 例如：gpt-4o、qwen-max、deepseek-chat 等。
	Model string

	// ByAzure 表示当前是否通过 Azure OpenAI 方式接入模型服务。
	// 如果为 true，后续请求参数组织方式可能与普通 OpenAI 不同。
	ByAzure bool

	// APIVersion 表示 Azure 或部分模型平台要求显式传入的 API 版本号。
	// 普通 OpenAI 兼容模式下，这个字段通常可以为空。
	APIVersion string

	// ChatModelFactory 允许未来在 infrastructure 层注入模型网关或路由工厂。
	// 若为空，则继续使用当前 provider 对应的默认模型实现。
	ChatModelFactory ChatModelFactory

	// SystemPrompt 表示系统提示词。
	// 它用于定义 AI 的全局角色、行为边界和回答风格。
	SystemPrompt string

	// Temperature 表示采样温度。
	// 数值越高，输出通常越发散；数值越低，输出通常越稳定。
	Temperature float64

	// MaxCompletionTokens 表示本次模型生成阶段允许输出的最大 token 数。
	// 用于限制回答长度，避免输出过长或消耗过多配额。
	MaxCompletionTokens int

	// HeartbeatInterval 表示流式输出时的心跳间隔。
	// 当模型长时间没有新 token 输出时，可按该间隔发送 keepalive，避免 SSE 连接被中断。
	HeartbeatInterval time.Duration
}
