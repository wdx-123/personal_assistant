package system

import (
	"context"
	"testing"

	"personal_assistant/global"
	"personal_assistant/internal/model/config"
	"personal_assistant/internal/model/consts"
	eventdto "personal_assistant/internal/model/dto/event"
	"personal_assistant/internal/model/dto/request"
	"personal_assistant/internal/model/entity"
)

func TestResolveOJTaskItemResult(t *testing.T) {
	t.Run("account unbound", func(t *testing.T) {
		item := &entity.OJTaskItem{
			Platform:           consts.OJPlatformLuogu,
			ResolvedQuestionID: 101,
			ResolutionStatus:   string(consts.OJTaskItemResolutionStatusResolved),
		}
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
		item := &entity.OJTaskItem{
			Platform:           consts.OJPlatformLeetcode,
			ResolvedQuestionID: 202,
			ResolutionStatus:   string(consts.OJTaskItemResolutionStatusResolved),
		}
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
		item := &entity.OJTaskItem{
			Platform:           consts.OJPlatformLanqiao,
			ResolvedQuestionID: 303,
			ResolutionStatus:   string(consts.OJTaskItemResolutionStatusResolved),
		}
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

	t.Run("question not found", func(t *testing.T) {
		item := &entity.OJTaskItem{
			Platform:         consts.OJPlatformLeetcode,
			ResolutionStatus: string(consts.OJTaskItemResolutionStatusPendingResolution),
		}
		status, reason := resolveOJTaskItemResult(
			2,
			item,
			nil,
			nil,
			nil,
			nil,
			nil,
			nil,
		)
		if status != string(consts.OJTaskExecutionUserItemResultPending) {
			t.Fatalf("unexpected status: %s", status)
		}
		if reason != string(consts.OJTaskExecutionUserItemReasonQuestionNotFound) {
			t.Fatalf("unexpected reason: %s", reason)
		}
	})
}

func TestTaskItemsToValidated(t *testing.T) {
	items := []*entity.OJTaskItem{
		{
			SortNo:                5,
			Platform:              consts.OJPlatformLeetcode,
			InputTitle:            "Two Sum",
			InputMode:             string(consts.OJTaskItemInputModeTitle),
			ResolvedQuestionID:    2,
			ResolvedQuestionCode:  "two-sum",
			ResolvedTitleSnapshot: "Two Sum",
			ResolutionStatus:      string(consts.OJTaskItemResolutionStatusResolved),
		},
		{
			SortNo:                2,
			Platform:              consts.OJPlatformLuogu,
			InputTitle:            "A+B",
			InputMode:             string(consts.OJTaskItemInputModeTitle),
			ResolvedQuestionID:    1,
			ResolvedQuestionCode:  "P1000",
			ResolvedTitleSnapshot: "A+B",
			ResolutionStatus:      string(consts.OJTaskItemResolutionStatusResolved),
		},
	}

	got := taskItemsToValidated(items)
	if len(got) != 2 {
		t.Fatalf("unexpected length: %d", len(got))
	}
	if got[0].ResolvedQuestionCode != "P1000" || got[0].SortNo != 1 {
		t.Fatalf("unexpected first item: %+v", got[0])
	}
	if got[1].ResolvedQuestionCode != "two-sum" || got[1].SortNo != 2 {
		t.Fatalf("unexpected second item: %+v", got[1])
	}
}

func TestBuildTaskIntakeRows(t *testing.T) {
	rows := buildTaskIntakeRows(11, []*entity.OJTaskItem{
		{
			MODEL:              entity.MODEL{ID: 101},
			TaskID:             11,
			Platform:           consts.OJPlatformLuogu,
			InputTitle:         "未知题目A",
			ResolutionStatus:   string(consts.OJTaskItemResolutionStatusPendingResolution),
			ResolvedQuestionID: 0,
			ResolutionNote:     "awaiting_question_sync",
		},
		{
			MODEL:              entity.MODEL{ID: 102},
			TaskID:             11,
			Platform:           consts.OJPlatformLeetcode,
			InputTitle:         "Two Sum",
			ResolutionStatus:   string(consts.OJTaskItemResolutionStatusResolved),
			ResolvedQuestionID: 2,
		},
	})
	if len(rows) != 1 {
		t.Fatalf("unexpected intake length: %d", len(rows))
	}
	if rows[0].TaskItemID != 101 || rows[0].InputTitle != "未知题目A" {
		t.Fatalf("unexpected intake row: %+v", rows[0])
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

func TestBuildOJQuestionUpsertOutboxEvent(t *testing.T) {
	oldCfg := global.Config
	global.Config = &config.Config{
		Messaging: config.Messaging{
			OJQuestionUpsertTopic: "oj_question_upsert",
		},
	}
	defer func() {
		global.Config = oldCfg
	}()

	event, err := buildOJQuestionUpsertOutboxEvent(context.Background(), &eventdto.QuestionUpsertedEvent{
		Platform:     consts.OJPlatformLeetcode,
		QuestionID:   9,
		QuestionCode: "two-sum",
		Title:        "Two Sum",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if event.EventType != "oj_question_upsert" {
		t.Fatalf("unexpected topic: %s", event.EventType)
	}
	if event.AggregateID != "9" {
		t.Fatalf("unexpected aggregate id: %s", event.AggregateID)
	}
	if event.AggregateType != "oj_question" {
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

func TestValidateItems(t *testing.T) {
	svc := &OJTaskService{}

	t.Run("normalize title payload", func(t *testing.T) {
		got, err := svc.validateItems([]request.OJTaskItemReq{{
			Platform:      consts.OJPlatformLuogu,
			Title:         "  P1001 A+B  ",
			AnalysisToken: " token ",
		}})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got) != 1 {
			t.Fatalf("unexpected item length: %d", len(got))
		}
		if got[0].Platform != consts.OJPlatformLuogu || got[0].Title != "P1001 A+B" || got[0].AnalysisToken != "token" {
			t.Fatalf("unexpected normalized item: %+v", got[0])
		}
	})

	t.Run("empty title invalid", func(t *testing.T) {
		_, err := svc.validateItems([]request.OJTaskItemReq{{
			Platform: consts.OJPlatformLeetcode,
			Title:    "   ",
		}})
		if err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestOJTaskAnalysisTokenCodec(t *testing.T) {
	oldCfg := global.Config
	global.Config = &config.Config{
		JWT: config.JWT{
			AccessTokenSecret: "test-secret",
		},
	}
	defer func() {
		global.Config = oldCfg
	}()

	codec := newOJTaskAnalysisTokenCodec()
	token, err := codec.Encode(consts.OJPlatformLeetcode, "Two Sum", 100)
	if err != nil {
		t.Fatalf("unexpected encode error: %v", err)
	}
	claims, err := codec.Decode(token)
	if err != nil {
		t.Fatalf("unexpected decode error: %v", err)
	}
	if claims.Platform != consts.OJPlatformLeetcode || claims.Title != "Two Sum" || claims.QuestionID != 100 {
		t.Fatalf("unexpected claims: %+v", claims)
	}
}

type assertErr string

func (e assertErr) Error() string {
	return string(e)
}
