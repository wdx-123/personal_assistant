package system

import (
	"context"
	"fmt"
	"strings"
	"time"

	"personal_assistant/internal/model/consts"
	"personal_assistant/internal/model/dto/request"
	dtoresp "personal_assistant/internal/model/dto/response"
	"personal_assistant/internal/model/entity"
	bizerrors "personal_assistant/pkg/errors"
)

// normalizedOJTaskItem 表示完成基础参数规范化后的题目草稿项。
// 这一层只保证字段组合合法，不保证题目已成功命中题库。
type normalizedOJTaskItem struct {
	Platform      string
	Title         string
	AnalysisToken string
}

// normalizedOJTaskDraft 表示完成入参校验后的任务草稿。
type normalizedOJTaskDraft struct {
	Title       string
	Description string
	Mode        string
	ExecuteAt   *time.Time
	OrgIDs      []uint
	Items       []normalizedOJTaskItem
}

// ojTaskAnalyzeCandidate 统一承载单平台精确命中的候选题目。
type ojTaskAnalyzeCandidate struct {
	Platform     string
	QuestionID   uint
	QuestionCode string
	Title        string
}

// AnalyzeTaskTitles 分析 OJTask 题目标题，仅在本地单平台题库中查找。
func (s *OJTaskService) AnalyzeTaskTitles(
	ctx context.Context,
	req *request.AnalyzeOJTaskTitlesReq,
) (*dtoresp.OJTaskAnalyzeResp, error) {
	if req == nil || len(req.Items) == 0 {
		return nil, bizerrors.NewWithMsg(bizerrors.CodeInvalidParams, "items 不能为空")
	}

	resp := &dtoresp.OJTaskAnalyzeResp{
		Resolved:  make([]*dtoresp.OJTaskAnalyzeResolvedItemResp, 0),
		Ambiguous: make([]*dtoresp.OJTaskAnalyzeAmbiguousItemResp, 0),
		Missing:   make([]*dtoresp.OJTaskAnalyzeMissingItemResp, 0),
	}
	for idx, raw := range req.Items {
		platform := strings.TrimSpace(raw.Platform)
		title := normalizeOJTaskTitle(raw.Title)
		if platform == "" || title == "" {
			return nil, bizerrors.NewWithMsg(bizerrors.CodeInvalidParams, "platform/title 不能为空")
		}

		candidates, err := s.findVerifiedTitleCandidates(ctx, nil, platform, title)
		if err != nil {
			return nil, err
		}
		switch len(candidates) {
		case 0:
			resp.Missing = append(resp.Missing, &dtoresp.OJTaskAnalyzeMissingItemResp{
				InputIndex: idx,
				Platform:   platform,
				Title:      title,
			})
		case 1:
			token, err := s.analysisTokenCodec.Encode(platform, title, candidates[0].QuestionID)
			if err != nil {
				return nil, bizerrors.Wrap(bizerrors.CodeInternalError, err)
			}
			resp.Resolved = append(resp.Resolved, mapAnalyzeResolvedItem(idx, title, candidates[0], token))
		default:
			resp.Ambiguous = append(resp.Ambiguous, mapAnalyzeAmbiguousItem(idx, platform, title, candidates, s.analysisTokenCodec))
		}
	}
	return resp, nil
}

