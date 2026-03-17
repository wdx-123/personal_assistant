package system

import (
	"context"
	stderrors "errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"personal_assistant/global"
	"personal_assistant/internal/model/consts"
	"personal_assistant/internal/model/entity"
	readmodel "personal_assistant/internal/model/readmodel"
	bizerrors "personal_assistant/pkg/errors"
	"personal_assistant/pkg/redislock"

	"go.uber.org/zap"
)

// ojTaskExecutionSnapshot 表示一次执行在落库前的内存快照。
// 它聚合执行用户、组织命中关系、题目结果草稿和汇总计数，
// 便于在单次事务中统一写入，避免产生部分成功的中间态。
type ojTaskExecutionSnapshot struct {
	Users              []*entity.OJTaskExecutionUser
	UserOrgDrafts      []*ojTaskExecutionUserOrgDraft
	UserItemDrafts     []*ojTaskExecutionUserItemDraft
	TotalUserCount     int
	CompletedUserCount int
	PendingUserCount   int
	TotalItemCount     int
	CompletedItemCount int
	PendingItemCount   int
}

// ojTaskExecutionUserOrgDraft 表示 execution_user_id 尚未生成前的组织命中草稿。
type ojTaskExecutionUserOrgDraft struct {
	UserID          uint
	OrgID           uint
	OrgNameSnapshot string
}

// ojTaskExecutionUserItemDraft 表示 execution_user_id 尚未生成前的题目结果草稿。
type ojTaskExecutionUserItemDraft struct {
	UserID       uint
	TaskItemID   uint
	ResultStatus string
	Reason       string
}

type ojTaskExecutionAttemptState string

const (
	ojTaskExecutionAttemptStateExecuted       ojTaskExecutionAttemptState = "executed"        // 执行成功
	ojTaskExecutionAttemptStateNoop           ojTaskExecutionAttemptState = "noop"            // 无操作，如状态不符或锁竞争失败
	ojTaskExecutionAttemptStateTerminalFailed ojTaskExecutionAttemptState = "terminal_failed" // 执行失败且不可重试，如数据不一致导致的失败
	ojTaskExecutionAttemptStateRetryableError ojTaskExecutionAttemptState = "retryable_error" // 执行失败但可重试，如数据库错误
)

type ojTaskExecutionAttemptResult struct {
	State ojTaskExecutionAttemptState
	Err   error
}

// DispatchPendingExecutions 扫描到期执行并按配置并发度触发本轮调度。
// 调度器只消费 scheduled / queued 且到达 planned_at 的执行记录。
func (s *OJTaskService) DispatchPendingExecutions(ctx context.Context) error {
	batchSize := resolveOJTaskDispatchBatchSize()
	workerCount := resolveOJTaskDispatchWorkerCount()
	rows, err := s.executionRepo.ListDueExecutions(
		ctx,
		[]string{string(consts.OJTaskExecutionStatusScheduled), string(consts.OJTaskExecutionStatusQueued)},
		time.Now().UTC(),
		batchSize,
	)
	if err != nil {
		return bizerrors.Wrap(bizerrors.CodeDBError, err)
	}
	if len(rows) == 0 {
		return nil
	}
	if workerCount <= 1 || len(rows) == 1 {
		var joined error
		for _, row := range rows {
			if err := dispatchAttemptResultError(s.dispatchSingleExecution(ctx, row)); err != nil {
				joined = stderrors.Join(joined, err)
			}
		}
		return joined
	}

	sem := make(chan struct{}, workerCount)
	errCh := make(chan error, len(rows))
	var wg sync.WaitGroup
	for _, row := range rows {
		if row == nil {
			continue
		}
		wg.Add(1)
		// 使用带缓冲信号量限制单轮并发数，避免一次扫描拉起过多 worker。
		sem <- struct{}{}
		go func(item *readmodel.OJTaskExecutionDispatch) {
			defer wg.Done()
			defer func() { <-sem }()
			if err := dispatchAttemptResultError(s.dispatchSingleExecution(ctx, item)); err != nil {
				errCh <- err
			}
		}(row)
	}
	wg.Wait()
	close(errCh)

	var joined error
	for err := range errCh {
		joined = stderrors.Join(joined, err)
	}
	return joined
}

