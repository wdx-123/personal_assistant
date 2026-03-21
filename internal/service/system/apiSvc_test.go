package system

import (
	"context"
	"testing"

	"personal_assistant/global"
	"personal_assistant/internal/model/consts"
	"personal_assistant/internal/model/entity"

	"github.com/gin-gonic/gin"
)

func TestApiServiceSyncAPIRestoresSoftDeletedAPI(t *testing.T) {
	env := newAuthorizationTestEnv(t)
	apiService := NewApiService(env.repoGroup, env.projection)

	router := gin.New()
	router.GET("/system/api/restored", func(_ *gin.Context) {})
	oldRouter := global.Router
	global.Router = router
	t.Cleanup(func() {
		global.Router = oldRouter
	})

	api := &entity.API{
		Path:      "/system/api/restored",
		Method:    "GET",
		Status:    1,
		SyncState: consts.APISyncStateRegistered,
	}
	if err := env.db.Create(api).Error; err != nil {
		t.Fatalf("create api: %v", err)
	}
	if err := env.db.Delete(api).Error; err != nil {
		t.Fatalf("soft delete api: %v", err)
	}

	added, restored, markedMissing, archived, total, err := apiService.SyncAPI(context.Background(), false)
	if err != nil {
		t.Fatalf("SyncAPI() error = %v", err)
	}
	if added != 0 || restored != 1 || markedMissing != 0 || archived != 0 {
		t.Fatalf(
			"unexpected sync counts: added=%d restored=%d missing=%d archived=%d",
			added,
			restored,
			markedMissing,
			archived,
		)
	}
	if total != 1 {
		t.Fatalf("total = %d, want 1", total)
	}

	var restoredAPI entity.API
	if err := env.db.Unscoped().First(&restoredAPI, api.ID).Error; err != nil {
		t.Fatalf("load restored api: %v", err)
	}
	if restoredAPI.DeletedAt.Valid {
		t.Fatalf("expected api %d to be restored from soft delete", api.ID)
	}
	if restoredAPI.SyncState != consts.APISyncStateRegistered {
		t.Fatalf("sync_state = %q, want %q", restoredAPI.SyncState, consts.APISyncStateRegistered)
	}
	if restoredAPI.LastSeenAt == nil {
		t.Fatalf("expected restored api to record last_seen_at")
	}
}

func TestPermissionProjectionRebuildExcludesMissingAPI(t *testing.T) {
	ctx := context.Background()
	env := newAuthorizationTestEnv(t)

	org := createOrg(t, env, 11)
	user := createUser(t, env, "00000020")
	currentOrgID := org.ID
	if err := env.db.Model(&entity.User{}).Where("id = ?", user.ID).Update("current_org_id", currentOrgID).Error; err != nil {
		t.Fatalf("set current org: %v", err)
	}

	role := createRole(t, env, "org_api_reader")
	api := &entity.API{
		Path:      "/system/api/projected",
		Method:    "GET",
		Status:    1,
		SyncState: consts.APISyncStateRegistered,
	}
	if err := env.db.Create(api).Error; err != nil {
		t.Fatalf("create api: %v", err)
	}
	if err := env.db.Create(&entity.RoleAPI{RoleID: role.ID, APIID: api.ID}).Error; err != nil {
		t.Fatalf("bind role api: %v", err)
	}
	assignUserRole(t, env, user.ID, org.ID, role.ID)

	if err := env.projection.RebuildAll(ctx); err != nil {
		t.Fatalf("initial rebuild: %v", err)
	}
	ok, err := env.authorization.CheckUserAPIPermission(ctx, user.ID, api.Path, api.Method)
	if err != nil {
		t.Fatalf("check permission before missing: %v", err)
	}
	if !ok {
		t.Fatalf("expected permission before api becomes missing")
	}

	if err := env.db.Model(&entity.API{}).Where("id = ?", api.ID).Update("sync_state", consts.APISyncStateMissing).Error; err != nil {
		t.Fatalf("mark api missing: %v", err)
	}
	if err := env.projection.RebuildAll(ctx); err != nil {
		t.Fatalf("rebuild after missing: %v", err)
	}

	ok, err = env.authorization.CheckUserAPIPermission(ctx, user.ID, api.Path, api.Method)
	if err != nil {
		t.Fatalf("check permission after missing: %v", err)
	}
	if ok {
		t.Fatalf("expected missing api to be excluded from projection")
	}
}

