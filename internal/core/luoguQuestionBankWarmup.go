package core

import (
	"context"
	"errors"
	"personal_assistant/global"
	"personal_assistant/internal/repository/interfaces"
	"personal_assistant/pkg/rediskey"
	"personal_assistant/pkg/redislock"
	"time"

	"go.uber.org/zap"
)

func StartLuoguQuestionBankWarmup(
	ctx context.Context,
	repo interfaces.LuoguQuestionBankRepository,
) {
	if ctx == nil {
		ctx = context.Background()
	}
	if repo == nil || global.Redis == nil || global.Log == nil || global.Config == nil {
		return
	}
	cfg := global.Config.Task
	if !cfg.LuoguQuestionBankWarmupEnabled {
		return
	}
	if cfg.LuoguQuestionBankWarmupBatchSize <= 0 || cfg.LuoguQuestionBankWarmupLockTTLSeconds <= 0 {
		return
	}

	go func() {
		lockTTL := time.Duration(cfg.LuoguQuestionBankWarmupLockTTLSeconds) * time.Second
		err := redislock.WithLock(ctx, redislock.LockKeyLuoguProblemBankWarmup, lockTTL, func() error {
			return warmupLuoguQuestionBank(ctx, repo, cfg.LuoguQuestionBankWarmupBatchSize)
		})
		if err != nil {
			if errors.Is(err, redislock.ErrLockFailed) {
				global.Log.Info("luogu question bank warmup skipped: lock is held")
				return
			}
			global.Log.Error("luogu question bank warmup failed", zap.Error(err))
		}
	}()
}

func warmupLuoguQuestionBank(
	ctx context.Context,
	repo interfaces.LuoguQuestionBankRepository,
	batchSize int,
) error {
	var lastID uint
	for {
		questions, err := repo.ListPIDIDAfterID(ctx, lastID, batchSize)
		if err != nil {
			return err
		}
		if len(questions) == 0 {
			return nil
		}

		pipe := global.Redis.Pipeline()
		for _, q := range questions {
			if q == nil || q.ID == 0 || q.Pid == "" {
				continue
			}
			pipe.HSet(ctx, rediskey.LuoguProblemBankHashKey, q.Pid, q.ID)
		}
		if _, err := pipe.Exec(ctx); err != nil {
			return err
		}
		lastID = questions[len(questions)-1].ID
	}
}