// ExecuteExecutionByID 通过 execution_id 触发单条执行。
// 它面向事件订阅器使用：只要执行已安全收口到终态或明确 no-op，就返回 nil 以便 ACK。
func (s *OJTaskService) ExecuteExecutionByID(ctx context.Context, executionID uint) error {
	result := s.executeSingleExecutionByID(ctx, executionID)
	return triggerAttemptResultError(result)
}

// dispatchSingleExecution 负责单条执行记录的完整流转：
// 分布式锁竞争 -> 执行记录抢占 -> 任务置 executing -> 生成快照 -> 事务落库。
func (s *OJTaskService) dispatchSingleExecution(
	ctx context.Context,
	row *readmodel.OJTaskExecutionDispatch,
) ojTaskExecutionAttemptResult {
	if row == nil || row.ExecutionID == 0 || row.TaskID == 0 {
		return ojTaskExecutionAttemptResult{State: ojTaskExecutionAttemptStateNoop}
	}
	if !isDispatchableOJTaskExecutionStatus(row.Status) {
		return ojTaskExecutionAttemptResult{State: ojTaskExecutionAttemptStateNoop}
	}

	lock, contended, err := acquireOJTaskExecutionLock(ctx, row.ExecutionID)
	if err != nil {
		return ojTaskExecutionAttemptResult{State: ojTaskExecutionAttemptStateRetryableError, Err: err}
	}
	if contended {
		return ojTaskExecutionAttemptResult{State: ojTaskExecutionAttemptStateNoop}
	}
	if lock != nil {
		defer releaseOJTaskExecutionLock(row.ExecutionID, lock)
	}

	// ClaimExecution 是真正的并发收口点，只有一个实例能成功推进到 executing。
	claimed, err := s.executionRepo.ClaimExecution(
		ctx,
		row.ExecutionID,
		[]string{string(consts.OJTaskExecutionStatusScheduled), string(consts.OJTaskExecutionStatusQueued)},
		time.Now().UTC(),
	)
	if err != nil {
		return ojTaskExecutionAttemptResult{
			State: ojTaskExecutionAttemptStateRetryableError,
			Err:   bizerrors.Wrap(bizerrors.CodeDBError, err),
		}
	}
	if !claimed {
		return ojTaskExecutionAttemptResult{State: ojTaskExecutionAttemptStateNoop}
	}

	task, err := s.taskRepo.GetByID(ctx, row.TaskID)
	if err != nil {
		return s.failExecutionAttempt(
			ctx,
			row.TaskID,
			row.ExecutionID,
			bizerrors.Wrap(bizerrors.CodeDBError, err),
		)
	}
	if task == nil {
		return s.failExecutionAttempt(ctx, row.TaskID, row.ExecutionID, bizerrors.New(bizerrors.CodeOJTaskNotFound))
	}

	execution, err := s.executionRepo.GetByID(ctx, row.ExecutionID)
	if err != nil {
		return s.failExecutionAttempt(
			ctx,
			row.TaskID,
			row.ExecutionID,
			bizerrors.Wrap(bizerrors.CodeDBError, err),
		)
	}
	if execution == nil {
		return s.failExecutionAttempt(
			ctx,
			row.TaskID,
			row.ExecutionID,
			bizerrors.New(bizerrors.CodeOJTaskExecutionNotFound),
		)
	}

	task.Status = string(consts.OJTaskStatusExecuting)
	if execution.RequestedBy > 0 {
		task.UpdatedBy = execution.RequestedBy
	}
	if err := s.taskRepo.Update(ctx, task); err != nil {
		return s.failExecutionAttempt(
			ctx,
			row.TaskID,
			row.ExecutionID,
			bizerrors.Wrap(bizerrors.CodeDBError, err),
		)
	}
	if err := s.preFlightCheck(ctx, task.ID); err != nil {
		return s.failExecutionAttempt(ctx, task.ID, execution.ID, err)
	}

	// 先在内存中冻结执行事实，再统一事务落库，减少查询侧读到中间态的概率。
	snapshot, err := s.buildExecutionSnapshot(ctx, task, execution)
	if err != nil {
		return s.failExecutionAttempt(ctx, task.ID, execution.ID, err)
	}
	if err := s.persistExecutionSnapshot(ctx, task, execution, snapshot); err != nil {
		return s.failExecutionAttempt(ctx, task.ID, execution.ID, err)
	}
	return ojTaskExecutionAttemptResult{State: ojTaskExecutionAttemptStateExecuted}
}

