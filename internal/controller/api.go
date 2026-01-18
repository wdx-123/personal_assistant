package controller

import (
	"personal_assistant/internal/controller/system"
)

// ApiGroup 控制器
type ApiGroup struct {
	SystemApiGroup system.Supplier
}

var ApiGroupApp *ApiGroup
