package eino

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	aidomain "personal_assistant/internal/domain/ai"
)

type validatedToolArguments struct {
	NormalizedJSON string
}

func validateToolCallArguments(spec aidomain.ToolSpec, raw string) (validatedToolArguments, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		raw = "{}"
	}

	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.UseNumber()

	var payload any
	if err := decoder.Decode(&payload); err != nil {
		return validatedToolArguments{NormalizedJSON: raw}, aidomain.NewRepairableInvalidParamErrorWithCause(
			"工具参数不是合法的 JSON 对象。",
			err,
			aidomain.ToolFieldError{
				Field:    "arguments",
				Reason:   "invalid_json",
				Expected: "合法的 JSON 对象",
				Example:  `{"platform":"leetcode"}`,
			},
		)
	}

	root, ok := payload.(map[string]any)
	if !ok {
		return validatedToolArguments{NormalizedJSON: raw}, aidomain.NewRepairableInvalidParamError(
			"工具参数必须是 JSON 对象。",
			aidomain.ToolFieldError{
				Field:    "arguments",
				Reason:   "invalid_type",
				Expected: "JSON object",
				Example:  `{"platform":"leetcode"}`,
			},
		)
	}

	normalizedRoot := make(map[string]any, len(spec.Parameters))
	fieldErrors := make([]aidomain.ToolFieldError, 0)
	hasMissing := false
	for _, param := range spec.Parameters {
		value, exists := root[param.Name]
		normalized, keep, errs, missing := validateToolParameter(param, value, exists, param.Name)
		if keep {
			normalizedRoot[param.Name] = normalized
		}
		fieldErrors = append(fieldErrors, errs...)
		hasMissing = hasMissing || missing
	}

	normalizedJSON := marshalNormalizedToolArgs(normalizedRoot, raw)
	if len(fieldErrors) == 0 {
		return validatedToolArguments{NormalizedJSON: normalizedJSON}, nil
	}

	if hasMissing {
		return validatedToolArguments{NormalizedJSON: normalizedJSON}, aidomain.NewMissingUserInputError(
			"缺少继续执行所需的信息，请先向用户追问。",
			fieldErrors...,
		)
	}
	return validatedToolArguments{NormalizedJSON: normalizedJSON}, aidomain.NewRepairableInvalidParamError(
		"工具参数不合法，请修正后重试。",
		fieldErrors...,
	)
}

func marshalNormalizedToolArgs(payload map[string]any, fallback string) string {
	raw, err := json.Marshal(payload)
	if err != nil {
		return fallback
	}
	return string(raw)
}

func validateToolParameter(
	param aidomain.ToolParameter,
	value any,
	exists bool,
	fieldPath string,
) (normalized any, keep bool, fieldErrors []aidomain.ToolFieldError, missing bool) {
	if !exists || value == nil {
		if param.Required {
			return nil, false, []aidomain.ToolFieldError{toolMissingFieldError(param, fieldPath)}, true
		}
		return nil, false, nil, false
	}

	switch param.Type {
	case aidomain.ToolParameterTypeString:
		return validateStringParameter(param, value, fieldPath)
	case aidomain.ToolParameterTypeInteger:
		return validateIntegerParameter(param, value, fieldPath)
	case aidomain.ToolParameterTypeNumber:
		return validateNumberParameter(param, value, fieldPath)
	case aidomain.ToolParameterTypeBoolean:
		boolValue, ok := value.(bool)
		if !ok {
			return value, true, []aidomain.ToolFieldError{toolInvalidFieldError(
				fieldPath,
				"invalid_type",
				"boolean",
				nil,
				firstExample(param),
			)}, false
		}
		return boolValue, true, nil, false
	case aidomain.ToolParameterTypeArray:
		return validateArrayParameter(param, value, fieldPath)
	case aidomain.ToolParameterTypeObject:
		return validateObjectParameter(param, value, fieldPath)
	default:
		return value, true, []aidomain.ToolFieldError{toolInvalidFieldError(
			fieldPath,
			"unsupported_type",
			string(param.Type),
			nil,
			firstExample(param),
		)}, false
	}
}

