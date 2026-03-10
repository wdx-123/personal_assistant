package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"personal_assistant/global"
	"personal_assistant/internal/model/consts"
	"personal_assistant/internal/model/dto/request"
	"personal_assistant/internal/model/entity"
	"personal_assistant/internal/repository"
	"personal_assistant/internal/repository/interfaces"
	repSystem "personal_assistant/internal/repository/system"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func TestActiveUserMWAllowsCachedActiveUser(t *testing.T) {
	repo := &stubActiveUserRepository{cachedFound: true, cachedActive: true}
	status := performActiveUserRequest(t, repo)

	if status != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", status, http.StatusNoContent)
	}
	if repo.getByIDCalls != 0 {
		t.Fatalf("GetByID calls = %d, want 0", repo.getByIDCalls)
	}
}

func TestActiveUserMWRejectsCachedInactiveUser(t *testing.T) {
	repo := &stubActiveUserRepository{cachedFound: true, cachedActive: false}
	status := performActiveUserRequest(t, repo)

	if status != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", status, http.StatusUnauthorized)
	}
	if repo.getByIDCalls != 0 {
		t.Fatalf("GetByID calls = %d, want 0", repo.getByIDCalls)
	}
}

func TestActiveUserMWFallsBackToDBOnCacheMiss(t *testing.T) {
	repo := &stubActiveUserRepository{
		user: &entity.User{Status: consts.UserStatusActive},
	}
	status := performActiveUserRequest(t, repo)

	if status != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", status, http.StatusNoContent)
	}
	if repo.getByIDCalls != 1 {
		t.Fatalf("GetByID calls = %d, want 1", repo.getByIDCalls)
	}
	if len(repo.cacheWrites) != 1 || !repo.cacheWrites[0] {
		t.Fatalf("cacheWrites = %v, want [true]", repo.cacheWrites)
	}
}

func TestActiveUserMWFallsBackToDBOnCacheError(t *testing.T) {
	repo := &stubActiveUserRepository{
		cacheErr: context.DeadlineExceeded,
		user:     &entity.User{Status: consts.UserStatusActive},
	}
	status := performActiveUserRequest(t, repo)

	if status != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", status, http.StatusNoContent)
	}
	if repo.getByIDCalls != 1 {
		t.Fatalf("GetByID calls = %d, want 1", repo.getByIDCalls)
	}
}

func TestActiveUserMWRejectsInactiveUserAfterDBFallback(t *testing.T) {
	repo := &stubActiveUserRepository{
		user: &entity.User{Status: consts.UserStatusDisabled, Freeze: true},
	}
	status := performActiveUserRequest(t, repo)

	if status != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", status, http.StatusUnauthorized)
	}
	if len(repo.cacheWrites) != 1 || repo.cacheWrites[0] {
		t.Fatalf("cacheWrites = %v, want [false]", repo.cacheWrites)
	}
}

func performActiveUserRequest(t *testing.T, userRepo interfaces.UserRepository) int {
	t.Helper()

	gin.SetMode(gin.TestMode)

	oldGroup := repository.GroupApp
	oldLog := global.Log
	global.Log = zap.NewNop()
	repository.GroupApp = &repository.Group{
		SystemRepositorySupplier: &stubActiveUserSupplier{userRepo: userRepo},
	}
	t.Cleanup(func() {
		repository.GroupApp = oldGroup
		global.Log = oldLog
	})

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("claims", &request.JwtCustomClaims{
			BaseClaims: request.BaseClaims{UserID: 1},
		})
		c.Next()
	})
	router.GET("/", ActiveUserMW(), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec.Code
}

type stubActiveUserSupplier struct {
	repSystem.Supplier
	userRepo interfaces.UserRepository
}

func (s *stubActiveUserSupplier) GetUserRepository() interfaces.UserRepository {
	return s.userRepo
}

type stubActiveUserRepository struct {
	interfaces.UserRepository
	cachedActive bool
	cachedFound  bool
	cacheErr     error
	user         *entity.User
	getByIDErr   error
	getByIDCalls int
	cacheWrites  []bool
}

func (r *stubActiveUserRepository) GetCachedActiveState(_ context.Context, _ uint) (bool, bool, error) {
	return r.cachedActive, r.cachedFound, r.cacheErr
}

func (r *stubActiveUserRepository) GetByID(_ context.Context, _ uint) (*entity.User, error) {
	r.getByIDCalls++
	return r.user, r.getByIDErr
}

func (r *stubActiveUserRepository) CacheActiveState(_ context.Context, _ uint, active bool) error {
	r.cacheWrites = append(r.cacheWrites, active)
	return nil
}
