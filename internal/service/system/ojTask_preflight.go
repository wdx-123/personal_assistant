package system

import (
	"context"

	"personal_assistant/global"
	"personal_assistant/internal/model/consts"
	"personal_assistant/internal/model/entity"
	bizerrors "personal_assistant/pkg/errors"
)

// preFlightCheck 在执行前对待解析题目做最后一次本地回填兜底。
// 该阶段只更新任务侧状态，不再反向修改题库真相。
func (s *OJTaskService) preFlightCheck(ctx context.Context, taskID uint) error {
	if !resolveOJTaskPreflightEnabled() || taskID == 0 {
		return nil
	}

	return s.txRunner.InTx(ctx, func(tx any) error {
		txTaskRepo := s.taskRepo.WithTx(tx)

		items, err := txTaskRepo.ListItemsByTaskID(ctx, taskID)
		if err != nil {
			return bizerrors.Wrap(bizerrors.CodeDBError, err)
		}
		intakes, err := txTaskRepo.ListIntakesByTaskID(ctx, taskID)
		if err != nil {
			return bizerrors.Wrap(bizerrors.CodeDBError, err)
		}
		intakeByTaskItemID := make(map[uint]*entity.OJQuestionIntake, len(intakes))
		for _, intake := range intakes {
			if intake == nil || intake.TaskItemID == 0 {
				continue
			}
			intakeByTaskItemID[intake.TaskItemID] = intake
		}

		for _, item := range items {
			if item == nil || item.ID == 0 {
				continue
			}
			itemChanged, intakeChanged, err := s.preFlightTaskItemTx(ctx, tx, item, intakeByTaskItemID[item.ID])
			if err != nil {
				return err
			}
			if itemChanged {
				if err := txTaskRepo.UpdateItem(ctx, item); err != nil {
					return bizerrors.Wrap(bizerrors.CodeDBError, err)
				}
			}
			if intakeChanged != nil {
				if err := txTaskRepo.UpdateIntake(ctx, intakeChanged); err != nil {
					return bizerrors.Wrap(bizerrors.CodeDBError, err)
				}
			}
		}
		return nil
	})
}

func (s *OJTaskService) preFlightTaskItemTx(
	ctx context.Context,
	tx any,
	item *entity.OJTaskItem,
	intake *entity.OJQuestionIntake,
) (bool, *entity.OJQuestionIntake, error) {
	if item == nil || item.ID == 0 {
		return false, nil, nil
	}

	if item.ResolutionStatus == string(consts.OJTaskItemResolutionStatusResolved) && item.ResolvedQuestionID > 0 {
		return false, nil, nil
	}
	if item.InputTitle == "" {
		item.ResolutionStatus = string(consts.OJTaskItemResolutionStatusInvalid)
		item.ResolutionNote = "preflight_input_title_missing"
		if intake != nil {
			intake.Status = item.ResolutionStatus
			intake.ResolutionNote = item.ResolutionNote
		}
		return true, intake, nil
	}
	if item.ResolutionStatus != string(consts.OJTaskItemResolutionStatusPendingResolution) {
		return false, nil, nil
	}

	candidates, err := s.findVerifiedTitleCandidates(ctx, tx, item.Platform, item.InputTitle)
	if err != nil {
		return false, nil, err
	}
	if len(candidates) == 1 {
		changed := applyResolvedCandidateToTaskItem(item, candidates[0], "preflight_auto_resolved")
		if intake != nil {
			intake.Status = string(consts.OJTaskItemResolutionStatusResolved)
			intake.ResolvedQuestionID = candidates[0].QuestionID
			intake.ResolutionNote = ""
		}
		return changed, intake, nil
	}

	item.ResolutionStatus = string(consts.OJTaskItemResolutionStatusInvalid)
	item.ResolutionNote = "preflight_resolution_failed"
	if intake != nil {
		intake.Status = string(consts.OJTaskItemResolutionStatusInvalid)
		intake.ResolutionNote = item.ResolutionNote
	}
	return true, intake, nil
}

func resolveOJTaskPreflightEnabled() bool {
	if global.Config == nil {
		return true
	}
	return global.Config.Task.OJTaskPreflightEnabled
}
