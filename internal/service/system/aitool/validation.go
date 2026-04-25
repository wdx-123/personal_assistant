package aitool

import (
	"fmt"
	"strings"
	"time"

	aidomain "personal_assistant/internal/domain/ai"
)

func aiIntPtr(value int) *int {
	return &value
}

func aiFloatPtr(value float64) *float64 {
	return &value
}

func aiFirstExample(param aidomain.ToolParameter) string {
	if len(param.Examples) == 0 {
		return ""
	}
	return param.Examples[0]
}

func aiMissingFieldError(param aidomain.ToolParameter, field string) aidomain.ToolFieldError {
	return aidomain.ToolFieldError{
		Field:    field,
		Reason:   "missing_required",
		Expected: aiExpectedSummary(param),
		Allowed:  append([]string(nil), param.Enum...),
		Example:  aiFirstExample(param),
	}
}

func aiInvalidFieldError(
	field string,
	reason string,
	expected string,
	allowed []string,
	example string,
) aidomain.ToolFieldError {
	return aidomain.ToolFieldError{
		Field:    field,
		Reason:   reason,
		Expected: expected,
		Allowed:  append([]string(nil), allowed...),
		Example:  example,
	}
}

func aiExpectedSummary(param aidomain.ToolParameter) string {
	parts := make([]string, 0, 6)
	parts = append(parts, string(param.Type))
	if strings.TrimSpace(param.Format) != "" {
		parts = append(parts, "format="+strings.TrimSpace(param.Format))
	}
	if len(param.Enum) > 0 {
		parts = append(parts, "enum="+strings.Join(param.Enum, "/"))
	}
	if param.Minimum != nil {
		parts = append(parts, "min="+trimFloatForPrompt(*param.Minimum))
	}
	if param.Maximum != nil {
		parts = append(parts, "max="+trimFloatForPrompt(*param.Maximum))
	}
	if param.MinLength != nil {
		parts = append(parts, fmt.Sprintf("min_length=%d", *param.MinLength))
	}
	if param.MaxLength != nil {
		parts = append(parts, fmt.Sprintf("max_length=%d", *param.MaxLength))
	}
	if param.MinItems != nil {
		parts = append(parts, fmt.Sprintf("min_items=%d", *param.MinItems))
	}
	if param.MaxItems != nil {
		parts = append(parts, fmt.Sprintf("max_items=%d", *param.MaxItems))
	}
	return strings.Join(parts, ", ")
}

func trimFloatForPrompt(value float64) string {
	if float64(int64(value)) == value {
		return fmt.Sprintf("%d", int64(value))
	}
	return fmt.Sprintf("%g", value)
}

func aiParseRFC3339Field(field string, raw string, required bool, example string) (time.Time, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		if required {
			return time.Time{}, aidomain.NewMissingUserInputError(
				field+" 缺失，无法继续执行。",
				aidomain.ToolFieldError{
					Field:    field,
					Reason:   "missing_required",
					Expected: "RFC3339 时间",
					Example:  example,
				},
			)
		}
		return time.Time{}, nil
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}, aidomain.NewRepairableInvalidParamErrorWithCause(
			field+" 必须是 RFC3339 时间。",
			err,
			aidomain.ToolFieldError{
				Field:    field,
				Reason:   "invalid_format",
				Expected: "RFC3339 时间",
				Example:  example,
			},
		)
	}
	return parsed.UTC(), nil
}

func aiValidateOptionalTimeRange(
	startField string,
	startRaw string,
	endField string,
	endRaw string,
) error {
	start, err := aiParseRFC3339Field(startField, startRaw, false, "2026-04-24T09:20:00Z")
	if err != nil {
		return err
	}
	end, err := aiParseRFC3339Field(endField, endRaw, false, "2026-04-24T10:20:00Z")
	if err != nil {
		return err
	}
	if !start.IsZero() && !end.IsZero() && !end.After(start) {
		return aidomain.NewRepairableInvalidParamError(
			endField+" 必须大于 "+startField+"。",
			aidomain.ToolFieldError{
				Field:    endField,
				Reason:   "invalid_range",
				Expected: endField + " > " + startField,
				Example:  "2026-04-24T10:20:00Z",
			},
		)
	}
	return nil
}

func aiValidateRequiredTimeRange(startField string, startRaw string, endField string, endRaw string) (time.Time, time.Time, error) {
	start, err := aiParseRFC3339Field(startField, startRaw, true, "2026-04-24T09:20:00Z")
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	end, err := aiParseRFC3339Field(endField, endRaw, true, "2026-04-24T10:20:00Z")
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	if !end.After(start) {
		return time.Time{}, time.Time{}, aidomain.NewRepairableInvalidParamError(
			endField+" 必须大于 "+startField+"。",
			aidomain.ToolFieldError{
				Field:    endField,
				Reason:   "invalid_range",
				Expected: endField + " > " + startField,
				Example:  "2026-04-24T10:20:00Z",
			},
		)
	}
	return start, end, nil
}

