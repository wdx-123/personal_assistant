package aitool

import (
	"strings"

	aidomain "personal_assistant/internal/domain/ai"
)

var toolGroupProfiles = map[aidomain.ToolGroupID]aidomain.ToolGroupProfile{
	aidomain.ToolGroupOJPersonal: {
		Summary:    "查询当前用户在 OJ 平台上的个人排名、统计和曲线。",
		WhenToUse:  "用户提问自己的刷题表现、排名、统计变化时使用。",
		DomainTags: []string{"oj", "personal"},
	},
	aidomain.ToolGroupOJOrg: {
		Summary:    "查询组织范围内的 OJ 排名汇总。",
		WhenToUse:  "用户提问当前组织或指定组织的排名汇总时使用。",
		DomainTags: []string{"oj", "org"},
	},
	aidomain.ToolGroupOJTask: {
		Summary:    "查询 OJ 任务、执行明细、用户执行明细和题目分析。",
		WhenToUse:  "用户提问 OJ 任务、作业执行、名单或题目分析时使用。",
		DomainTags: []string{"oj", "task"},
	},
	aidomain.ToolGroupObservabilityTrace: {
		Summary:    "查询链路追踪详情与追踪摘要。",
		WhenToUse:  "用户提问 trace、request 链路、失败调用排查时使用。",
		DomainTags: []string{"observability", "trace"},
	},
	aidomain.ToolGroupObservabilityMetrics: {
		Summary:    "查询运行时指标与 HTTP 观测指标。",
		WhenToUse:  "用户提问运行时指标、HTTP 指标趋势、状态码分布时使用。",
		DomainTags: []string{"observability", "metrics"},
	},
}

func newAIToolDescriptor(
	spec aidomain.ToolSpec,
	groupID aidomain.ToolGroupID,
	summary string,
	whenToUse string,
	tags ...string,
) aidomain.ToolDescriptor {
	return aidomain.ToolDescriptor{
		Spec:    spec,
		GroupID: groupID,
		Brief: aidomain.ToolBrief{
			Name:          spec.Name,
			Summary:       strings.TrimSpace(summary),
			WhenToUse:     strings.TrimSpace(whenToUse),
			RequiredSlots: requiredToolSlots(spec),
			DomainTags:    append([]string(nil), tags...),
		},
	}
}

func buildToolGroupBrief(groupID aidomain.ToolGroupID, visibleTools []aidomain.Tool) aidomain.ToolGroupBrief {
	profile, ok := toolGroupProfiles[groupID]
	if !ok {
		return aidomain.ToolGroupBrief{
			ID:        groupID,
			ToolNames: collectToolNames(visibleTools),
		}
	}
	return aidomain.ToolGroupBrief{
		ID:         groupID,
		Summary:    profile.Summary,
		WhenToUse:  profile.WhenToUse,
		ToolNames:  collectToolNames(visibleTools),
		DomainTags: append([]string(nil), profile.DomainTags...),
	}
}

func collectToolNames(tools []aidomain.Tool) []string {
	items := make([]string, 0, len(tools))
	for _, tool := range tools {
		if tool == nil {
			continue
		}
		name := strings.TrimSpace(tool.Spec().Name)
		if name == "" {
			continue
		}
		items = append(items, name)
	}
	return items
}

func requiredToolSlots(spec aidomain.ToolSpec) []string {
	items := make([]string, 0, len(spec.Parameters))
	for _, param := range spec.Parameters {
		if !param.Required {
			continue
		}
		items = append(items, param.Name)
	}
	return items
}

func truncateToolSummary(input string, limit int) string {
	if limit <= 0 {
		return ""
	}
	runes := []rune(strings.TrimSpace(input))
	if len(runes) <= limit {
		return string(runes)
	}
	return string(runes[:limit])
}
