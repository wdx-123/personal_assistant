package system

import (
	"context"
	stderrors "errors"
	"fmt"
	"strings"
	"time"

	"personal_assistant/global"
	"personal_assistant/internal/infrastructure"
	lq "personal_assistant/internal/infrastructure/lanqiao"
	"personal_assistant/internal/model/consts"
	eventdto "personal_assistant/internal/model/dto/event"
	"personal_assistant/internal/model/dto/request"
	resp "personal_assistant/internal/model/dto/response"
	"personal_assistant/internal/model/entity"
	"personal_assistant/internal/repository/interfaces"
	bizerrors "personal_assistant/pkg/errors"
	"personal_assistant/pkg/rediskey"
	"personal_assistant/pkg/redislock"
	"personal_assistant/pkg/util"

	"github.com/go-redis/redis/v8"
	"go.uber.org/zap"
)

const (
	lanqiaoCredentialHashNamespace    = "oj:lanqiao:credential"
	lanqiaoPhoneCipherScope           = "oj:lanqiao:phone"
	lanqiaoPasswordCipherScope        = "oj:lanqiao:password"
	defaultLanqiaoSyncIntervalSeconds = 3600
	defaultLanqiaoSyncUserInterval    = 10
	defaultLanqiaoSyncRecentLimit     = 50
	defaultLanqiaoFailureThreshold    = 3
	defaultLanqiaoFailureCounterTTL   = 24 * time.Hour
	defaultLanqiaoDisableTTL          = 48 * time.Hour
	defaultLanqiaoSubmissionDedupTTL  = 7 * 24 * time.Hour
	defaultLanqiaoStatsRefreshCron    = "@daily"

	lanqiaoSyncAlertReasonSyncDisabled       = "sync_disabled"
	lanqiaoSyncAlertReasonCredentialInvalid  = "credential_invalid"
	lanqiaoSyncAlertReasonStatusCheckFailed  = "status_check_failed"
	lanqiaoSyncAlertDisabledMessage          = "蓝桥同步已达到失败阈值，请更换账号或检查后端同步状态"
	lanqiaoSyncAlertCredentialInvalidMessage = "检测到蓝桥登录失效，已暂停自动同步，请重新绑定账号"
	lanqiaoSyncAlertStatusCheckMessage       = "蓝桥同步状态检测失败，请检查后端同步状态"
	lanqiaoCredentialInvalidBindMessage      = "蓝桥账号或密码不正确，请检查后重新绑定"
)

