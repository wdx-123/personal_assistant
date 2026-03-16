package system

import (
	"context"
	"errors"
	"strings"
	"time"

	"personal_assistant/internal/model/consts"
	"personal_assistant/internal/model/dto/request"
	"personal_assistant/internal/model/entity"
	readmodel "personal_assistant/internal/model/readmodel"
	"personal_assistant/internal/repository/interfaces"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type ojTaskExecutionRepository struct {
	db *gorm.DB
}

func NewOJTaskExecutionRepository(db *gorm.DB) interfaces.OJTaskExecutionRepository {
	return &ojTaskExecutionRepository{db: db}
}

func (r *ojTaskExecutionRepository) WithTx(tx any) interfaces.OJTaskExecutionRepository {
	if transaction, ok := tx.(*gorm.DB); ok {
		return &ojTaskExecutionRepository{db: transaction}
	}
	return r
}

func (r *ojTaskExecutionRepository) Create(ctx context.Context, execution *entity.OJTaskExecution) error {
	return r.db.WithContext(ctx).Create(execution).Error
}

func (r *ojTaskExecutionRepository) Update(ctx context.Context, execution *entity.OJTaskExecution) error {
	return r.db.WithContext(ctx).Save(execution).Error
}

// GetByID 根据执行记录 ID 获取 OJTaskExecution 实体；未找到时返回 nil。
func (r *ojTaskExecutionRepository) GetByID(ctx context.Context, executionID uint) (*entity.OJTaskExecution, error) {
	var execution entity.OJTaskExecution
	err := r.db.WithContext(ctx).First(&execution, executionID).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &execution, nil
}

// GetByTaskID 根据任务版本 ID 获取对应的 OJTaskExecution 实体；未找到时返回 nil。
func (r *ojTaskExecutionRepository) GetByTaskID(ctx context.Context, taskID uint) (*entity.OJTaskExecution, error) {
	var execution entity.OJTaskExecution
	err := r.db.WithContext(ctx).Where("task_id = ?", taskID).First(&execution).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &execution, nil
}

func (r *ojTaskExecutionRepository) GetByTaskIDForUpdate(ctx context.Context, taskID uint) (*entity.OJTaskExecution, error) {
	var execution entity.OJTaskExecution
	err := r.db.WithContext(ctx).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("task_id = ?", taskID).
		First(&execution).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &execution, nil
}

func (r *ojTaskExecutionRepository) ListDueExecutions(
	ctx context.Context,
	statuses []string,
	before time.Time,
	limit int,
) ([]*readmodel.OJTaskExecutionDispatch, error) {
	if len(statuses) == 0 {
		return nil, nil
	}
	query := r.db.WithContext(ctx).
		Table("oj_task_executions").
		Select("id AS execution_id, task_id, status, planned_at").
		Where("status IN ? AND planned_at <= ?", statuses, before).
		Order("planned_at ASC, id ASC")
	if limit > 0 {
		query = query.Limit(limit)
	}
	var rows []*readmodel.OJTaskExecutionDispatch
	if err := query.Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (r *ojTaskExecutionRepository) ClaimExecution(
	ctx context.Context,
	executionID uint,
	fromStatuses []string,
	startedAt time.Time,
) (bool, error) {
	result := r.db.WithContext(ctx).
		Model(&entity.OJTaskExecution{}).
		Where("id = ? AND status IN ?", executionID, fromStatuses).
		Updates(map[string]any{
			"status":     string(consts.OJTaskExecutionStatusExecuting),
			"started_at": startedAt,
		})
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected > 0, nil
}

func (r *ojTaskExecutionRepository) BatchCreateExecutionUsers(
	ctx context.Context,
	rows []*entity.OJTaskExecutionUser,
	batchSize int,
) error {
	if len(rows) == 0 {
		return nil
	}
	if batchSize <= 0 {
		batchSize = 100
	}
	return r.db.WithContext(ctx).CreateInBatches(rows, batchSize).Error
}

func (r *ojTaskExecutionRepository) ListExecutionUsersByExecutionID(
	ctx context.Context,
	executionID uint,
) ([]*entity.OJTaskExecutionUser, error) {
	var rows []*entity.OJTaskExecutionUser
	err := r.db.WithContext(ctx).
		Where("execution_id = ?", executionID).
		Order("user_id ASC").
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	return rows, nil
}

func (r *ojTaskExecutionRepository) BatchCreateExecutionUserOrgs(
	ctx context.Context,
	rows []*entity.OJTaskExecutionUserOrg,
	batchSize int,
) error {
	if len(rows) == 0 {
		return nil
	}
	if batchSize <= 0 {
		batchSize = 100
	}
	return r.db.WithContext(ctx).CreateInBatches(rows, batchSize).Error
}

func (r *ojTaskExecutionRepository) BatchCreateExecutionUserItems(
	ctx context.Context,
	rows []*entity.OJTaskExecutionUserItem,
	batchSize int,
) error {
	if len(rows) == 0 {
		return nil
	}
	if batchSize <= 0 {
		batchSize = 100
	}
	return r.db.WithContext(ctx).CreateInBatches(rows, batchSize).Error
}

func (r *ojTaskExecutionRepository) GetVisibleExecutionDetail(
	ctx context.Context,
	userID uint,
	isSuperAdmin bool,
	taskID, executionID uint,
) (*readmodel.OJTaskVisibleTask, error) {
	base := r.visibleExecutionTaskBase(ctx, userID, isSuperAdmin, taskID, executionID)
	var row readmodel.OJTaskVisibleTask
	err := r.db.WithContext(ctx).
		Table("oj_tasks").
		Select(`
			oj_tasks.id AS task_id,
			COALESCE(oj_tasks.root_task_id, oj_tasks.id) AS root_task_id,
			oj_tasks.parent_task_id,
			oj_tasks.version_no,
			oj_tasks.title,
			oj_tasks.description,
			oj_tasks.mode,
			oj_tasks.status,
			oj_tasks.execute_at,
			oj_tasks.created_by,
			oj_tasks.updated_by,
			oj_tasks.created_at,
			oj_tasks.updated_at,
			oj_task_executions.id AS execution_id,
			oj_task_executions.status AS execution_status,
			oj_task_executions.total_user_count,
			oj_task_executions.completed_user_count,
			oj_task_executions.pending_user_count,
			oj_task_executions.total_item_count,
			oj_task_executions.completed_item_count,
			oj_task_executions.pending_item_count,
			(SELECT COUNT(1) FROM oj_task_orgs WHERE oj_task_orgs.task_id = oj_tasks.id AND oj_task_orgs.deleted_at IS NULL) AS org_count,
			(SELECT COUNT(1) FROM oj_task_items WHERE oj_task_items.task_id = oj_tasks.id AND oj_task_items.deleted_at IS NULL) AS item_count,
			oj_task_executions.trigger_type,
			oj_task_executions.planned_at,
			oj_task_executions.started_at,
			oj_task_executions.finished_at,
			oj_task_executions.error_message,
			oj_task_executions.requested_by`).
		Joins("JOIN oj_task_executions ON oj_task_executions.task_id = oj_tasks.id").
		Where("oj_tasks.id IN (?)", base.Select("DISTINCT oj_tasks.id")).
		Limit(1).
		Scan(&row).Error
	if err != nil {
		return nil, err
	}
	if row.TaskID == 0 {
		return nil, nil
	}
	return &row, nil
}

func (r *ojTaskExecutionRepository) ListVisibleExecutionUsers(
	ctx context.Context,
	userID uint,
	isSuperAdmin bool,
	taskID, executionID uint,
	req *request.OJTaskExecutionUserListReq,
) ([]*readmodel.OJTaskExecutionUserListItem, int64, error) {
	base := r.visibleExecutionUserBase(ctx, userID, isSuperAdmin, taskID, executionID, req)

	var total int64
	if err := base.Session(&gorm.Session{}).
		Distinct("oj_task_execution_users.id").
		Count(&total).Error; err != nil {
		return nil, 0, err
	}

	query := base.Select(`
		DISTINCT oj_task_execution_users.id AS execution_user_id,
		oj_task_execution_users.execution_id,
		oj_task_execution_users.user_id,
		oj_task_execution_users.user_uuid_snapshot,
		oj_task_execution_users.username_snapshot,
		oj_task_execution_users.avatar_snapshot,
		oj_task_execution_users.user_status_snapshot,
		oj_task_execution_users.completed_item_count,
		oj_task_execution_users.pending_item_count,
		oj_task_execution_users.all_completed`).
		Order("oj_task_execution_users.pending_item_count DESC, oj_task_execution_users.user_id ASC")
	if req != nil && req.Page > 0 && req.PageSize > 0 {
		query = query.Offset((req.Page - 1) * req.PageSize).Limit(req.PageSize)
	}

	var rows []*readmodel.OJTaskExecutionUserListItem
	if err := query.Scan(&rows).Error; err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

func (r *ojTaskExecutionRepository) GetVisibleExecutionUser(
	ctx context.Context,
	userID uint,
	isSuperAdmin bool,
	taskID, executionID, targetUserID uint,
) (*readmodel.OJTaskExecutionUserListItem, error) {
	var row readmodel.OJTaskExecutionUserListItem
	err := r.visibleExecutionUserBase(ctx, userID, isSuperAdmin, taskID, executionID, nil).
		Where("oj_task_execution_users.user_id = ?", targetUserID).
		Select(`
			DISTINCT oj_task_execution_users.id AS execution_user_id,
			oj_task_execution_users.execution_id,
			oj_task_execution_users.user_id,
			oj_task_execution_users.user_uuid_snapshot,
			oj_task_execution_users.username_snapshot,
			oj_task_execution_users.avatar_snapshot,
			oj_task_execution_users.user_status_snapshot,
			oj_task_execution_users.completed_item_count,
			oj_task_execution_users.pending_item_count,
			oj_task_execution_users.all_completed`).
		Limit(1).
		Scan(&row).Error
	if err != nil {
		return nil, err
	}
	if row.ExecutionUserID == 0 {
		return nil, nil
	}
	return &row, nil
}

func (r *ojTaskExecutionRepository) ListExecutionUserOrgs(
	ctx context.Context,
	executionUserID uint,
) ([]*readmodel.OJTaskExecutionUserOrgItem, error) {
	var rows []*readmodel.OJTaskExecutionUserOrgItem
	err := r.db.WithContext(ctx).
		Table("oj_task_execution_user_orgs").
		Select("execution_user_id, org_id, org_name_snapshot").
		Where("execution_user_id = ?", executionUserID).
		Order("org_id ASC").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	return rows, nil
}

func (r *ojTaskExecutionRepository) ListExecutionUserOrgsByExecutionUserIDs(
	ctx context.Context,
	executionUserIDs []uint,
) ([]*readmodel.OJTaskExecutionUserOrgItem, error) {
	if len(executionUserIDs) == 0 {
		return nil, nil
	}
	var rows []*readmodel.OJTaskExecutionUserOrgItem
	err := r.db.WithContext(ctx).
		Table("oj_task_execution_user_orgs").
		Select("execution_user_id, org_id, org_name_snapshot").
		Where("execution_user_id IN ?", executionUserIDs).
		Order("execution_user_id ASC, org_id ASC").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	return rows, nil
}

func (r *ojTaskExecutionRepository) ListExecutionUserItems(
	ctx context.Context,
	executionID, targetUserID uint,
) ([]*readmodel.OJTaskExecutionUserItemDetail, error) {
	var rows []*readmodel.OJTaskExecutionUserItemDetail
	err := r.db.WithContext(ctx).
		Table("oj_task_execution_user_items").
		Select(`
			oj_task_execution_user_items.execution_user_id,
			oj_task_execution_user_items.execution_id,
			oj_task_execution_user_items.user_id,
			oj_task_execution_user_items.task_item_id,
			oj_task_items.sort_no,
			oj_task_items.platform,
			oj_task_items.question_code,
			oj_task_items.platform_question_id,
			oj_task_items.question_title_snapshot,
			oj_task_execution_user_items.result_status,
			oj_task_execution_user_items.reason`).
		Joins("JOIN oj_task_items ON oj_task_items.id = oj_task_execution_user_items.task_item_id").
		Where("oj_task_execution_user_items.execution_id = ? AND oj_task_execution_user_items.user_id = ?", executionID, targetUserID).
		Order("oj_task_items.sort_no ASC, oj_task_execution_user_items.id ASC").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	return rows, nil
}

func (r *ojTaskExecutionRepository) visibleExecutionTaskBase(
	ctx context.Context,
	userID uint,
	isSuperAdmin bool,
	taskID, executionID uint,
) *gorm.DB {
	query := r.db.WithContext(ctx).
		Table("oj_tasks").
		Joins("JOIN oj_task_executions ON oj_task_executions.task_id = oj_tasks.id").
		Where("oj_tasks.id = ? AND oj_task_executions.id = ?", taskID, executionID)
	if !isSuperAdmin {
		query = query.
			Joins("JOIN oj_task_orgs ON oj_task_orgs.task_id = oj_tasks.id").
			Joins(
				"JOIN org_members ON org_members.org_id = oj_task_orgs.org_id AND org_members.user_id = ? AND org_members.member_status = ?",
				userID,
				consts.OrgMemberStatusActive,
			)
	}
	return query
}

func (r *ojTaskExecutionRepository) visibleExecutionUserBase(
	ctx context.Context,
	userID uint,
	isSuperAdmin bool,
	taskID, executionID uint,
	req *request.OJTaskExecutionUserListReq,
) *gorm.DB {
	query := r.db.WithContext(ctx).
		Table("oj_task_execution_users").
		Joins("JOIN oj_task_executions ON oj_task_executions.id = oj_task_execution_users.execution_id").
		Joins("JOIN oj_tasks ON oj_tasks.id = oj_task_executions.task_id").
		Where("oj_tasks.id = ? AND oj_task_executions.id = ?", taskID, executionID)
	if !isSuperAdmin {
		query = query.
			Joins("JOIN oj_task_orgs ON oj_task_orgs.task_id = oj_tasks.id").
			Joins(
				"JOIN org_members ON org_members.org_id = oj_task_orgs.org_id AND org_members.user_id = ? AND org_members.member_status = ?",
				userID,
				consts.OrgMemberStatusActive,
			)
	}
	if req == nil {
		return query
	}
	if req.AllCompleted != nil {
		query = query.Where("oj_task_execution_users.all_completed = ?", *req.AllCompleted)
	}
	if username := strings.TrimSpace(req.Username); username != "" {
		query = query.Where("oj_task_execution_users.username_snapshot LIKE ?", "%"+username+"%")
	}
	return query
}
