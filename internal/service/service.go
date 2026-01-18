package service

import (
	"personal_assistant/internal/service/system"
)

type Group struct {
	SystemServiceSupplier system.Supplier
}

var GroupApp *Group