// BindLanqiaoAccount 绑定蓝桥账号
func (s *OJService) BindLanqiaoAccount(
	ctx context.Context,
	userID uint,
	req *request.BindLanqiaoAccountReq,
) (*resp.BindOJAccountResp, error) {
	if req == nil {
		return nil, bizerrors.New(bizerrors.CodeInvalidParams)
	}
	if userID == 0 {
		return nil, bizerrors.New(bizerrors.CodeLoginRequired)
	}

	phone, err := normalizeLanqiaoPhone(req.Phone)
	if err != nil {
		return nil, bizerrors.NewWithMsg(bizerrors.CodeOJCredentialInvalid, "蓝桥手机号格式不合法")
	}
	if strings.TrimSpace(req.Password) == "" {
		return nil, bizerrors.NewWithMsg(bizerrors.CodeOJCredentialInvalid, "蓝桥密码不能为空")
	}

	credentialHash, phoneCipher, passwordCipher, maskedPhone, err := s.buildLanqiaoCredentialSnapshot(phone, req.Password)
	if err != nil {
		return nil, bizerrors.Wrap(bizerrors.CodeInternalError, err)
	}

	existingDetail, err := s.lanqiaoRepo.GetByUserID(ctx, userID)
	if err != nil {
		return nil, bizerrors.Wrap(bizerrors.CodeDBError, err)
	}
	conflictDetail, err := s.lanqiaoRepo.GetByCredentialHash(ctx, credentialHash)
	if err != nil {
		return nil, bizerrors.Wrap(bizerrors.CodeDBError, err)
	}
	if conflictDetail != nil && conflictDetail.UserID != userID {
		return nil, bizerrors.New(bizerrors.CodeOJAccountBound)
	}

	out, err := infrastructure.Lanqiao().SolveStats(ctx, phone, req.Password, 0)
	if err != nil {
		return nil, s.wrapLanqiaoRemoteError(err)
	}
	if out == nil || !out.OK {
		return nil, bizerrors.New(bizerrors.CodeOJSyncFailed)
	}

	now := time.Now()
	resetCurve := existingDetail != nil && existingDetail.CredentialHash != credentialHash
	submitSuccessCount, submitFailedCount := extractLanqiaoSubmitStats(out)

	var (
		saved       *entity.LanqiaoUserDetail
		passedCount int64
	)
	if s.txRunner == nil {
		return nil, bizerrors.NewWithMsg(bizerrors.CodeInternalError, "事务执行器未初始化")
	}
	err = s.txRunner.InTx(ctx, func(tx any) error {
		detailRepo := s.lanqiaoRepo.WithTx(tx)
		questionBankRepo := s.lanqiaoQuestionBankRepo.WithTx(tx)
		userQuestionRepo := s.lanqiaoUserQuestionRepo.WithTx(tx)

		if resetCurve {
			if err := detailRepo.DeleteByUserID(ctx, userID); err != nil {
				return err
			}
		}

		detail := &entity.LanqiaoUserDetail{
			CredentialHash:       credentialHash,
			PhoneCipher:          phoneCipher,
			PasswordCipher:       passwordCipher,
			MaskedPhone:          maskedPhone,
			SubmitSuccessCount:   submitSuccessCount,
			SubmitFailedCount:    submitFailedCount,
			SubmitStatsUpdatedAt: &now,
			LastBindAt:           &now,
			LastSyncAt:           &now,
			UserID:               userID,
		}
		saved, err = detailRepo.UpsertByUserID(ctx, detail)
		if err != nil {
			return err
		}

		_, passedCount, err = s.persistLanqiaoPassedFacts(
			ctx,
			tx,
			questionBankRepo,
			userQuestionRepo,
			saved.ID,
			out.Data.Problems,
		)
		return err
	})
	if err != nil {
		return nil, bizerrors.Wrap(bizerrors.CodeDBError, err)
	}

	if err := s.clearLanqiaoRuntimeState(ctx, userID); err != nil {
		global.Log.Warn("failed to clear lanqiao runtime state", zap.Uint("user_id", userID), zap.Error(err))
	}
	if err := s.publishOJProfileProjectionEvent(ctx, userID, "lanqiao"); err != nil {
		global.Log.Error("failed to publish lanqiao profile projection event", zap.Uint("user_id", userID), zap.Error(err))
	}
	if err := s.publishOJDailyStatsProjectionEvent(ctx, userID, "lanqiao", resetCurve); err != nil {
		global.Log.Error("failed to publish lanqiao daily stats projection event", zap.Uint("user_id", userID), zap.Bool("reset", resetCurve), zap.Error(err))
	}

	return &resp.BindOJAccountResp{
		Platform:           "lanqiao",
		Identifier:         saved.MaskedPhone,
		PassedNumber:       int(passedCount),
		SubmitSuccessCount: saved.SubmitSuccessCount,
		SubmitFailedCount:  saved.SubmitFailedCount,
	}, nil
}

func (s *OJService) SyncAllLanqiaoUsers(ctx context.Context) error {
	lockKey := redislock.LockKeyLanqiaoSyncAllUsers
	err := redislock.WithLock(ctx, lockKey, 30*time.Second, func() error {
		return s.syncAllLanqiaoUsersLocked(ctx)
	})
	if err != nil && stderrors.Is(err, redislock.ErrLockFailed) {
		global.Log.Info("lanqiao sync skipped: lock is held", zap.String("lock_key", lockKey))
		return nil
	}
	return err
}

func (s *OJService) syncAllLanqiaoUsersLocked(ctx context.Context) error {
	users, err := s.lanqiaoRepo.GetAll(ctx)
	if err != nil {
		return err
	}
	return s.syncLanqiaoUsersWithRateLimit(ctx, users)
}

