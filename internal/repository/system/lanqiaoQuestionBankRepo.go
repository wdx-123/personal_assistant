package system

import (
	"context"
	"errors"
	"strconv"

	"personal_assistant/global"
	"personal_assistant/internal/model/entity"
	"personal_assistant/internal/repository/interfaces"
	"personal_assistant/pkg/rediskey"

	"github.com/go-redis/redis/v8"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type lanqiaoQuestionBankRepository struct {
	db *gorm.DB
}

func NewLanqiaoQuestionBankRepository(db *gorm.DB) interfaces.LanqiaoQuestionBankRepository {
	return &lanqiaoQuestionBankRepository{db: db}
}

func (r *lanqiaoQuestionBankRepository) WithTx(tx any) interfaces.LanqiaoQuestionBankRepository {
	if transaction, ok := tx.(*gorm.DB); ok {
		return &lanqiaoQuestionBankRepository{db: transaction}
	}
	return r
}

func (r *lanqiaoQuestionBankRepository) Create(
	ctx context.Context,
	question *entity.LanqiaoQuestionBank,
) error {
	return r.db.WithContext(ctx).Create(question).Error
}

func (r *lanqiaoQuestionBankRepository) Update(
	ctx context.Context,
	question *entity.LanqiaoQuestionBank,
) error {
	return r.db.WithContext(ctx).Save(question).Error
}

func (r *lanqiaoQuestionBankRepository) GetByID(
	ctx context.Context,
	id uint,
) (*entity.LanqiaoQuestionBank, error) {
	var q entity.LanqiaoQuestionBank
	err := r.db.WithContext(ctx).First(&q, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &q, nil
}

func (r *lanqiaoQuestionBankRepository) GetByProblemID(
	ctx context.Context,
	problemID int,
) (*entity.LanqiaoQuestionBank, error) {
	var q entity.LanqiaoQuestionBank
	err := r.db.WithContext(ctx).Where("problem_id = ?", problemID).First(&q).Error
	if err != nil {
		return nil, err
	}
	return &q, nil
}

func (r *lanqiaoQuestionBankRepository) ListByExactTitle(
	ctx context.Context,
	title string,
) ([]*entity.LanqiaoQuestionBank, error) {
	var questions []*entity.LanqiaoQuestionBank
	err := r.db.WithContext(ctx).
		Where("title = ?", title).
		Order("source_status ASC, id ASC").
		Find(&questions).Error
	if err != nil {
		return nil, err
	}
	return questions, nil
}

func (r *lanqiaoQuestionBankRepository) SearchByTitle(
	ctx context.Context,
	keyword string,
	limit int,
) ([]*entity.LanqiaoQuestionBank, error) {
	query := r.db.WithContext(ctx).
		Where("title LIKE ?", "%"+keyword+"%").
		Order("source_status ASC, id ASC")
	if limit > 0 {
		query = query.Limit(limit)
	}
	var questions []*entity.LanqiaoQuestionBank
	if err := query.Find(&questions).Error; err != nil {
		return nil, err
	}
	return questions, nil
}

func (r *lanqiaoQuestionBankRepository) GetCachedID(
	ctx context.Context,
	problemID int,
) (uint, bool, error) {
	if problemID <= 0 || global.Redis == nil {
		return 0, false, nil
	}
	val, err := global.Redis.HGet(ctx, rediskey.LanqiaoProblemBankHashKey, strconv.Itoa(problemID)).Result()
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

func (r *lanqiaoQuestionBankRepository) CacheID(
	ctx context.Context,
	problemID int,
	id uint,
) error {
	if problemID <= 0 || id == 0 || global.Redis == nil {
		return nil
	}
	return global.Redis.HSet(ctx, rediskey.LanqiaoProblemBankHashKey, strconv.Itoa(problemID), id).Err()
}

func (r *lanqiaoQuestionBankRepository) EnsureQuestionID(
	ctx context.Context,
	question *entity.LanqiaoQuestionBank,
) (uint, error) {
	if question == nil || question.ProblemID <= 0 {
		return 0, errors.New("invalid lanqiao question")
	}

	if err := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "problem_id"}},
			DoNothing: true,
		}).
		Create(question).Error; err != nil {
		return 0, err
	}

	if question.ID == 0 {
		existing, err := r.GetByProblemID(ctx, question.ProblemID)
		if err != nil {
			return 0, err
		}
		question = existing
	}

	if question != nil && question.ID > 0 {
		_ = r.CacheID(context.Background(), question.ProblemID, question.ID)
		return question.ID, nil
	}
	return 0, nil
}
