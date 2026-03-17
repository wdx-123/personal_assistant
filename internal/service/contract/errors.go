package contract

import "errors"

var (
	ErrInvalidPlatform   = errors.New("invalid platform")
	ErrInvalidIdentifier = errors.New("invalid identifier")
	ErrBindCoolDown      = errors.New("bind operation is in cooldown")
	ErrOJAccountNotBound = errors.New("oj account not bound")
	ErrInvalidCredential = errors.New("invalid credential")
	ErrOJSyncDisabled    = errors.New("oj sync disabled")
)
