package core

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"personal_assistant/global"
	"personal_assistant/internal/model/config"

	"github.com/qdrant/go-client/qdrant"
	"go.uber.org/zap"
)

// InitQdrant 初始化项目级 Qdrant client，并按配置确保基础 collection 可用。
//
// 参数：
//   - ctx：启动期上下文；为空时会回退到 context.Background()。
//
// 返回值：
//   - *qdrant.Client：初始化成功后的官方 Qdrant gRPC client；禁用时返回 nil。
//   - error：配置缺失、连接失败、健康检查失败或 collection 校验失败时返回原始错误链。
//
// 启动策略：
//   - qdrant.enabled=false 时跳过初始化，不影响主服务启动。
//   - qdrant.enabled=true 时采用 fail-fast，避免后续 RAG/向量能力在运行期才暴露不可用。
//
// 边界说明：
//   - 本函数只做基础设施装配和 collection 准备，不负责 embedding、索引写入或检索业务。
//   - 具体退出进程的决策由 internal/init 编排层处理，便于测试和复用。
func InitQdrant(ctx context.Context) (*qdrant.Client, error) {
	if global.Config == nil {
		return nil, errors.New("global config is nil")
	}
	qdrantCfg := global.Config.Qdrant
	if !qdrantCfg.Enabled {
		global.Log.Info("Qdrant initialization skipped: disabled")
		return nil, nil
	}

	client, err := newQdrantClient(qdrantCfg)
	if err != nil {
		return nil, err
	}

	runCtx, cancel := context.WithTimeout(normalizeContext(ctx), qdrantTimeout(qdrantCfg))
	defer cancel()

	health, err := client.HealthCheck(runCtx)
	if err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("qdrant health check failed: %w", err)
	}
	global.Log.Info("Qdrant health check succeeded", zap.String("version", health.GetVersion()))

	if qdrantCfg.InitCollection {
		if err := ensureQdrantCollection(runCtx, client, qdrantCfg); err != nil {
			_ = client.Close()
			return nil, err
		}
	}
	return client, nil
}

// newQdrantClient 根据配置创建官方 Qdrant gRPC client。
//
// 说明：
//   - github.com/qdrant/go-client 使用 gRPC 协议，端口应指向 6334，而不是 HTTP/REST 的 6333。
//   - APIKey 只在 client 配置中传递，不写入日志，避免敏感信息泄漏。
//   - 这里只负责构造连接对象，连通性由 InitQdrant 的 HealthCheck 统一验证。
func newQdrantClient(qdrantCfg config.Qdrant) (*qdrant.Client, error) {
	host, err := qdrantGRPCHost(qdrantCfg)
	if err != nil {
		return nil, err
	}
	port := qdrantCfg.GRPCPort
	if port <= 0 {
		port = 6334
	}
	client, err := qdrant.NewClient(&qdrant.Config{
		Host:   host,
		Port:   port,
		APIKey: strings.TrimSpace(qdrantCfg.APIKey),
		UseTLS: qdrantCfg.UseTLS,
	})
	if err != nil {
		return nil, fmt.Errorf("create qdrant client failed: %w", err)
	}
	return client, nil
}

