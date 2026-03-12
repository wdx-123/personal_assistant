package system

import (
	"context"
	"errors"
	"strings"

	"personal_assistant/global"
	eventdto "personal_assistant/internal/model/dto/event"
	"personal_assistant/internal/repository"
	"personal_assistant/internal/repository/interfaces"
	"personal_assistant/pkg/rankingcache"
)

// CacheProjectionService 负责消费缓存投影事件，并把数据库中的最新用户快照投影到 Redis。
type CacheProjectionService struct {
	// readModelRepo 提供排行榜投影所需的只读聚合视图查询。
	readModelRepo interfaces.RankingReadModelRepository
	// userRepo 负责同步用户活跃态缓存，供鉴权和快速状态判断复用。
	userRepo interfaces.UserRepository
}

// NewCacheProjectionService 创建缓存投影服务，并注入只读聚合查询与用户仓储依赖。
func NewCacheProjectionService(repositoryGroup *repository.Group) *CacheProjectionService {
	return &CacheProjectionService{
		// 排行榜缓存需要的是聚合后的读模型，而不是在 Service 中手动拼多表数据。
		readModelRepo: repositoryGroup.SystemRepositorySupplier.GetRankingReadModelRepository(),
		// 用户活跃态缓存继续通过 UserRepository 统一维护，避免 Redis 写入细节散落在多个 Service。
		userRepo: repositoryGroup.SystemRepositorySupplier.GetUserRepository(),
	}
}

// HandleCacheProjectionEvent 根据事件内容重建或删除单个用户的排行榜投影缓存。
func (s *CacheProjectionService) HandleCacheProjectionEvent(
	ctx context.Context,
	event *eventdto.CacheProjectionEvent,
) error {
	// 事件为 nil 代表上游调用不合法，直接返回错误给订阅链路处理。
	if event == nil {
		return errors.New("nil cache projection event")
	}
	// 用户 ID 为空说明事件无效；Redis 未初始化时则退化为 no-op，避免启动阶段误报。
	if event.UserID == 0 || global.Redis == nil {
		return nil
	}

	// removeOrgIDs 收集需要先从组织榜单中移除的组织 ID，防止用户迁移组织后残留旧排名。
	removeOrgIDs := make([]uint, 0, len(event.AffectedOrgIDs)+1)
	// 先纳入上游明确传入的受影响组织列表，例如退出组织、踢人等场景。
	removeOrgIDs = append(removeOrgIDs, event.AffectedOrgIDs...)
	// 再补上旧 current_org，确保 current_org 变更时旧组织榜单中的成员记录会被清理。
	if event.OldCurrentOrgID != nil && *event.OldCurrentOrgID > 0 {
		removeOrgIDs = append(removeOrgIDs, *event.OldCurrentOrgID)
	}

	// 删除事件不需要再回源读取读模型，只需清理 Redis 投影并同步活跃态缓存为 false。
	if strings.TrimSpace(event.Kind) == eventdto.CacheProjectionKindUserDeleted {
		// 删除用户详情 hash 和各个平台排行榜中的成员记录。
		if err := rankingcache.DeleteProjection(ctx, global.Redis, event.UserID, removeOrgIDs); err != nil {
			return err
		}
		// 用户已被删除，活跃态缓存必须同步失效，避免后续请求继续视为有效账号。
		return s.userRepo.CacheActiveState(ctx, event.UserID, false)
	}

	// 对于非删除事件，统一回源查询当前最新读模型，避免依赖事件中携带的过时快照。
	item, err := s.readModelRepo.GetByUserID(ctx, event.UserID)
	if err != nil {
		return err
	}
	// 读模型不存在通常表示用户已被删除或已不再满足投影条件，此时执行幂等清理。
	if item == nil {
		// 先清理用户详情 hash 和旧排行榜成员，保证缓存最终与数据库状态一致。
		if err := rankingcache.DeleteProjection(ctx, global.Redis, event.UserID, removeOrgIDs); err != nil {
			return err
		}
		// 同步用户活跃态缓存为 false，避免读取侧继续命中旧状态。
		return s.userRepo.CacheActiveState(ctx, event.UserID, false)
	}

	// 读模型转换为 Redis 投影结构，统一封装字段映射与活跃态计算逻辑。
	projection := rankingcache.FromReadModel(item)
	// 先写入用户详情 hash，供排行榜列表和详情回填共用。
	if err := rankingcache.WriteProjection(ctx, global.Redis, projection); err != nil {
		return err
	}
	// 再同步全站榜单与组织榜单，过程中会移除旧组织残留并按最新状态重建排名。
	if err := rankingcache.SyncProjectionRanks(ctx, global.Redis, projection, removeOrgIDs); err != nil {
		return err
	}
	// 最后回写用户活跃态缓存，使鉴权链路和排行榜读路径共享同一份状态结果。
	return s.userRepo.CacheActiveState(ctx, event.UserID, projection.Active)
}

