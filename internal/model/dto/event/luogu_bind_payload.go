package event

import lg "personal_assistant/internal/infrastructure/luogu"

type LuoguBindPayload struct {
	Passed []lg.PassedProblem `json:"passed"`
}
