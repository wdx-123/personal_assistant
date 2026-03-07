package casbin

import (
	"path/filepath"
	"testing"

	casbinlib "github.com/casbin/casbin/v2"
	"github.com/casbin/casbin/v2/model"
)

func TestModelSupportsThreePartPermissionChecks(t *testing.T) {
	modelPath := filepath.Join("..", "..", "configs", "model.conf")
	m, err := model.NewModelFromFile(modelPath)
	if err != nil {
		t.Fatalf("load model: %v", err)
	}

	enforcer, err := casbinlib.NewEnforcer(m)
	if err != nil {
		t.Fatalf("new enforcer: %v", err)
	}

	if _, err := enforcer.AddRoleForUser("85@1", "org_admin"); err != nil {
		t.Fatalf("add role for user: %v", err)
	}
	if _, err := enforcer.AddPermissionForUser("org_admin", "/system/menu/my:GET", "access"); err != nil {
		t.Fatalf("add api permission: %v", err)
	}
	if _, err := enforcer.AddPermissionForUser("org_admin", "menu_dashboard", "read"); err != nil {
		t.Fatalf("add menu permission: %v", err)
	}

	ok, err := enforcer.Enforce("85@1", "/system/menu/my:GET", "access")
	if err != nil {
		t.Fatalf("enforce api permission: %v", err)
	}
	if !ok {
		t.Fatal("expected api permission to be granted")
	}

	ok, err = enforcer.Enforce("85@1", "menu_dashboard", "read")
	if err != nil {
		t.Fatalf("enforce menu permission: %v", err)
	}
	if !ok {
		t.Fatal("expected menu permission to be granted")
	}

	ok, err = enforcer.Enforce("85@1", "menu_dashboard", "access")
	if err != nil {
		t.Fatalf("enforce mismatched action: %v", err)
	}
	if ok {
		t.Fatal("expected mismatched action to be denied")
	}
}