// validateDraft 校验并规范化任务草稿。
func (s *OJTaskService) validateDraft(
	ctx context.Context,
	title, description, mode string,
	executeAt *time.Time,
	orgIDs []uint,
	items []request.OJTaskItemReq,
) (normalizedOJTaskDraft, error) {
	trimmedTitle := strings.TrimSpace(title)
	if trimmedTitle == "" {
		return normalizedOJTaskDraft{}, bizerrors.NewWithMsg(bizerrors.CodeInvalidParams, "任务标题不能为空")
	}

	normalizedMode := strings.TrimSpace(mode)
	if !consts.IsValidOJTaskMode(normalizedMode) {
		return normalizedOJTaskDraft{}, bizerrors.New(bizerrors.CodeInvalidParams)
	}

	now := time.Now()
	var normalizedExecuteAt *time.Time
	switch normalizedMode {
	case string(consts.OJTaskModeImmediate):
		if executeAt != nil {
			return normalizedOJTaskDraft{}, bizerrors.NewWithMsg(
				bizerrors.CodeOJTaskExecuteAtInvalid,
				"立即任务不允许传 execute_at",
			)
		}
	case string(consts.OJTaskModeScheduled):
		if executeAt == nil {
			return normalizedOJTaskDraft{}, bizerrors.NewWithMsg(
				bizerrors.CodeOJTaskExecuteAtInvalid,
				"定时任务必须传 execute_at",
			)
		}
		if !executeAt.After(now) {
			return normalizedOJTaskDraft{}, bizerrors.NewWithMsg(
				bizerrors.CodeOJTaskExecuteAtInvalid,
				"execute_at 必须是未来时间",
			)
		}
		value := executeAt.UTC()
		normalizedExecuteAt = &value
	}

	validatedOrgIDs, err := s.validateOrgIDs(ctx, orgIDs)
	if err != nil {
		return normalizedOJTaskDraft{}, err
	}
	normalizedItems, err := s.validateItems(items)
	if err != nil {
		return normalizedOJTaskDraft{}, err
	}

	return normalizedOJTaskDraft{
		Title:       trimmedTitle,
		Description: strings.TrimSpace(description),
		Mode:        normalizedMode,
		ExecuteAt:   normalizedExecuteAt,
		OrgIDs:      validatedOrgIDs,
		Items:       normalizedItems,
	}, nil
}

func (s *OJTaskService) validateItems(items []request.OJTaskItemReq) ([]normalizedOJTaskItem, error) {
	if len(items) == 0 {
		return nil, bizerrors.NewWithMsg(bizerrors.CodeInvalidParams, "items 不能为空")
	}

	normalized := make([]normalizedOJTaskItem, 0, len(items))
	for _, item := range items {
		platform := strings.TrimSpace(item.Platform)
		title := normalizeOJTaskTitle(item.Title)
		if platform == "" || title == "" {
			return nil, bizerrors.NewWithMsg(bizerrors.CodeInvalidParams, "题目 platform/title 不能为空")
		}
		normalized = append(normalized, normalizedOJTaskItem{
			Platform:      platform,
			Title:         title,
			AnalysisToken: strings.TrimSpace(item.AnalysisToken),
		})
	}
	return normalized, nil
}

// materializeDraftTx 在事务内把规范化草稿转换为可持久化草稿。
func (s *OJTaskService) materializeDraftTx(
	ctx context.Context,
	tx any,
	draft normalizedOJTaskDraft,
) (validatedOJTaskDraft, error) {
	items, err := s.materializeDraftItemsTx(ctx, tx, draft.Items)
	if err != nil {
		return validatedOJTaskDraft{}, err
	}
	return validatedOJTaskDraft{
		Title:       draft.Title,
		Description: draft.Description,
		Mode:        draft.Mode,
		ExecuteAt:   draft.ExecuteAt,
		OrgIDs:      draft.OrgIDs,
		Items:       items,
	}, nil
}

func (s *OJTaskService) materializeDraftItemsTx(
	ctx context.Context,
	tx any,
	items []normalizedOJTaskItem,
) ([]validatedOJTaskItem, error) {
	if len(items) == 0 {
		return nil, bizerrors.NewWithMsg(bizerrors.CodeInvalidParams, "items 不能为空")
	}

	seen := make(map[string]struct{}, len(items))
	result := make([]validatedOJTaskItem, 0, len(items))
	for _, item := range items {
		materialized, err := s.materializeDraftItemTx(ctx, tx, item)
		if err != nil {
			return nil, err
		}
		key := normalizeMaterializedTaskItemKey(materialized)
		if _, ok := seen[key]; ok {
			continue
		}
		materialized.SortNo = len(result) + 1
		result = append(result, materialized)
		seen[key] = struct{}{}
	}
	if len(result) == 0 {
		return nil, bizerrors.NewWithMsg(bizerrors.CodeInvalidParams, "items 不能为空")
	}
	return result, nil
}

