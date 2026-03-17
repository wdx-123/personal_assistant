package system

import (
	"context"
	"errors"
	"strings"

	"personal_assistant/internal/model/consts"
	"personal_assistant/internal/model/dto/request"
	"personal_assistant/internal/model/entity"
	readmodel "personal_assistant/internal/model/readmodel"
	"personal_assistant/internal/repository/interfaces"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type ojTaskRepository struct {
	db *gorm.DB
}

func NewOJTaskRepository(db *gorm.DB) interfaces.OJTaskRepository {
	return &ojTaskRepository{db: db}
}

func (r *ojTaskRepository) WithTx(tx any) interfaces.OJTaskRepository {
	if transaction, ok := tx.(*gorm.DB); ok {
		return &ojTaskRepository{db: transaction}
	}
	return r
}

func (r *ojTaskRepository) Create(ctx context.Context, task *entity.OJTask) error {
	return r.db.WithContext(ctx).Create(task).Error
}

func (r *ojTaskRepository) Update(ctx context.Context, task *entity.OJTask) error {
	return r.db.WithContext(ctx).Save(task).Error
}

func (r *ojTaskRepository) UpdateRootTaskID(ctx context.Context, taskID, rootTaskID uint) error {
	return r.db.WithContext(ctx).
		Model(&entity.OJTask{}).
		Where("id = ?", taskID).
		Update("root_task_id", rootTaskID).Error
}

// GetByID 获取指定 ID 的任务版本实体；未找到时返回 nil。
func (r *ojTaskRepository) GetByID(ctx context.Context, taskID uint) (*entity.OJTask, error) {
	var task entity.OJTask
	err := r.db.WithContext(ctx).First(&task, taskID).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &task, nil
}

func (r *ojTaskRepository) GetByIDForUpdate(ctx context.Context, taskID uint) (*entity.OJTask, error) {
	var task entity.OJTask
	err := r.db.WithContext(ctx).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		First(&task, taskID).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &task, nil
}

func (r *ojTaskRepository) GetLatestVersionByRootIDForUpdate(
	ctx context.Context,
	rootTaskID uint,
) (*entity.OJTask, error) {
	var task entity.OJTask
	err := r.db.WithContext(ctx).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("root_task_id = ?", rootTaskID).
		Order("version_no DESC, id DESC").
		First(&task).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &task, nil
}

func (r *ojTaskRepository) CreateOrgs(ctx context.Context, orgs []*entity.OJTaskOrg) error {
	if len(orgs) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).CreateInBatches(orgs, 100).Error
}

func (r *ojTaskRepository) ReplaceOrgs(ctx context.Context, taskID uint, orgs []*entity.OJTaskOrg) error {
	if err := r.db.WithContext(ctx).
		Unscoped().
		Where("task_id = ?", taskID).
		Delete(&entity.OJTaskOrg{}).Error; err != nil {
		return err
	}
	return r.CreateOrgs(ctx, orgs)
}

func (r *ojTaskRepository) ListOrgsByTaskID(ctx context.Context, taskID uint) ([]*entity.OJTaskOrg, error) {
	var orgs []*entity.OJTaskOrg
	err := r.db.WithContext(ctx).
		Where("task_id = ?", taskID).
		Order("org_id ASC").
		Find(&orgs).Error
	if err != nil {
		return nil, err
	}
	return orgs, nil
}

// ListTaskOrgsWithNames 获取指定任务版本关联的组织信息列表，包含组织 ID 和名称。
func (r *ojTaskRepository) ListTaskOrgsWithNames(
	ctx context.Context,
	taskID uint,
) ([]*readmodel.OJTaskOrgInfo, error) {
	var rows []*readmodel.OJTaskOrgInfo
	err := r.db.WithContext(ctx).
		Table("oj_task_orgs").
		Select("oj_task_orgs.task_id, oj_task_orgs.org_id, COALESCE(orgs.name, '') AS org_name").
		Joins("LEFT JOIN orgs ON orgs.id = oj_task_orgs.org_id").
		Where("oj_task_orgs.task_id = ?", taskID).
		Order("oj_task_orgs.org_id ASC").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	return rows, nil
}

func (r *ojTaskRepository) CreateItems(ctx context.Context, items []*entity.OJTaskItem) error {
	if len(items) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).CreateInBatches(items, 100).Error
}

func (r *ojTaskRepository) GetItemByID(ctx context.Context, itemID uint) (*entity.OJTaskItem, error) {
	var item entity.OJTaskItem
	err := r.db.WithContext(ctx).First(&item, itemID).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &item, nil
}

