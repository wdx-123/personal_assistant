package eino

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	aidomain "personal_assistant/internal/domain/ai"
	bizerrors "personal_assistant/pkg/errors"
)

const maxRepairAttemptsPerTurn = 2

type runtimeValidatingTool interface {
	Validate(ctx context.Context, call aidomain.ToolCall, callCtx aidomain.ToolCallContext) error
}

type toolRepairState struct {
	attempts int
	seen     map[string]int
}

func newToolRepairState() *toolRepairState {
	return &toolRepairState{
		seen: make(map[string]int),
	}
}

func (s *toolRepairState) consume(key string) bool {
	if s == nil {
		return false
	}
	s.attempts++
	s.seen[key]++
	return s.attempts <= maxRepairAttemptsPerTurn
}

func classifyToolError(err error) *aidomain.ToolIssueError {
	if issue := aidomain.FromToolIssueError(err); issue != nil {
		return issue
	}

	var bizErr *bizerrors.BizError
	if errors.As(err, &bizErr) && bizErr != nil {
		switch bizErr.Code {
		case bizerrors.CodeInvalidParams, bizerrors.CodeBindFailed, bizerrors.CodeValidateFailed:
			message := strings.TrimSpace(bizErr.Message)
			fieldErrors := inferFieldErrorsFromMessage(message)
			if seemsMissingUserInput(message) {
				return aidomain.NewMissingUserInputError(message, fieldErrors...)
			}
			return aidomain.NewRepairableInvalidParamError(message, fieldErrors...)
		case bizerrors.CodePermissionDenied, bizerrors.CodeDBError, bizerrors.CodeInternalError, bizerrors.CodeThirdPartyError:
			return aidomain.NewTerminalToolError(strings.TrimSpace(bizErr.Message), err)
		}
	}

	return aidomain.NewTerminalToolError("工具调用失败。", err)
}

func seemsMissingUserInput(message string) bool {
	message = strings.TrimSpace(message)
	if message == "" {
		return false
	}
	needles := []string{
		"不能为空",
		"缺失",
		"未提供",
		"必须指定",
		"需要提供",
	}
	for _, needle := range needles {
		if strings.Contains(message, needle) {
			return true
		}
	}
	return false
}

func inferFieldErrorsFromMessage(message string) []aidomain.ToolFieldError {
	message = strings.TrimSpace(message)
	if message == "" {
		return nil
	}
	field := inferFieldName(message)
	if field == "" {
		return nil
	}

	reason := "invalid_param"
	switch {
	case strings.Contains(message, "不能为空"), strings.Contains(message, "缺失"), strings.Contains(message, "未提供"), strings.Contains(message, "必须指定"):
		reason = "missing_required"
	case strings.Contains(message, "仅支持"):
		reason = "invalid_enum"
	case strings.Contains(message, "RFC3339"), strings.Contains(message, "格式"):
		reason = "invalid_format"
	case strings.Contains(message, "必须大于"), strings.Contains(message, "不能超过"):
		reason = "invalid_range"
	}

	return []aidomain.ToolFieldError{
		{
			Field:    field,
			Reason:   reason,
			Expected: message,
		},
	}
}

func inferFieldName(message string) string {
	candidates := []string{
		"platform",
		"scope",
		"org_id",
		"page",
		"page_size",
		"task_id",
		"execution_id",
		"target_user_id",
		"username",
		"request_id",
		"trace_id",
		"root_stage",
		"status",
		"metric",
		"granularity",
		"start_at",
		"end_at",
		"limit",
		"offset",
		"status_class",
		"service",
		"route_template",
		"method",
		"error_code",
	}
	for _, candidate := range candidates {
		if strings.Contains(message, candidate) {
			return candidate
		}
	}
	return ""
}

func marshalToolObservation(observation aidomain.ToolObservation) (compact string, pretty string) {
	raw, err := json.Marshal(observation)
	if err != nil {
		return observation.Message, observation.Message
	}
	compact = string(raw)

	indented, err := json.MarshalIndent(observation, "", "  ")
	if err != nil {
		return compact, compact
	}
	return compact, "```json\n" + string(indented) + "\n```"
}
