package ai

// ToolDescriptor 收口单个 tool 对 selector 和 registry 可见的稳定描述信息。
// 具体业务实现留在 service/app 层，但 metadata 真相应稳定且可复用。
type ToolDescriptor struct {
	Spec    ToolSpec
	GroupID ToolGroupID
	Brief   ToolBrief
}

// ToolGroupProfile 描述单个工具组的固定语义和使用边界。
// ToolNames 属于运行时聚合结果，因此不放在静态 profile 中。
type ToolGroupProfile struct {
	Summary    string
	WhenToUse  string
	DomainTags []string
}
