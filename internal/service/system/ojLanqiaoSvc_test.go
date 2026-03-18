package system

import (
	"context"
	stderrors "errors"
	"strings"
	"testing"
	"time"

	"personal_assistant/global"
	lq "personal_assistant/internal/infrastructure/lanqiao"
	bizerrors "personal_assistant/pkg/errors"
	"personal_assistant/pkg/rediskey"

	"github.com/go-redis/redis/v8"
)

func TestNormalizeLanqiaoPhone(t *testing.T) {
	t.Parallel()

	got, err := normalizeLanqiaoPhone("+86 138-0000-0000")
	if err != nil {
		t.Fatalf("normalizeLanqiaoPhone() error = %v", err)
	}
	if got != "13800000000" {
		t.Fatalf("normalizeLanqiaoPhone() = %q, want %q", got, "13800000000")
	}

	if _, err := normalizeLanqiaoPhone("abc"); err == nil {
		t.Fatal("normalizeLanqiaoPhone() error = nil, want invalid input error")
	}
}

func TestExtractLanqiaoSubmitStatsUsesStatsPayload(t *testing.T) {
	t.Parallel()

	out := &lq.SolveStatsResponse{}
	out.Data.Stats = &lq.SolveStatsStats{
		TotalPassed: 5,
		TotalFailed: 2,
	}
	out.Data.Problems = []lq.SolveStatsProblem{
		{ProblemID: 1, IsPassed: true},
		{ProblemID: 2, IsPassed: true},
		{ProblemID: 3, IsPassed: true},
	}

	successCount, failedCount := extractLanqiaoSubmitStats(out)
	if successCount != 5 || failedCount != 2 {
		t.Fatalf("extractLanqiaoSubmitStats() = (%d,%d), want (5,2)", successCount, failedCount)
	}
}

func TestWrapLanqiaoRemoteErrorClassifiesCredentialInvalid(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		err  error
	}{
		{
			name: "status401",
			err: &lq.RemoteHTTPError{
				StatusCode: 401,
				Message:    "unauthorized",
			},
		},
		{
			name: "status403",
			err: &lq.RemoteHTTPError{
				StatusCode: 403,
				Message:    "forbidden",
			},
		},
		{
			name: "login400",
			err: &lq.RemoteHTTPError{
				StatusCode: 400,
				Message:    "lanqiao solve_stats: [login] Lanqiao login HTTP 400",
			},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := (&OJService{}).wrapLanqiaoRemoteError(tc.err)
			bizErr := bizerrors.FromError(err)
			if bizErr == nil {
				t.Fatalf("wrapLanqiaoRemoteError() returned non-BizError: %T", err)
			}
			if bizErr.Code != bizerrors.CodeOJCredentialInvalid {
				t.Fatalf("Code = %d, want %d", bizErr.Code, bizerrors.CodeOJCredentialInvalid)
			}
			if bizErr.Message != lanqiaoCredentialInvalidBindMessage {
				t.Fatalf("Message = %q, want %q", bizErr.Message, lanqiaoCredentialInvalidBindMessage)
			}
		})
	}
}

func TestWrapLanqiaoRemoteErrorKeepsGenericSyncFailureForOtherRemoteErrors(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		err  error
	}{
		{
			name: "generic400",
			err: &lq.RemoteHTTPError{
				StatusCode: 400,
				Message:    "bad request",
			},
		},
		{
			name: "status502",
			err: &lq.RemoteHTTPError{
				StatusCode: 502,
				Message:    "upstream timeout",
			},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := (&OJService{}).wrapLanqiaoRemoteError(tc.err)
			bizErr := bizerrors.FromError(err)
			if bizErr == nil {
				t.Fatalf("wrapLanqiaoRemoteError() returned non-BizError: %T", err)
			}
			if bizErr.Code != bizerrors.CodeOJSyncFailed {
				t.Fatalf("Code = %d, want %d", bizErr.Code, bizerrors.CodeOJSyncFailed)
			}
			if bizErr.Message != bizerrors.CodeOJSyncFailed.Message() {
				t.Fatalf("Message = %q, want %q", bizErr.Message, bizerrors.CodeOJSyncFailed.Message())
			}
		})
	}
}

