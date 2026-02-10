package system

import (
	"context"
	"personal_assistant/internal/model/entity"
	"personal_assistant/internal/repository/interfaces"

	"gorm.io/gorm"
)

type luoguUserQuestionRepository struct {
	db *gorm.DB
}

func NewLuoguUserQuestionRepository(db *gorm.DB) interfaces.LuoguUserQuestionRepository {
	return &luoguUserQuestionRepository{db: db}
}

func (r *luoguUserQuestionRepository) GetSolvedProblemIDs(
	ctx context.Context,
	luoguUserDetailID uint,
) (map[uint]struct{}, error) {
	var records []entity.LuoguUserQuestion
	err := r.db.WithContext(ctx).
		Select("luogu_question_id").
		Where("luogu_user_detail_id = ?", luoguUserDetailID).
		Find(&records).Error
	if err != nil {
		return nil, err
	}

	idSet := make(map[uint]struct{}, len(records))
	for _, record := range records {
		idSet[record.LuoguQuestionID] = struct{}{}
	}
	return idSet, nil
}

func (r *luoguUserQuestionRepository) BatchCreate(
	ctx context.Context,
	records []*entity.LuoguUserQuestion,
) error {
	if len(records) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).CreateInBatches(records, 100).Error
}