func validateStringParameter(
	param aidomain.ToolParameter,
	value any,
	fieldPath string,
) (normalized any, keep bool, fieldErrors []aidomain.ToolFieldError, missing bool) {
	stringValue, ok := value.(string)
	if !ok {
		return value, true, []aidomain.ToolFieldError{toolInvalidFieldError(
			fieldPath,
			"invalid_type",
			toolExpectedSummary(param),
			param.Enum,
			firstExample(param),
		)}, false
	}

	stringValue = strings.TrimSpace(stringValue)
	if stringValue == "" {
		if param.Required {
			return nil, false, []aidomain.ToolFieldError{toolMissingFieldError(param, fieldPath)}, true
		}
		return nil, false, nil, false
	}

	if shouldNormalizeLowercase(param) {
		stringValue = strings.ToLower(stringValue)
	}

	fieldErrors = make([]aidomain.ToolFieldError, 0)
	if len(param.Enum) > 0 && !containsString(param.Enum, stringValue) {
		fieldErrors = append(fieldErrors, toolInvalidFieldError(
			fieldPath,
			"invalid_enum",
			toolExpectedSummary(param),
			param.Enum,
			firstExample(param),
		))
	}
	if strings.TrimSpace(param.Format) == aidomain.ToolParameterFormatRFC3339 {
		parsed, err := time.Parse(time.RFC3339, stringValue)
		if err != nil {
			fieldErrors = append(fieldErrors, toolInvalidFieldError(
				fieldPath,
				"invalid_format",
				toolExpectedSummary(param),
				param.Enum,
				firstExample(param),
			))
		} else {
			stringValue = parsed.UTC().Format(time.RFC3339)
		}
	}
	if strings.TrimSpace(param.Pattern) != "" {
		matched, err := regexp.MatchString(strings.TrimSpace(param.Pattern), stringValue)
		if err != nil || !matched {
			fieldErrors = append(fieldErrors, toolInvalidFieldError(
				fieldPath,
				"invalid_pattern",
				toolExpectedSummary(param),
				param.Enum,
				firstExample(param),
			))
		}
	}
	if param.MinLength != nil && len([]rune(stringValue)) < *param.MinLength {
		fieldErrors = append(fieldErrors, toolInvalidFieldError(
			fieldPath,
			"min_length",
			toolExpectedSummary(param),
			param.Enum,
			firstExample(param),
		))
	}
	if param.MaxLength != nil && len([]rune(stringValue)) > *param.MaxLength {
		fieldErrors = append(fieldErrors, toolInvalidFieldError(
			fieldPath,
			"max_length",
			toolExpectedSummary(param),
			param.Enum,
			firstExample(param),
		))
	}
	return stringValue, true, fieldErrors, false
}

func validateIntegerParameter(
	param aidomain.ToolParameter,
	value any,
	fieldPath string,
) (normalized any, keep bool, fieldErrors []aidomain.ToolFieldError, missing bool) {
	integerValue, ok := parseIntegerValue(value)
	if !ok {
		return value, true, []aidomain.ToolFieldError{toolInvalidFieldError(
			fieldPath,
			"invalid_type",
			toolExpectedSummary(param),
			nil,
			firstExample(param),
		)}, false
	}
	fieldErrors = validateNumericBounds(param, float64(integerValue), fieldPath)
	return integerValue, true, fieldErrors, false
}

func validateNumberParameter(
	param aidomain.ToolParameter,
	value any,
	fieldPath string,
) (normalized any, keep bool, fieldErrors []aidomain.ToolFieldError, missing bool) {
	numberValue, ok := parseNumberValue(value)
	if !ok {
		return value, true, []aidomain.ToolFieldError{toolInvalidFieldError(
			fieldPath,
			"invalid_type",
			toolExpectedSummary(param),
			nil,
			firstExample(param),
		)}, false
	}
	fieldErrors = validateNumericBounds(param, numberValue, fieldPath)
	return numberValue, true, fieldErrors, false
}

func validateNumericBounds(param aidomain.ToolParameter, value float64, fieldPath string) []aidomain.ToolFieldError {
	fieldErrors := make([]aidomain.ToolFieldError, 0)
	if param.Minimum != nil && value < *param.Minimum {
		fieldErrors = append(fieldErrors, toolInvalidFieldError(
			fieldPath,
			"out_of_range",
			toolExpectedSummary(param),
			nil,
			firstExample(param),
		))
	}
	if param.Maximum != nil && value > *param.Maximum {
		fieldErrors = append(fieldErrors, toolInvalidFieldError(
			fieldPath,
			"out_of_range",
			toolExpectedSummary(param),
			nil,
			firstExample(param),
		))
	}
	return fieldErrors
}

func validateArrayParameter(
	param aidomain.ToolParameter,
	value any,
	fieldPath string,
) (normalized any, keep bool, fieldErrors []aidomain.ToolFieldError, missing bool) {
	items, ok := value.([]any)
	if !ok {
		return value, true, []aidomain.ToolFieldError{toolInvalidFieldError(
			fieldPath,
			"invalid_type",
			toolExpectedSummary(param),
			nil,
			firstExample(param),
		)}, false
	}
	if len(items) == 0 && param.Required {
		return []any{}, true, []aidomain.ToolFieldError{toolMissingFieldError(param, fieldPath)}, true
	}
	fieldErrors = make([]aidomain.ToolFieldError, 0)
	if param.MinItems != nil && len(items) < *param.MinItems {
		fieldErrors = append(fieldErrors, toolInvalidFieldError(
			fieldPath,
			"min_items",
			toolExpectedSummary(param),
			nil,
			firstExample(param),
		))
	}
	if param.MaxItems != nil && len(items) > *param.MaxItems {
		fieldErrors = append(fieldErrors, toolInvalidFieldError(
			fieldPath,
			"max_items",
			toolExpectedSummary(param),
			nil,
			firstExample(param),
		))
	}

	normalizedItems := make([]any, 0, len(items))
	if param.Items == nil {
		return items, true, fieldErrors, false
	}
	for idx, item := range items {
		childPath := fmt.Sprintf("%s[%d]", fieldPath, idx)
		childNormalized, childKeep, childErrors, childMissing := validateToolParameter(*param.Items, item, true, childPath)
		if childKeep {
			normalizedItems = append(normalizedItems, childNormalized)
		}
		fieldErrors = append(fieldErrors, childErrors...)
		missing = missing || childMissing
	}
	return normalizedItems, true, fieldErrors, missing
}