// executeSingleExecutionByID 通过 execution_id 执行单条记录，面向事件订阅器使用。
func (s *OJTaskService) executeSingleExecutionByID(
	ctx context.Context,
	executionID uint,
) ojTaskExecutionAttemptResult {
	if executionID == 0 {
		return ojTaskExecutionAttemptResult{State: ojTaskExecutionAttemptStateNoop}
	}
	row, err := s.executionRepo.GetDispatchExecutionByID(ctx, executionID)
	if err != nil {
		return ojTaskExecutionAttemptResult{
			State: ojTaskExecutionAttemptStateRetryableError,
			Err:   bizerrors.Wrap(bizerrors.CodeDBError, err),
		}
	}
	if row == nil {
		return ojTaskExecutionAttemptResult{State: ojTaskExecutionAttemptStateNoop}
	}
	return s.dispatchSingleExecution(ctx, row)
}

// failExecutionAttempt 将执行推进到失败状态，并根据失败原因区分可重试与不可重试两类结果。
func (s *OJTaskService) failExecutionAttempt(
	ctx context.Context,
	taskID, executionID uint,
	cause error,
) ojTaskExecutionAttemptResult {
	if cause == nil {
		cause = bizerrors.NewWithMsg(bizerrors.CodeInternalError, "oj task execution failed")
	}
	if err := s.markExecutionFailed(ctx, taskID, executionID, cause); err != nil {
		return ojTaskExecutionAttemptResult{
			State: ojTaskExecutionAttemptStateRetryableError,
			Err:   stderrors.Join(cause, bizerrors.Wrap(bizerrors.CodeDBError, err)),
		}
	}
	return ojTaskExecutionAttemptResult{
		State: ojTaskExecutionAttemptStateTerminalFailed,
		Err:   cause,
	}
}

// dispatchAttemptResultError 只对执行失败的结果返回错误，供调度器使用以决定是否继续重试。
func dispatchAttemptResultError(result ojTaskExecutionAttemptResult) error {
	switch result.State {
	case ojTaskExecutionAttemptStateTerminalFailed, ojTaskExecutionAttemptStateRetryableError:
		return result.Err
	default:
		return nil
	}
}

// triggerAttemptResultError 只对执行失败的结果返回错误，供事件订阅器使用以决定是否 ACK。
func triggerAttemptResultError(result ojTaskExecutionAttemptResult) error {
	if result.State == ojTaskExecutionAttemptStateRetryableError {
		return result.Err
	}
	return nil
}

// isDispatchableOJTaskExecutionStatus 判断执行状态是否符合调度条件。
func isDispatchableOJTaskExecutionStatus(status string) bool {
	switch status {
	case string(consts.OJTaskExecutionStatusScheduled), string(consts.OJTaskExecutionStatusQueued):
		return true
	default:
		return false
	}
}

