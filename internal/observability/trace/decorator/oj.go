package decorator

import (
	"context"

	eventdto "personal_assistant/internal/model/dto/event"
	"personal_assistant/internal/model/dto/request"
	resp "personal_assistant/internal/model/dto/response"
	"personal_assistant/internal/service/contract"
)

type tracedOJService struct {
	next contract.OJServiceContract
}

func WrapOJService(next contract.OJServiceContract) contract.OJServiceContract {
	if next == nil {
		return nil
	}
	return &tracedOJService{next: next}
}

func (t *tracedOJService) BindOJAccount(
	ctx context.Context,
	userID uint,
	req *request.BindOJAccountReq,
) (*resp.BindOJAccountResp, error) {
	return runTraced(ctx, "oj", "BindOJAccount", func(inner context.Context) (*resp.BindOJAccountResp, error) {
		return t.next.BindOJAccount(inner, userID, req)
	})
}

func (t *tracedOJService) GetRankingList(
	ctx context.Context,
	userID uint,
	req *request.OJRankingListReq,
) (*resp.OJRankingListResp, error) {
	return runTraced(ctx, "oj", "GetRankingList", func(inner context.Context) (*resp.OJRankingListResp, error) {
		return t.next.GetRankingList(inner, userID, req)
	})
}

func (t *tracedOJService) GetUserStats(
	ctx context.Context,
	userID uint,
	req *request.OJStatsReq,
) (*resp.BindOJAccountResp, error) {
	return runTraced(ctx, "oj", "GetUserStats", func(inner context.Context) (*resp.BindOJAccountResp, error) {
		return t.next.GetUserStats(inner, userID, req)
	})
}

func (t *tracedOJService) SyncAllLuoguUsers(ctx context.Context) error {
	return runTracedErr(ctx, "oj", "SyncAllLuoguUsers", t.next.SyncAllLuoguUsers)
}

func (t *tracedOJService) SyncAllLeetcodeUsers(ctx context.Context) error {
	return runTracedErr(ctx, "oj", "SyncAllLeetcodeUsers", t.next.SyncAllLeetcodeUsers)
}

func (t *tracedOJService) RebuildRankingCaches(ctx context.Context) error {
	return runTracedErr(ctx, "oj", "RebuildRankingCaches", t.next.RebuildRankingCaches)
}

func (t *tracedOJService) HandleLuoguBindPayload(
	ctx context.Context,
	userID uint,
	payload *eventdto.LuoguBindPayload,
) error {
	return runTracedErr(ctx, "oj", "HandleLuoguBindPayload", func(inner context.Context) error {
		return t.next.HandleLuoguBindPayload(inner, userID, payload)
	})
}

func (t *tracedOJService) HandleLeetcodeBindSignal(ctx context.Context, userID uint) error {
	return runTracedErr(ctx, "oj", "HandleLeetcodeBindSignal", func(inner context.Context) error {
		return t.next.HandleLeetcodeBindSignal(inner, userID)
	})
}

var _ contract.OJServiceContract = (*tracedOJService)(nil)
