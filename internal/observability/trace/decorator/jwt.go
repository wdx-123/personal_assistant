package decorator

import (
	"context"
	stdErrors "errors"

	resp "personal_assistant/internal/model/dto/response"
	"personal_assistant/internal/model/entity"
	"personal_assistant/internal/service/contract"
	erro "personal_assistant/pkg/errors"
	"personal_assistant/pkg/observability/servicetrace"
)

type tracedJWTService struct {
	next contract.JWTServiceContract
}

func WrapJWTService(next contract.JWTServiceContract) contract.JWTServiceContract {
	if next == nil {
		return nil
	}
	return &tracedJWTService{next: next}
}

func (t *tracedJWTService) IssueLoginTokens(
	ctx context.Context,
	user entity.User,
) (*resp.LoginResponse, string, int64, *erro.JWTError) {
	opt := newTraceOptions("jwt", "IssueLoginTokens")
	spanCtx, spanEvent := servicetrace.Start(ctx, opt)
	out, refreshToken, refreshExpiresAt, jwtErr := t.next.IssueLoginTokens(spanCtx, user)
	if jwtErr != nil {
		err := jwtErr.Err
		if err == nil {
			err = stdErrors.New(jwtErr.Message)
		}
		servicetrace.Finish(spanCtx, opt, spanEvent, err)
		return out, refreshToken, refreshExpiresAt, jwtErr
	}
	servicetrace.Finish(spanCtx, opt, spanEvent, nil)
	return out, refreshToken, refreshExpiresAt, nil
}

func (t *tracedJWTService) IsInBlacklist(jwt string) bool {
	return t.next.IsInBlacklist(jwt)
}

func (t *tracedJWTService) GetAccessToken(
	ctx context.Context,
	token string,
) (*resp.RefreshTokenResponse, *erro.JWTError) {
	opt := newTraceOptions("jwt", "GetAccessToken")
	spanCtx, spanEvent := servicetrace.Start(ctx, opt)
	out, jwtErr := t.next.GetAccessToken(spanCtx, token)
	if jwtErr != nil {
		err := jwtErr.Err
		if err == nil {
			err = stdErrors.New(jwtErr.Message)
		}
		servicetrace.Finish(spanCtx, opt, spanEvent, err)
		return out, jwtErr
	}
	servicetrace.Finish(spanCtx, opt, spanEvent, nil)
	return out, nil
}

func (t *tracedJWTService) JoinInBlacklist(ctx context.Context, jwtList entity.JwtBlacklist) error {
	return runTracedErr(ctx, "jwt", "JoinInBlacklist", func(inner context.Context) error {
		return t.next.JoinInBlacklist(inner, jwtList)
	})
}

var _ contract.JWTServiceContract = (*tracedJWTService)(nil)