// buildExecutionSnapshot 根据任务组织、题单和用户刷题事实构建本次执行快照。
// 该阶段不写库，只负责冻结“本次执行看到的业务事实”。
func (s *OJTaskService) buildExecutionSnapshot(
	ctx context.Context,
	task *entity.OJTask,
	execution *entity.OJTaskExecution,
) (*ojTaskExecutionSnapshot, error) {
	taskOrgs, err := s.taskRepo.ListTaskOrgsWithNames(ctx, task.ID)
	if err != nil {
		return nil, bizerrors.Wrap(bizerrors.CodeDBError, err)
	}
	taskItems, err := s.taskRepo.ListItemsByTaskID(ctx, task.ID)
	if err != nil {
		return nil, bizerrors.Wrap(bizerrors.CodeDBError, err)
	}
	// 先按 sort_no、再按主键排序，确保同序号场景下快照顺序仍然稳定可预测。
	sort.Slice(taskItems, func(i, j int) bool {
		if taskItems[i].SortNo == taskItems[j].SortNo {
			return taskItems[i].ID < taskItems[j].ID
		}
		return taskItems[i].SortNo < taskItems[j].SortNo
	})

	orgIDs := make([]uint, 0, len(taskOrgs))
	orgNameMap := make(map[uint]string, len(taskOrgs))
	for _, org := range taskOrgs {
		if org == nil || org.OrgID == 0 {
			continue
		}
		orgIDs = append(orgIDs, org.OrgID)
		orgNameMap[org.OrgID] = org.OrgName
	}

	userOrgPairs, err := s.orgMemberRepo.ListActiveUserOrgPairsByOrgIDs(ctx, orgIDs)
	if err != nil {
		return nil, bizerrors.Wrap(bizerrors.CodeDBError, err)
	}

	userOrgMap := make(map[uint][]uint)
	for _, pair := range userOrgPairs {
		if pair == nil || pair.UserID == 0 || pair.OrgID == 0 {
			continue
		}
		userOrgMap[pair.UserID] = append(userOrgMap[pair.UserID], pair.OrgID)
	}

	userIDs := make([]uint, 0, len(userOrgMap))
	for userID := range userOrgMap {
		userIDs = append(userIDs, userID)
	}
	sort.Slice(userIDs, func(i, j int) bool { return userIDs[i] < userIDs[j] })

	snapshot := &ojTaskExecutionSnapshot{
		Users:          make([]*entity.OJTaskExecutionUser, 0, len(userIDs)),
		UserOrgDrafts:  make([]*ojTaskExecutionUserOrgDraft, 0, len(userOrgPairs)),
		UserItemDrafts: make([]*ojTaskExecutionUserItemDraft, 0, len(userIDs)*len(taskItems)),
	}
	if len(userIDs) == 0 {
		return snapshot, nil
	}

	users, err := s.userRepo.GetByIDs(ctx, userIDs)
	if err != nil {
		return nil, bizerrors.Wrap(bizerrors.CodeDBError, err)
	}
	userMap := make(map[uint]*entity.User, len(users))
	for _, user := range users {
		if user == nil || user.ID == 0 {
			continue
		}
		userMap[user.ID] = user
	}
	for _, userID := range userIDs {
		if _, ok := userMap[userID]; !ok {
			return nil, bizerrors.NewWithMsg(bizerrors.CodeInternalError, fmt.Sprintf("用户 %d 不存在", userID))
		}
	}

	luoguDetailMap, luoguSolvedMap, err := s.loadLuoguExecutionFacts(ctx, userIDs)
	if err != nil {
		return nil, err
	}
	leetcodeDetailMap, leetcodeSolvedMap, err := s.loadLeetcodeExecutionFacts(ctx, userIDs)
	if err != nil {
		return nil, err
	}
	lanqiaoDetailMap, lanqiaoSolvedMap, err := s.loadLanqiaoExecutionFacts(ctx, userIDs)
	if err != nil {
		return nil, err
	}

	for _, userID := range userIDs {
		user := userMap[userID]
		orgs := normalizeUintSlice(userOrgMap[userID])
		// 组织列表先去重再排序，保证命中组织快照稳定且无重复。
		sort.Slice(orgs, func(i, j int) bool { return orgs[i] < orgs[j] })

		executionUser := &entity.OJTaskExecutionUser{
			ExecutionID:        execution.ID,
			UserID:             user.ID,
			UserUUIDSnapshot:   user.UUID.String(),
			UsernameSnapshot:   user.Username,
			AvatarSnapshot:     user.Avatar,
			UserStatusSnapshot: int8(user.Status),
		}

		for _, orgID := range orgs {
			snapshot.UserOrgDrafts = append(snapshot.UserOrgDrafts, &ojTaskExecutionUserOrgDraft{
				UserID:          user.ID,
				OrgID:           orgID,
				OrgNameSnapshot: orgNameMap[orgID],
			})
		}

		for _, item := range taskItems {
			resultStatus, reason := resolveOJTaskItemResult(
				user.ID,
				item,
				luoguDetailMap,
				luoguSolvedMap,
				leetcodeDetailMap,
				leetcodeSolvedMap,
				lanqiaoDetailMap,
				lanqiaoSolvedMap,
			)
			if resultStatus == string(consts.OJTaskExecutionUserItemResultCompleted) {
				executionUser.CompletedItemCount++
				snapshot.CompletedItemCount++
			} else {
				executionUser.PendingItemCount++
				snapshot.PendingItemCount++
			}
			snapshot.TotalItemCount++
			snapshot.UserItemDrafts = append(snapshot.UserItemDrafts, &ojTaskExecutionUserItemDraft{
				UserID:       user.ID,
				TaskItemID:   item.ID,
				ResultStatus: resultStatus,
				Reason:       reason,
			})
		}

		executionUser.AllCompleted = executionUser.PendingItemCount == 0
		if executionUser.AllCompleted {
			snapshot.CompletedUserCount++
		} else {
			snapshot.PendingUserCount++
		}
		snapshot.TotalUserCount++
		snapshot.Users = append(snapshot.Users, executionUser)
	}

	return snapshot, nil
}

