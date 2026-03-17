package system

import (
	"context"
	stderrors "errors"
	"sort"
	"strconv"
	"strings"
	"time"

	"personal_assistant/internal/model/consts"
	eventdto "personal_assistant/internal/model/dto/event"
	"personal_assistant/internal/model/dto/request"
	dtoresp "personal_assistant/internal/model/dto/response"
	"personal_assistant/internal/model/entity"
	readmodel "personal_assistant/internal/model/readmodel"
	"personal_assistant/internal/repository"
	"personal_assistant/internal/repository/interfaces"
	svccontract "personal_assistant/internal/service/contract"
	bizerrors "personal_assistant/pkg/errors"

	"gorm.io/gorm"
)

type validatedOJTaskItem struct {
	SortNo                int
	Platform              string
	QuestionCode          string
	PlatformQuestionID    uint
	QuestionTitleSnapshot string
}

type validatedOJTaskDraft struct {
	Title       string
	Description string
	Mode        string
	ExecuteAt   *time.Time
	OrgIDs      []uint
	Items       []validatedOJTaskItem
}

// OJTaskService OJ 任务业务编排服务。
type OJTaskService struct {
	txRunner                 repository.TxRunner
	taskRepo                 interfaces.OJTaskRepository
	executionRepo            interfaces.OJTaskExecutionRepository
	orgRepo                  interfaces.OrgRepository
	orgMemberRepo            interfaces.OrgMemberRepository
	userRepo                 interfaces.UserRepository
	luoguQuestionRepo        interfaces.LuoguQuestionBankRepository
	leetcodeQuestionRepo     interfaces.LeetcodeQuestionBankRepository
	lanqiaoQuestionRepo      interfaces.LanqiaoQuestionBankRepository
	luoguDetailRepo          interfaces.LuoguUserDetailRepository
	leetcodeDetailRepo       interfaces.LeetcodeUserDetailRepository
	lanqiaoDetailRepo        interfaces.LanqiaoUserDetailRepository
	luoguUserQuestionRepo    interfaces.LuoguUserQuestionRepository
	leetcodeUserQuestionRepo interfaces.LeetcodeUserQuestionRepository
	lanqiaoUserQuestionRepo  interfaces.LanqiaoUserQuestionRepository
	triggerPublisher         ojTaskExecutionTriggerEventPublisher
	authorizationService     svccontract.AuthorizationServiceContract
}

func NewOJTaskService(
	repositoryGroup *repository.Group,
	authorizationService svccontract.AuthorizationServiceContract,
) *OJTaskService {
	return &OJTaskService{
		txRunner:                 repositoryGroup,
		taskRepo:                 repositoryGroup.SystemRepositorySupplier.GetOJTaskRepository(),
		executionRepo:            repositoryGroup.SystemRepositorySupplier.GetOJTaskExecutionRepository(),
		orgRepo:                  repositoryGroup.SystemRepositorySupplier.GetOrgRepository(),
		orgMemberRepo:            repositoryGroup.SystemRepositorySupplier.GetOrgMemberRepository(),
		userRepo:                 repositoryGroup.SystemRepositorySupplier.GetUserRepository(),
		luoguQuestionRepo:        repositoryGroup.SystemRepositorySupplier.GetLuoguQuestionBankRepository(),
		leetcodeQuestionRepo:     repositoryGroup.SystemRepositorySupplier.GetLeetcodeQuestionBankRepository(),
		lanqiaoQuestionRepo:      repositoryGroup.SystemRepositorySupplier.GetLanqiaoQuestionBankRepository(),
		luoguDetailRepo:          repositoryGroup.SystemRepositorySupplier.GetLuoguUserDetailRepository(),
		leetcodeDetailRepo:       repositoryGroup.SystemRepositorySupplier.GetLeetcodeUserDetailRepository(),
		lanqiaoDetailRepo:        repositoryGroup.SystemRepositorySupplier.GetLanqiaoUserDetailRepository(),
		luoguUserQuestionRepo:    repositoryGroup.SystemRepositorySupplier.GetLuoguUserQuestionRepository(),
		leetcodeUserQuestionRepo: repositoryGroup.SystemRepositorySupplier.GetLeetcodeUserQuestionRepository(),
		lanqiaoUserQuestionRepo:  repositoryGroup.SystemRepositorySupplier.GetLanqiaoUserQuestionRepository(),
		triggerPublisher: newOJTaskExecutionTriggerOutboxPublisher(
			repositoryGroup.SystemRepositorySupplier.GetOutboxRepository(),
		),
		authorizationService: authorizationService,
	}
}