func aiValidateObservabilityMetricsArgs(args aiObservabilityMetricsArgs) error {
	_, _, err := aiValidateRequiredTimeRange("start_at", args.StartAt, "end_at", args.EndAt)
	if err != nil {
		return err
	}
	if args.StatusClass != 0 && (args.StatusClass < 1 || args.StatusClass > 5) {
		return aidomain.NewRepairableInvalidParamError(
			"status_class 仅支持 1~5 的状态码段。",
			aidomain.ToolFieldError{
				Field:    "status_class",
				Reason:   "out_of_range",
				Expected: "1~5 之间的整数",
				Example:  "2",
			},
		)
	}
	return nil
}

func aiValidateRuntimeMetricsArgs(args aiRuntimeMetricsArgs) error {
	metric := strings.TrimSpace(args.Metric)
	switch metric {
	case "task_execution_total":
		if _, _, err := aiValidateRequiredTimeRange("start_at", args.StartAt, "end_at", args.EndAt); err != nil {
			return err
		}
		return aiValidateRuntimeStatus(strings.TrimSpace(args.Status), []string{"success", "error", "skipped"})
	case "task_duration_seconds":
		start, end, err := aiValidateRequiredTimeRange("start_at", args.StartAt, "end_at", args.EndAt)
		if err != nil {
			return err
		}
		if end.Sub(start) > 7*24*time.Hour {
			return aidomain.NewRepairableInvalidParamError(
				"duration 查询时间范围不能超过 7 天。",
				aidomain.ToolFieldError{
					Field:    "end_at",
					Reason:   "invalid_range",
					Expected: "与 start_at 的间隔不超过 7 天",
					Example:  "2026-04-24T10:20:00Z",
				},
			)
		}
		return aiValidateRuntimeStatus(strings.TrimSpace(args.Status), []string{"success", "error", "skipped"})
	case "outbox_publish_duration_seconds", "event_consume_duration_seconds":
		start, end, err := aiValidateRequiredTimeRange("start_at", args.StartAt, "end_at", args.EndAt)
		if err != nil {
			return err
		}
		if end.Sub(start) > 7*24*time.Hour {
			return aidomain.NewRepairableInvalidParamError(
				"duration 查询时间范围不能超过 7 天。",
				aidomain.ToolFieldError{
					Field:    "end_at",
					Reason:   "invalid_range",
					Expected: "与 start_at 的间隔不超过 7 天",
					Example:  "2026-04-24T10:20:00Z",
				},
			)
		}
		return aiValidateRuntimeStatus(strings.TrimSpace(args.Status), []string{"success", "error"})
	case "event_consume_total":
		if _, _, err := aiValidateRequiredTimeRange("start_at", args.StartAt, "end_at", args.EndAt); err != nil {
			return err
		}
		return aiValidateRuntimeStatus(strings.TrimSpace(args.Status), []string{"success", "error"})
	case "outbox_events_total":
		status := strings.TrimSpace(strings.ToLower(args.Status))
		if err := aiValidateRuntimeStatus(status, []string{"pending", "published", "failed"}); err != nil {
			return err
		}
		if status == "" || status == "pending" {
			return nil
		}
		_, _, err := aiValidateRequiredTimeRange("start_at", args.StartAt, "end_at", args.EndAt)
		return err
	default:
		return aidomain.NewRepairableInvalidParamError(
			"metric 取值不合法。",
			aidomain.ToolFieldError{
				Field:    "metric",
				Reason:   "invalid_enum",
				Expected: "task_execution_total/task_duration_seconds/outbox_events_total/outbox_publish_duration_seconds/event_consume_total/event_consume_duration_seconds",
				Allowed: []string{
					"task_execution_total",
					"task_duration_seconds",
					"outbox_events_total",
					"outbox_publish_duration_seconds",
					"event_consume_total",
					"event_consume_duration_seconds",
				},
				Example: "task_execution_total",
			},
		)
	}
}

func aiValidateRuntimeStatus(status string, allowed []string) error {
	status = strings.TrimSpace(strings.ToLower(status))
	if status == "" {
		return nil
	}
	for _, item := range allowed {
		if status == item {
			return nil
		}
	}
	return aidomain.NewRepairableInvalidParamError(
		"status 取值不合法。",
		aidomain.ToolFieldError{
			Field:    "status",
			Reason:   "invalid_enum",
			Expected: "仅支持指定状态枚举",
			Allowed:  allowed,
			Example:  allowed[0],
		},
	)
}

func aiValidateTraceSummaryArgs(args aiTraceSummaryArgs) error {
	return aiValidateOptionalTimeRange("start_at", args.StartAt, "end_at", args.EndAt)
}