// loadLuoguExecutionFacts 读取洛谷平台执行判定所需的账号绑定与做题事实。
func (s *OJTaskService) loadLuoguExecutionFacts(
	ctx context.Context,
	userIDs []uint,
) (map[uint]*entity.LuoguUserDetail, map[uint]map[uint]struct{}, error) {
	details, err := s.luoguDetailRepo.GetByUserIDs(ctx, userIDs)
	if err != nil {
		return nil, nil, bizerrors.Wrap(bizerrors.CodeDBError, err)
	}
	detailMap := make(map[uint]*entity.LuoguUserDetail, len(details))
	detailIDs := make([]uint, 0, len(details))
	for _, detail := range details {
		if detail == nil || detail.ID == 0 {
			continue
		}
		detailMap[detail.UserID] = detail
		detailIDs = append(detailIDs, detail.ID)
	}
	solvedMap, err := s.luoguUserQuestionRepo.GetSolvedProblemIDsByDetailIDs(ctx, detailIDs)
	if err != nil {
		return nil, nil, bizerrors.Wrap(bizerrors.CodeDBError, err)
	}
	return detailMap, solvedMap, nil
}

// loadLeetcodeExecutionFacts 读取 LeetCode 平台执行判定所需的账号绑定与做题事实。
func (s *OJTaskService) loadLeetcodeExecutionFacts(
	ctx context.Context,
	userIDs []uint,
) (map[uint]*entity.LeetcodeUserDetail, map[uint]map[uint]struct{}, error) {
	details, err := s.leetcodeDetailRepo.GetByUserIDs(ctx, userIDs)
	if err != nil {
		return nil, nil, bizerrors.Wrap(bizerrors.CodeDBError, err)
	}
	detailMap := make(map[uint]*entity.LeetcodeUserDetail, len(details))
	detailIDs := make([]uint, 0, len(details))
	for _, detail := range details {
		if detail == nil || detail.ID == 0 {
			continue
		}
		detailMap[detail.UserID] = detail
		detailIDs = append(detailIDs, detail.ID)
	}
	solvedMap, err := s.leetcodeUserQuestionRepo.GetSolvedProblemIDsByDetailIDs(ctx, detailIDs)
	if err != nil {
		return nil, nil, bizerrors.Wrap(bizerrors.CodeDBError, err)
	}
	return detailMap, solvedMap, nil
}

