package service

import (
	"personal_assistant/internal/service/contract"
)

type Group struct {
	SystemServiceSupplier contract.Supplier
}

var GroupApp *Group
