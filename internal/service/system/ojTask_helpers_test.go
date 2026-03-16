package system

import (
	"testing"

	"personal_assistant/internal/model/consts"
	"personal_assistant/internal/model/entity"
)

func TestResolveOJTaskItemResult(t *testing.T) {
	t.Run("account unbound", func(t *testing.T) {
		item := &entity.OJTaskItem{Platform: consts.OJPlatformLuogu, PlatformQuestionID: 101}
		status, reason := resolveOJTaskItemResult(
			1,
			item,
			map[uint]*entity.LuoguUserDetail{},
			map[uint]map[uint]struct{}{},
			nil,
			nil,
			nil,
			nil,
		)
		if status != string(consts.OJTaskExecutionUserItemResultPending) {
			t.Fatalf("unexpected status: %s", status)
		}
		if reason != string(consts.OJTaskExecutionUserItemReasonAccountUnbound) {
			t.Fatalf("unexpected reason: %s", reason)
		}
	})

	t.Run("completed", func(t *testing.T) {
		item := &entity.OJTaskItem{Platform: consts.OJPlatformLeetcode, PlatformQuestionID: 202}
		status, reason := resolveOJTaskItemResult(
			2,
			item,
			nil,
			nil,
			map[uint]*entity.LeetcodeUserDetail{2: {MODEL: entity.MODEL{ID: 99}, UserID: 2}},
			map[uint]map[uint]struct{}{99: {202: {}}},
			nil,
			nil,
		)
		if status != string(consts.OJTaskExecutionUserItemResultCompleted) {
			t.Fatalf("unexpected status: %s", status)
		}
		if reason != "" {
			t.Fatalf("unexpected reason: %s", reason)
		}
	})

	t.Run("unsolved", func(t *testing.T) {
		item := &entity.OJTaskItem{Platform: consts.OJPlatformLanqiao, PlatformQuestionID: 303}
		status, reason := resolveOJTaskItemResult(
			3,
			item,
			nil,
			nil,
			nil,
			nil,
			map[uint]*entity.LanqiaoUserDetail{3: {MODEL: entity.MODEL{ID: 66}, UserID: 3}},
			map[uint]map[uint]struct{}{66: {}},
		)
		if status != string(consts.OJTaskExecutionUserItemResultPending) {
			t.Fatalf("unexpected status: %s", status)
		}
		if reason != string(consts.OJTaskExecutionUserItemReasonUnsolved) {
			t.Fatalf("unexpected reason: %s", reason)
		}
	})
}

func TestTaskItemsToValidated(t *testing.T) {
	items := []*entity.OJTaskItem{
		{SortNo: 5, Platform: consts.OJPlatformLeetcode, QuestionCode: "two-sum", PlatformQuestionID: 2, QuestionTitleSnapshot: "Two Sum"},
		{SortNo: 2, Platform: consts.OJPlatformLuogu, QuestionCode: "P1000", PlatformQuestionID: 1, QuestionTitleSnapshot: "A+B"},
	}

	got := taskItemsToValidated(items)
	if len(got) != 2 {
		t.Fatalf("unexpected length: %d", len(got))
	}
	if got[0].QuestionCode != "P1000" || got[0].SortNo != 1 {
		t.Fatalf("unexpected first item: %+v", got[0])
	}
	if got[1].QuestionCode != "two-sum" || got[1].SortNo != 2 {
		t.Fatalf("unexpected second item: %+v", got[1])
	}
}