// loadLanqiaoExecutionFacts 读取蓝桥平台执行判定所需的账号绑定与做题事实。
func (s *OJTaskService) loadLanqiaoExecutionFacts(
	ctx context.Context,
	userIDs []uint,
) (map[uint]*entity.LanqiaoUserDetail, map[uint]map[uint]struct{}, error) {
	details, err := s.lanqiaoDetailRepo.GetByUserIDs(ctx, userIDs)
	if err != nil {
		return nil, nil, bizerrors.Wrap(bizerrors.CodeDBError, err)
	}
	detailMap := make(map[uint]*entity.LanqiaoUserDetail, len(details))
	detailIDs := make([]uint, 0, len(details))
	for _, detail := range details {
		if detail == nil || detail.ID == 0 {
			continue
		}
		detailMap[detail.UserID] = detail
		detailIDs = append(detailIDs, detail.ID)
	}
	solvedMap, err := s.lanqiaoUserQuestionRepo.GetSolvedProblemIDsByDetailIDs(ctx, detailIDs)
	if err != nil {
		return nil, nil, bizerrors.Wrap(bizerrors.CodeDBError, err)
	}
	return detailMap, solvedMap, nil
}

// resolveOJTaskItemResult 计算单个用户在单个任务题目上的执行结果。
// 判定优先级为：未绑定账号 -> pending/account_unbound；已完成 -> completed；其余 -> pending/unsolved。
func resolveOJTaskItemResult(
	userID uint,
	item *entity.OJTaskItem,
	luoguDetailMap map[uint]*entity.LuoguUserDetail,
	luoguSolvedMap map[uint]map[uint]struct{},
	leetcodeDetailMap map[uint]*entity.LeetcodeUserDetail,
	leetcodeSolvedMap map[uint]map[uint]struct{},
	lanqiaoDetailMap map[uint]*entity.LanqiaoUserDetail,
	lanqiaoSolvedMap map[uint]map[uint]struct{},
) (string, string) {
	if item == nil {
		return string(consts.OJTaskExecutionUserItemResultPending), string(consts.OJTaskExecutionUserItemReasonQuestionNotFound)
	}
	if item.ResolutionStatus != string(consts.OJTaskItemResolutionStatusResolved) || item.ResolvedQuestionID == 0 {
		return string(consts.OJTaskExecutionUserItemResultPending), string(consts.OJTaskExecutionUserItemReasonQuestionNotFound)
	}

	switch item.Platform {
	case consts.OJPlatformLuogu:
		detail := luoguDetailMap[userID]
		if detail == nil {
			return string(consts.OJTaskExecutionUserItemResultPending), string(consts.OJTaskExecutionUserItemReasonAccountUnbound)
		}
		if _, ok := luoguSolvedMap[detail.ID][item.ResolvedQuestionID]; ok {
			return string(consts.OJTaskExecutionUserItemResultCompleted), ""
		}
	case consts.OJPlatformLeetcode:
		detail := leetcodeDetailMap[userID]
		if detail == nil {
			return string(consts.OJTaskExecutionUserItemResultPending), string(consts.OJTaskExecutionUserItemReasonAccountUnbound)
		}
		if _, ok := leetcodeSolvedMap[detail.ID][item.ResolvedQuestionID]; ok {
			return string(consts.OJTaskExecutionUserItemResultCompleted), ""
		}
	case consts.OJPlatformLanqiao:
		detail := lanqiaoDetailMap[userID]
		if detail == nil {
			return string(consts.OJTaskExecutionUserItemResultPending), string(consts.OJTaskExecutionUserItemReasonAccountUnbound)
		}
		if _, ok := lanqiaoSolvedMap[detail.ID][item.ResolvedQuestionID]; ok {
			return string(consts.OJTaskExecutionUserItemResultCompleted), ""
		}
	}
	return string(consts.OJTaskExecutionUserItemResultPending), string(consts.OJTaskExecutionUserItemReasonUnsolved)
}