func (s *OJService) RefreshAllLanqiaoSubmissionStats(ctx context.Context) error {
	lockKey := redislock.LockKeyLanqiaoSyncAllUsers
	err := redislock.WithLock(ctx, lockKey, 30*time.Second, func() error {
		users, err := s.lanqiaoRepo.GetAll(ctx)
		if err != nil {
			return err
		}
		activeUsers, err := s.buildActiveUserSetFromLanqiaoDetails(ctx, users)
		if err != nil {
			return err
		}
		intervalSeconds := resolveLanqiaoSyncUserIntervalSeconds()
		ticker := time.NewTicker(time.Duration(intervalSeconds) * time.Second)
		defer ticker.Stop()

		for _, u := range users {
			if u == nil || !activeUsers[u.UserID] {
				continue
			}
			disabled, err := s.isLanqiaoSyncDisabled(ctx, u.UserID)
			if err != nil {
				return err
			}
			if disabled {
				continue
			}
			if err := s.waitTickerOrCancel(ctx, ticker); err != nil {
				return err
			}

			err = redislock.WithLock(ctx, redislock.LockKeyLanqiaoSyncSingleUser(u.UserID), 10*time.Second, func() error {
				return s.refreshSingleLanqiaoSubmissionStats(ctx, u)
			})
			if err != nil {
				if stderrors.Is(err, redislock.ErrLockFailed) {
					continue
				}
				s.handleLanqiaoSyncFailure(ctx, u.UserID, err)
				global.Log.Error("failed to refresh lanqiao submission stats", zap.Uint("user_id", u.UserID), zap.Error(err))
				continue
			}
			s.clearLanqiaoFailureCounter(ctx, u.UserID)
		}
		return nil
	})
	if err != nil && stderrors.Is(err, redislock.ErrLockFailed) {
		global.Log.Info("lanqiao stats refresh skipped: lock is held", zap.String("lock_key", lockKey))
		return nil
	}
	return err
}

func (s *OJService) syncLanqiaoUsersWithRateLimit(
	ctx context.Context,
	users []*entity.LanqiaoUserDetail,
) error {
	activeUsers, err := s.buildActiveUserSetFromLanqiaoDetails(ctx, users)
	if err != nil {
		return err
	}

	intervalSeconds := resolveLanqiaoSyncUserIntervalSeconds()
	ticker := time.NewTicker(time.Duration(intervalSeconds) * time.Second)
	defer ticker.Stop()

	for _, u := range users {
		if u == nil || !activeUsers[u.UserID] {
			continue
		}
		disabled, err := s.isLanqiaoSyncDisabled(ctx, u.UserID)
		if err != nil {
			return err
		}
		if disabled {
			continue
		}
		if err := s.waitTickerOrCancel(ctx, ticker); err != nil {
			global.Log.Info("sync lanqiao users canceled", zap.Error(err))
			return err
		}

		err = redislock.WithLock(ctx, redislock.LockKeyLanqiaoSyncSingleUser(u.UserID), 10*time.Second, func() error {
			return s.syncSingleLanqiaoUser(ctx, u)
		})
		if err != nil {
			if stderrors.Is(err, redislock.ErrLockFailed) {
				global.Log.Warn("skip syncing lanqiao user: lock held", zap.Uint("user_id", u.UserID))
				continue
			}
			s.handleLanqiaoSyncFailure(ctx, u.UserID, err)
			global.Log.Error("failed to sync lanqiao user", zap.Uint("user_id", u.UserID), zap.Error(err))
			continue
		}
		s.clearLanqiaoFailureCounter(ctx, u.UserID)
	}

	return nil
}

func (s *OJService) syncSingleLanqiaoUser(
	ctx context.Context,
	detail *entity.LanqiaoUserDetail,
) error {
	if detail == nil {
		return stderrors.New("nil lanqiao detail")
	}
	phone, password, err := s.decryptLanqiaoCredentials(detail)
	if err != nil {
		return err
	}

	out, err := infrastructure.Lanqiao().SolveStats(ctx, phone, password, resolveLanqiaoSyncRecentLimit())
	if err != nil {
		return err
	}
	if out == nil || !out.OK {
		return stderrors.New("lanqiao api response not ok")
	}

	seenProblems, err := s.filterLanqiaoIncrementalProblems(ctx, detail.UserID, out.Data.Problems)
	if err != nil {
		return err
	}

	now := time.Now()
	var (
		newRecords  int
		passedCount int64
	)
	if s.txRunner == nil {
		return stderrors.New("transaction runner is nil")
	}
	err = s.txRunner.InTx(ctx, func(tx any) error {
		detailRepo := s.lanqiaoRepo.WithTx(tx)
		questionBankRepo := s.lanqiaoQuestionBankRepo.WithTx(tx)
		userQuestionRepo := s.lanqiaoUserQuestionRepo.WithTx(tx)

		updated := *detail
		updated.LastSyncAt = &now
		if _, err := detailRepo.UpsertByUserID(ctx, &updated); err != nil {
			return err
		}

		newRecords, passedCount, err = s.persistLanqiaoPassedFacts(
			ctx,
			tx,
			questionBankRepo,
			userQuestionRepo,
			detail.ID,
			seenProblems,
		)
		return err
	})
	if err != nil {
		return err
	}

	if newRecords > 0 {
		if err := s.publishOJProfileProjectionEvent(ctx, detail.UserID, "lanqiao"); err != nil {
			global.Log.Error("failed to publish lanqiao profile projection event", zap.Uint("user_id", detail.UserID), zap.Error(err))
		}
		if err := s.publishOJDailyStatsProjectionEvent(ctx, detail.UserID, "lanqiao", false); err != nil {
			global.Log.Error("failed to publish lanqiao daily stats projection event", zap.Uint("user_id", detail.UserID), zap.Error(err))
		}
		global.Log.Info("synced lanqiao user records", zap.Uint("user_id", detail.UserID), zap.Int("new_records", newRecords), zap.Int64("passed_count", passedCount))
	}

	return nil
}