func (s *OJTaskService) materializeDraftItemTx(
	ctx context.Context,
	tx any,
	item normalizedOJTaskItem,
) (validatedOJTaskItem, error) {
	if item.AnalysisToken != "" {
		claims, err := s.analysisTokenCodec.Decode(item.AnalysisToken)
		if err != nil {
			return validatedOJTaskItem{}, bizerrors.NewWithMsg(bizerrors.CodeInvalidParams, "analysis_token 非法或已过期")
		}
		if claims.Platform != item.Platform || normalizeOJTaskTitle(claims.Title) != item.Title {
			return validatedOJTaskItem{}, bizerrors.NewWithMsg(bizerrors.CodeInvalidParams, "analysis_token 与题目输入不匹配")
		}
		return s.resolveQuestionByIDTx(ctx, tx, item.Platform, item.Title, claims.QuestionID)
	}

	candidates, err := s.findVerifiedTitleCandidates(ctx, tx, item.Platform, item.Title)
	if err != nil {
		return validatedOJTaskItem{}, err
	}
	switch len(candidates) {
	case 0:
		return buildPendingValidatedOJTaskItem(item.Platform, item.Title, "awaiting_question_sync"), nil
	case 1:
		return buildResolvedValidatedOJTaskItem(item.Platform, item.Title, candidates[0]), nil
	default:
		return validatedOJTaskItem{}, bizerrors.NewWithMsg(
			bizerrors.CodeOJTaskQuestionAmbiguous,
			fmt.Sprintf("%s 平台题目标题存在多个候选：%s", item.Platform, item.Title),
		)
	}
}

func (s *OJTaskService) resolveQuestionByIDTx(
	ctx context.Context,
	tx any,
	platform string,
	inputTitle string,
	questionID uint,
) (validatedOJTaskItem, error) {
	candidate, err := s.getVerifiedCandidateByID(ctx, tx, platform, questionID)
	if err != nil {
		return validatedOJTaskItem{}, err
	}
	return buildResolvedValidatedOJTaskItem(platform, inputTitle, candidate), nil
}

func (s *OJTaskService) findVerifiedTitleCandidates(
	ctx context.Context,
	tx any,
	platform string,
	title string,
) ([]ojTaskAnalyzeCandidate, error) {
	candidates, err := s.findExactTitleCandidates(ctx, tx, platform, title)
	if err != nil {
		return nil, err
	}
	return deduplicateAnalyzeCandidates(candidates), nil
}

func (s *OJTaskService) findExactTitleCandidates(
	ctx context.Context,
	tx any,
	platform string,
	title string,
) ([]ojTaskAnalyzeCandidate, error) {
	switch platform {
	case consts.OJPlatformLuogu:
		repo := s.luoguQuestionRepo
		if tx != nil {
			repo = repo.WithTx(tx)
		}
		rows, err := repo.ListByExactTitle(ctx, title)
		if err != nil {
			return nil, bizerrors.Wrap(bizerrors.CodeDBError, err)
		}
		return mapLuoguCandidates(rows), nil
	case consts.OJPlatformLeetcode:
		repo := s.leetcodeQuestionRepo
		if tx != nil {
			repo = repo.WithTx(tx)
		}
		rows, err := repo.ListByExactTitle(ctx, title)
		if err != nil {
			return nil, bizerrors.Wrap(bizerrors.CodeDBError, err)
		}
		return mapLeetcodeCandidates(rows), nil
	case consts.OJPlatformLanqiao:
		repo := s.lanqiaoQuestionRepo
		if tx != nil {
			repo = repo.WithTx(tx)
		}
		rows, err := repo.ListByExactTitle(ctx, title)
		if err != nil {
			return nil, bizerrors.Wrap(bizerrors.CodeDBError, err)
		}
		return mapLanqiaoCandidates(rows), nil
	default:
		return nil, bizerrors.New(bizerrors.CodeInvalidParams)
	}
}

