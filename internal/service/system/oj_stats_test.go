package system

import (
	"context"
	"testing"
	"time"

	"personal_assistant/global"
	"personal_assistant/internal/model/config"
	"personal_assistant/internal/model/dto/request"
	"personal_assistant/internal/model/entity"
	"personal_assistant/internal/repository/interfaces"
	svccontract "personal_assistant/internal/service/contract"
	"personal_assistant/pkg/rediskey"

	"github.com/alicebob/miniredis/v2"
	"github.com/go-redis/redis/v8"
	"go.uber.org/zap"
)

func TestBuildLanqiaoSyncAlertReturnsNilBelowThreshold(t *testing.T) {
	setupOJStatsRedis(t, 3)
	if err := global.Redis.Set(context.Background(), rediskey.LanqiaoSyncFailKey(1), "2", time.Hour).Err(); err != nil {
		t.Fatalf("seed fail count error = %v", err)
	}

	alert := (&OJService{}).buildLanqiaoSyncAlert(context.Background(), 1)
	if alert != nil {
		t.Fatalf("buildLanqiaoSyncAlert() = %+v, want nil", alert)
	}
}

func TestBuildLanqiaoSyncAlertReturnsAlertAtThreshold(t *testing.T) {
	setupOJStatsRedis(t, 3)
	if err := global.Redis.Set(context.Background(), rediskey.LanqiaoSyncFailKey(1), "3", time.Hour).Err(); err != nil {
		t.Fatalf("seed fail count error = %v", err)
	}

	alert := (&OJService{}).buildLanqiaoSyncAlert(context.Background(), 1)
	if alert == nil {
		t.Fatal("buildLanqiaoSyncAlert() = nil, want alert")
	}
	if !alert.Danger || alert.Reason != lanqiaoSyncAlertReasonSyncDisabled || !alert.SyncDisabled {
		t.Fatalf("unexpected alert = %+v", alert)
	}
	if alert.FailureThreshold != 3 {
		t.Fatalf("FailureThreshold = %d, want 3", alert.FailureThreshold)
	}
}

func TestBuildLanqiaoSyncAlertReturnsAlertWhenDisabled(t *testing.T) {
	setupOJStatsRedis(t, 3)
	if err := global.Redis.Set(context.Background(), rediskey.LanqiaoSyncDisableKey(1), "1", time.Hour).Err(); err != nil {
		t.Fatalf("seed disable flag error = %v", err)
	}

	alert := (&OJService{}).buildLanqiaoSyncAlert(context.Background(), 1)
	if alert == nil {
		t.Fatal("buildLanqiaoSyncAlert() = nil, want alert")
	}
	if alert.Reason != lanqiaoSyncAlertReasonSyncDisabled || !alert.SyncDisabled {
		t.Fatalf("unexpected alert = %+v", alert)
	}
}

func TestBuildLanqiaoSyncAlertReturnsStatusCheckFailedOnRedisError(t *testing.T) {
	srv := setupOJStatsRedis(t, 3)
	srv.Close()

	alert := (&OJService{}).buildLanqiaoSyncAlert(context.Background(), 1)
	if alert == nil {
		t.Fatal("buildLanqiaoSyncAlert() = nil, want alert")
	}
	if alert.Reason != lanqiaoSyncAlertReasonStatusCheckFailed {
		t.Fatalf("Reason = %q, want %q", alert.Reason, lanqiaoSyncAlertReasonStatusCheckFailed)
	}
	if alert.SyncDisabled {
		t.Fatalf("SyncDisabled = %t, want false", alert.SyncDisabled)
	}
}

func TestGetUserStatsLanqiaoReturnsStatsWithoutAlert(t *testing.T) {
	setupOJStatsRedis(t, 3)
	svc := &OJService{
		lanqiaoRepo: &stubOJStatsLanqiaoDetailRepository{
			detail: &entity.LanqiaoUserDetail{
				MODEL:              entity.MODEL{ID: 9},
				MaskedPhone:        "138****8000",
				SubmitSuccessCount: 11,
				SubmitFailedCount:  2,
				UserID:             1,
			},
		},
		lanqiaoUserQuestionRepo: &stubOJStatsLanqiaoQuestionRepository{
			passedCount: 7,
		},
	}

	out, err := svc.GetUserStats(context.Background(), 1, &request.OJStatsReq{Platform: "lanqiao"})
	if err != nil {
		t.Fatalf("GetUserStats() error = %v", err)
	}
	if out.Platform != "lanqiao" || out.Identifier != "138****8000" || out.PassedNumber != 7 {
		t.Fatalf("unexpected stats response = %+v", out)
	}
	if out.LanqiaoSyncAlert != nil {
		t.Fatalf("LanqiaoSyncAlert = %+v, want nil", out.LanqiaoSyncAlert)
	}
}

