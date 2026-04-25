package eino

import (
	"strings"
	"testing"

	aidomain "personal_assistant/internal/domain/ai"
)

func TestValidateToolCallArgumentsMissingRequiredField(t *testing.T) {
	spec := aidomain.ToolSpec{
		Name: "get_my_oj_stats",
		Parameters: []aidomain.ToolParameter{
			{
				Name:     "platform",
				Type:     aidomain.ToolParameterTypeString,
				Required: true,
				Enum:     []string{"leetcode", "luogu", "lanqiao"},
			},
		},
	}

	_, err := validateToolCallArguments(spec, `{}`)
	issue := aidomain.FromToolIssueError(err)
	if issue == nil {
		t.Fatalf("validateToolCallArguments() error = %v, want ToolIssueError", err)
	}
	if issue.Classification != aidomain.ToolObservationMissingUserInput {
		t.Fatalf("classification = %q, want %q", issue.Classification, aidomain.ToolObservationMissingUserInput)
	}
}

func TestValidateToolCallArgumentsRejectsInvalidEnum(t *testing.T) {
	spec := aidomain.ToolSpec{
		Name: "query_observability_metrics",
		Parameters: []aidomain.ToolParameter{
			{
				Name:     "granularity",
				Type:     aidomain.ToolParameterTypeString,
				Required: true,
				Enum:     []string{"1m", "5m", "1d", "1w"},
			},
		},
	}

	_, err := validateToolCallArguments(spec, `{"granularity":"1h"}`)
	issue := aidomain.FromToolIssueError(err)
	if issue == nil {
		t.Fatalf("validateToolCallArguments() error = %v, want ToolIssueError", err)
	}
	if issue.Classification != aidomain.ToolObservationRepairableInvalidParam {
		t.Fatalf("classification = %q, want %q", issue.Classification, aidomain.ToolObservationRepairableInvalidParam)
	}
}

func TestValidateToolCallArgumentsNormalizesRFC3339(t *testing.T) {
	spec := aidomain.ToolSpec{
		Name: "query_observability_metrics",
		Parameters: []aidomain.ToolParameter{
			{
				Name:     "start_at",
				Type:     aidomain.ToolParameterTypeString,
				Required: true,
				Format:   aidomain.ToolParameterFormatRFC3339,
			},
		},
	}

	validated, err := validateToolCallArguments(spec, `{"start_at":"2026-04-24T17:20:00+08:00"}`)
	if err != nil {
		t.Fatalf("validateToolCallArguments() error = %v", err)
	}
	if !strings.Contains(validated.NormalizedJSON, `"2026-04-24T09:20:00Z"`) {
		t.Fatalf("NormalizedJSON = %q, want UTC RFC3339", validated.NormalizedJSON)
	}
}

func TestValidateToolCallArgumentsValidatesNestedArrayObjects(t *testing.T) {
	spec := aidomain.ToolSpec{
		Name: "analyze_task_titles",
		Parameters: []aidomain.ToolParameter{
			{
				Name:     "items",
				Type:     aidomain.ToolParameterTypeArray,
				Required: true,
				MinItems: func() *int { v := 1; return &v }(),
				Items: &aidomain.ToolParameter{
					Type: aidomain.ToolParameterTypeObject,
					Properties: []aidomain.ToolParameter{
						{
							Name:     "platform",
							Type:     aidomain.ToolParameterTypeString,
							Required: true,
							Enum:     []string{"leetcode", "luogu", "lanqiao"},
						},
						{
							Name:      "title",
							Type:      aidomain.ToolParameterTypeString,
							Required:  true,
							MinLength: func() *int { v := 1; return &v }(),
							MaxLength: func() *int { v := 255; return &v }(),
						},
					},
				},
			},
		},
	}

	_, err := validateToolCallArguments(spec, `{"items":[{"platform":"codeforces","title":"A"}]}`)
	issue := aidomain.FromToolIssueError(err)
	if issue == nil {
		t.Fatalf("validateToolCallArguments() error = %v, want ToolIssueError", err)
	}
	if issue.Classification != aidomain.ToolObservationRepairableInvalidParam {
		t.Fatalf("classification = %q, want %q", issue.Classification, aidomain.ToolObservationRepairableInvalidParam)
	}
}
