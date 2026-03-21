package system

import (
	"context"
	"fmt"
	"testing"

	"personal_assistant/internal/model/consts"
	"personal_assistant/internal/model/dto/request"
	"personal_assistant/pkg/util"

	"github.com/mojocn/base64Captcha"
)

func TestUserServicePhoneLoginPopulatesIsSuperAdmin(t *testing.T) {
	ctx := context.Background()

	cases := []struct {
		name            string
		label           string
		grantSuperAdmin bool
		want            bool
	}{
		{name: "super admin", label: "3201", grantSuperAdmin: true, want: true},
		{name: "regular user", label: "3202", grantSuperAdmin: false, want: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			env := newAuthorizationTestEnv(t)
			user := createUser(t, env, tc.label)
			rawPassword := "Pass1234"
			user.Password = util.BcryptHash(rawPassword)
			if err := env.db.Save(user).Error; err != nil {
				t.Fatalf("save user password: %v", err)
			}
			if tc.grantSuperAdmin {
				grantGlobalRole(t, env, user.ID, consts.RoleCodeSuperAdmin)
			}

			captchaID := fmt.Sprintf("%s-login", t.Name())
			captchaAnswer := "123456"
			mustSetCaptcha(t, captchaID, captchaAnswer)

			loggedIn, err := env.userService.PhoneLogin(ctx, &request.LoginReq{
				Phone:     user.Phone,
				Password:  rawPassword,
				Captcha:   captchaAnswer,
				CaptchaID: captchaID,
			})
			if err != nil {
				t.Fatalf("PhoneLogin() error = %v", err)
			}
			if loggedIn.IsSuperAdmin != tc.want {
				t.Fatalf("PhoneLogin() is_super_admin = %v, want %v", loggedIn.IsSuperAdmin, tc.want)
			}
		})
	}
}

func TestUserServiceUpdateProfilePreservesIsSuperAdmin(t *testing.T) {
	ctx := context.Background()
	env := newAuthorizationTestEnv(t)

	user := createUser(t, env, "3203")
	grantGlobalRole(t, env, user.ID, consts.RoleCodeSuperAdmin)
	signature := "新的签名"

	updated, err := env.userService.UpdateProfile(ctx, user.ID, &request.UpdateProfileReq{
		Signature: &signature,
	})
	if err != nil {
		t.Fatalf("UpdateProfile() error = %v", err)
	}
	if !updated.IsSuperAdmin {
		t.Fatal("UpdateProfile() is_super_admin = false, want true")
	}
	if updated.Signature != signature {
		t.Fatalf("UpdateProfile() signature = %q, want %q", updated.Signature, signature)
	}
}

func TestUserServiceChangePhonePreservesIsSuperAdmin(t *testing.T) {
	ctx := context.Background()
	env := newAuthorizationTestEnv(t)

	user := createUser(t, env, "3204")
	rawPassword := "Pass5678"
	user.Password = util.BcryptHash(rawPassword)
	if err := env.db.Save(user).Error; err != nil {
		t.Fatalf("save user password: %v", err)
	}
	grantGlobalRole(t, env, user.ID, consts.RoleCodeSuperAdmin)

	captchaID := fmt.Sprintf("%s-change-phone", t.Name())
	captchaAnswer := "654321"
	mustSetCaptcha(t, captchaID, captchaAnswer)

	updated, err := env.userService.ChangePhone(ctx, user.ID, &request.ChangePhoneReq{
		Password:  rawPassword,
		NewPhone:  "13900003204",
		Captcha:   captchaAnswer,
		CaptchaID: captchaID,
	})
	if err != nil {
		t.Fatalf("ChangePhone() error = %v", err)
	}
	if !updated.IsSuperAdmin {
		t.Fatal("ChangePhone() is_super_admin = false, want true")
	}
	if updated.Phone != "13900003204" {
		t.Fatalf("ChangePhone() phone = %q, want %q", updated.Phone, "13900003204")
	}
}

func mustSetCaptcha(t *testing.T, captchaID, answer string) {
	t.Helper()
	if err := base64Captcha.DefaultMemStore.Set(captchaID, answer); err != nil {
		t.Fatalf("set captcha: %v", err)
	}
}