func (s *OJTaskService) CreateTask(
	ctx context.Context,
	operatorID uint,
	req *request.CreateOJTaskReq,
) (*dtoresp.OJTaskCreateResp, error) {
	draft, err := s.validateDraft(ctx, req.Title, req.Description, req.Mode, req.ExecuteAt, req.OrgIDs, req.Items)
	if err != nil {
		return nil, err
	}
	if err := s.authorizeManageOrgIDs(ctx, operatorID, draft.OrgIDs); err != nil {
		return nil, err
	}

	var out *dtoresp.OJTaskCreateResp
	err = s.txRunner.InTx(ctx, func(tx any) error {
		created, innerErr := s.createTaskVersionTx(
			ctx,
			tx,
			nil,
			nil,
			1,
			operatorID,
			draft,
			resolveExecutionTrigger(draft.Mode),
		)
		if innerErr != nil {
			return innerErr
		}
		out = created
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (s *OJTaskService) UpdateTask(
	ctx context.Context,
	operatorID, taskID uint,
	req *request.UpdateOJTaskReq,
) error {
	draft, err := s.validateDraft(ctx, req.Title, req.Description, req.Mode, req.ExecuteAt, req.OrgIDs, req.Items)
	if err != nil {
		return err
	}

	return s.txRunner.InTx(ctx, func(tx any) error {
		txTaskRepo := s.taskRepo.WithTx(tx)
		txExecutionRepo := s.executionRepo.WithTx(tx)

		task, execution, currentOrgIDs, innerErr := s.loadEditableScheduledTaskTx(ctx, txTaskRepo, txExecutionRepo, taskID)
		if innerErr != nil {
			return innerErr
		}
		if err := s.authorizeManageOrgIDs(ctx, operatorID, mergeUintSlices(currentOrgIDs, draft.OrgIDs)); err != nil {
			return err
		}
		if draft.ExecuteAt == nil {
			return bizerrors.New(bizerrors.CodeOJTaskExecuteAtInvalid)
		}

		task.Title = draft.Title
		task.Description = draft.Description
		task.ExecuteAt = draft.ExecuteAt
		task.UpdatedBy = operatorID
		if err := txTaskRepo.Update(ctx, task); err != nil {
			return bizerrors.Wrap(bizerrors.CodeDBError, err)
		}
		if err := txTaskRepo.ReplaceOrgs(ctx, task.ID, buildTaskOrgRows(task.ID, draft.OrgIDs)); err != nil {
			return bizerrors.Wrap(bizerrors.CodeDBError, err)
		}
		if err := txTaskRepo.ReplaceItems(ctx, task.ID, buildTaskItemRows(task.ID, draft.Items)); err != nil {
			return bizerrors.Wrap(bizerrors.CodeDBError, err)
		}

		execution.PlannedAt = *draft.ExecuteAt
		execution.RequestedBy = operatorID
		if err := txExecutionRepo.Update(ctx, execution); err != nil {
			return bizerrors.Wrap(bizerrors.CodeDBError, err)
		}
		return nil
	})
}

func (s *OJTaskService) DeleteTask(ctx context.Context, operatorID, taskID uint) error {
	return s.txRunner.InTx(ctx, func(tx any) error {
		txTaskRepo := s.taskRepo.WithTx(tx)
		txExecutionRepo := s.executionRepo.WithTx(tx)

		task, execution, currentOrgIDs, err := s.loadEditableScheduledTaskTx(ctx, txTaskRepo, txExecutionRepo, taskID)
		if err != nil {
			return err
		}
		if err := s.authorizeManageOrgIDs(ctx, operatorID, currentOrgIDs); err != nil {
			return err
		}

		now := time.Now()
		task.Status = string(consts.OJTaskStatusDeleted)
		task.UpdatedBy = operatorID
		execution.Status = string(consts.OJTaskExecutionStatusCancelled)
		execution.FinishedAt = &now
		execution.ErrorMessage = ""

		if err := txTaskRepo.Update(ctx, task); err != nil {
			return bizerrors.Wrap(bizerrors.CodeDBError, err)
		}
		if err := txExecutionRepo.Update(ctx, execution); err != nil {
			return bizerrors.Wrap(bizerrors.CodeDBError, err)
		}
		return nil
	})
}

// ExecuteTaskNow 立即执行任务
func (s *OJTaskService) ExecuteTaskNow(
	ctx context.Context,
	operatorID, taskID uint,
) (*dtoresp.OJTaskCreateResp, error) {
	var out *dtoresp.OJTaskCreateResp
	err := s.txRunner.InTx(ctx, func(tx any) error {
		txTaskRepo := s.taskRepo.WithTx(tx)
		txExecutionRepo := s.executionRepo.WithTx(tx)

		task, execution, currentOrgIDs, err := s.loadEditableScheduledTaskTx(ctx, txTaskRepo, txExecutionRepo, taskID)
		if err != nil {
			return err
		}
		if err := s.authorizeManageOrgIDs(ctx, operatorID, currentOrgIDs); err != nil {
			return err
		}

		now := time.Now()
		task.Status = string(consts.OJTaskStatusQueued)
		task.UpdatedBy = operatorID
		execution.Status = string(consts.OJTaskExecutionStatusQueued)
		execution.TriggerType = string(consts.OJTaskExecutionTriggerExecuteNow)
		execution.PlannedAt = now
		execution.RequestedBy = operatorID
		execution.StartedAt = nil
		execution.FinishedAt = nil
		execution.ErrorMessage = ""
		execution.TotalUserCount = 0
		execution.CompletedUserCount = 0
		execution.PendingUserCount = 0
		execution.TotalItemCount = 0
		execution.CompletedItemCount = 0
		execution.PendingItemCount = 0

		if err := txTaskRepo.Update(ctx, task); err != nil {
			return bizerrors.Wrap(bizerrors.CodeDBError, err)
		}
		if err := txExecutionRepo.Update(ctx, execution); err != nil {
			return bizerrors.Wrap(bizerrors.CodeDBError, err)
		}
		if err := s.publishQueuedExecutionTriggerInTx(ctx, tx, execution.ID, task.ID); err != nil {
			return err
		}
		out = &dtoresp.OJTaskCreateResp{
			TaskID:      task.ID,
			ExecutionID: execution.ID,
			Status:      task.Status,
		}
		return nil
	})
	return out, err
}

func (s *OJTaskService) ReviseTask(
	ctx context.Context,
	operatorID, taskID uint,
	req *request.ReviseOJTaskReq,
) (*dtoresp.OJTaskCreateResp, error) {
	draft, err := s.validateDraft(ctx, req.Title, req.Description, req.Mode, req.ExecuteAt, req.OrgIDs, req.Items)
	if err != nil {
		return nil, err
	}

	sourceTask, sourceExecution, sourceOrgIDs, err := s.loadTaskWithExecution(ctx, taskID)
	if err != nil {
		return nil, err
	}
	if sourceTask.Status == string(consts.OJTaskStatusDeleted) {
		return nil, bizerrors.New(bizerrors.CodeOJTaskDeleted)
	}
	if sourceExecution == nil {
		return nil, bizerrors.New(bizerrors.CodeOJTaskExecutionNotFound)
	}
	if err := s.authorizeManageOrgIDs(ctx, operatorID, mergeUintSlices(sourceOrgIDs, draft.OrgIDs)); err != nil {
		return nil, err
	}

	var out *dtoresp.OJTaskCreateResp
	err = s.txRunner.InTx(ctx, func(tx any) error {
		rootID := effectiveRootTaskID(sourceTask)
		versionNo, innerErr := s.nextVersionNoTx(ctx, tx, rootID)
		if innerErr != nil {
			return innerErr
		}
		created, innerErr := s.createTaskVersionTx(
			ctx,
			tx,
			&rootID,
			&sourceTask.ID,
			versionNo,
			operatorID,
			draft,
			resolveExecutionTrigger(draft.Mode),
		)
		if innerErr != nil {
			return innerErr
		}
		out = created
		return nil
	})
	return out, err
}

func (s *OJTaskService) RetryTask(
	ctx context.Context,
	operatorID, taskID uint,
) (*dtoresp.OJTaskCreateResp, error) {
	sourceTask, sourceExecution, sourceOrgIDs, err := s.loadTaskWithExecution(ctx, taskID)
	if err != nil {
		return nil, err
	}
	if sourceTask.Status == string(consts.OJTaskStatusDeleted) {
		return nil, bizerrors.New(bizerrors.CodeOJTaskDeleted)
	}
	if sourceExecution == nil {
		return nil, bizerrors.New(bizerrors.CodeOJTaskExecutionNotFound)
	}
	if sourceExecution.Status != string(consts.OJTaskExecutionStatusSucceeded) &&
		sourceExecution.Status != string(consts.OJTaskExecutionStatusFailed) {
		return nil, bizerrors.NewWithMsg(bizerrors.CodeOJTaskNotEditable, "仅已完成或已失败的版本允许重试")
	}
	if err := s.authorizeManageOrgIDs(ctx, operatorID, sourceOrgIDs); err != nil {
		return nil, err
	}

	items, err := s.taskRepo.ListItemsByTaskID(ctx, taskID)
	if err != nil {
		return nil, bizerrors.Wrap(bizerrors.CodeDBError, err)
	}
	draft := validatedOJTaskDraft{
		Title:       sourceTask.Title,
		Description: sourceTask.Description,
		Mode:        string(consts.OJTaskModeImmediate),
		OrgIDs:      sourceOrgIDs,
		Items:       taskItemsToValidated(items),
	}

	var out *dtoresp.OJTaskCreateResp
	err = s.txRunner.InTx(ctx, func(tx any) error {
		rootID := effectiveRootTaskID(sourceTask)
		versionNo, innerErr := s.nextVersionNoTx(ctx, tx, rootID)
		if innerErr != nil {
			return innerErr
		}
		created, innerErr := s.createTaskVersionTx(
			ctx,
			tx,
			&rootID,
			&sourceTask.ID,
			versionNo,
			operatorID,
			draft,
			consts.OJTaskExecutionTriggerRetry,
		)
		if innerErr != nil {
			return innerErr
		}
		out = created
		return nil
	})
	return out, err
}

func (s *OJTaskService) GetVisibleTaskList(
	ctx context.Context,
	userID uint,
	req *request.OJTaskListReq,
) ([]*dtoresp.OJTaskListItemResp, int64, error) {
	req = normalizeOJTaskListReq(req)

	isSuperAdmin, err := s.isSuperAdmin(ctx, userID)
	if err != nil {
		return nil, 0, err
	}
	rows, total, err := s.taskRepo.ListVisibleTasks(ctx, userID, isSuperAdmin, req)
	if err != nil {
		return nil, 0, bizerrors.Wrap(bizerrors.CodeDBError, err)
	}

	items := make([]*dtoresp.OJTaskListItemResp, 0, len(rows))
	for _, row := range rows {
		items = append(items, mapTaskListItem(row))
	}
	return items, total, nil
}

func (s *OJTaskService) GetTaskDetail(
	ctx context.Context,
	userID, taskID uint,
) (*dtoresp.OJTaskDetailResp, error) {
	row, _, err := s.getVisibleTaskOrError(ctx, userID, taskID)
	if err != nil {
		return nil, err
	}

	orgs, err := s.taskRepo.ListTaskOrgsWithNames(ctx, taskID)
	if err != nil {
		return nil, bizerrors.Wrap(bizerrors.CodeDBError, err)
	}
	items, err := s.taskRepo.ListItemsByTaskID(ctx, taskID)
	if err != nil {
		return nil, bizerrors.Wrap(bizerrors.CodeDBError, err)
	}
	return mapTaskDetail(row, orgs, items), nil
}

func (s *OJTaskService) GetTaskVersions(
	ctx context.Context,
	userID, taskID uint,
) (*dtoresp.OJTaskVersionListResp, error) {
	row, isSuperAdmin, err := s.getVisibleTaskOrError(ctx, userID, taskID)
	if err != nil {
		return nil, err
	}

	versions, err := s.taskRepo.ListVisibleVersions(ctx, userID, isSuperAdmin, row.RootTaskID)
	if err != nil {
		return nil, bizerrors.Wrap(bizerrors.CodeDBError, err)
	}

	resp := &dtoresp.OJTaskVersionListResp{
		RootTaskID: row.RootTaskID,
		Versions:   make([]*dtoresp.OJTaskVersionItemResp, 0, len(versions)),
	}
	for _, item := range versions {
		resp.Versions = append(resp.Versions, mapTaskVersion(item))
	}
	return resp, nil
}

func (s *OJTaskService) GetTaskExecutionDetail(
	ctx context.Context,
	userID, taskID, executionID uint,
) (*dtoresp.OJTaskExecutionResp, error) {
	row, _, err := s.getVisibleExecutionOrError(ctx, userID, taskID, executionID)
	if err != nil {
		return nil, err
	}
	return mapExecutionResp(row), nil
}

func (s *OJTaskService) GetTaskExecutionUsers(
	ctx context.Context,
	userID, taskID, executionID uint,
	req *request.OJTaskExecutionUserListReq,
) (*dtoresp.OJTaskExecutionUserListResp, error) {
	req = normalizeOJTaskExecutionUserListReq(req)

	if _, _, err := s.getVisibleExecutionOrError(ctx, userID, taskID, executionID); err != nil {
		return nil, err
	}

	isSuperAdmin, err := s.isSuperAdmin(ctx, userID)
	if err != nil {
		return nil, err
	}
	rows, total, err := s.executionRepo.ListVisibleExecutionUsers(ctx, userID, isSuperAdmin, taskID, executionID, req)
	if err != nil {
		return nil, bizerrors.Wrap(bizerrors.CodeDBError, err)
	}
	orgMap, err := s.executionUserOrgMap(ctx, rows)
	if err != nil {
		return nil, err
	}

	resp := &dtoresp.OJTaskExecutionUserListResp{
		List:     make([]*dtoresp.OJTaskExecutionUserSummaryResp, 0, len(rows)),
		Total:    total,
		Page:     req.Page,
		PageSize: req.PageSize,
	}
	for _, row := range rows {
		resp.List = append(resp.List, mapExecutionUserSummary(row, orgMap[row.ExecutionUserID]))
	}
	return resp, nil
}

func (s *OJTaskService) GetTaskExecutionUserDetail(
	ctx context.Context,
	userID, taskID, executionID, targetUserID uint,
) (*dtoresp.OJTaskExecutionUserDetailResp, error) {
	if _, _, err := s.getVisibleExecutionOrError(ctx, userID, taskID, executionID); err != nil {
		return nil, err
	}

	isSuperAdmin, err := s.isSuperAdmin(ctx, userID)
	if err != nil {
		return nil, err
	}
	row, err := s.executionRepo.GetVisibleExecutionUser(ctx, userID, isSuperAdmin, taskID, executionID, targetUserID)
	if err != nil {
		return nil, bizerrors.Wrap(bizerrors.CodeDBError, err)
	}
	if row == nil {
		return nil, bizerrors.New(bizerrors.CodeUserNotFound)
	}

	orgs, err := s.executionRepo.ListExecutionUserOrgs(ctx, row.ExecutionUserID)
	if err != nil {
		return nil, bizerrors.Wrap(bizerrors.CodeDBError, err)
	}
	items, err := s.executionRepo.ListExecutionUserItems(ctx, executionID, targetUserID)
	if err != nil {
		return nil, bizerrors.Wrap(bizerrors.CodeDBError, err)
	}
	return mapExecutionUserDetail(row, orgs, items), nil
}

func (s *OJTaskService) validateDraft(
	ctx context.Context,
	title, description, mode string,
	executeAt *time.Time,
	orgIDs []uint,
	items []request.OJTaskItemReq,
) (validatedOJTaskDraft, error) {
	trimmedTitle := strings.TrimSpace(title)
	if trimmedTitle == "" {
		return validatedOJTaskDraft{}, bizerrors.NewWithMsg(bizerrors.CodeInvalidParams, "任务标题不能为空")
	}

	normalizedMode := strings.TrimSpace(mode)
	if !consts.IsValidOJTaskMode(normalizedMode) {
		return validatedOJTaskDraft{}, bizerrors.New(bizerrors.CodeInvalidParams)
	}

	now := time.Now()
	var normalizedExecuteAt *time.Time
	switch normalizedMode {
	case string(consts.OJTaskModeImmediate):
		if executeAt != nil {
			return validatedOJTaskDraft{}, bizerrors.NewWithMsg(
				bizerrors.CodeOJTaskExecuteAtInvalid,
				"立即任务不允许传 execute_at",
			)
		}
	case string(consts.OJTaskModeScheduled):
		if executeAt == nil {
			return validatedOJTaskDraft{}, bizerrors.NewWithMsg(
				bizerrors.CodeOJTaskExecuteAtInvalid,
				"定时任务必须传 execute_at",
			)
		}
		if !executeAt.After(now) {
			return validatedOJTaskDraft{}, bizerrors.NewWithMsg(
				bizerrors.CodeOJTaskExecuteAtInvalid,
				"execute_at 必须是未来时间",
			)
		}
		value := executeAt.UTC()
		normalizedExecuteAt = &value
	}

	validatedOrgIDs, err := s.validateOrgIDs(ctx, orgIDs)
	if err != nil {
		return validatedOJTaskDraft{}, err
	}
	validatedItems, err := s.validateItems(ctx, items)
	if err != nil {
		return validatedOJTaskDraft{}, err
	}

	return validatedOJTaskDraft{
		Title:       trimmedTitle,
		Description: strings.TrimSpace(description),
		Mode:        normalizedMode,
		ExecuteAt:   normalizedExecuteAt,
		OrgIDs:      validatedOrgIDs,
		Items:       validatedItems,
	}, nil
}

func (s *OJTaskService) validateOrgIDs(ctx context.Context, orgIDs []uint) ([]uint, error) {
	if len(orgIDs) == 0 {
		return nil, bizerrors.NewWithMsg(bizerrors.CodeInvalidParams, "org_ids 不能为空")
	}

	seen := make(map[uint]struct{}, len(orgIDs))
	normalized := make([]uint, 0, len(orgIDs))
	for _, orgID := range orgIDs {
		if orgID == 0 {
			return nil, bizerrors.New(bizerrors.CodeInvalidParams)
		}
		if _, ok := seen[orgID]; ok {
			continue
		}
		org, err := s.orgRepo.GetByID(ctx, orgID)
		if err != nil {
			if stderrors.Is(err, gorm.ErrRecordNotFound) {
				return nil, bizerrors.New(bizerrors.CodeOrgNotFound)
			}
			return nil, bizerrors.Wrap(bizerrors.CodeDBError, err)
		}
		if org == nil || org.ID == 0 {
			return nil, bizerrors.New(bizerrors.CodeOrgNotFound)
		}
		seen[orgID] = struct{}{}
		normalized = append(normalized, orgID)
	}
	if len(normalized) == 0 {
		return nil, bizerrors.NewWithMsg(bizerrors.CodeInvalidParams, "org_ids 不能为空")
	}
	return normalized, nil
}

func (s *OJTaskService) validateItems(
	ctx context.Context,
	items []request.OJTaskItemReq,
) ([]validatedOJTaskItem, error) {
	if len(items) == 0 {
		return nil, bizerrors.NewWithMsg(bizerrors.CodeInvalidParams, "items 不能为空")
	}

	seen := make(map[string]struct{}, len(items))
	normalized := make([]validatedOJTaskItem, 0, len(items))
	for _, item := range items {
		platform := strings.TrimSpace(item.Platform)
		questionCode := strings.TrimSpace(item.QuestionCode)
		if platform == "" || questionCode == "" {
			return nil, bizerrors.New(bizerrors.CodeInvalidParams)
		}
		key := platform + ":" + questionCode
		if _, ok := seen[key]; ok {
			continue
		}
		validated, err := s.validateQuestion(ctx, platform, questionCode)
		if err != nil {
			return nil, err
		}
		validated.SortNo = len(normalized) + 1
		normalized = append(normalized, validated)
		seen[key] = struct{}{}
	}

	if len(normalized) == 0 {
		return nil, bizerrors.NewWithMsg(bizerrors.CodeInvalidParams, "items 不能为空")
	}
	return normalized, nil
}

func (s *OJTaskService) validateQuestion(
	ctx context.Context,
	platform, questionCode string,
) (validatedOJTaskItem, error) {
	switch platform {
	case consts.OJPlatformLuogu:
		question, err := s.luoguQuestionRepo.GetByPID(ctx, questionCode)
		if err != nil {
			if stderrors.Is(err, gorm.ErrRecordNotFound) {
				return validatedOJTaskItem{}, bizerrors.New(bizerrors.CodeOJTaskQuestionNotFound)
			}
			return validatedOJTaskItem{}, bizerrors.Wrap(bizerrors.CodeDBError, err)
		}
		if question == nil || question.ID == 0 {
			return validatedOJTaskItem{}, bizerrors.New(bizerrors.CodeOJTaskQuestionNotFound)
		}
		return validatedOJTaskItem{
			Platform:              platform,
			QuestionCode:          questionCode,
			PlatformQuestionID:    question.ID,
			QuestionTitleSnapshot: question.Title,
		}, nil
	case consts.OJPlatformLeetcode:
		question, err := s.leetcodeQuestionRepo.GetByTitleSlug(ctx, questionCode)
		if err != nil {
			if stderrors.Is(err, gorm.ErrRecordNotFound) {
				return validatedOJTaskItem{}, bizerrors.New(bizerrors.CodeOJTaskQuestionNotFound)
			}
			return validatedOJTaskItem{}, bizerrors.Wrap(bizerrors.CodeDBError, err)
		}
		if question == nil || question.ID == 0 {
			return validatedOJTaskItem{}, bizerrors.New(bizerrors.CodeOJTaskQuestionNotFound)
		}
		return validatedOJTaskItem{
			Platform:              platform,
			QuestionCode:          questionCode,
			PlatformQuestionID:    question.ID,
			QuestionTitleSnapshot: question.Title,
		}, nil
	case consts.OJPlatformLanqiao:
		problemID, err := strconv.Atoi(questionCode)
		if err != nil || problemID <= 0 {
			return validatedOJTaskItem{}, bizerrors.NewWithMsg(
				bizerrors.CodeOJTaskQuestionNotFound,
				"蓝桥题目编码非法",
			)
		}
		question, err := s.lanqiaoQuestionRepo.GetByProblemID(ctx, problemID)
		if err != nil {
			if stderrors.Is(err, gorm.ErrRecordNotFound) {
				return validatedOJTaskItem{}, bizerrors.New(bizerrors.CodeOJTaskQuestionNotFound)
			}
			return validatedOJTaskItem{}, bizerrors.Wrap(bizerrors.CodeDBError, err)
		}
		if question == nil || question.ID == 0 {
			return validatedOJTaskItem{}, bizerrors.New(bizerrors.CodeOJTaskQuestionNotFound)
		}
		return validatedOJTaskItem{
			Platform:              platform,
			QuestionCode:          questionCode,
			PlatformQuestionID:    question.ID,
			QuestionTitleSnapshot: question.Title,
		}, nil
	default:
		return validatedOJTaskItem{}, bizerrors.New(bizerrors.CodeInvalidParams)
	}
}

func (s *OJTaskService) authorizeManageOrgIDs(
	ctx context.Context,
	operatorID uint,
	orgIDs []uint,
) error {
	for _, orgID := range normalizeUintSlice(orgIDs) {
		if err := s.authorizationService.AuthorizeOrgCapability(
			ctx,
			operatorID,
			orgID,
			consts.CapabilityCodeOJTaskManage,
		); err != nil {
			return err
		}
	}
	return nil
}

func (s *OJTaskService) createTaskVersionTx(
	ctx context.Context,
	tx any,
	rootTaskID, parentTaskID *uint,
	versionNo int,
	operatorID uint,
	draft validatedOJTaskDraft,
	trigger consts.OJTaskExecutionTriggerType,
) (*dtoresp.OJTaskCreateResp, error) {
	txTaskRepo := s.taskRepo.WithTx(tx)
	txExecutionRepo := s.executionRepo.WithTx(tx)

	taskStatus := string(consts.OJTaskStatusScheduled)
	executionStatus := string(consts.OJTaskExecutionStatusScheduled)
	plannedAt := time.Now().UTC()
	if draft.Mode == string(consts.OJTaskModeImmediate) {
		taskStatus = string(consts.OJTaskStatusQueued)
		executionStatus = string(consts.OJTaskExecutionStatusQueued)
	} else if draft.ExecuteAt != nil {
		plannedAt = *draft.ExecuteAt
	}

	task := &entity.OJTask{
		RootTaskID:   rootTaskID,
		ParentTaskID: parentTaskID,
		VersionNo:    versionNo,
		Title:        draft.Title,
		Description:  draft.Description,
		Mode:         draft.Mode,
		Status:       taskStatus,
		ExecuteAt:    draft.ExecuteAt,
		CreatedBy:    operatorID,
		UpdatedBy:    operatorID,
	}
	if err := txTaskRepo.Create(ctx, task); err != nil {
		return nil, bizerrors.Wrap(bizerrors.CodeDBError, err)
	}

	if task.RootTaskID == nil || *task.RootTaskID == 0 {
		if err := txTaskRepo.UpdateRootTaskID(ctx, task.ID, task.ID); err != nil {
			return nil, bizerrors.Wrap(bizerrors.CodeDBError, err)
		}
		task.RootTaskID = &task.ID
	}

	if err := txTaskRepo.CreateOrgs(ctx, buildTaskOrgRows(task.ID, draft.OrgIDs)); err != nil {
		return nil, bizerrors.Wrap(bizerrors.CodeDBError, err)
	}
	if err := txTaskRepo.CreateItems(ctx, buildTaskItemRows(task.ID, draft.Items)); err != nil {
		return nil, bizerrors.Wrap(bizerrors.CodeDBError, err)
	}

	execution := &entity.OJTaskExecution{
		TaskID:      task.ID,
		TriggerType: string(trigger),
		PlannedAt:   plannedAt,
		RequestedBy: operatorID,
		Status:      executionStatus,
	}
	if err := txExecutionRepo.Create(ctx, execution); err != nil {
		return nil, bizerrors.Wrap(bizerrors.CodeDBError, err)
	}
	if execution.Status == string(consts.OJTaskExecutionStatusQueued) {
		if err := s.publishQueuedExecutionTriggerInTx(ctx, tx, execution.ID, task.ID); err != nil {
			return nil, err
		}
	}

	return &dtoresp.OJTaskCreateResp{
		TaskID:      task.ID,
		ExecutionID: execution.ID,
		Status:      task.Status,
	}, nil
}

func (s *OJTaskService) loadEditableScheduledTaskTx(
	ctx context.Context,
	taskRepo interfaces.OJTaskRepository,
	executionRepo interfaces.OJTaskExecutionRepository,
	taskID uint,
) (*entity.OJTask, *entity.OJTaskExecution, []uint, error) {
	task, err := taskRepo.GetByIDForUpdate(ctx, taskID)
	if err != nil {
		return nil, nil, nil, bizerrors.Wrap(bizerrors.CodeDBError, err)
	}
	if task == nil {
		return nil, nil, nil, bizerrors.New(bizerrors.CodeOJTaskNotFound)
	}
	if task.Status == string(consts.OJTaskStatusDeleted) {
		return nil, nil, nil, bizerrors.New(bizerrors.CodeOJTaskDeleted)
	}

	execution, err := executionRepo.GetByTaskIDForUpdate(ctx, taskID)
	if err != nil {
		return nil, nil, nil, bizerrors.Wrap(bizerrors.CodeDBError, err)
	}
	if execution == nil {
		return nil, nil, nil, bizerrors.New(bizerrors.CodeOJTaskExecutionNotFound)
	}
	if task.Mode != string(consts.OJTaskModeScheduled) ||
		task.Status != string(consts.OJTaskStatusScheduled) ||
		execution.Status != string(consts.OJTaskExecutionStatusScheduled) {
		return nil, nil, nil, bizerrors.New(bizerrors.CodeOJTaskNotEditable)
	}

	orgs, err := taskRepo.ListOrgsByTaskID(ctx, taskID)
	if err != nil {
		return nil, nil, nil, bizerrors.Wrap(bizerrors.CodeDBError, err)
	}
	return task, execution, taskOrgIDs(orgs), nil
}

func (s *OJTaskService) loadTaskWithExecution(
	ctx context.Context,
	taskID uint,
) (*entity.OJTask, *entity.OJTaskExecution, []uint, error) {
	task, err := s.taskRepo.GetByID(ctx, taskID)
	if err != nil {
		return nil, nil, nil, bizerrors.Wrap(bizerrors.CodeDBError, err)
	}
	if task == nil {
		return nil, nil, nil, bizerrors.New(bizerrors.CodeOJTaskNotFound)
	}

	execution, err := s.executionRepo.GetByTaskID(ctx, taskID)
	if err != nil {
		return nil, nil, nil, bizerrors.Wrap(bizerrors.CodeDBError, err)
	}
	orgs, err := s.taskRepo.ListOrgsByTaskID(ctx, taskID)
	if err != nil {
		return nil, nil, nil, bizerrors.Wrap(bizerrors.CodeDBError, err)
	}
	return task, execution, taskOrgIDs(orgs), nil
}

func (s *OJTaskService) nextVersionNoTx(
	ctx context.Context,
	tx any,
	rootTaskID uint,
) (int, error) {
	task, err := s.taskRepo.WithTx(tx).GetLatestVersionByRootIDForUpdate(ctx, rootTaskID)
	if err != nil {
		return 0, bizerrors.Wrap(bizerrors.CodeDBError, err)
	}
	if task == nil {
		return 1, nil
	}
	return task.VersionNo + 1, nil
}

func (s *OJTaskService) isSuperAdmin(ctx context.Context, userID uint) (bool, error) {
	ok, err := s.authorizationService.IsSuperAdmin(ctx, userID)
	if err != nil {
		return false, bizerrors.Wrap(bizerrors.CodeDBError, err)
	}
	return ok, nil
}

func (s *OJTaskService) getVisibleTaskOrError(
	ctx context.Context,
	userID, taskID uint,
) (*readmodel.OJTaskVisibleTask, bool, error) {
	task, err := s.taskRepo.GetByID(ctx, taskID)
	if err != nil {
		return nil, false, bizerrors.Wrap(bizerrors.CodeDBError, err)
	}
	if task == nil {
		return nil, false, bizerrors.New(bizerrors.CodeOJTaskNotFound)
	}

	isSuperAdmin, err := s.isSuperAdmin(ctx, userID)
	if err != nil {
		return nil, false, err
	}
	row, err := s.taskRepo.GetVisibleTask(ctx, userID, isSuperAdmin, taskID)
	if err != nil {
		return nil, false, bizerrors.Wrap(bizerrors.CodeDBError, err)
	}
	if row == nil {
		return nil, false, bizerrors.New(bizerrors.CodeOJTaskVisibleDenied)
	}
	return row, isSuperAdmin, nil
}

func (s *OJTaskService) getVisibleExecutionOrError(
	ctx context.Context,
	userID, taskID, executionID uint,
) (*readmodel.OJTaskVisibleTask, bool, error) {
	task, err := s.taskRepo.GetByID(ctx, taskID)
	if err != nil {
		return nil, false, bizerrors.Wrap(bizerrors.CodeDBError, err)
	}
	if task == nil {
		return nil, false, bizerrors.New(bizerrors.CodeOJTaskNotFound)
	}

	execution, err := s.executionRepo.GetByID(ctx, executionID)
	if err != nil {
		return nil, false, bizerrors.Wrap(bizerrors.CodeDBError, err)
	}
	if execution == nil || execution.TaskID != taskID {
		return nil, false, bizerrors.New(bizerrors.CodeOJTaskExecutionNotFound)
	}

	isSuperAdmin, err := s.isSuperAdmin(ctx, userID)
	if err != nil {
		return nil, false, err
	}
	row, err := s.executionRepo.GetVisibleExecutionDetail(ctx, userID, isSuperAdmin, taskID, executionID)
	if err != nil {
		return nil, false, bizerrors.Wrap(bizerrors.CodeDBError, err)
	}
	if row == nil {
		return nil, false, bizerrors.New(bizerrors.CodeOJTaskVisibleDenied)
	}
	return row, isSuperAdmin, nil
}

func (s *OJTaskService) executionUserOrgMap(
	ctx context.Context,
	rows []*readmodel.OJTaskExecutionUserListItem,
) (map[uint][]*dtoresp.OJTaskExecutionUserOrgResp, error) {
	result := make(map[uint][]*dtoresp.OJTaskExecutionUserOrgResp, len(rows))
	if len(rows) == 0 {
		return result, nil
	}

	executionUserIDs := make([]uint, 0, len(rows))
	for _, row := range rows {
		if row == nil || row.ExecutionUserID == 0 {
			continue
		}
		executionUserIDs = append(executionUserIDs, row.ExecutionUserID)
	}

	orgs, err := s.executionRepo.ListExecutionUserOrgsByExecutionUserIDs(ctx, executionUserIDs)
	if err != nil {
		return nil, bizerrors.Wrap(bizerrors.CodeDBError, err)
	}
	for _, item := range orgs {
		if item == nil {
			continue
		}
		result[item.ExecutionUserID] = append(result[item.ExecutionUserID], &dtoresp.OJTaskExecutionUserOrgResp{
			OrgID:           item.OrgID,
			OrgNameSnapshot: item.OrgNameSnapshot,
		})
	}
	return result, nil
}

func resolveExecutionTrigger(mode string) consts.OJTaskExecutionTriggerType {
	if mode == string(consts.OJTaskModeImmediate) {
		return consts.OJTaskExecutionTriggerCreateImmediate
	}
	return consts.OJTaskExecutionTriggerScheduleDue
}

func normalizeOJTaskListReq(req *request.OJTaskListReq) *request.OJTaskListReq {
	if req == nil {
		req = &request.OJTaskListReq{}
	}
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 10
	}
	return req
}

func normalizeOJTaskExecutionUserListReq(req *request.OJTaskExecutionUserListReq) *request.OJTaskExecutionUserListReq {
	if req == nil {
		req = &request.OJTaskExecutionUserListReq{}
	}
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 10
	}
	return req
}

func effectiveRootTaskID(task *entity.OJTask) uint {
	if task == nil {
		return 0
	}
	if task.RootTaskID != nil && *task.RootTaskID > 0 {
		return *task.RootTaskID
	}
	return task.ID
}

func mergeUintSlices(left, right []uint) []uint {
	merged := make([]uint, 0, len(left)+len(right))
	merged = append(merged, left...)
	merged = append(merged, right...)
	return normalizeUintSlice(merged)
}

func normalizeUintSlice(values []uint) []uint {
	seen := make(map[uint]struct{}, len(values))
	out := make([]uint, 0, len(values))
	for _, value := range values {
		if value == 0 {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func (s *OJTaskService) publishQueuedExecutionTriggerInTx(
	ctx context.Context,
	tx any,
	executionID, taskID uint,
) error {
	if executionID == 0 || taskID == 0 {
		return nil
	}
	if s.triggerPublisher == nil {
		return bizerrors.NewWithMsg(bizerrors.CodeInternalError, "oj task trigger publisher not initialized")
	}
	if err := s.triggerPublisher.PublishInTx(ctx, tx, &eventdto.OJTaskExecutionTriggerEvent{
		ExecutionID: executionID,
		TaskID:      taskID,
	}); err != nil {
		return bizerrors.Wrap(bizerrors.CodeDBError, err)
	}
	return nil
}

func buildTaskOrgRows(taskID uint, orgIDs []uint) []*entity.OJTaskOrg {
	rows := make([]*entity.OJTaskOrg, 0, len(orgIDs))
	for _, orgID := range orgIDs {
		rows = append(rows, &entity.OJTaskOrg{TaskID: taskID, OrgID: orgID})
	}
	return rows
}

func buildTaskItemRows(taskID uint, items []validatedOJTaskItem) []*entity.OJTaskItem {
	rows := make([]*entity.OJTaskItem, 0, len(items))
	for _, item := range items {
		rows = append(rows, &entity.OJTaskItem{
			TaskID:                taskID,
			SortNo:                item.SortNo,
			Platform:              item.Platform,
			QuestionCode:          item.QuestionCode,
			PlatformQuestionID:    item.PlatformQuestionID,
			QuestionTitleSnapshot: item.QuestionTitleSnapshot,
		})
	}
	return rows
}

func taskItemsToValidated(items []*entity.OJTaskItem) []validatedOJTaskItem {
	out := make([]validatedOJTaskItem, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		out = append(out, validatedOJTaskItem{
			SortNo:                item.SortNo,
			Platform:              item.Platform,
			QuestionCode:          item.QuestionCode,
			PlatformQuestionID:    item.PlatformQuestionID,
			QuestionTitleSnapshot: item.QuestionTitleSnapshot,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].SortNo == out[j].SortNo {
			return out[i].QuestionCode < out[j].QuestionCode
		}
		return out[i].SortNo < out[j].SortNo
	})
	for i := range out {
		out[i].SortNo = i + 1
	}
	return out
}

func taskOrgIDs(orgs []*entity.OJTaskOrg) []uint {
	out := make([]uint, 0, len(orgs))
	for _, org := range orgs {
		if org == nil || org.OrgID == 0 {
			continue
		}
		out = append(out, org.OrgID)
	}
	return normalizeUintSlice(out)
}

func formatTimePtr(value *time.Time) string {
	if value == nil || value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339)
}

func formatTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339)
}

func mapTaskListItem(row *readmodel.OJTaskListItem) *dtoresp.OJTaskListItemResp {
	if row == nil {
		return nil
	}
	return &dtoresp.OJTaskListItemResp{
		TaskID:             row.TaskID,
		RootTaskID:         row.RootTaskID,
		ParentTaskID:       row.ParentTaskID,
		VersionNo:          row.VersionNo,
		Title:              row.Title,
		Description:        row.Description,
		Mode:               row.Mode,
		Status:             row.Status,
		ExecuteAt:          formatTimePtr(row.ExecuteAt),
		CreatedBy:          row.CreatedBy,
		UpdatedBy:          row.UpdatedBy,
		CreatedAt:          formatTime(row.CreatedAt),
		UpdatedAt:          formatTime(row.UpdatedAt),
		ExecutionID:        row.ExecutionID,
		ExecutionStatus:    row.ExecutionStatus,
		TotalUserCount:     row.TotalUserCount,
		CompletedUserCount: row.CompletedUserCount,
		PendingUserCount:   row.PendingUserCount,
		TotalItemCount:     row.TotalItemCount,
		CompletedItemCount: row.CompletedItemCount,
		PendingItemCount:   row.PendingItemCount,
		OrgCount:           row.OrgCount,
		ItemCount:          row.ItemCount,
	}
}

func mapTaskDetail(
	row *readmodel.OJTaskVisibleTask,
	orgs []*readmodel.OJTaskOrgInfo,
	items []*entity.OJTaskItem,
) *dtoresp.OJTaskDetailResp {
	if row == nil {
		return nil
	}

	resp := &dtoresp.OJTaskDetailResp{
		TaskID:       row.TaskID,
		RootTaskID:   row.RootTaskID,
		ParentTaskID: row.ParentTaskID,
		VersionNo:    row.VersionNo,
		Title:        row.Title,
		Description:  row.Description,
		Mode:         row.Mode,
		Status:       row.Status,
		ExecuteAt:    formatTimePtr(row.ExecuteAt),
		CreatedBy:    row.CreatedBy,
		UpdatedBy:    row.UpdatedBy,
		CreatedAt:    formatTime(row.CreatedAt),
		UpdatedAt:    formatTime(row.UpdatedAt),
		Orgs:         make([]*dtoresp.OJTaskOrgItemResp, 0, len(orgs)),
		Items:        make([]*dtoresp.OJTaskItemResp, 0, len(items)),
	}

	for _, org := range orgs {
		if org == nil {
			continue
		}
		resp.Orgs = append(resp.Orgs, &dtoresp.OJTaskOrgItemResp{OrgID: org.OrgID, OrgName: org.OrgName})
	}
	for _, item := range items {
		if item == nil {
			continue
		}
		resp.Items = append(resp.Items, &dtoresp.OJTaskItemResp{
			ID:                    item.ID,
			SortNo:                item.SortNo,
			Platform:              item.Platform,
			QuestionCode:          item.QuestionCode,
			PlatformQuestionID:    item.PlatformQuestionID,
			QuestionTitleSnapshot: item.QuestionTitleSnapshot,
		})
	}
	if row.ExecutionID > 0 {
		resp.CurrentExecution = mapExecutionResp(row)
	}
	return resp
}

func mapTaskVersion(item *readmodel.OJTaskVersionItem) *dtoresp.OJTaskVersionItemResp {
	if item == nil {
		return nil
	}
	return &dtoresp.OJTaskVersionItemResp{
		TaskID:          item.TaskID,
		RootTaskID:      item.RootTaskID,
		ParentTaskID:    item.ParentTaskID,
		VersionNo:       item.VersionNo,
		Title:           item.Title,
		Mode:            item.Mode,
		Status:          item.Status,
		ExecuteAt:       formatTimePtr(item.ExecuteAt),
		CreatedAt:       formatTime(item.CreatedAt),
		ExecutionID:     item.ExecutionID,
		ExecutionStatus: item.ExecutionStatus,
	}
}

func mapExecutionResp(row *readmodel.OJTaskVisibleTask) *dtoresp.OJTaskExecutionResp {
	if row == nil {
		return nil
	}
	return &dtoresp.OJTaskExecutionResp{
		ExecutionID:        row.ExecutionID,
		TaskID:             row.TaskID,
		TriggerType:        row.TriggerType,
		RequestedBy:        row.RequestedBy,
		Status:             row.ExecutionStatus,
		PlannedAt:          formatTimePtr(row.PlannedAt),
		StartedAt:          formatTimePtr(row.StartedAt),
		FinishedAt:         formatTimePtr(row.FinishedAt),
		ErrorMessage:       row.ErrorMessage,
		TotalUserCount:     row.TotalUserCount,
		CompletedUserCount: row.CompletedUserCount,
		PendingUserCount:   row.PendingUserCount,
		TotalItemCount:     row.TotalItemCount,
		CompletedItemCount: row.CompletedItemCount,
		PendingItemCount:   row.PendingItemCount,
	}
}

func mapExecutionUserSummary(
	row *readmodel.OJTaskExecutionUserListItem,
	orgs []*dtoresp.OJTaskExecutionUserOrgResp,
) *dtoresp.OJTaskExecutionUserSummaryResp {
	if row == nil {
		return nil
	}
	return &dtoresp.OJTaskExecutionUserSummaryResp{
		UserID:             row.UserID,
		UserUUIDSnapshot:   row.UserUUIDSnapshot,
		UsernameSnapshot:   row.UsernameSnapshot,
		AvatarSnapshot:     row.AvatarSnapshot,
		UserStatusSnapshot: row.UserStatusSnapshot,
		CompletedItemCount: row.CompletedItemCount,
		PendingItemCount:   row.PendingItemCount,
		AllCompleted:       row.AllCompleted,
		Orgs:               orgs,
	}
}

func mapExecutionUserDetail(
	row *readmodel.OJTaskExecutionUserListItem,
	orgs []*readmodel.OJTaskExecutionUserOrgItem,
	items []*readmodel.OJTaskExecutionUserItemDetail,
) *dtoresp.OJTaskExecutionUserDetailResp {
	if row == nil {
		return nil
	}

	resp := &dtoresp.OJTaskExecutionUserDetailResp{
		UserID:             row.UserID,
		UserUUIDSnapshot:   row.UserUUIDSnapshot,
		UsernameSnapshot:   row.UsernameSnapshot,
		AvatarSnapshot:     row.AvatarSnapshot,
		UserStatusSnapshot: row.UserStatusSnapshot,
		CompletedItemCount: row.CompletedItemCount,
		PendingItemCount:   row.PendingItemCount,
		AllCompleted:       row.AllCompleted,
		Orgs:               make([]*dtoresp.OJTaskExecutionUserOrgResp, 0, len(orgs)),
		CompletedItems:     make([]*dtoresp.OJTaskExecutionUserItemResp, 0),
		PendingItems:       make([]*dtoresp.OJTaskExecutionUserItemResp, 0),
	}

	for _, org := range orgs {
		if org == nil {
			continue
		}
		resp.Orgs = append(resp.Orgs, &dtoresp.OJTaskExecutionUserOrgResp{
			OrgID:           org.OrgID,
			OrgNameSnapshot: org.OrgNameSnapshot,
		})
	}
	for _, item := range items {
		mapped := mapExecutionUserItem(item)
		if mapped == nil {
			continue
		}
		if mapped.ResultStatus == string(consts.OJTaskExecutionUserItemResultCompleted) {
			resp.CompletedItems = append(resp.CompletedItems, mapped)
			continue
		}
		resp.PendingItems = append(resp.PendingItems, mapped)
	}
	return resp
}

func mapExecutionUserItem(item *readmodel.OJTaskExecutionUserItemDetail) *dtoresp.OJTaskExecutionUserItemResp {
	if item == nil {
		return nil
	}
	return &dtoresp.OJTaskExecutionUserItemResp{
		TaskItemID:            item.TaskItemID,
		SortNo:                item.SortNo,
		Platform:              item.Platform,
		QuestionCode:          item.QuestionCode,
		PlatformQuestionID:    item.PlatformQuestionID,
		QuestionTitleSnapshot: item.QuestionTitleSnapshot,
		ResultStatus:          item.ResultStatus,
		Reason:                item.Reason,
	}
}