func TestRoleServiceGetRoleMenuAPIMapExcludesNonProjectableAPIs(t *testing.T) {
	ctx := context.Background()
	env := newAuthorizationTestEnv(t)
	roleService := NewRoleService(env.repoGroup, env.projection)

	role := createRole(t, env, "org_menu_manager")
	menu := &entity.Menu{
		Name:   "Role Menu",
		Code:   "role_menu",
		Status: 1,
	}
	if err := env.db.Create(menu).Error; err != nil {
		t.Fatalf("create menu: %v", err)
	}

	registeredAPI := &entity.API{
		Path:      "/system/api/registered",
		Method:    "GET",
		Status:    1,
		SyncState: consts.APISyncStateRegistered,
	}
	missingAPI := &entity.API{
		Path:      "/system/api/missing",
		Method:    "GET",
		Status:    1,
		SyncState: consts.APISyncStateMissing,
	}
	disabledAPI := &entity.API{
		Path:      "/system/api/disabled",
		Method:    "GET",
		Status:    0,
		SyncState: consts.APISyncStateRegistered,
	}
	for _, api := range []*entity.API{registeredAPI, missingAPI, disabledAPI} {
		if err := env.db.Create(api).Error; err != nil {
			t.Fatalf("create api %s: %v", api.Path, err)
		}
	}
	if err := env.db.Model(&entity.API{}).Where("id = ?", disabledAPI.ID).Update("status", 0).Error; err != nil {
		t.Fatalf("disable api: %v", err)
	}
	for _, apiID := range []uint{registeredAPI.ID, missingAPI.ID, disabledAPI.ID} {
		if err := env.db.Create(&entity.MenuAPI{MenuID: menu.ID, APIID: apiID}).Error; err != nil {
			t.Fatalf("bind menu api %d: %v", apiID, err)
		}
	}
	if err := env.db.Exec("INSERT INTO role_menus (role_id, menu_id) VALUES (?, ?)", role.ID, menu.ID).Error; err != nil {
		t.Fatalf("bind role menu: %v", err)
	}
	for _, apiID := range []uint{registeredAPI.ID, missingAPI.ID} {
		if err := env.db.Create(&entity.RoleAPI{RoleID: role.ID, APIID: apiID}).Error; err != nil {
			t.Fatalf("bind role api %d: %v", apiID, err)
		}
	}

	mapping, err := roleService.GetRoleMenuAPIMap(ctx, role.ID, nil)
	if err != nil {
		t.Fatalf("GetRoleMenuAPIMap() error = %v", err)
	}
	if len(mapping.MenuTree) != 1 {
		t.Fatalf("menu tree len = %d, want 1", len(mapping.MenuTree))
	}
	if len(mapping.MenuTree[0].APIs) != 1 {
		t.Fatalf("menu api len = %d, want 1", len(mapping.MenuTree[0].APIs))
	}
	if mapping.MenuTree[0].APIs[0].ID != registeredAPI.ID {
		t.Fatalf("menu api id = %d, want %d", mapping.MenuTree[0].APIs[0].ID, registeredAPI.ID)
	}
	if len(mapping.DirectAPIIDs) != 1 || mapping.DirectAPIIDs[0] != registeredAPI.ID {
		t.Fatalf("direct_api_ids = %v, want [%d]", mapping.DirectAPIIDs, registeredAPI.ID)
	}
	if len(mapping.AssignedAPIIDs) != 1 || mapping.AssignedAPIIDs[0] != registeredAPI.ID {
		t.Fatalf("assigned_api_ids = %v, want [%d]", mapping.AssignedAPIIDs, registeredAPI.ID)
	}
}
