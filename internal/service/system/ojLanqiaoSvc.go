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
	"personal_assistant/internal/model/dto/request"
	resp "personal_assistant/internal/model/dto/response"
	"personal_assistant/internal/model/entity"
	"personal_assistant/internal/repository/interfaces"
	bizerrors "personal_assistant/pkg/errors"
	"personal_assistant/pkg/rediskey"
	"personal_assistant/pkg/redislock"
	"personal_assistant/pkg/util"

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
)

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
				s.recordLanqiaoSyncFailure(ctx, u.UserID, err)
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
			s.recordLanqiaoSyncFailure(ctx, u.UserID, err)
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
		questionID, err := questionBankRepo.EnsureQuestionID(ctx, &entity.LanqiaoQuestionBank{
			ProblemID: problemID,
			Title:     strings.TrimSpace(problem.ProblemName),
		})
		if err != nil {
			return 0, 0, err
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

func (s *OJService) wrapLanqiaoRemoteError(err error) error {
	if err == nil {
		return nil
	}
	var remoteErr *lq.RemoteHTTPError
	if stderrors.As(err, &remoteErr) {
		if remoteErr.StatusCode == 401 || remoteErr.StatusCode == 403 {
			return bizerrors.NewWithMsg(bizerrors.CodeOJCredentialInvalid, "蓝桥账号或密码不正确")
		}
		return bizerrors.Wrap(bizerrors.CodeOJSyncFailed, err)
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

func (s *OJService) isLanqiaoSyncDisabled(ctx context.Context, userID uint) (bool, error) {
	if global.Redis == nil || userID == 0 {
		return false, nil
	}
	exists, err := global.Redis.Exists(ctx, rediskey.LanqiaoSyncDisableKey(userID)).Result()
	if err != nil {
		return false, err
	}
	return exists > 0, nil
}

func (s *OJService) recordLanqiaoSyncFailure(ctx context.Context, userID uint, err error) {
	if global.Redis == nil || userID == 0 || err == nil {
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
	if err := global.Redis.Set(ctx, rediskey.LanqiaoSyncDisableKey(userID), "1", resolveLanqiaoDisableTTL()).Err(); err != nil {
		global.Log.Error("failed to disable lanqiao sync", zap.Uint("user_id", userID), zap.Error(err))
		return
	}
	_ = global.Redis.Del(ctx, failKey).Err()
	global.Log.Warn("lanqiao sync auto-disabled", zap.Uint("user_id", userID), zap.Error(err))
}

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
