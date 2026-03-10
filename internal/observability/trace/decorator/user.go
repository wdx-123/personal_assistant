package decorator

import (
	"context"

	"personal_assistant/internal/model/dto/request"
	resp "personal_assistant/internal/model/dto/response"
	"personal_assistant/internal/model/entity"
	"personal_assistant/internal/service/contract"
)

// tracedUserService 是 UserServiceContract 的装饰器实现，添加了分布式追踪功能
type tracedUserService struct {
	next contract.UserServiceContract
}

// WrapUserService 包装 UserServiceContract，返回一个带有追踪功能的装饰器实例
func WrapUserService(next contract.UserServiceContract) contract.UserServiceContract {
	if next == nil {
		return nil
	}
	return &tracedUserService{next: next}
}

func (t *tracedUserService) Register(ctx context.Context, req *request.RegisterReq) (*entity.User, error) {
	return runTraced(ctx, "user", "Register", func(inner context.Context) (*entity.User, error) {
		return t.next.Register(inner, req)
	})
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

// DeactivateAccount 注销账号
func (t *tracedUserService) DeactivateAccount(
	ctx context.Context,
	userID uint,
	req *request.DeactivateAccountReq,
) error {
	return runTracedErr(ctx, "user", "DeactivateAccount", func(inner context.Context) error {
		return t.next.DeactivateAccount(inner, userID, req)
	})
}

func (t *tracedUserService) UpdateUserStatus(
	ctx context.Context,
	operatorID, targetUserID uint,
	req *request.AdminUpdateUserStatusReq,
) error {
	return runTracedErr(ctx, "user", "UpdateUserStatus", func(inner context.Context) error {
		return t.next.UpdateUserStatus(inner, operatorID, targetUserID, req)
	})
}

func (t *tracedUserService) CleanupDisabledUsers(ctx context.Context) (int, error) {
	return runTraced(ctx, "user", "CleanupDisabledUsers", func(inner context.Context) (int, error) {
		return t.next.CleanupDisabledUsers(inner)
	})
}

var _ contract.UserServiceContract = (*tracedUserService)(nil)