// RebuildAll 全量重建排行榜相关缓存，通常用于手动修复或批量同步后的兜底重建。
func (s *CacheProjectionService) RebuildAll(ctx context.Context) error {
	// Redis 不可用时直接跳过，避免在降级场景下阻断调用链路。
	if global.Redis == nil {
		return nil
	}

	// 先清理用户活跃态缓存，防止旧状态与全量重建后的排行榜数据不一致。
	if err := s.deleteByPattern(ctx, "user:active_state:*"); err != nil {
		return err
	}
	// 清理用户详情 hash，确保后续写入的是最新全量快照。
	if err := s.deleteByPattern(ctx, "ranking:user:*"); err != nil {
		return err
	}
	// 清理组织维度排行榜，避免组织迁移、退组等历史数据残留。
	if err := s.deleteByPattern(ctx, "ranking:org:*"); err != nil {
		return err
	}
	// 清理全站排行榜，为重新写入各平台分数做准备。
	if err := s.deleteByPattern(ctx, "ranking:all_members:*"); err != nil {
		return err
	}

	// 回源查询全部可投影用户的聚合读模型，作为本次全量重建的数据源。
	items, err := s.readModelRepo.ListAll(ctx)
	if err != nil {
		return err
	}
	// 预分配切片容量，减少全量转换时的额外扩容成本。
	projections := make([]*rankingcache.UserProjection, 0, len(items))
	for _, item := range items {
		// 统一通过转换函数生成 Redis 投影，避免字段映射规则在多个位置重复实现。
		projection := rankingcache.FromReadModel(item)
		// 理论上空读模型不会生成有效投影，这里保持防御式判断以确保重建流程稳定。
		if projection == nil {
			continue
		}
		// 收集所有有效投影，供后续批量写入详情 hash。
		projections = append(projections, projection)
	}

	// 先批量写入用户详情 hash，降低逐条往返 Redis 的网络开销。
	if err := rankingcache.WriteProjections(ctx, global.Redis, projections); err != nil {
		return err
	}
	for _, projection := range projections {
		// 防御式判断，避免未来调整转换逻辑后引入空指针风险。
		if projection == nil {
			continue
		}

		// 逐个同步排行榜 zset，确保每个平台和组织维度都按最新快照重建。
		if err := rankingcache.SyncProjectionRanks(ctx, global.Redis, projection, nil); err != nil {
			return err
		}

		// 同步用户活跃态缓存，保证重建后所有读取入口看到的都是一致状态。
		if err := s.userRepo.CacheActiveState(ctx, projection.UserID, projection.Active); err != nil {
			return err
		}
	}
	// 全量重建全部完成，返回 nil 表示缓存与数据库状态已重新对齐。
	return nil
}

// deleteByPattern 使用 SCAN 按批删除匹配的 Redis key，避免 KEYS 带来的阻塞风险。
func (s *CacheProjectionService) deleteByPattern(ctx context.Context, pattern string) error {
	// Redis 未初始化时直接跳过，让调用方可以在降级场景下安全复用该方法。
	if global.Redis == nil {
		return nil
	}

	// cursor 记录 SCAN 游标，直到回到 0 表示遍历完成。
	var cursor uint64
	for {
		// 每轮最多扫描 200 个 key，在删除效率和 Redis 压力之间取一个保守平衡。
		keys, nextCursor, err := global.Redis.Scan(ctx, cursor, pattern, 200).Result()
		if err != nil {
			return err
		}
		// 只有扫描到命中的 key 才执行删除，避免空批次产生无意义写操作。
		if len(keys) > 0 {
			// 批量删除当前页 key，确保同类缓存会被完整清理。
			if err := global.Redis.Del(ctx, keys...).Err(); err != nil {
				return err
			}
		}
		// 推进到下一轮游标，继续遍历剩余 key。
		cursor = nextCursor
		// 游标归零表示整轮扫描结束，可以安全退出。
		if cursor == 0 {
			return nil
		}
	}
}