func (r *ojTaskRepository) UpdateItem(ctx context.Context, item *entity.OJTaskItem) error {
	return r.db.WithContext(ctx).Save(item).Error
}

func (r *ojTaskRepository) ReplaceItems(ctx context.Context, taskID uint, items []*entity.OJTaskItem) error {
	if err := r.db.WithContext(ctx).
		Unscoped().
		Where("task_id = ?", taskID).
		Delete(&entity.OJTaskItem{}).Error; err != nil {
		return err
	}
	return r.CreateItems(ctx, items)
}

func (r *ojTaskRepository) ListItemsByTaskID(ctx context.Context, taskID uint) ([]*entity.OJTaskItem, error) {
	var items []*entity.OJTaskItem
	err := r.db.WithContext(ctx).
		Where("task_id = ?", taskID).
		Order("sort_no ASC, id ASC").
		Find(&items).Error
	if err != nil {
		return nil, err
	}
	return items, nil
}

func (r *ojTaskRepository) CreateIntakes(ctx context.Context, rows []*entity.OJQuestionIntake) error {
	if len(rows) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).CreateInBatches(rows, 100).Error
}

func (r *ojTaskRepository) UpdateIntake(ctx context.Context, intake *entity.OJQuestionIntake) error {
	return r.db.WithContext(ctx).Save(intake).Error
}

func (r *ojTaskRepository) ReplaceIntakes(ctx context.Context, taskID uint, rows []*entity.OJQuestionIntake) error {
	if err := r.db.WithContext(ctx).
		Unscoped().
		Where("task_id = ?", taskID).
		Delete(&entity.OJQuestionIntake{}).Error; err != nil {
		return err
	}
	return r.CreateIntakes(ctx, rows)
}

func (r *ojTaskRepository) ListIntakesByTaskID(ctx context.Context, taskID uint) ([]*entity.OJQuestionIntake, error) {
	var rows []*entity.OJQuestionIntake
	err := r.db.WithContext(ctx).
		Where("task_id = ?", taskID).
		Order("task_item_id ASC").
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	return rows, nil
}

func (r *ojTaskRepository) ListPendingIntakesByTitle(
	ctx context.Context,
	platform, title string,
) ([]*entity.OJQuestionIntake, error) {
	var rows []*entity.OJQuestionIntake
	err := r.db.WithContext(ctx).
		Where("platform = ? AND input_title = ? AND status = ?", platform, title, string(consts.OJTaskItemResolutionStatusPendingResolution)).
		Order("task_item_id ASC").
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	return rows, nil
}

