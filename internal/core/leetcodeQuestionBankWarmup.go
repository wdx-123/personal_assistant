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

func StartLeetcodeQuestionBankWarmup(
	ctx context.Context,
	repo interfaces.LeetcodeQuestionBankRepository,
) {
	if ctx == nil {
		ctx = context.Background()
	}
	if repo == nil || global.Redis == nil || global.Log == nil || global.Config == nil {
		return
	}
	cfg := global.Config.Task
	if !cfg.LeetcodeQuestionBankWarmupEnabled {
		return
	}
	if cfg.LeetcodeQuestionBankWarmupBatchSize <= 0 || cfg.LeetcodeQuestionBankWarmupLockTTLSeconds <= 0 {
		return
	}

	go func() {
		lockTTL := time.Duration(cfg.LeetcodeQuestionBankWarmupLockTTLSeconds) * time.Second
		err := redislock.WithLock(ctx, redislock.LockKeyLeetcodeProblemBankWarmup, lockTTL, func() error {
			return warmupLeetcodeQuestionBank(ctx, repo, cfg.LeetcodeQuestionBankWarmupBatchSize)
		})
		if err != nil {
			if errors.Is(err, redislock.ErrLockFailed) {
				global.Log.Info("leetcode question bank warmup skipped: lock is held")
				return
			}
			global.Log.Error("leetcode question bank warmup failed", zap.Error(err))
		}
	}()
}

func warmupLeetcodeQuestionBank(
	ctx context.Context,
	repo interfaces.LeetcodeQuestionBankRepository,
	batchSize int,
) error {
	var lastID uint
	for {
		questions, err := repo.ListTitleSlugIDAfterID(ctx, lastID, batchSize)
		if err != nil {
			return err
		}
		if len(questions) == 0 {
			return nil
		}

		pipe := global.Redis.Pipeline()
		for _, q := range questions {
			if q == nil || q.ID == 0 || q.TitleSlug == "" {
				continue
			}
			pipe.HSet(ctx, rediskey.LeetcodeProblemBankHashKey, q.TitleSlug, q.ID)
		}
		if _, err := pipe.Exec(ctx); err != nil {
			return err
		}
		lastID = questions[len(questions)-1].ID
	}
}
