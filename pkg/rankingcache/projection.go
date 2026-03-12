package rankingcache

import (
	"strconv"
	"strings"

	readmodel "personal_assistant/internal/model/readmodel"
)

const (
	PlatformLuogu    = "luogu"
	PlatformLeetcode = "leetcode"
)

const (
	hashFieldUsername        = "username"
	hashFieldAvatar          = "avatar"
	hashFieldCurrentOrgID    = "current_org_id"
	hashFieldCurrentOrgName  = "current_org_name"
	hashFieldActive          = "active"
	hashFieldLuoguIdentifier = "luogu_identifier"
	hashFieldLuoguAvatar     = "luogu_avatar"
	hashFieldLuoguScore      = "luogu_score"
	hashFieldLeetcodeSlug    = "leetcode_identifier"
	hashFieldLeetcodeAvatar  = "leetcode_avatar"
	hashFieldLeetcodeScore   = "leetcode_score"
	hashActiveValueTrue      = "1"
	hashActiveValueFalse     = "0"
)

// UserProjection 是从数据库读模型转换而来的用户数据投影结构，包含排行榜相关的所有字段。
type PlatformProfile struct {
	Identifier string // 在平台上的唯一标识，例如用户名或 slug。
	Avatar     string // 平台头像 URL，可能与用户主头像不同。
	Score      int    //	在平台上的分数或排名指标，用于排行榜排序。
}

type UserProjection struct {
	UserID         uint
	Username       string
	Avatar         string
	CurrentOrgID   *uint  // 当前所在组织 ID，可能为 nil 表示未加入任何组织。
	CurrentOrgName string // 当前所在组织名称，供展示使用。
	Active         bool   // 用户是否活跃。
	Luogu          PlatformProfile
	Leetcode       PlatformProfile
}

// FromReadModel 从数据库读模型构建 UserProjection 对象，确保字段映射的正确性和完整性。
func FromReadModel(item *readmodel.Ranking) *UserProjection {
	if item == nil {
		return nil
	}
	return &UserProjection{
		UserID:         item.UserID,
		Username:       item.Username,
		Avatar:         item.Avatar,
		CurrentOrgID:   item.CurrentOrgID,
		CurrentOrgName: item.CurrentOrgName,
		Active:         item.IsActive(),
		Luogu: PlatformProfile{
			Identifier: item.LuoguIdentifier,
			Avatar:     item.LuoguAvatar,
			Score:      item.LuoguScore,
		},
		Leetcode: PlatformProfile{
			Identifier: item.LeetcodeIdentifier,
			Avatar:     item.LeetcodeAvatar,
			Score:      item.LeetcodeScore,
		},
	}
}

// HashValues 将 UserProjection 转换为适合 Redis hash 存储的字段-值映射，确保数据类型和格式的正确处理。
func (p *UserProjection) HashValues() map[string]interface{} {
	if p == nil {
		return nil
	}

	values := map[string]interface{}{
		hashFieldUsername:        p.Username,
		hashFieldAvatar:          p.Avatar,
		hashFieldCurrentOrgName:  p.CurrentOrgName,
		hashFieldLuoguIdentifier: p.Luogu.Identifier,
		hashFieldLuoguAvatar:     p.Luogu.Avatar,
		hashFieldLuoguScore:      p.Luogu.Score,
		hashFieldLeetcodeSlug:    p.Leetcode.Identifier,
		hashFieldLeetcodeAvatar:  p.Leetcode.Avatar,
		hashFieldLeetcodeScore:   p.Leetcode.Score,
	}
	if p.Active {
		values[hashFieldActive] = hashActiveValueTrue
	} else {
		values[hashFieldActive] = hashActiveValueFalse
	}
	if p.CurrentOrgID != nil && *p.CurrentOrgID > 0 {
		values[hashFieldCurrentOrgID] = strconv.FormatUint(uint64(*p.CurrentOrgID), 10)
	} else {
		values[hashFieldCurrentOrgID] = ""
	}
	return values
}

// ProjectionFromHash 从 Redis hash 中重建 UserProjection 对象，确保字段映射和类型转换的正确处理。
func ProjectionFromHash(userID uint, values map[string]string) (*UserProjection, bool) {
	if userID == 0 || len(values) == 0 {
		return nil, false
	}

	out := &UserProjection{
		UserID:         userID,
		Username:       strings.TrimSpace(values[hashFieldUsername]),
		Avatar:         strings.TrimSpace(values[hashFieldAvatar]),
		CurrentOrgName: strings.TrimSpace(values[hashFieldCurrentOrgName]),
		Active:         strings.TrimSpace(values[hashFieldActive]) == hashActiveValueTrue,
		Luogu: PlatformProfile{
			Identifier: strings.TrimSpace(values[hashFieldLuoguIdentifier]),
			Avatar:     strings.TrimSpace(values[hashFieldLuoguAvatar]),
			Score:      parseInt(values[hashFieldLuoguScore]),
		},
		Leetcode: PlatformProfile{
			Identifier: strings.TrimSpace(values[hashFieldLeetcodeSlug]),
			Avatar:     strings.TrimSpace(values[hashFieldLeetcodeAvatar]),
			Score:      parseInt(values[hashFieldLeetcodeScore]),
		},
	}

	// 解析 current_org_id 字段，确保正确处理空值和无效值。
	if rawOrgID := strings.TrimSpace(values[hashFieldCurrentOrgID]); rawOrgID != "" {
		if parsed, err := strconv.ParseUint(rawOrgID, 10, 64); err == nil && parsed > 0 {
			orgID := uint(parsed)
			out.CurrentOrgID = &orgID
		}
	}
	return out, true
}

// Platform 返回指定平台的用户资料，默认为洛谷。
func (p *UserProjection) Platform(platform string) PlatformProfile {
	switch NormalizePlatform(platform) {
	case PlatformLeetcode:
		return p.Leetcode
	default:
		return p.Luogu
	}
}

// NormalizePlatform 将输入的平台标识规范化为预定义的常量值，默认为洛谷。
func NormalizePlatform(platform string) string {
	switch strings.ToLower(strings.TrimSpace(platform)) {
	case PlatformLeetcode:
		return PlatformLeetcode
	default:
		return PlatformLuogu
	}
}

// parseInt 是一个辅助函数，用于将字符串解析为整数，解析失败时返回 0。
func parseInt(raw string) int {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0
	}
	out, err := strconv.Atoi(raw)
	if err != nil {
		return 0
	}
	return out
}