func (s *OJService) refreshSingleLanqiaoSubmissionStats(
	ctx context.Context,
	detail *entity.LanqiaoUserDetail,
) error {
	if detail == nil {
		return stderrors.New("nil lanqiao detail")
	}
	phone, password, err := s.decryptLanqiaoCredentials(detail)
	if err != nil {
		return err
	}

	out, err := infrastructure.Lanqiao().SolveStats(ctx, phone, password, -1)
	if err != nil {
		return err
	}
	if out == nil || !out.OK {
		return stderrors.New("lanqiao stats response not ok")
	}

	successCount, failedCount := extractLanqiaoSubmitStats(out)
	now := time.Now()
	updated := *detail
	updated.SubmitSuccessCount = successCount
	updated.SubmitFailedCount = failedCount
	updated.SubmitStatsUpdatedAt = &now
	_, err = s.lanqiaoRepo.UpsertByUserID(ctx, &updated)
	return err
}

func (s *OJService) persistLanqiaoPassedFacts(
	ctx context.Context,
	tx any,
	questionBankRepo interfaces.LanqiaoQuestionBankRepository,
	userQuestionRepo interfaces.LanqiaoUserQuestionRepository,
	lanqiaoUserDetailID uint,
	problems []lq.SolveStatsProblem,
) (int, int64, error) {
	if lanqiaoUserDetailID == 0 {
		return 0, 0, stderrors.New("invalid lanqiao detail id")
	}

	passedByProblem := make(map[int]lq.SolveStatsProblem)
	for _, problem := range problems {
		if !problem.IsPassed || problem.ProblemID <= 0 {
			continue
		}
		existing, ok := passedByProblem[problem.ProblemID]
		if !ok {
			passedByProblem[problem.ProblemID] = problem
			continue
		}
		currentTime := parseLanqiaoProblemTime(problem.CreatedAt)
		existingTime := parseLanqiaoProblemTime(existing.CreatedAt)
		if !currentTime.IsZero() && (existingTime.IsZero() || currentTime.Before(existingTime)) {
			passedByProblem[problem.ProblemID] = problem
		}
	}
	if len(passedByProblem) == 0 {
		count, err := userQuestionRepo.CountPassed(ctx, lanqiaoUserDetailID)
		return 0, count, err
	}

	existingSolved, err := userQuestionRepo.GetSolvedProblemIDs(ctx, lanqiaoUserDetailID)
	if err != nil {
		return 0, 0, err
	}

	newRelations := make([]*entity.LanqiaoUserQuestion, 0, len(passedByProblem))
	for problemID, problem := range passedByProblem {
		question := &entity.LanqiaoQuestionBank{
			ProblemID: problemID,
			Title:     strings.TrimSpace(problem.ProblemName),
		}
		questionID, err := questionBankRepo.EnsureQuestionID(ctx, question)
		if err != nil {
			return 0, 0, err
		}
		if questionID > 0 && s.questionUpsertPublisher != nil {
			if err := s.questionUpsertPublisher.PublishInTx(ctx, tx, &eventdto.QuestionUpsertedEvent{
				Platform:     consts.OJPlatformLanqiao,
				QuestionID:   questionID,
				QuestionCode: fmt.Sprintf("%d", problemID),
				Title:        strings.TrimSpace(question.Title),
			}); err != nil {
				return 0, 0, err
			}
		}
		if _, ok := existingSolved[questionID]; ok {
			continue
		}
		newRelations = append(newRelations, &entity.LanqiaoUserQuestion{
			LanqiaoUserDetailID: lanqiaoUserDetailID,
			LanqiaoQuestionID:   questionID,
			SolvedAt:            normalizeLanqiaoSolvedAt(problem.CreatedAt),
		})
		existingSolved[questionID] = struct{}{}
	}

	if err := userQuestionRepo.BatchCreate(ctx, newRelations); err != nil {
		return 0, 0, err
	}
	passedCount, err := userQuestionRepo.CountPassed(ctx, lanqiaoUserDetailID)
	if err != nil {
		return 0, 0, err
	}
	return len(newRelations), passedCount, nil
}