// persistExecutionSnapshot 在单事务中持久化快照并推进成功状态。
// 写入顺序为 execution_user -> execution_user_org -> execution_user_item -> execution 汇总 -> task 汇总。
func (s *OJTaskService) persistExecutionSnapshot(
	ctx context.Context,
	task *entity.OJTask,
	execution *entity.OJTaskExecution,
	snapshot *ojTaskExecutionSnapshot,
) error {
	return s.txRunner.InTx(ctx, func(tx any) error {
		txTaskRepo := s.taskRepo.WithTx(tx)
		txExecutionRepo := s.executionRepo.WithTx(tx)

		if err := txExecutionRepo.BatchCreateExecutionUsers(ctx, snapshot.Users, resolveOJTaskSnapshotInsertBatchSize()); err != nil {
			return bizerrors.Wrap(bizerrors.CodeDBError, err)
		}
		createdUsers, err := txExecutionRepo.ListExecutionUsersByExecutionID(ctx, execution.ID)
		if err != nil {
			return bizerrors.Wrap(bizerrors.CodeDBError, err)
		}
		// 批量插入后重新查询 execution_user，
		// 后续 user_org / user_item 需要依赖真实 execution_user_id 建立关系。
		executionUserIDMap := make(map[uint]uint, len(createdUsers))
		for _, item := range createdUsers {
			if item == nil || item.UserID == 0 {
				continue
			}
			executionUserIDMap[item.UserID] = item.ID
		}

		userOrgRows := make([]*entity.OJTaskExecutionUserOrg, 0, len(snapshot.UserOrgDrafts))
		for _, draft := range snapshot.UserOrgDrafts {
			executionUserID, ok := executionUserIDMap[draft.UserID]
			if !ok {
				return bizerrors.NewWithMsg(bizerrors.CodeInternalError, fmt.Sprintf("execution user missing: %d", draft.UserID))
			}
			userOrgRows = append(userOrgRows, &entity.OJTaskExecutionUserOrg{
				ExecutionUserID: executionUserID,
				OrgID:           draft.OrgID,
				OrgNameSnapshot: draft.OrgNameSnapshot,
			})
		}
		if err := txExecutionRepo.BatchCreateExecutionUserOrgs(ctx, userOrgRows, resolveOJTaskSnapshotInsertBatchSize()); err != nil {
			return bizerrors.Wrap(bizerrors.CodeDBError, err)
		}

		userItemRows := make([]*entity.OJTaskExecutionUserItem, 0, len(snapshot.UserItemDrafts))
		for _, draft := range snapshot.UserItemDrafts {
			executionUserID, ok := executionUserIDMap[draft.UserID]
			if !ok {
				return bizerrors.NewWithMsg(bizerrors.CodeInternalError, fmt.Sprintf("execution user missing: %d", draft.UserID))
			}
			userItemRows = append(userItemRows, &entity.OJTaskExecutionUserItem{
				ExecutionID:     execution.ID,
				UserID:          draft.UserID,
				ExecutionUserID: executionUserID,
				TaskItemID:      draft.TaskItemID,
				ResultStatus:    draft.ResultStatus,
				Reason:          draft.Reason,
			})
		}
		if err := txExecutionRepo.BatchCreateExecutionUserItems(ctx, userItemRows, resolveOJTaskSnapshotInsertBatchSize()); err != nil {
			return bizerrors.Wrap(bizerrors.CodeDBError, err)
		}

		finishedAt := time.Now().UTC()
		execution.Status = string(consts.OJTaskExecutionStatusSucceeded)
		execution.FinishedAt = &finishedAt
		execution.ErrorMessage = ""
		execution.TotalUserCount = snapshot.TotalUserCount
		execution.CompletedUserCount = snapshot.CompletedUserCount
		execution.PendingUserCount = snapshot.PendingUserCount
		execution.TotalItemCount = snapshot.TotalItemCount
		execution.CompletedItemCount = snapshot.CompletedItemCount
		execution.PendingItemCount = snapshot.PendingItemCount
		if err := txExecutionRepo.Update(ctx, execution); err != nil {
			return bizerrors.Wrap(bizerrors.CodeDBError, err)
		}

		task.Status = string(consts.OJTaskStatusSucceeded)
		if execution.RequestedBy > 0 {
			task.UpdatedBy = execution.RequestedBy
		}
		if err := txTaskRepo.Update(ctx, task); err != nil {
			return bizerrors.Wrap(bizerrors.CodeDBError, err)
		}
		return nil
	})
}

