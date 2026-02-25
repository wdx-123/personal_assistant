package imageops

import "context"

// SoftDeleteRepo 定义图片软删除所需的最小仓储能力。
type SoftDeleteRepo interface {
	Delete(ctx context.Context, ids []uint) error
}

// SoftDeleteResult 描述一次软删除请求的输入与可执行集合。
type SoftDeleteResult struct {
	RequestedIDs []uint
	DeletableIDs []uint
}

// SoftDeleteByIDs 对图片 ID 做去重与过滤后执行软删除。
// 过滤规则：移除 0 值 ID；保留首次出现顺序。
func SoftDeleteByIDs(
	ctx context.Context,
	repo SoftDeleteRepo,
	ids []uint,
) (*SoftDeleteResult, error) {
	result := &SoftDeleteResult{
		RequestedIDs: append([]uint(nil), ids...),
		DeletableIDs: normalizeIDs(ids),
	}
	if len(result.DeletableIDs) == 0 {
		return result, nil
	}
	if err := repo.Delete(ctx, result.DeletableIDs); err != nil {
		return result, err
	}
	return result, nil
}

func normalizeIDs(ids []uint) []uint {
	if len(ids) == 0 {
		return nil
	}
	seen := make(map[uint]struct{}, len(ids))
	out := make([]uint, 0, len(ids))
	for _, id := range ids {
		if id == 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}