func (s *OJService) filterLanqiaoIncrementalProblems(
	ctx context.Context,
	userID uint,
	problems []lq.SolveStatsProblem,
) ([]lq.SolveStatsProblem, error) {
	if len(problems) == 0 {
		return nil, nil
	}
	result := make([]lq.SolveStatsProblem, 0, len(problems))
	for _, problem := range problems {
		if !problem.IsPassed || problem.ProblemID <= 0 {
			continue
		}
		fingerprint := util.FileHashBytes([]byte(fmt.Sprintf("%d|%s|%t", problem.ProblemID, strings.TrimSpace(problem.CreatedAt), problem.IsPassed)))
		ok, err := s.markLanqiaoSubmissionSeen(ctx, userID, fingerprint)
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}
		result = append(result, problem)
	}
	return result, nil
}

func (s *OJService) buildLanqiaoCredentialSnapshot(
	phone string,
	password string,
) (credentialHash string, phoneCipher string, passwordCipher string, maskedPhone string, err error) {
	if global.SensitiveDataCodec == nil {
		return "", "", "", "", stderrors.New("sensitive data codec is nil")
	}
	credentialHash, err = global.SensitiveDataCodec.HashIndex(lanqiaoCredentialHashNamespace, phone, password)
	if err != nil {
		return "", "", "", "", err
	}
	phoneCipher, err = global.SensitiveDataCodec.Encrypt(lanqiaoPhoneCipherScope, phone)
	if err != nil {
		return "", "", "", "", err
	}
	passwordCipher, err = global.SensitiveDataCodec.Encrypt(lanqiaoPasswordCipherScope, password)
	if err != nil {
		return "", "", "", "", err
	}
	return credentialHash, phoneCipher, passwordCipher, util.DesensitizePhone(phone), nil
}

func (s *OJService) decryptLanqiaoCredentials(detail *entity.LanqiaoUserDetail) (string, string, error) {
	if detail == nil {
		return "", "", stderrors.New("nil lanqiao detail")
	}
	if global.SensitiveDataCodec == nil {
		return "", "", stderrors.New("sensitive data codec is nil")
	}
	phone, err := global.SensitiveDataCodec.Decrypt(lanqiaoPhoneCipherScope, detail.PhoneCipher)
	if err != nil {
		return "", "", err
	}
	password, err := global.SensitiveDataCodec.Decrypt(lanqiaoPasswordCipherScope, detail.PasswordCipher)
	if err != nil {
		return "", "", err
	}
	return phone, password, nil
}

func (s *OJService) buildActiveUserSetFromLanqiaoDetails(
	ctx context.Context,
	details []*entity.LanqiaoUserDetail,
) (map[uint]bool, error) {
	userIDs := make([]uint, 0, len(details))
	for _, item := range details {
		if item != nil && item.UserID > 0 {
			userIDs = append(userIDs, item.UserID)
		}
	}
	return s.buildActiveUserSet(ctx, userIDs)
}

// classifyLanqiaoRemoteError 分类蓝桥远程错误
func classifyLanqiaoRemoteError(err error) string {
	// 1. 如果错误为空，则返回空字符串
	if err == nil {
		return ""
	}
	// 2. 如果错误不是 RemoteHTTPError，则返回空字符串
	var remoteErr *lq.RemoteHTTPError
	if !stderrors.As(err, &remoteErr) || remoteErr == nil {
		return ""
	}
	// 3. 根据状态码分类错误
	switch remoteErr.StatusCode {
	// 4. 如果状态码为 401 或 403，则返回蓝桥账号无效
	case 401, 403:
		return lanqiaoSyncAlertReasonCredentialInvalid
	case 400:
		if isLanqiaoLoginErrorMessage(remoteErr.Message) || isLanqiaoLoginErrorMessage(remoteErr.Body) {
			return lanqiaoSyncAlertReasonCredentialInvalid
		}
	}
	return ""
}