func TestWrapLanqiaoRemoteErrorDoesNotExposeRemoteLoginMessage(t *testing.T) {
	t.Parallel()

	err := (&OJService{}).wrapLanqiaoRemoteError(&lq.RemoteHTTPError{
		StatusCode: 400,
		Message:    "lanqiao solve_stats: [login] Lanqiao login HTTP 400",
	})
	bizErr := bizerrors.FromError(err)
	if bizErr == nil {
		t.Fatalf("wrapLanqiaoRemoteError() returned non-BizError: %T", err)
	}
	if strings.Contains(strings.ToLower(bizErr.Message), "login http 400") {
		t.Fatalf("Message leaked remote login detail: %q", bizErr.Message)
	}
}

func TestHandleLanqiaoSyncFailureDisablesImmediatelyForCredentialInvalid(t *testing.T) {
	setupOJStatsRedis(t, 3)

	ctx := context.Background()
	userID := uint(7)
	if err := global.Redis.Set(ctx, rediskey.LanqiaoSyncFailKey(userID), "2", time.Hour).Err(); err != nil {
		t.Fatalf("seed fail count error = %v", err)
	}

	(&OJService{}).handleLanqiaoSyncFailure(ctx, userID, &lq.RemoteHTTPError{
		StatusCode: 400,
		Message:    "lanqiao solve_stats: [login] Lanqiao login HTTP 400",
	})

	reason, err := global.Redis.Get(ctx, rediskey.LanqiaoSyncDisableKey(userID)).Result()
	if err != nil {
		t.Fatalf("disable reason error = %v", err)
	}
	if reason != lanqiaoSyncAlertReasonCredentialInvalid {
		t.Fatalf("disable reason = %q, want %q", reason, lanqiaoSyncAlertReasonCredentialInvalid)
	}
	_, err = global.Redis.Get(ctx, rediskey.LanqiaoSyncFailKey(userID)).Result()
	if !stderrors.Is(err, redis.Nil) {
		t.Fatalf("fail counter err = %v, want redis.Nil", err)
	}
}

func TestHandleLanqiaoSyncFailureUsesThresholdForGenericErrors(t *testing.T) {
	setupOJStatsRedis(t, 3)

	ctx := context.Background()
	userID := uint(9)
	svc := &OJService{}
	syncErr := &lq.RemoteHTTPError{
		StatusCode: 502,
		Message:    "upstream timeout",
	}

	for attempt := 1; attempt <= 2; attempt++ {
		svc.handleLanqiaoSyncFailure(ctx, userID, syncErr)

		failCount, err := global.Redis.Get(ctx, rediskey.LanqiaoSyncFailKey(userID)).Int()
		if err != nil {
			t.Fatalf("attempt %d fail count error = %v", attempt, err)
		}
		if failCount != attempt {
			t.Fatalf("attempt %d fail count = %d, want %d", attempt, failCount, attempt)
		}
		_, err = global.Redis.Get(ctx, rediskey.LanqiaoSyncDisableKey(userID)).Result()
		if !stderrors.Is(err, redis.Nil) {
			t.Fatalf("attempt %d disable reason err = %v, want redis.Nil", attempt, err)
		}
	}

	svc.handleLanqiaoSyncFailure(ctx, userID, syncErr)

	reason, err := global.Redis.Get(ctx, rediskey.LanqiaoSyncDisableKey(userID)).Result()
	if err != nil {
		t.Fatalf("disable reason error = %v", err)
	}
	if reason != lanqiaoSyncAlertReasonSyncDisabled {
		t.Fatalf("disable reason = %q, want %q", reason, lanqiaoSyncAlertReasonSyncDisabled)
	}
	_, err = global.Redis.Get(ctx, rediskey.LanqiaoSyncFailKey(userID)).Result()
	if !stderrors.Is(err, redis.Nil) {
		t.Fatalf("fail counter err = %v, want redis.Nil", err)
	}
}
