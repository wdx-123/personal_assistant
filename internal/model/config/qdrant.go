package config

// Qdrant describes vector database connection settings.
type Qdrant struct {
	Endpoint string `json:"endpoint" yaml:"endpoint"`
	APIKey   string `json:"api_key" yaml:"api_key"`
}