func isLanqiaoLoginErrorMessage(raw string) bool {
	raw = strings.ToLower(strings.TrimSpace(raw))
	if raw == "" {
		return false
	}
	return strings.Contains(raw, "[login]") || strings.Contains(raw, "lanqiao login http 400")
}

func (s *OJService) wrapLanqiaoRemoteError(err error) error {
	if err == nil {
		return nil
	}
	if classifyLanqiaoRemoteError(err) == lanqiaoSyncAlertReasonCredentialInvalid {
		return bizerrors.NewWithMsg(bizerrors.CodeOJCredentialInvalid, lanqiaoCredentialInvalidBindMessage)
	}
	return bizerrors.Wrap(bizerrors.CodeOJSyncFailed, err)
}

func (s *OJService) markLanqiaoSubmissionSeen(
	ctx context.Context,
	userID uint,
	fingerprint string,
) (bool, error) {
	if global.Redis == nil || userID == 0 || strings.TrimSpace(fingerprint) == "" {
		return true, nil
	}
	return global.Redis.SetNX(
		ctx,
		rediskey.LanqiaoSubmissionSeenKey(userID, fingerprint),
		"1",
		resolveLanqiaoSubmissionDedupTTL(),
	).Result()
}

func (s *OJService) clearLanqiaoRuntimeState(ctx context.Context, userID uint) error {
	if global.Redis == nil || userID == 0 {
		return nil
	}
	return global.Redis.Del(
		ctx,
		rediskey.LanqiaoSyncFailKey(userID),
		rediskey.LanqiaoSyncDisableKey(userID),
	).Err()
}

func (s *OJService) clearLanqiaoFailureCounter(ctx context.Context, userID uint) {
	if global.Redis == nil || userID == 0 {
		return
	}
	if err := global.Redis.Del(ctx, rediskey.LanqiaoSyncFailKey(userID)).Err(); err != nil {
		global.Log.Warn("failed to clear lanqiao failure counter", zap.Uint("user_id", userID), zap.Error(err))
	}
}

func normalizeLanqiaoSyncDisableReason(reason string) string {
	reason = strings.ToLower(strings.TrimSpace(reason))
	switch reason {
	case lanqiaoSyncAlertReasonCredentialInvalid:
		return lanqiaoSyncAlertReasonCredentialInvalid
	case "", "1", lanqiaoSyncAlertReasonSyncDisabled:
		return lanqiaoSyncAlertReasonSyncDisabled
	default:
		// 兼容历史值和未知值，统一回落为普通停用。
		return lanqiaoSyncAlertReasonSyncDisabled
	}
}

// getLanqiaoSyncDisableReason 获取蓝桥同步禁用原因
func (s *OJService) getLanqiaoSyncDisableReason(ctx context.Context, userID uint) (string, error) {
	if global.Redis == nil || userID == 0 {
		return "", nil
	}
	reason, err := global.Redis.Get(ctx, rediskey.LanqiaoSyncDisableKey(userID)).Result()
	if err != nil {
		if stderrors.Is(err, redis.Nil) {
			return "", nil
		}
		return "", err
	}
	return normalizeLanqiaoSyncDisableReason(reason), nil
}

// setLanqiaoSyncDisableReason 设置蓝桥同步禁用原因
func (s *OJService) setLanqiaoSyncDisableReason(ctx context.Context, userID uint, reason string) error {
	if global.Redis == nil || userID == 0 {
		return nil
	}
	return global.Redis.Set(
		ctx,
		rediskey.LanqiaoSyncDisableKey(userID),
		normalizeLanqiaoSyncDisableReason(reason),
		resolveLanqiaoDisableTTL(),
	).Err()
}

// isLanqiaoSyncDisabled 判断蓝桥同步是否禁用
func (s *OJService) isLanqiaoSyncDisabled(ctx context.Context, userID uint) (bool, error) {
	reason, err := s.getLanqiaoSyncDisableReason(ctx, userID)
	if err != nil {
		return false, err
	}
	return reason != "", nil
}

