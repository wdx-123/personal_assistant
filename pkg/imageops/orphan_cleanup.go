package imageops

import (
	"context"

	"personal_assistant/pkg/storage"
)

// OrphanCleanupRepo 定义孤儿文件清理所需的仓储能力。
type OrphanCleanupRepo interface {
	FindOrphanKeys(ctx context.Context) (keys []string, drivers []string, err error)
	HardDeleteByKeys(ctx context.Context, keys []string) error
}

// DriverResolver 根据驱动名解析存储驱动实例。
type DriverResolver func(name string) storage.Driver

// FallbackDriver 获取当前默认存储驱动。
type FallbackDriver func() storage.Driver

// OrphanCleanupResult 描述一次孤儿文件清理的结构化结果。
type OrphanCleanupResult struct {
	TotalCandidates int
	SuccessKeys     []string // 存储删除成功的
	FailedKeys      []string // 存储删除失败的
	NoDriverKeys    []string // 找不到可用驱动的
}

// CleanOrphanFiles 执行孤儿文件清理：
// 1) 查找孤儿 key
// 2) 逐个删除物理文件
// 3) 对成功 key 执行硬删除收尾
func CleanOrphanFiles(
	ctx context.Context,
	repo OrphanCleanupRepo,
	resolve DriverResolver,
	fallback FallbackDriver,
) (*OrphanCleanupResult, error) {
	keys, drivers, err := repo.FindOrphanKeys(ctx)
	if err != nil {
		return nil, err
	}
	result := &OrphanCleanupResult{
		TotalCandidates: len(keys),
		SuccessKeys:     make([]string, 0, len(keys)),
		FailedKeys:      make([]string, 0, len(keys)),
		NoDriverKeys:    make([]string, 0, len(keys)),
	}
	if len(keys) == 0 {
		return result, nil
	}

	// 按顺序处理每个 key，尝试找到对应的驱动并删除。
	// 驱动匹配规则：索引对应优先，后备兜底。
	for i, key := range keys {
		driverName := ""
		if i < len(drivers) {
			driverName = drivers[i]
		}

		var drv storage.Driver
		if resolve != nil {
			drv = resolve(driverName)
		}
		if drv == nil && fallback != nil {
			drv = fallback()
		}
		if drv == nil {
			result.NoDriverKeys = append(result.NoDriverKeys, key)
			continue
		}

		if delErr := drv.Delete(ctx, key); delErr != nil {
			result.FailedKeys = append(result.FailedKeys, key)
			continue
		}
		result.SuccessKeys = append(result.SuccessKeys, key)
	}

	// 对成功删除的 key 执行仓储层的硬删除。
	if len(result.SuccessKeys) > 0 {
		if err := repo.HardDeleteByKeys(ctx, result.SuccessKeys); err != nil {
			return result, err
		}
	}
	return result, nil
}
