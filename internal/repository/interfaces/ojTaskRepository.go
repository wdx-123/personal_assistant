package interfaces

import (
	"context"

	"personal_assistant/internal/model/dto/request"
	"personal_assistant/internal/model/entity"
	readmodel "personal_assistant/internal/model/readmodel"
)

// OJTaskRepository OJ 任务版本相关仓储。
type OJTaskRepository interface {
	WithTx(tx any) OJTaskRepository
	Create(ctx context.Context, task *entity.OJTask) error
	Update(ctx context.Context, task *entity.OJTask) error
	UpdateRootTaskID(ctx context.Context, taskID, rootTaskID uint) error
	GetByID(ctx context.Context, taskID uint) (*entity.OJTask, error)
	GetByIDForUpdate(ctx context.Context, taskID uint) (*entity.OJTask, error)
	GetLatestVersionByRootIDForUpdate(ctx context.Context, rootTaskID uint) (*entity.OJTask, error)
	CreateOrgs(ctx context.Context, orgs []*entity.OJTaskOrg) error
	ReplaceOrgs(ctx context.Context, taskID uint, orgs []*entity.OJTaskOrg) error
	ListOrgsByTaskID(ctx context.Context, taskID uint) ([]*entity.OJTaskOrg, error)
	ListTaskOrgsWithNames(ctx context.Context, taskID uint) ([]*readmodel.OJTaskOrgInfo, error)
	CreateItems(ctx context.Context, items []*entity.OJTaskItem) error
	ReplaceItems(ctx context.Context, taskID uint, items []*entity.OJTaskItem) error
	ListItemsByTaskID(ctx context.Context, taskID uint) ([]*entity.OJTaskItem, error)
	ListVisibleTasks(ctx context.Context, userID uint, isSuperAdmin bool, req *request.OJTaskListReq) ([]*readmodel.OJTaskListItem, int64, error)
	GetVisibleTask(ctx context.Context, userID uint, isSuperAdmin bool, taskID uint) (*readmodel.OJTaskVisibleTask, error)
	ListVisibleVersions(ctx context.Context, userID uint, isSuperAdmin bool, rootTaskID uint) ([]*readmodel.OJTaskVersionItem, error)
}
