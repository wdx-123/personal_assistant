package system

import (
	"context"
	"strings"

	"personal_assistant/internal/model/consts"
	eventdto "personal_assistant/internal/model/dto/event"
	"personal_assistant/internal/model/entity"
	bizerrors "personal_assistant/pkg/errors"
)

// HandleQuestionUpserted 在权威题库变更后尝试自动回填 pending_resolution 的任务项。
func (s *OJTaskService) HandleQuestionUpserted(ctx context.Context, event *eventdto.QuestionUpsertedEvent) error {
	if event == nil || strings.TrimSpace(event.Platform) == "" || strings.TrimSpace(event.Title) == "" {
		return bizerrors.NewWithMsg(bizerrors.CodeInvalidParams, "question upsert event invalid")
	}

	title := normalizeOJTaskTitle(event.Title)
	return s.txRunner.InTx(ctx, func(tx any) error {
		txTaskRepo := s.taskRepo.WithTx(tx)

		candidates, err := s.findVerifiedTitleCandidates(ctx, tx, event.Platform, title)
		if err != nil {
			return err
		}
		if len(candidates) != 1 {
			return nil
		}

		intakes, err := txTaskRepo.ListPendingIntakesByTitle(ctx, event.Platform, title)
		if err != nil {
			return bizerrors.Wrap(bizerrors.CodeDBError, err)
		}
		for _, intake := range intakes {
			if intake == nil || intake.TaskItemID == 0 {
				continue
			}
			item, err := txTaskRepo.GetItemByID(ctx, intake.TaskItemID)
			if err != nil {
				return bizerrors.Wrap(bizerrors.CodeDBError, err)
			}
			if item == nil || item.ID == 0 || item.ResolutionStatus != string(consts.OJTaskItemResolutionStatusPendingResolution) {
				continue
			}
			if normalizeOJTaskTitle(item.InputTitle) != title {
				continue
			}

			if applyResolvedCandidateToTaskItem(item, candidates[0], "auto_resolved_from_question_upsert") {
				if err := txTaskRepo.UpdateItem(ctx, item); err != nil {
					return bizerrors.Wrap(bizerrors.CodeDBError, err)
				}
			}
			intake.Status = string(consts.OJTaskItemResolutionStatusResolved)
			intake.ResolvedQuestionID = candidates[0].QuestionID
			intake.ResolutionNote = ""
			if err := txTaskRepo.UpdateIntake(ctx, intake); err != nil {
				return bizerrors.Wrap(bizerrors.CodeDBError, err)
			}
		}
		return nil
	})
}

func applyResolvedCandidateToTaskItem(
	item *entity.OJTaskItem,
	candidate ojTaskAnalyzeCandidate,
	note string,
) bool {
	if item == nil {
		return false
	}
	changed := item.ResolvedQuestionID != candidate.QuestionID ||
		item.ResolvedQuestionCode != candidate.QuestionCode ||
		item.ResolvedTitleSnapshot != candidate.Title ||
		item.ResolutionStatus != string(consts.OJTaskItemResolutionStatusResolved) ||
		item.ResolutionNote != note

	item.ResolvedQuestionID = candidate.QuestionID
	item.ResolvedQuestionCode = candidate.QuestionCode
	item.ResolvedTitleSnapshot = candidate.Title
	item.ResolutionStatus = string(consts.OJTaskItemResolutionStatusResolved)
	item.ResolutionNote = note
	return changed
}
