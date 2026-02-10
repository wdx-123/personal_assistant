package system

import (
	"context"
	"errors"
	"personal_assistant/global"
	"personal_assistant/internal/model/entity"
	"personal_assistant/internal/repository/interfaces"
	"personal_assistant/pkg/rediskey"
	"strconv"

	"github.com/go-redis/redis/v8"
	"go.uber.org/zap"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type leetcodeQuestionBankRepository struct {
	db *gorm.DB
}

func NewLeetcodeQuestionBankRepository(db *gorm.DB) interfaces.LeetcodeQuestionBankRepository {
	return &leetcodeQuestionBankRepository{db: db}
}

func (r *leetcodeQuestionBankRepository) GetAllTitleSlugMap(
	ctx context.Context,
) (map[string]uint, error) {
	var questions []entity.LeetcodeQuestionBank
	err := r.db.WithContext(ctx).Select("id", "title_slug").Find(&questions).Error
	if err != nil {
		return nil, err
	}

	slugMap := make(map[string]uint, len(questions))
	for _, q := range questions {
		slugMap[q.TitleSlug] = q.ID
	}
	return slugMap, nil
}

func (r *leetcodeQuestionBankRepository) BatchCreate(
	ctx context.Context,
	questions []*entity.LeetcodeQuestionBank,
) error {
	if len(questions) == 0 {
		return nil
	}
	if err := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "title_slug"}},
			DoNothing: true,
		}).
		CreateInBatches(questions, 100).Error; err != nil {
		return err
	}

	go func() {
		bgCtx := context.Background()
		pipeline := global.Redis.Pipeline()
		key := rediskey.LeetcodeProblemBankHashKey

		for _, q := range questions {
			if q.ID != 0 && q.TitleSlug != "" {
				pipeline.HSet(bgCtx, key, q.TitleSlug, q.ID)
			}
		}

		if _, err := pipeline.Exec(bgCtx); err != nil {
			global.Log.Error("failed to sync leetcode problems to redis", zap.Error(err))
		}
	}()

	return nil
}

func (r *leetcodeQuestionBankRepository) GetByTitleSlug(
	ctx context.Context,
	titleSlug string,
) (*entity.LeetcodeQuestionBank, error) {
	var q entity.LeetcodeQuestionBank
	err := r.db.WithContext(ctx).Where("title_slug = ?", titleSlug).First(&q).Error
	return &q, err
}

func (r *leetcodeQuestionBankRepository) GetCachedID(
	ctx context.Context,
	titleSlug string,
) (uint, bool, error) {
	if titleSlug == "" {
		return 0, false, nil
	}
	val, err := global.Redis.HGet(ctx, rediskey.LeetcodeProblemBankHashKey, titleSlug).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return 0, false, nil
		}
		return 0, false, err
	}
	id, err := strconv.Atoi(val)
	if err != nil || id <= 0 {
		return 0, false, nil
	}
	return uint(id), true, nil
}

func (r *leetcodeQuestionBankRepository) CacheID(
	ctx context.Context,
	titleSlug string,
	id uint,
) error {
	if titleSlug == "" || id == 0 {
		return nil
	}
	return global.Redis.HSet(ctx, rediskey.LeetcodeProblemBankHashKey, titleSlug, id).Err()
}

func (r *leetcodeQuestionBankRepository) EnsureQuestionID(
	ctx context.Context,
	question *entity.LeetcodeQuestionBank,
) (uint, error) {
	if question == nil || question.TitleSlug == "" {
		return 0, errors.New("invalid leetcode question")
	}

	if err := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "title_slug"}},
			DoNothing: true,
		}).
		Create(question).Error; err != nil {
		return 0, err
	}

	if question.ID == 0 {
		existing, err := r.GetByTitleSlug(ctx, question.TitleSlug)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return 0, nil
			}
			return 0, err
		}
		question = existing
	}

	if question.ID > 0 {
		_ = r.CacheID(context.Background(), question.TitleSlug, question.ID)
	}
	return question.ID, nil
}

func (r *leetcodeQuestionBankRepository) ListTitleSlugIDAfterID(
	ctx context.Context,
	lastID uint,
	limit int,
) ([]*entity.LeetcodeQuestionBank, error) {
	var questions []*entity.LeetcodeQuestionBank
	err := r.db.WithContext(ctx).
		Select("id", "title_slug").
		Where("id > ?", lastID).
		Order("id ASC").
		Limit(limit).
		Find(&questions).Error
	if err != nil {
		return nil, err
	}
	return questions, nil
}
