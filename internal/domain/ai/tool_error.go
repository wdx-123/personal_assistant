package ai

import (
	"errors"
	"fmt"
)

// ToolObservationClassification 表示一次 tool 失败对 ReAct 的恢复语义。
type ToolObservationClassification string

const (
	// ToolObservationMissingUserInput 表示缺少继续执行所需的用户输入。
	ToolObservationMissingUserInput ToolObservationClassification = "missing_user_input"
	// ToolObservationRepairableInvalidParam 表示参数错误，但模型可在同一轮修正。
	ToolObservationRepairableInvalidParam ToolObservationClassification = "repairable_invalid_param"
	// ToolObservationTerminalToolError 表示错误不可恢复，应终止本轮 tool loop。
	ToolObservationTerminalToolError ToolObservationClassification = "terminal_tool_error"
)

// ToolFieldError 描述单个字段的错误详情，供模型自修正和前端排障使用。
type ToolFieldError struct {
	Field    string   `json:"field"`
	Reason   string   `json:"reason"`
	Expected string   `json:"expected,omitempty"`
	Allowed  []string `json:"allowed,omitempty"`
	Example  string   `json:"example,omitempty"`
}

// ToolObservation 表示 runtime 回喂给模型的结构化 tool observation。
type ToolObservation struct {
	Classification ToolObservationClassification `json:"classification"`
	ToolName       string                        `json:"tool_name"`
	Message        string                        `json:"message"`
	FieldErrors    []ToolFieldError              `json:"field_errors,omitempty"`
}

// ToolIssueError 表示一次带恢复语义的 tool 错误。
type ToolIssueError struct {
	Classification ToolObservationClassification
	Message        string
	FieldErrors    []ToolFieldError
	Cause          error
}

func (e *ToolIssueError) Error() string {
	if e == nil {
		return ""
	}
	if e.Cause != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Cause)
	}
	return e.Message
}

func (e *ToolIssueError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

// Observation 把当前错误转换为可回喂模型的结构化 observation。
func (e *ToolIssueError) Observation(toolName string) ToolObservation {
	if e == nil {
		return ToolObservation{ToolName: toolName}
	}
	return ToolObservation{
		Classification: e.Classification,
		ToolName:       toolName,
		Message:        e.Message,
		FieldErrors:    append([]ToolFieldError(nil), e.FieldErrors...),
	}
}

// NewToolIssueError 创建结构化 tool 错误。
func NewToolIssueError(
	classification ToolObservationClassification,
	message string,
	fieldErrors []ToolFieldError,
	cause error,
) *ToolIssueError {
	return &ToolIssueError{
		Classification: classification,
		Message:        message,
		FieldErrors:    append([]ToolFieldError(nil), fieldErrors...),
		Cause:          cause,
	}
}

// NewMissingUserInputError 创建“缺少用户输入”错误。
func NewMissingUserInputError(message string, fieldErrors ...ToolFieldError) *ToolIssueError {
	return NewToolIssueError(ToolObservationMissingUserInput, message, fieldErrors, nil)
}

// NewRepairableInvalidParamError 创建“参数可修复”错误。
func NewRepairableInvalidParamError(message string, fieldErrors ...ToolFieldError) *ToolIssueError {
	return NewToolIssueError(ToolObservationRepairableInvalidParam, message, fieldErrors, nil)
}

// NewRepairableInvalidParamErrorWithCause 创建“参数可修复”错误并携带原始 cause。
func NewRepairableInvalidParamErrorWithCause(
	message string,
	cause error,
	fieldErrors ...ToolFieldError,
) *ToolIssueError {
	return NewToolIssueError(ToolObservationRepairableInvalidParam, message, fieldErrors, cause)
}

// NewTerminalToolError 创建“不可恢复”错误。
func NewTerminalToolError(message string, cause error) *ToolIssueError {
	return NewToolIssueError(ToolObservationTerminalToolError, message, nil, cause)
}

// FromToolIssueError 从 error 链中提取 ToolIssueError。
func FromToolIssueError(err error) *ToolIssueError {
	if err == nil {
		return nil
	}
	var issue *ToolIssueError
	if errors.As(err, &issue) {
		return issue
	}
	return nil
}