func mapLuoguCandidates(rows []*entity.LuoguQuestionBank) []ojTaskAnalyzeCandidate {
	candidates := make([]ojTaskAnalyzeCandidate, 0, len(rows))
	for _, row := range rows {
		if row == nil || row.ID == 0 || row.SourceStatus != int8(consts.OJQuestionSourceStatusVerified) {
			continue
		}
		candidates = append(candidates, ojTaskAnalyzeCandidate{
			Platform:     consts.OJPlatformLuogu,
			QuestionID:   row.ID,
			QuestionCode: row.Pid,
			Title:        row.Title,
		})
	}
	return candidates
}

func mapLeetcodeCandidates(rows []*entity.LeetcodeQuestionBank) []ojTaskAnalyzeCandidate {
	candidates := make([]ojTaskAnalyzeCandidate, 0, len(rows))
	for _, row := range rows {
		if row == nil || row.ID == 0 || row.SourceStatus != int8(consts.OJQuestionSourceStatusVerified) {
			continue
		}
		candidates = append(candidates, ojTaskAnalyzeCandidate{
			Platform:     consts.OJPlatformLeetcode,
			QuestionID:   row.ID,
			QuestionCode: row.TitleSlug,
			Title:        row.Title,
		})
	}
	return candidates
}

func mapLanqiaoCandidates(rows []*entity.LanqiaoQuestionBank) []ojTaskAnalyzeCandidate {
	candidates := make([]ojTaskAnalyzeCandidate, 0, len(rows))
	for _, row := range rows {
		if row == nil || row.ID == 0 || row.SourceStatus != int8(consts.OJQuestionSourceStatusVerified) {
			continue
		}
		candidates = append(candidates, ojTaskAnalyzeCandidate{
			Platform:     consts.OJPlatformLanqiao,
			QuestionID:   row.ID,
			QuestionCode: fmt.Sprintf("%d", row.ProblemID),
			Title:        row.Title,
		})
	}
	return candidates
}

func (s *OJTaskService) getVerifiedCandidateByID(
	ctx context.Context,
	tx any,
	platform string,
	questionID uint,
) (ojTaskAnalyzeCandidate, error) {
	if questionID == 0 {
		return ojTaskAnalyzeCandidate{}, bizerrors.New(bizerrors.CodeOJTaskQuestionNotFound)
	}
	switch platform {
	case consts.OJPlatformLuogu:
		repo := s.luoguQuestionRepo
		if tx != nil {
			repo = repo.WithTx(tx)
		}
		row, err := repo.GetByID(ctx, questionID)
		if err != nil {
			return ojTaskAnalyzeCandidate{}, bizerrors.Wrap(bizerrors.CodeDBError, err)
		}
		if row == nil || row.ID == 0 || row.SourceStatus != int8(consts.OJQuestionSourceStatusVerified) {
			return ojTaskAnalyzeCandidate{}, bizerrors.New(bizerrors.CodeOJTaskQuestionNotFound)
		}
		return ojTaskAnalyzeCandidate{
			Platform:     platform,
			QuestionID:   row.ID,
			QuestionCode: row.Pid,
			Title:        row.Title,
		}, nil
	case consts.OJPlatformLeetcode:
		repo := s.leetcodeQuestionRepo
		if tx != nil {
			repo = repo.WithTx(tx)
		}
		row, err := repo.GetByID(ctx, questionID)
		if err != nil {
			return ojTaskAnalyzeCandidate{}, bizerrors.Wrap(bizerrors.CodeDBError, err)
		}
		if row == nil || row.ID == 0 || row.SourceStatus != int8(consts.OJQuestionSourceStatusVerified) {
			return ojTaskAnalyzeCandidate{}, bizerrors.New(bizerrors.CodeOJTaskQuestionNotFound)
		}
		return ojTaskAnalyzeCandidate{
			Platform:     platform,
			QuestionID:   row.ID,
			QuestionCode: row.TitleSlug,
			Title:        row.Title,
		}, nil
	case consts.OJPlatformLanqiao:
		repo := s.lanqiaoQuestionRepo
		if tx != nil {
			repo = repo.WithTx(tx)
		}
		row, err := repo.GetByID(ctx, questionID)
		if err != nil {
			return ojTaskAnalyzeCandidate{}, bizerrors.Wrap(bizerrors.CodeDBError, err)
		}
		if row == nil || row.ID == 0 || row.SourceStatus != int8(consts.OJQuestionSourceStatusVerified) {
			return ojTaskAnalyzeCandidate{}, bizerrors.New(bizerrors.CodeOJTaskQuestionNotFound)
		}
		return ojTaskAnalyzeCandidate{
			Platform:     platform,
			QuestionID:   row.ID,
			QuestionCode: fmt.Sprintf("%d", row.ProblemID),
			Title:        row.Title,
		}, nil
	default:
		return ojTaskAnalyzeCandidate{}, bizerrors.New(bizerrors.CodeInvalidParams)
	}
}