// ensureQdrantCollection 确保配置指定的单向量 collection 已存在且 schema 匹配。
//
// 核心流程：
//  1. 校验 collection 名称、向量维度和距离算法配置。
//  2. 调用 CollectionExists 判断是否已存在。
//  3. 不存在则按配置创建；已存在则读取 collection 信息并校验向量参数。
//
// 生产约束：
//   - 已存在 collection 的维度或距离算法不匹配时必须返回错误。
//   - 这里不自动删除或重建 collection，避免误删线上已有向量数据。
func ensureQdrantCollection(
	ctx context.Context,
	client *qdrant.Client,
	qdrantCfg config.Qdrant,
) error {
	collectionName := strings.TrimSpace(qdrantCfg.CollectionName)
	if collectionName == "" {
		return errors.New("qdrant collection name is empty")
	}
	if qdrantCfg.VectorSize <= 0 {
		return fmt.Errorf("qdrant vector size must be positive: %d", qdrantCfg.VectorSize)
	}
	distance, err := parseQdrantDistance(qdrantCfg.Distance)
	if err != nil {
		return err
	}

	exists, err := client.CollectionExists(ctx, collectionName)
	if err != nil {
		return fmt.Errorf("check qdrant collection failed: %w", err)
	}
	if !exists {
		if err := client.CreateCollection(ctx, &qdrant.CreateCollection{
			CollectionName: collectionName,
			VectorsConfig: qdrant.NewVectorsConfig(&qdrant.VectorParams{
				Size:     uint64(qdrantCfg.VectorSize),
				Distance: distance,
			}),
		}); err != nil {
			return fmt.Errorf("create qdrant collection failed: %w", err)
		}
		global.Log.Info("Qdrant collection created", zap.String("collection", collectionName))
		return nil
	}

	info, err := client.GetCollectionInfo(ctx, collectionName)
	if err != nil {
		return fmt.Errorf("get qdrant collection info failed: %w", err)
	}
	// 只接受单向量 collection；多向量 collection 需要明确的向量名映射，不能静默兼容。
	params := info.GetConfig().GetParams().GetVectorsConfig().GetParams()
	if params == nil {
		return fmt.Errorf("qdrant collection %q is not a single-vector collection", collectionName)
	}
	if params.GetSize() != uint64(qdrantCfg.VectorSize) || params.GetDistance() != distance {
		return fmt.Errorf(
			"qdrant collection %q config mismatch: got size=%d distance=%s, want size=%d distance=%s",
			collectionName,
			params.GetSize(),
			params.GetDistance().String(),
			qdrantCfg.VectorSize,
			distance.String(),
		)
	}
	global.Log.Info("Qdrant collection verified", zap.String("collection", collectionName))
	return nil
}

// qdrantGRPCHost 返回官方 Go client 所需的 gRPC host。
//
// GRPCHost 优先级高于 Endpoint；Endpoint 仅作为兼容上一阶段 HTTP/REST 配置的 host 来源。
// 注意这里不会复用 Endpoint 端口，因为官方 Go client 连接的是 gRPC 端口，默认 6334。
func qdrantGRPCHost(qdrantCfg config.Qdrant) (string, error) {
	if host := strings.TrimSpace(qdrantCfg.GRPCHost); host != "" {
		return host, nil
	}
	endpoint := strings.TrimSpace(qdrantCfg.Endpoint)
	if endpoint == "" {
		return "", errors.New("qdrant grpc_host or endpoint is required")
	}
	parsed, err := url.Parse(endpoint)
	if err != nil {
		return "", fmt.Errorf("parse qdrant endpoint failed: %w", err)
	}
	if host := parsed.Hostname(); host != "" {
		return host, nil
	}
	return "", fmt.Errorf("qdrant endpoint has no host: %q", endpoint)
}

// parseQdrantDistance 将配置中的稳定字符串转换为 Qdrant gRPC 枚举。
//
// 配置层保留字符串是为了让 .env/configs.yaml 可读；转换集中在 core 层，避免业务层依赖 SDK 枚举。
func parseQdrantDistance(distance string) (qdrant.Distance, error) {
	switch strings.ToLower(strings.TrimSpace(distance)) {
	case "", "cosine":
		return qdrant.Distance_Cosine, nil
	case "dot":
		return qdrant.Distance_Dot, nil
	case "euclid", "euclidean":
		return qdrant.Distance_Euclid, nil
	case "manhattan":
		return qdrant.Distance_Manhattan, nil
	default:
		return qdrant.Distance_UnknownDistance, fmt.Errorf("unsupported qdrant distance: %q", distance)
	}
}

// qdrantTimeout 返回启动期 Qdrant 操作超时时间。
//
// 配置缺失或非法时使用 10 秒兜底，避免启动阶段长期卡死。
func qdrantTimeout(qdrantCfg config.Qdrant) time.Duration {
	if qdrantCfg.TimeoutSeconds <= 0 {
		return 10 * time.Second
	}
	return time.Duration(qdrantCfg.TimeoutSeconds) * time.Second
}

// normalizeContext 统一处理 nil context，避免启动编排或测试替身传空值时 panic。
func normalizeContext(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}
