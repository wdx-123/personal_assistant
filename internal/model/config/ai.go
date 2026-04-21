package config

// AI 描述 AI runtime 与 Eino 相关配置。
type AI struct {
	Provider            string  `json:"provider" yaml:"provider"`
	APIKey              string  `json:"api_key" yaml:"api_key"`
	BaseURL             string  `json:"base_url" yaml:"base_url"`
	Model               string  `json:"model" yaml:"model"`
	ByAzure             bool    `json:"by_azure" yaml:"by_azure"`
	APIVersion          string  `json:"api_version" yaml:"api_version"`
	SystemPrompt        string  `json:"system_prompt" yaml:"system_prompt"`
	Temperature         float64 `json:"temperature" yaml:"temperature"`
	MaxCompletionTokens int     `json:"max_completion_tokens" yaml:"max_completion_tokens"`
}
