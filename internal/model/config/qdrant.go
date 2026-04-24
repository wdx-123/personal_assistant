package config

// Qdrant 描述项目级向量数据库连接和启动期 collection 初始化配置。
//
// 注意：
//   - Endpoint 保留给 HTTP/REST 访问语义，官方 Go client 实际使用 GRPCHost/GRPCPort。
//   - VectorSize 必须和后续 embedding 模型输出维度一致，否则写入向量时会失败。
//   - APIKey 属于敏感配置，只允许通过 .env 或运行环境注入，禁止写入模板真实值。
type Qdrant struct {
	Enabled  bool   `json:"enabled" yaml:"enabled"`     // 是否启用 Qdrant 初始化；开启后连接失败会阻断启动
	Endpoint string `json:"endpoint" yaml:"endpoint"`   // Qdrant HTTP/REST 地址，例如 http://127.0.0.1:6333
	GRPCHost string `json:"grpc_host" yaml:"grpc_host"` // Qdrant gRPC host；为空时从 Endpoint 中解析 host
	GRPCPort int    `json:"grpc_port" yaml:"grpc_port"` // Qdrant gRPC 端口，官方 Go client 默认使用 6334
	APIKey   string `json:"api_key" yaml:"api_key"`     // Qdrant API key；仅从安全环境变量读取真实值

	// CollectionName 是启动期需要确保存在的单向量 collection 名称。
	CollectionName string `json:"collection_name" yaml:"collection_name"`

	// VectorSize 是 collection 的向量维度，必须和 embedding 模型输出保持一致。
	VectorSize int `json:"vector_size" yaml:"vector_size"`

	// Distance 是向量相似度算法，当前支持 cosine、dot、euclid/euclidean、manhattan。
	Distance string `json:"distance" yaml:"distance"`

	InitCollection bool `json:"init_collection" yaml:"init_collection"` // 是否在启动期自动创建/校验 collection

	// TimeoutSeconds 控制 health check、collection 创建和校验的启动期超时时间。
	TimeoutSeconds int  `json:"timeout_seconds" yaml:"timeout_seconds"`
	UseTLS         bool `json:"use_tls" yaml:"use_tls"` // 是否使用 TLS 连接 Qdrant gRPC 服务
}
