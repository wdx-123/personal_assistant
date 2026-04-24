package eino

import (
	aidomain "personal_assistant/internal/domain/ai"

	"github.com/cloudwego/eino/schema"
)

// buildSchemaToolInfo 把 domain 层 ToolSpec 转成 Eino 的 ToolInfo。
func buildSchemaToolInfo(spec aidomain.ToolSpec) (*schema.ToolInfo, error) {
	// 先把参数列表转成按名称索引的 schema 参数定义。
	params := make(map[string]*schema.ParameterInfo, len(spec.Parameters))
	for _, param := range spec.Parameters {
		info, err := buildSchemaParameterInfo(param)
		if err != nil {
			return nil, err
		}
		params[param.Name] = info
	}

	// ToolInfo 只承载 name、描述和参数协议，不包含任何业务实现。
	return &schema.ToolInfo{
		Name:        spec.Name,
		Desc:        spec.Description,
		ParamsOneOf: schema.NewParamsOneOfByParams(params),
	}, nil
}

// buildSchemaParameterInfo 递归把 domain 层参数定义转换成 Eino 参数协议。
func buildSchemaParameterInfo(param aidomain.ToolParameter) (*schema.ParameterInfo, error) {
	// 先填充当前参数节点的基础元信息。
	info := &schema.ParameterInfo{
		Type:     schema.DataType(param.Type),
		Desc:     param.Description,
		Enum:     param.Enum,
		Required: param.Required,
	}
	if param.Items != nil {
		// array 参数需要继续递归描述元素结构。
		itemInfo, err := buildSchemaParameterInfo(*param.Items)
		if err != nil {
			return nil, err
		}
		info.ElemInfo = itemInfo
	}
	if len(param.Properties) > 0 {
		// object 参数需要递归构建所有子字段定义。
		subParams := make(map[string]*schema.ParameterInfo, len(param.Properties))
		for _, child := range param.Properties {
			childInfo, err := buildSchemaParameterInfo(child)
			if err != nil {
				return nil, err
			}
			subParams[child.Name] = childInfo
		}
		info.SubParams = subParams
	}
	return info, nil
}
