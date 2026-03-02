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
	"go.uber.org/zap"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type luoguQuestionBankRepository struct {
	db *gorm.DB
}

func NewLuoguQuestionBankRepository(db *gorm.DB) interfaces.LuoguQuestionBankRepository {
	return &luoguQuestionBankRepository{db: db}
}

func (r *luoguQuestionBankRepository) GetAllPIDMap(
	ctx context.Context,
) (map[string]uint, error) {
	// 全量加载时，暂不强制走Redis（因为数据量大，HGETALL 可能阻塞）
	// 但可以在这里做 Redis 预热
	var questions []entity.LuoguQuestionBank
	// 只查询 ID 和 Pid 字段以减少内存消耗
	err := r.db.WithContext(ctx).Select("id", "pid").Find(&questions).Error
	if err != nil {
		return nil, err
	}

	pidMap := make(map[string]uint, len(questions))
	for _, q := range questions {
		pidMap[q.Pid] = q.ID
	}
	return pidMap, nil
}

func (r *luoguQuestionBankRepository) BatchCreate(
	ctx context.Context,
	questions []*entity.LuoguQuestionBank,
) error {
	if len(questions) == 0 {
		return nil
	}
	// 1. 使用 CreateInBatches 分批插入，防止 SQL 过长
	if err := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "pid"}},
			DoNothing: true,
		}).
		CreateInBatches(questions, 100).Error; err != nil {
		return err
	}

	// 2. 异步回填 Redis，保证缓存一致性
	go func() {
		// 创建一个新的 context，避免因父 context 取消而中断
		bgCtx := context.Background()
		pipeline := global.Redis.Pipeline()
		key := rediskey.LuoguProblemBankHashKey

		for _, q := range questions {
			if q.ID != 0 && q.Pid != "" {
				pipeline.HSet(bgCtx, key, q.Pid, q.ID)
			}
		}

		if _, err := pipeline.Exec(bgCtx); err != nil {
			global.Log.Error("failed to sync luogu problems to redis", zap.Error(err))
		}
	}()

	return nil
}

func (r *luoguQuestionBankRepository) GetByPID(
	ctx context.Context,
	pid string,
) (*entity.LuoguQuestionBank, error) {
	var q entity.LuoguQuestionBank
	err := r.db.WithContext(ctx).Where("pid = ?", pid).First(&q).Error
	return &q, err
}

// 从 Redis 缓存里根据洛谷题号 PID（比如 P1000）查到你本地题库表的 ID，并告诉你“有没有命中缓存”。
func (r *luoguQuestionBankRepository) GetCachedID(
	ctx context.Context,
	pid string,
) (uint, bool, error) {
	if pid == "" {
		return 0, false, nil
	}
	val, err := global.Redis.HGet(ctx, rediskey.LuoguProblemBankHashKey, pid).Result()
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

// CacheID 写缓存
func (r *luoguQuestionBankRepository) CacheID(
	ctx context.Context,
	pid string,
	id uint,
) error {
	if pid == "" || id == 0 {
		return nil
	}
	return global.Redis.HSet(ctx, rediskey.LuoguProblemBankHashKey, pid, id).Err()
}

// 确保某个洛谷题目（按 PID 唯一）在数据库里存在，并拿到它的本地 ID（如果已存在就查出来），最后顺便写入 Redis 缓存
func (r *luoguQuestionBankRepository) EnsureQuestionID(
	ctx context.Context,
	question *entity.LuoguQuestionBank,
) (uint, error) {
	if question == nil || question.Pid == "" {
		return 0, errors.New("invalid luogu question")
	}

	if err := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "pid"}},
			DoNothing: true,
		}).
		Create(question).Error; err != nil {
		return 0, err
	}

	if question.ID == 0 {
		existing, err := r.GetByPID(ctx, question.Pid)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return 0, nil
			}
			return 0, err
		}
		question = existing
	}

	if question.ID > 0 {
		_ = r.CacheID(context.Background(), question.Pid, question.ID)
	}
	return question.ID, nil
}

// 从题库表里“按自增 ID 向后分页”查询一批数据
func (r *luoguQuestionBankRepository) ListPIDIDAfterID(
	ctx context.Context,
	lastID uint,
	limit int,
) ([]*entity.LuoguQuestionBank, error) {
	var questions []*entity.LuoguQuestionBank
	err := r.db.WithContext(ctx).
		Select("id", "pid").
		Where("id > ?", lastID).
		Order("id ASC").
		Limit(limit).
		Find(&questions).Error
	if err != nil {
		return nil, err
	}
	return questions, nil
}