func buildResolvedValidatedOJTaskItem(
	platform string,
	inputTitle string,
	candidate ojTaskAnalyzeCandidate,
) validatedOJTaskItem {
	return validatedOJTaskItem{
		Platform:              platform,
		InputTitle:            inputTitle,
		InputMode:             string(consts.OJTaskItemInputModeTitle),
		ResolvedQuestionID:    candidate.QuestionID,
		ResolvedQuestionCode:  candidate.QuestionCode,
		ResolvedTitleSnapshot: candidate.Title,
		ResolutionStatus:      string(consts.OJTaskItemResolutionStatusResolved),
	}
}

func buildPendingValidatedOJTaskItem(platform, inputTitle, note string) validatedOJTaskItem {
	return validatedOJTaskItem{
		Platform:         platform,
		InputTitle:       inputTitle,
		InputMode:        string(consts.OJTaskItemInputModeTitle),
		ResolutionStatus: string(consts.OJTaskItemResolutionStatusPendingResolution),
		ResolutionNote:   note,
	}
}

func normalizeMaterializedTaskItemKey(item validatedOJTaskItem) string {
	return fmt.Sprintf(
		"%s|%s|%d|%s",
		item.Platform,
		item.InputTitle,
		item.ResolvedQuestionID,
		item.ResolutionStatus,
	)
}

func normalizeOJTaskTitle(title string) string {
	return strings.TrimSpace(title)
}

func mapAnalyzeResolvedItem(
	inputIndex int,
	title string,
	candidate ojTaskAnalyzeCandidate,
	token string,
) *dtoresp.OJTaskAnalyzeResolvedItemResp {
	return &dtoresp.OJTaskAnalyzeResolvedItemResp{
		InputIndex:            inputIndex,
		Platform:              candidate.Platform,
		Title:                 title,
		AnalysisToken:         token,
		ResolvedQuestionID:    candidate.QuestionID,
		ResolvedQuestionCode:  candidate.QuestionCode,
		ResolvedTitleSnapshot: candidate.Title,
	}
}

func mapAnalyzeAmbiguousItem(
	inputIndex int,
	platform string,
	title string,
	candidates []ojTaskAnalyzeCandidate,
	codec *ojTaskAnalysisTokenCodec,
) *dtoresp.OJTaskAnalyzeAmbiguousItemResp {
	resp := &dtoresp.OJTaskAnalyzeAmbiguousItemResp{
		InputIndex: inputIndex,
		Platform:   platform,
		Title:      title,
		Options:    make([]*dtoresp.OJTaskAnalyzeCandidateResp, 0, len(candidates)),
	}
	for _, candidate := range candidates {
		token := ""
		if codec != nil {
			if encoded, err := codec.Encode(candidate.Platform, title, candidate.QuestionID); err == nil {
				token = encoded
			}
		}
		resp.Options = append(resp.Options, &dtoresp.OJTaskAnalyzeCandidateResp{
			AnalysisToken:         token,
			ResolvedQuestionID:    candidate.QuestionID,
			ResolvedQuestionCode:  candidate.QuestionCode,
			ResolvedTitleSnapshot: candidate.Title,
		})
	}
	return resp
}

func deduplicateAnalyzeCandidates(candidates []ojTaskAnalyzeCandidate) []ojTaskAnalyzeCandidate {
	seen := make(map[string]struct{}, len(candidates))
	result := make([]ojTaskAnalyzeCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		key := fmt.Sprintf("%s:%d", candidate.Platform, candidate.QuestionID)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, candidate)
	}
	return result
}
