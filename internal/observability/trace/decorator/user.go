package decorator

import (
	"context"

	"personal_assistant/internal/model/dto/request"
	resp "personal_assistant/internal/model/dto/response"
	"personal_assistant/internal/model/entity"
	"personal_assistant/internal/service/contract"
	"personal_assistant/pkg/observability/servicetrace"

	"github.com/gin-gonic/gin"
)

type tracedUserService struct {
	next contract.UserServiceContract
}

func WrapUserService(next contract.UserServiceContract) contract.UserServiceContract {
	if next == nil {
		return nil
	}
	return &tracedUserService{next: next}
}

func (t *tracedUserService) Register(ctx *gin.Context, req *request.RegisterReq) (*entity.User, error) {
	if ctx == nil || ctx.Request == nil {
		return t.next.Register(ctx, req)
	}
	opt := newTraceOptions("user", "Register")
	spanCtx, spanEvent := servicetrace.Start(ctx.Request.Context(), opt)
	ctx.Request = ctx.Request.WithContext(spanCtx)
	out, err := t.next.Register(ctx, req)
	servicetrace.Finish(spanCtx, opt, spanEvent, err)
	return out, err
}

func (t *tracedUserService) PhoneLogin(ctx context.Context, req *request.LoginReq) (*entity.User, error) {
	return runTraced(ctx, "user", "PhoneLogin", func(inner context.Context) (*entity.User, error) {
		return t.next.PhoneLogin(inner, req)
	})
}

func (t *tracedUserService) UpdateProfile(
	ctx context.Context,
	userID uint,
	req *request.UpdateProfileReq,
) (*entity.User, error) {
	return runTraced(ctx, "user", "UpdateProfile", func(inner context.Context) (*entity.User, error) {
		return t.next.UpdateProfile(inner, userID, req)
	})
}

func (t *tracedUserService) ChangePhone(
	ctx context.Context,
	userID uint,
	req *request.ChangePhoneReq,
) (*entity.User, error) {
	return runTraced(ctx, "user", "ChangePhone", func(inner context.Context) (*entity.User, error) {
		return t.next.ChangePhone(inner, userID, req)
	})
}

func (t *tracedUserService) ChangePassword(
	ctx context.Context,
	userID uint,
	req *request.ChangePasswordReq,
) error {
	return runTracedErr(ctx, "user", "ChangePassword", func(inner context.Context) error {
		return t.next.ChangePassword(inner, userID, req)
	})
}

func (t *tracedUserService) GetUserList(
	ctx context.Context,
	req *request.UserListReq,
) (*resp.PageDataUser, error) {
	return runTraced(ctx, "user", "GetUserList", func(inner context.Context) (*resp.PageDataUser, error) {
		return t.next.GetUserList(inner, req)
	})
}

func (t *tracedUserService) GetUserDetail(ctx context.Context, id uint) (*entity.User, error) {
	return runTraced(ctx, "user", "GetUserDetail", func(inner context.Context) (*entity.User, error) {
		return t.next.GetUserDetail(inner, id)
	})
}

func (t *tracedUserService) GetUserRoles(ctx context.Context, userID, orgID uint) ([]*entity.Role, error) {
	return runTraced(ctx, "user", "GetUserRoles", func(inner context.Context) ([]*entity.Role, error) {
		return t.next.GetUserRoles(inner, userID, orgID)
	})
}

func (t *tracedUserService) AssignRole(ctx context.Context, req *request.AssignUserRoleReq) error {
	return runTracedErr(ctx, "user", "AssignRole", func(inner context.Context) error {
		return t.next.AssignRole(inner, req)
	})
}

var _ contract.UserServiceContract = (*tracedUserService)(nil)