func validateObjectParameter(
	param aidomain.ToolParameter,
	value any,
	fieldPath string,
) (normalized any, keep bool, fieldErrors []aidomain.ToolFieldError, missing bool) {
	objectValue, ok := value.(map[string]any)
	if !ok {
		return value, true, []aidomain.ToolFieldError{toolInvalidFieldError(
			fieldPath,
			"invalid_type",
			toolExpectedSummary(param),
			nil,
			firstExample(param),
		)}, false
	}
	normalizedObject := make(map[string]any, len(param.Properties))
	fieldErrors = make([]aidomain.ToolFieldError, 0)
	for _, child := range param.Properties {
		childPath := child.Name
		if fieldPath != "" {
			childPath = fieldPath + "." + child.Name
		}
		childValue, exists := objectValue[child.Name]
		childNormalized, childKeep, childErrors, childMissing := validateToolParameter(child, childValue, exists, childPath)
		if childKeep {
			normalizedObject[child.Name] = childNormalized
		}
		fieldErrors = append(fieldErrors, childErrors...)
		missing = missing || childMissing
	}
	return normalizedObject, true, fieldErrors, missing
}

func parseIntegerValue(value any) (int64, bool) {
	switch typed := value.(type) {
	case json.Number:
		integerValue, err := typed.Int64()
		return integerValue, err == nil
	case float64:
		if float64(int64(typed)) != typed {
			return 0, false
		}
		return int64(typed), true
	case int:
		return int64(typed), true
	case int64:
		return typed, true
	default:
		return 0, false
	}
}

func parseNumberValue(value any) (float64, bool) {
	switch typed := value.(type) {
	case json.Number:
		floatValue, err := typed.Float64()
		return floatValue, err == nil
	case float64:
		return typed, true
	case int:
		return float64(typed), true
	case int64:
		return float64(typed), true
	default:
		return 0, false
	}
}

func toolMissingFieldError(param aidomain.ToolParameter, fieldPath string) aidomain.ToolFieldError {
	return aidomain.ToolFieldError{
		Field:    fieldPath,
		Reason:   "missing_required",
		Expected: toolExpectedSummary(param),
		Allowed:  append([]string(nil), param.Enum...),
		Example:  firstExample(param),
	}
}

func toolInvalidFieldError(fieldPath, reason, expected string, allowed []string, example string) aidomain.ToolFieldError {
	return aidomain.ToolFieldError{
		Field:    fieldPath,
		Reason:   reason,
		Expected: expected,
		Allowed:  append([]string(nil), allowed...),
		Example:  example,
	}
}

func toolExpectedSummary(param aidomain.ToolParameter) string {
	parts := []string{string(param.Type)}
	if strings.TrimSpace(param.Format) != "" {
		parts = append(parts, "format="+strings.TrimSpace(param.Format))
	}
	if len(param.Enum) > 0 {
		parts = append(parts, "enum="+strings.Join(param.Enum, "/"))
	}
	if param.Minimum != nil {
		parts = append(parts, "min="+formatConstraintNumber(*param.Minimum))
	}
	if param.Maximum != nil {
		parts = append(parts, "max="+formatConstraintNumber(*param.Maximum))
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

func formatConstraintNumber(value float64) string {
	if float64(int64(value)) == value {
		return fmt.Sprintf("%d", int64(value))
	}
	return fmt.Sprintf("%g", value)
}

func firstExample(param aidomain.ToolParameter) string {
	if len(param.Examples) == 0 {
		return ""
	}
	return param.Examples[0]
}

func shouldNormalizeLowercase(param aidomain.ToolParameter) bool {
	if len(param.Enum) == 0 {
		return false
	}
	for _, item := range param.Enum {
		trimmed := strings.TrimSpace(item)
		if trimmed == "" || strings.ToLower(trimmed) != trimmed {
			return false
		}
	}
	return true
}

func containsString(values []string, target string) bool {
	for _, item := range values {
		if target == strings.TrimSpace(item) {
			return true
		}
	}
	return false
}
