package system

import (
	"context"

	aidomain "personal_assistant/internal/domain/ai"
	"personal_assistant/internal/service/system/aiselect"
)

type aiToolExecutionPlan = aiselect.ExecutionPlan

func (s *AIService) buildAIToolExecutionPlan(
	ctx context.Context,
	query string,
	history []aidomain.Message,
	visibleTools []aidomain.Tool,
	principal aidomain.AIToolPrincipal,
) (aiToolExecutionPlan, error) {
	if s == nil || s.toolPlanner == nil {
		return aiToolExecutionPlan{Tools: visibleTools}, nil
	}
	return s.toolPlanner.BuildExecutionPlan(ctx, query, history, visibleTools, principal)
}