func TestGetUserStatsLanqiaoReturnsAlert(t *testing.T) {
	setupOJStatsRedis(t, 3)
	if err := global.Redis.Set(context.Background(), rediskey.LanqiaoSyncFailKey(1), "3", time.Hour).Err(); err != nil {
		t.Fatalf("seed fail count error = %v", err)
	}

	svc := &OJService{
		lanqiaoRepo: &stubOJStatsLanqiaoDetailRepository{
			detail: &entity.LanqiaoUserDetail{
				MODEL:              entity.MODEL{ID: 9},
				MaskedPhone:        "138****8000",
				SubmitSuccessCount: 11,
				SubmitFailedCount:  2,
				UserID:             1,
			},
		},
		lanqiaoUserQuestionRepo: &stubOJStatsLanqiaoQuestionRepository{
			passedCount: 7,
		},
	}

	out, err := svc.GetUserStats(context.Background(), 1, &request.OJStatsReq{Platform: "lanqiao"})
	if err != nil {
		t.Fatalf("GetUserStats() error = %v", err)
	}
	if out.LanqiaoSyncAlert == nil {
		t.Fatal("LanqiaoSyncAlert = nil, want alert")
	}
	if out.LanqiaoSyncAlert.Reason != lanqiaoSyncAlertReasonSyncDisabled {
		t.Fatalf("Reason = %q, want %q", out.LanqiaoSyncAlert.Reason, lanqiaoSyncAlertReasonSyncDisabled)
	}
}

func TestGetUserStatsLanqiaoReturnsNotBound(t *testing.T) {
	setupOJStatsRedis(t, 3)
	svc := &OJService{
		lanqiaoRepo: &stubOJStatsLanqiaoDetailRepository{},
		lanqiaoUserQuestionRepo: &stubOJStatsLanqiaoQuestionRepository{
			passedCount: 7,
		},
	}

	_, err := svc.GetUserStats(context.Background(), 1, &request.OJStatsReq{Platform: "lanqiao"})
	if err != svccontract.ErrOJAccountNotBound {
		t.Fatalf("GetUserStats() error = %v, want %v", err, svccontract.ErrOJAccountNotBound)
	}
}

func setupOJStatsRedis(t *testing.T, threshold int) *miniredis.Miniredis {
	t.Helper()

	srv, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis.Run() error = %v", err)
	}
	client := redis.NewClient(&redis.Options{Addr: srv.Addr()})

	oldRedis := global.Redis
	oldConfig := global.Config
	oldLog := global.Log
	global.Redis = client
	global.Config = &config.Config{
		Task: config.Task{
			LanqiaoFailureThreshold: threshold,
		},
	}
	global.Log = zap.NewNop()

	t.Cleanup(func() {
		global.Redis = oldRedis
		global.Config = oldConfig
		global.Log = oldLog
		if err := client.Close(); err != nil {
			t.Errorf("client.Close() error = %v", err)
		}
		srv.Close()
	})

	return srv
}

type stubOJStatsLanqiaoDetailRepository struct {
	interfaces.LanqiaoUserDetailRepository
	detail *entity.LanqiaoUserDetail
	err    error
}

func (r *stubOJStatsLanqiaoDetailRepository) GetByUserID(_ context.Context, _ uint) (*entity.LanqiaoUserDetail, error) {
	return r.detail, r.err
}

type stubOJStatsLanqiaoQuestionRepository struct {
	interfaces.LanqiaoUserQuestionRepository
	passedCount int64
	err         error
}

func (r *stubOJStatsLanqiaoQuestionRepository) CountPassed(_ context.Context, _ uint) (int64, error) {
	return r.passedCount, r.err
}