// handleLanqiaoSyncFailure 处理蓝桥同步失败
func (s *OJService) handleLanqiaoSyncFailure(ctx context.Context, userID uint, err error) {
	// 1. 如果 Redis 为空，或者用户 ID 为 0，或者错误为空，则返回
	if global.Redis == nil || userID == 0 || err == nil {
		return
	}
	// 2. 如果错误为蓝桥账号无效，则设置蓝桥同步禁用原因并清除失败计数器
	if classifyLanqiaoRemoteError(err) == lanqiaoSyncAlertReasonCredentialInvalid {
		if redisErr := s.setLanqiaoSyncDisableReason(ctx, userID, lanqiaoSyncAlertReasonCredentialInvalid); redisErr != nil {
			global.Log.Error("failed to disable lanqiao sync after credential invalid", zap.Uint("user_id", userID), zap.Error(redisErr))
			return
		}
		s.clearLanqiaoFailureCounter(ctx, userID)
		global.Log.Warn("lanqiao sync disabled due to invalid credential", zap.Uint("user_id", userID), zap.Error(err))
		return
	}
	failKey := rediskey.LanqiaoSyncFailKey(userID)
	failCount, redisErr := global.Redis.Incr(ctx, failKey).Result()
	if redisErr != nil {
		global.Log.Error("failed to incr lanqiao sync failure counter", zap.Uint("user_id", userID), zap.Error(redisErr))
		return
	}
	_ = global.Redis.Expire(ctx, failKey, resolveLanqiaoFailureCounterTTL()).Err()
	if failCount < int64(resolveLanqiaoFailureThreshold()) {
		return
	}
	if redisErr := s.setLanqiaoSyncDisableReason(ctx, userID, lanqiaoSyncAlertReasonSyncDisabled); redisErr != nil {
		global.Log.Error("failed to disable lanqiao sync", zap.Uint("user_id", userID), zap.Error(redisErr))
		return
	}
	s.clearLanqiaoFailureCounter(ctx, userID)
	global.Log.Warn("lanqiao sync auto-disabled", zap.Uint("user_id", userID), zap.Error(err))
}

// buildLanqiaoSyncAlert 构建蓝桥同步告警
func (s *OJService) buildLanqiaoSyncAlert(ctx context.Context, userID uint) *resp.LanqiaoSyncAlertResp {
	if userID == 0 {
		return nil
	}

	// 获取蓝桥同步失败阈值
	threshold := resolveLanqiaoFailureThreshold()
	if global.Redis == nil {
		return buildLanqiaoStatusCheckFailedAlert(threshold)
	}

	// 获取蓝桥同步禁用原因
	disableReason, err := s.getLanqiaoSyncDisableReason(ctx, userID)
	if err != nil {
		s.logLanqiaoSyncAlertCheckError(userID, err)
		return buildLanqiaoStatusCheckFailedAlert(threshold)
	}

	// 根据禁用原因构建告警
	switch disableReason {
	case lanqiaoSyncAlertReasonCredentialInvalid:
		return buildLanqiaoCredentialInvalidAlert(threshold)
	case lanqiaoSyncAlertReasonSyncDisabled:
		return buildLanqiaoSyncDisabledAlert(threshold)
	}

	failCount, err := global.Redis.Get(ctx, rediskey.LanqiaoSyncFailKey(userID)).Int()
	if err != nil {
		if stderrors.Is(err, redis.Nil) {
			return nil
		}
		s.logLanqiaoSyncAlertCheckError(userID, err)
		return buildLanqiaoStatusCheckFailedAlert(threshold)
	}
	if failCount >= threshold {
		return buildLanqiaoSyncDisabledAlert(threshold)
	}
	return nil
}

// logLanqiaoSyncAlertCheckError 记录蓝桥同步告警检查错误
func (s *OJService) logLanqiaoSyncAlertCheckError(userID uint, err error) {
	if err == nil || global.Log == nil {
		return
	}
	global.Log.Error("failed to check lanqiao sync alert state", zap.Uint("user_id", userID), zap.Error(err))
}

// buildLanqiaoSyncDisabledAlert 构建蓝桥同步禁用告警
func buildLanqiaoSyncDisabledAlert(threshold int) *resp.LanqiaoSyncAlertResp {
	return &resp.LanqiaoSyncAlertResp{
		Danger:           true,
		Reason:           lanqiaoSyncAlertReasonSyncDisabled,
		Message:          lanqiaoSyncAlertDisabledMessage,
		FailureThreshold: threshold,
		SyncDisabled:     true,
	}
}