func (r *ojTaskRepository) ListVisibleTasks(
	ctx context.Context,
	userID uint,
	isSuperAdmin bool,
	req *request.OJTaskListReq,
) ([]*readmodel.OJTaskListItem, int64, error) {
	filteredIDs := r.filteredVisibleTaskIDQuery(ctx, userID, isSuperAdmin, req)

	var total int64
	if err := r.db.WithContext(ctx).
		Table("oj_tasks").
		Where("id IN (?)", filteredIDs).
		Count(&total).Error; err != nil {
		return nil, 0, err
	}

	query := r.taskSummaryQuery(ctx).
		Where("oj_tasks.id IN (?)", filteredIDs).
		Order("oj_tasks.created_at DESC, oj_tasks.id DESC")

	if req != nil && req.Page > 0 && req.PageSize > 0 {
		query = query.Offset((req.Page - 1) * req.PageSize).Limit(req.PageSize)
	}

	var rows []*readmodel.OJTaskListItem
	if err := query.Scan(&rows).Error; err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

func (r *ojTaskRepository) GetVisibleTask(
	ctx context.Context,
	userID uint,
	isSuperAdmin bool,
	taskID uint,
) (*readmodel.OJTaskVisibleTask, error) {
	idQuery := r.visibleTaskIDBaseQuery(ctx, userID, isSuperAdmin, nil).Where("oj_tasks.id = ?", taskID)

	var row readmodel.OJTaskVisibleTask
	err := r.taskDetailQuery(ctx).
		Where("oj_tasks.id IN (?)", idQuery.Select("DISTINCT oj_tasks.id")).
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

func (r *ojTaskRepository) ListVisibleVersions(
	ctx context.Context,
	userID uint,
	isSuperAdmin bool,
	rootTaskID uint,
) ([]*readmodel.OJTaskVersionItem, error) {
	base := r.visibleTaskIDBaseQuery(ctx, userID, isSuperAdmin, nil).
		Where("COALESCE(oj_tasks.root_task_id, oj_tasks.id) = ?", rootTaskID)

	var rows []*readmodel.OJTaskVersionItem
	err := r.db.WithContext(ctx).
		Table("oj_tasks").
		Select(`
			oj_tasks.id AS task_id,
			COALESCE(oj_tasks.root_task_id, oj_tasks.id) AS root_task_id,
			oj_tasks.parent_task_id,
			oj_tasks.version_no,
			oj_tasks.title,
			oj_tasks.mode,
			oj_tasks.status,
			oj_tasks.execute_at,
			oj_tasks.created_at,
			oj_task_executions.id AS execution_id,
			oj_task_executions.status AS execution_status`).
		Joins("JOIN oj_task_executions ON oj_task_executions.task_id = oj_tasks.id").
		Where("oj_tasks.id IN (?)", base.Select("DISTINCT oj_tasks.id")).
		Order("oj_tasks.version_no DESC, oj_tasks.id DESC").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	return rows, nil
}

func (r *ojTaskRepository) filteredVisibleTaskIDQuery(
	ctx context.Context,
	userID uint,
	isSuperAdmin bool,
	req *request.OJTaskListReq,
) *gorm.DB {
	visibleIDs := r.visibleTaskIDBaseQuery(ctx, userID, isSuperAdmin, req).
		Select("DISTINCT oj_tasks.id AS task_id, COALESCE(oj_tasks.root_task_id, oj_tasks.id) AS root_task_id, oj_tasks.version_no")

	if req == nil || req.OnlyLatest == nil || *req.OnlyLatest {
		latestVersion := r.db.WithContext(ctx).
			Table("(?) AS visible_tasks", visibleIDs).
			Select("visible_tasks.root_task_id, MAX(visible_tasks.version_no) AS version_no").
			Group("visible_tasks.root_task_id")
		return r.db.WithContext(ctx).
			Table("(?) AS visible_tasks", visibleIDs).
			Select("visible_tasks.task_id").
			Joins("JOIN (?) latest ON latest.root_task_id = visible_tasks.root_task_id AND latest.version_no = visible_tasks.version_no", latestVersion)
	}
	return r.db.WithContext(ctx).Table("(?) AS visible_tasks", visibleIDs).Select("visible_tasks.task_id")
}

func (r *ojTaskRepository) visibleTaskIDBaseQuery(
	ctx context.Context,
	userID uint,
	isSuperAdmin bool,
	req *request.OJTaskListReq,
) *gorm.DB {
	query := r.db.WithContext(ctx).
		Table("oj_tasks").
		Joins("JOIN oj_task_executions ON oj_task_executions.task_id = oj_tasks.id")
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
	if req.OrgID != nil && *req.OrgID > 0 {
		if isSuperAdmin {
			query = query.Joins("JOIN oj_task_orgs filter_task_orgs ON filter_task_orgs.task_id = oj_tasks.id")
			query = query.Where("filter_task_orgs.org_id = ?", *req.OrgID)
		} else {
			query = query.Where("oj_task_orgs.org_id = ?", *req.OrgID)
		}
	}
	if req.RootTaskID != nil && *req.RootTaskID > 0 {
		query = query.Where("COALESCE(oj_tasks.root_task_id, oj_tasks.id) = ?", *req.RootTaskID)
	}
	if mode := strings.TrimSpace(req.Mode); mode != "" {
		query = query.Where("oj_tasks.mode = ?", mode)
	}
	if status := strings.TrimSpace(req.Status); status != "" {
		query = query.Where("oj_tasks.status = ?", status)
	}
	if keyword := strings.TrimSpace(req.Keyword); keyword != "" {
		query = query.Where("oj_tasks.title LIKE ?", "%"+keyword+"%")
	}
	return query
}

func (r *ojTaskRepository) taskSummaryQuery(ctx context.Context) *gorm.DB {
	return r.db.WithContext(ctx).
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
			(SELECT COUNT(1) FROM oj_task_items WHERE oj_task_items.task_id = oj_tasks.id AND oj_task_items.deleted_at IS NULL) AS item_count`).
		Joins("JOIN oj_task_executions ON oj_task_executions.task_id = oj_tasks.id")
}

func (r *ojTaskRepository) taskDetailQuery(ctx context.Context) *gorm.DB {
	return r.taskSummaryQuery(ctx).Select(`
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
		oj_task_executions.requested_by`)
}
