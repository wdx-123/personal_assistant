package aitool

// NewRegistry 创建 AI tool 注册表。
func NewRegistry(deps Deps) *Registry {
	return newAIToolRegistry(deps)
}
