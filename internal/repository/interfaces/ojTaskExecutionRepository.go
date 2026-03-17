package interfaces

import (
	"context"
	"time"

	"personal_assistant/internal/model/dto/request"
	"personal_assistant/internal/model/entity"
	readmodel "personal_assistant/internal/model/readmodel"
)

// OJTaskExecutionRepository OJ 任务执行与快照仓储。
type OJTaskExecutionRepository interface {
	WithTx(tx any) OJTaskExecutionRepository
	Create(ctx context.Context, execution *entity.OJTaskExecution) error
	Update(ctx context.Context, execution *entity.OJTaskExecution) error
	GetByID(ctx context.Context, executionID uint) (*entity.OJTaskExecution, error)
	GetDispatchExecutionByID(ctx context.Context, executionID uint) (*readmodel.OJTaskExecutionDispatch, error)
	GetByTaskID(ctx context.Context, taskID uint) (*entity.OJTaskExecution, error)
	GetByTaskIDForUpdate(ctx context.Context, taskID uint) (*entity.OJTaskExecution, error)
	ListDueExecutions(ctx context.Context, statuses []string, before time.Time, limit int) ([]*readmodel.OJTaskExecutionDispatch, error)
	ClaimExecution(ctx context.Context, executionID uint, fromStatuses []string, startedAt time.Time) (bool, error)
	BatchCreateExecutionUsers(ctx context.Context, rows []*entity.OJTaskExecutionUser, batchSize int) error
	ListExecutionUsersByExecutionID(ctx context.Context, executionID uint) ([]*entity.OJTaskExecutionUser, error)
	BatchCreateExecutionUserOrgs(ctx context.Context, rows []*entity.OJTaskExecutionUserOrg, batchSize int) error
	BatchCreateExecutionUserItems(ctx context.Context, rows []*entity.OJTaskExecutionUserItem, batchSize int) error
	GetVisibleExecutionDetail(ctx context.Context, userID uint, isSuperAdmin bool, taskID, executionID uint) (*readmodel.OJTaskVisibleTask, error)
	ListVisibleExecutionUsers(ctx context.Context, userID uint, isSuperAdmin bool, taskID, executionID uint, req *request.OJTaskExecutionUserListReq) ([]*readmodel.OJTaskExecutionUserListItem, int64, error)
	GetVisibleExecutionUser(ctx context.Context, userID uint, isSuperAdmin bool, taskID, executionID, targetUserID uint) (*readmodel.OJTaskExecutionUserListItem, error)
	ListExecutionUserOrgs(ctx context.Context, executionUserID uint) ([]*readmodel.OJTaskExecutionUserOrgItem, error)
	ListExecutionUserOrgsByExecutionUserIDs(ctx context.Context, executionUserIDs []uint) ([]*readmodel.OJTaskExecutionUserOrgItem, error)
	ListExecutionUserItems(ctx context.Context, executionID, targetUserID uint) ([]*readmodel.OJTaskExecutionUserItemDetail, error)
}