// buildLanqiaoCredentialInvalidAlert 构建蓝桥账号无效告警
func buildLanqiaoCredentialInvalidAlert(threshold int) *resp.LanqiaoSyncAlertResp {
	return &resp.LanqiaoSyncAlertResp{
		Danger:           true,
		Reason:           lanqiaoSyncAlertReasonCredentialInvalid,
		Message:          lanqiaoSyncAlertCredentialInvalidMessage,
		FailureThreshold: threshold,
		SyncDisabled:     true,
	}
}

// buildLanqiaoStatusCheckFailedAlert 构建蓝桥状态检查失败告警
func buildLanqiaoStatusCheckFailedAlert(threshold int) *resp.LanqiaoSyncAlertResp {
	return &resp.LanqiaoSyncAlertResp{
		Danger:           true,
		Reason:           lanqiaoSyncAlertReasonStatusCheckFailed,
		Message:          lanqiaoSyncAlertStatusCheckMessage,
		FailureThreshold: threshold,
		SyncDisabled:     false,
	}
}

// extractLanqiaoSubmitStats 提取蓝桥提交统计
func extractLanqiaoSubmitStats(out *lq.SolveStatsResponse) (int, int) {
	if out == nil || out.Data.Stats == nil {
		return 0, 0
	}
	return out.Data.Stats.TotalPassed, out.Data.Stats.TotalFailed
}

func normalizeLanqiaoPhone(phone string) (string, error) {
	phone = strings.TrimSpace(phone)
	replacer := strings.NewReplacer(" ", "", "-", "", "(", "", ")", "", "+", "")
	phone = replacer.Replace(phone)
	if strings.HasPrefix(phone, "86") && len(phone) == 13 {
		phone = strings.TrimPrefix(phone, "86")
	}
	if len(phone) != 11 {
		return "", stderrors.New("invalid lanqiao phone")
	}
	for _, ch := range phone {
		if ch < '0' || ch > '9' {
			return "", stderrors.New("invalid lanqiao phone")
		}
	}
	return phone, nil
}

func parseLanqiaoProblemTime(raw string) time.Time {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}
	}
	layouts := []string{
		time.RFC3339,
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05",
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, raw); err == nil {
			return t
		}
		if t, err := time.ParseInLocation(layout, raw, ojDailyStatsLocation()); err == nil {
			return t
		}
	}
	return time.Time{}
}

func normalizeLanqiaoSolvedAt(raw string) time.Time {
	if solvedAt := parseLanqiaoProblemTime(raw); !solvedAt.IsZero() {
		return solvedAt
	}
	return time.Now().In(ojDailyStatsLocation())
}

func resolveLanqiaoSyncUserIntervalSeconds() int {
	if global.Config == nil || global.Config.Task.LanqiaoSyncUserIntervalSeconds <= 0 {
		return defaultLanqiaoSyncUserInterval
	}
	return global.Config.Task.LanqiaoSyncUserIntervalSeconds
}

func resolveLanqiaoSyncRecentLimit() int {
	if global.Config == nil || global.Config.Task.LanqiaoSyncRecentLimit <= 0 {
		return defaultLanqiaoSyncRecentLimit
	}
	return global.Config.Task.LanqiaoSyncRecentLimit
}

func resolveLanqiaoFailureThreshold() int {
	if global.Config == nil || global.Config.Task.LanqiaoFailureThreshold <= 0 {
		return defaultLanqiaoFailureThreshold
	}
	return global.Config.Task.LanqiaoFailureThreshold
}

func resolveLanqiaoFailureCounterTTL() time.Duration {
	if global.Config == nil || global.Config.Task.LanqiaoFailureCounterTTLSeconds <= 0 {
		return defaultLanqiaoFailureCounterTTL
	}
	return time.Duration(global.Config.Task.LanqiaoFailureCounterTTLSeconds) * time.Second
}

func resolveLanqiaoDisableTTL() time.Duration {
	if global.Config == nil || global.Config.Task.LanqiaoDisableTTLSeconds <= 0 {
		return defaultLanqiaoDisableTTL
	}
	return time.Duration(global.Config.Task.LanqiaoDisableTTLSeconds) * time.Second
}

func resolveLanqiaoSubmissionDedupTTL() time.Duration {
	if global.Config == nil || global.Config.Task.LanqiaoSubmissionDedupTTLSeconds <= 0 {
		return defaultLanqiaoSubmissionDedupTTL
	}
	return time.Duration(global.Config.Task.LanqiaoSubmissionDedupTTLSeconds) * time.Second
}