// markExecutionFailed 将执行记录和任务版本统一收口到 failed 状态，并保留错误信息。
func (s *OJTaskService) markExecutionFailed(
	ctx context.Context,
	taskID, executionID uint,
	cause error,
) error {
	message := strings.TrimSpace(cause.Error())
	if message == "" {
		message = "oj task execution failed"
	}
	finishedAt := time.Now().UTC()

	execution, err := s.executionRepo.GetByID(ctx, executionID)
	if err != nil {
		return err
	}
	if execution != nil {
		execution.Status = string(consts.OJTaskExecutionStatusFailed)
		execution.FinishedAt = &finishedAt
		execution.ErrorMessage = message
		if execution.StartedAt == nil {
			execution.StartedAt = &finishedAt
		}
		execution.TotalUserCount = 0
		execution.CompletedUserCount = 0
		execution.PendingUserCount = 0
		execution.TotalItemCount = 0
		execution.CompletedItemCount = 0
		execution.PendingItemCount = 0
		if err := s.executionRepo.Update(ctx, execution); err != nil {
			return err
		}
	}

	task, err := s.taskRepo.GetByID(ctx, taskID)
	if err != nil {
		return err
	}
	if task != nil {
		task.Status = string(consts.OJTaskStatusFailed)
		if execution != nil && execution.RequestedBy > 0 {
			task.UpdatedBy = execution.RequestedBy
		}
		if err := s.taskRepo.Update(ctx, task); err != nil {
			return err
		}
	}
	return nil
}

// acquireOJTaskExecutionLock 尝试获取执行级分布式锁。
// “锁已被其他实例持有”视为正常竞争结果，不作为系统错误返回。
func acquireOJTaskExecutionLock(ctx context.Context, executionID uint) (*redislock.RedisLock, bool, error) {
	if global.Redis == nil || (global.Config != nil && !global.Config.Task.DistributedLockEnabled) {
		return nil, false, nil
	}
	lock := redislock.NewRedisLock(ctx, fmt.Sprintf("oj_task:execution:%d", executionID), resolveOJTaskExecutionLockTTL())
	if err := lock.TryLock(); err != nil {
		if stderrors.Is(err, redislock.ErrLockFailed) {
			return nil, true, nil
		}
		return nil, false, bizerrors.Wrap(bizerrors.CodeRedisError, err)
	}
	return lock, false, nil
}

// releaseOJTaskExecutionLock 释放执行级分布式锁。
// 解锁失败只记录告警，不回滚已经完成的业务结果。
func releaseOJTaskExecutionLock(executionID uint, lock *redislock.RedisLock) {
	if lock == nil {
		return
	}
	unlockCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := lock.UnlockWithContext(unlockCtx); err != nil {
		global.Log.Warn("释放 OJTask 执行锁失败", zap.Uint("execution_id", executionID), zap.Error(err))
	}
}

// resolveOJTaskDispatchBatchSize 返回单轮调度扫描批次大小。
func resolveOJTaskDispatchBatchSize() int {
	if global.Config == nil || global.Config.Task.OJTaskDispatchBatchSize <= 0 {
		return 10
	}
	return global.Config.Task.OJTaskDispatchBatchSize
}

// resolveOJTaskDispatchWorkerCount 返回单轮调度并发 worker 数。
func resolveOJTaskDispatchWorkerCount() int {
	if global.Config == nil || global.Config.Task.OJTaskDispatchWorkerCount <= 0 {
		return 1
	}
	return global.Config.Task.OJTaskDispatchWorkerCount
}

// resolveOJTaskSnapshotInsertBatchSize 返回执行快照批量写入大小。
func resolveOJTaskSnapshotInsertBatchSize() int {
	if global.Config == nil || global.Config.Task.OJTaskSnapshotInsertBatchSize <= 0 {
		return 500
	}
	return global.Config.Task.OJTaskSnapshotInsertBatchSize
}

// resolveOJTaskExecutionLockTTL 返回 OJ 任务执行锁的 TTL，单位为 time.Duration。
func resolveOJTaskExecutionLockTTL() time.Duration {
	if global.Config == nil || global.Config.Task.OJTaskExecutionLockTTLSeconds <= 0 {
		return 60 * time.Second
	}
	return time.Duration(global.Config.Task.OJTaskExecutionLockTTLSeconds) * time.Second
}
