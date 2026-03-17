package system

import (
	"context"
	"testing"

	"personal_assistant/global"
	"personal_assistant/internal/model/config"
	"personal_assistant/internal/model/consts"
	eventdto "personal_assistant/internal/model/dto/event"
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

func TestBuildOJTaskExecutionTriggerOutboxEvent(t *testing.T) {
	oldCfg := global.Config
	global.Config = &config.Config{
		Messaging: config.Messaging{
			OJTaskExecutionTriggerTopic: "oj_task_execution_trigger",
		},
	}
	defer func() {
		global.Config = oldCfg
	}()

	event, err := buildOJTaskExecutionTriggerOutboxEvent(context.Background(), &eventdto.OJTaskExecutionTriggerEvent{
		ExecutionID: 11,
		TaskID:      22,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if event.EventType != "oj_task_execution_trigger" {
		t.Fatalf("unexpected topic: %s", event.EventType)
	}
	if event.AggregateID != "11" {
		t.Fatalf("unexpected aggregate id: %s", event.AggregateID)
	}
	if event.AggregateType != "oj_task_execution" {
		t.Fatalf("unexpected aggregate type: %s", event.AggregateType)
	}
}

func TestTriggerAttemptResultError(t *testing.T) {
	terminalErr := triggerAttemptResultError(ojTaskExecutionAttemptResult{
		State: ojTaskExecutionAttemptStateTerminalFailed,
		Err:   assertErr("terminal"),
	})
	if terminalErr != nil {
		t.Fatalf("terminal failed should ack, got err: %v", terminalErr)
	}

	retryErr := triggerAttemptResultError(ojTaskExecutionAttemptResult{
		State: ojTaskExecutionAttemptStateRetryableError,
		Err:   assertErr("retryable"),
	})
	if retryErr == nil || retryErr.Error() != "retryable" {
		t.Fatalf("unexpected retryable error: %v", retryErr)
	}
}

type assertErr string

func (e assertErr) Error() string {
	return string(e)
}
