package rankingcache

import (
	"context"
	"strconv"

	"personal_assistant/pkg/rediskey"

	"github.com/go-redis/redis/v8"
)

func WriteProjection(
	ctx context.Context,
	client redis.Cmdable,
	projection *UserProjection,
) error {
	if client == nil || projection == nil || projection.UserID == 0 {
		return nil
	}
	return client.HSet(
		ctx,
		rediskey.RankingUserHashKey(projection.UserID),
		projection.HashValues(),
	).Err()
}

// WriteProjections 批量写入多个用户的排行榜投影数据，确保输入数据的有效性和 Redis 操作的效率。
func WriteProjections(
	ctx context.Context,
	client redis.Cmdable,
	projections []*UserProjection,
) error {
	if client == nil || len(projections) == 0 {
		return nil
	}
	pipe := client.Pipeline()
	for _, item := range projections {
		if item == nil || item.UserID == 0 {
			continue
		}
		pipe.HSet(ctx, rediskey.RankingUserHashKey(item.UserID), item.HashValues())
	}
	_, err := pipe.Exec(ctx)
	return err
}

// SyncProjectionRanks 同步用户在全站排行榜和组织排行榜中的排名
func SyncProjectionRanks(
	ctx context.Context,
	client redis.Cmdable,
	projection *UserProjection,
	orgIDsToRemove []uint,
) error {
	// projection 为空或用户 ID 无效时直接返回，避免执行无意义的 Redis 操作。
	if client == nil || projection == nil || projection.UserID == 0 {
		return nil
	}

	// 将用户 ID 转换为字符串形式，作为 Redis 中 zset 的成员值。
	member := strconv.FormatUint(uint64(projection.UserID), 10)
	// 计算需要从排行榜中移除的组织 ID 列表，确保当前组织 ID（如果存在）也被包含在内。
	removeOrgIDs := dedupeOrgIDs(orgIDsToRemove, projection.CurrentOrgID)
	pipe := client.Pipeline()
	// 先从全站排行榜和相关组织排行榜中移除用户，确保旧排名被清除。
	for _, platform := range []string{PlatformLuogu, PlatformLeetcode, PlatformLanqiao} {
		profile := projection.Platform(platform)
		pipe.ZRem(ctx, rediskey.RankingAllMembersZSetKey(platform), member)
		for _, orgID := range removeOrgIDs {
			pipe.ZRem(ctx, rediskey.RankingOrgZSetKey(orgID, platform), member)
		}
		if !projection.Active || profile.Identifier == "" {
			continue
		}

		score := float64(profile.Score)
		pipe.ZAdd(ctx, rediskey.RankingAllMembersZSetKey(platform), &redis.Z{
			Score:  score,
			Member: member,
		})
		if projection.CurrentOrgID != nil && *projection.CurrentOrgID > 0 {
			pipe.ZAdd(ctx, rediskey.RankingOrgZSetKey(*projection.CurrentOrgID, platform), &redis.Z{
				Score:  score,
				Member: member,
			})
		}
	}

	_, err := pipe.Exec(ctx)
	return err
}

// DeleteProjection 从排行榜相关的 Redis 结构中删除指定用户的所有数据，确保用户 ID 和相关组织 ID 的正确处理。
func DeleteProjection(
	ctx context.Context,
	client redis.Cmdable,
	userID uint,
	orgIDs []uint,
) error {
	if client == nil || userID == 0 {
		return nil
	}
	// 将用户 ID 转换为字符串形式，作为 Redis 中 zset 的成员值。
	member := strconv.FormatUint(uint64(userID), 10)
	pipe := client.Pipeline()
	// 删除用户详情 hash，确保用户的详细信息不再可用。
	pipe.Del(ctx, rediskey.RankingUserHashKey(userID))
	// 从全站排行榜和相关组织排行榜中移除用户，确保用户不再出现在任何排行榜中。
	for _, platform := range []string{PlatformLuogu, PlatformLeetcode, PlatformLanqiao} {
		pipe.ZRem(ctx, rediskey.RankingAllMembersZSetKey(platform), member)
		for _, orgID := range dedupeOrgIDs(orgIDs, nil) {
			pipe.ZRem(ctx, rediskey.RankingOrgZSetKey(orgID, platform), member)
		}
	}
	_, err := pipe.Exec(ctx)
	return err
}

// dedupeOrgIDs 对组织 ID 列表进行去重，并确保当前组织 ID（如果存在）被包含在结果中。
func dedupeOrgIDs(orgIDs []uint, currentOrgID *uint) []uint {
	index := make(map[uint]struct{}, len(orgIDs)+1)
	result := make([]uint, 0, len(orgIDs)+1)
	for _, orgID := range orgIDs {
		if orgID == 0 {
			continue
		}
		if _, ok := index[orgID]; ok {
			continue
		}
		index[orgID] = struct{}{}
		result = append(result, orgID)
	}
	if currentOrgID != nil && *currentOrgID > 0 {
		if _, ok := index[*currentOrgID]; !ok {
			result = append(result, *currentOrgID)
		}
	}
	return result
}
