package system

import (
	"context"
	"errors"
	"personal_assistant/internal/infrastructure"
	lc "personal_assistant/internal/infrastructure/leetcode"
	"personal_assistant/internal/model/dto/request"
	resp "personal_assistant/internal/model/dto/response"
	"personal_assistant/internal/model/entity"
	"personal_assistant/internal/repository"
	"personal_assistant/internal/repository/interfaces"
	"strconv"
	"strings"
)

var (
	ErrInvalidPlatform   = errors.New("invalid platform")
	ErrInvalidIdentifier = errors.New("invalid identifier")
)

type OJService struct {
	leetcodeRepo interfaces.LeetcodeUserDetailRepository
	luoguRepo    interfaces.LuoguUserDetailRepository
}

func NewOJService(repositoryGroup *repository.Group) *OJService {
	return &OJService{
		leetcodeRepo: repositoryGroup.SystemRepositorySupplier.GetLeetcodeUserDetailRepository(),
		luoguRepo:    repositoryGroup.SystemRepositorySupplier.GetLuoguUserDetailRepository(),
	}
}

func (s *OJService) BindOJAccount(
	ctx context.Context,
	userID uint,
	req *request.BindOJAccountReq,
) (*resp.BindOJAccountResp, error) {
	if req == nil {
		return nil, ErrInvalidIdentifier
	}

	platform := strings.ToLower(strings.TrimSpace(req.Platform))
	identifier := strings.TrimSpace(req.Identifier)
	if identifier == "" {
		return nil, ErrInvalidIdentifier
	}

	const sleepSec = 0.5

	switch platform {
	case "leetcode":
		return s.bindLeetCode(ctx, userID, identifier, sleepSec)
	case "luogu":
		return s.bindLuogu(ctx, userID, identifier, sleepSec)
	default:
		return nil, ErrInvalidPlatform
	}
}

func (s *OJService) bindLeetCode(
	ctx context.Context,
	userID uint,
	identifier string,
	sleepSec float64,
) (*resp.BindOJAccountResp, error) {
	out, err := infrastructure.LeetCode().PublicProfile(ctx, identifier, sleepSec)
	if err != nil {
		return nil, err
	}
	if out == nil || !out.OK {
		return nil, errors.New("leetcode request failed")
	}

	easy, medium, hard := extractLeetCodeCounts(out)
	total := easy + medium + hard

	detail := &entity.LeetcodeUserDetail{
		UserSlug:     strings.TrimSpace(out.Data.Profile.UserSlug),
		RealName:     strings.TrimSpace(out.Data.Profile.RealName),
		UserAvatar:   strings.TrimSpace(out.Data.Profile.UserAvatar),
		EasyNumber:   easy,
		MediumNumber: medium,
		HardNumber:   hard,
		TotalNumber:  total,
		UserID:       userID,
	}

	saved, err := s.leetcodeRepo.UpsertByUserID(ctx, detail)
	if err != nil {
		return nil, err
	}

	return &resp.BindOJAccountResp{
		Platform:     "leetcode",
		Identifier:   saved.UserSlug,
		RealName:     saved.RealName,
		UserAvatar:   saved.UserAvatar,
		EasyNumber:   saved.EasyNumber,
		MediumNumber: saved.MediumNumber,
		HardNumber:   saved.HardNumber,
		TotalNumber:  saved.TotalNumber,
	}, nil
}

func extractLeetCodeCounts(out *lc.PublicProfileResponse) (int, int, int) {
	if out == nil {
		return 0, 0, 0
	}

	list := out.Data.SubmitStatsGlobal.AcSubmissionNum
	if len(list) == 0 {
		list = out.Data.SubmitStats.AcSubmissionNum
	}

	easy, medium, hard := 0, 0, 0
	for _, item := range list {
		switch strings.ToLower(strings.TrimSpace(item.Difficulty)) {
		case "easy":
			easy = item.Count
		case "medium":
			medium = item.Count
		case "hard":
			hard = item.Count
		}
	}
	return easy, medium, hard
}

func (s *OJService) bindLuogu(
	ctx context.Context,
	userID uint,
	identifier string,
	sleepSec float64,
) (*resp.BindOJAccountResp, error) {
	uid, err := strconv.Atoi(identifier)
	if err != nil || uid <= 0 {
		return nil, ErrInvalidIdentifier
	}

	out, err := infrastructure.Luogu().GetPractice(ctx, uid, sleepSec)
	if err != nil {
		return nil, err
	}
	if out == nil || !out.OK {
		return nil, errors.New("luogu request failed")
	}

	passed := out.Data.PassedCount
	if passed <= 0 {
		passed = len(out.Data.Passed)
	}

	detail := &entity.LuoguUserDetail{
		Identification: identifier,
		RealName:       strings.TrimSpace(out.Data.User.Name),
		UserAvatar:     strings.TrimSpace(out.Data.User.Avatar),
		PassedNumber:   passed,
		UserID:         userID,
	}

	saved, err := s.luoguRepo.UpsertByUserID(ctx, detail)
	if err != nil {
		return nil, err
	}

	return &resp.BindOJAccountResp{
		Platform:     "luogu",
		Identifier:   saved.Identification,
		RealName:     saved.RealName,
		UserAvatar:   saved.UserAvatar,
		PassedNumber: saved.PassedNumber,
	}, nil
}
