package system

import (
	"context"
	"testing"
	"time"

	"personal_assistant/global"
	"personal_assistant/internal/model/config"
	"personal_assistant/pkg/rediskey"

	"github.com/alicebob/miniredis/v2"
	"github.com/go-redis/redis/v8"
)

func TestUserRepositoryActiveStateCacheRoundTrip(t *testing.T) {
	repo := newCacheOnlyUserRepository(t, 60, 0)
	ctx := context.Background()

	if err := repo.CacheActiveState(ctx, 101, true); err != nil {
		t.Fatalf("CacheActiveState(active) error = %v", err)
	}
	active, found, err := repo.GetCachedActiveState(ctx, 101)
	if err != nil {
		t.Fatalf("GetCachedActiveState(active) error = %v", err)
	}
	if !found || !active {
		t.Fatalf("GetCachedActiveState(active) = (%v, %v), want (true, true)", active, found)
	}

	if err := repo.CacheActiveState(ctx, 102, false); err != nil {
		t.Fatalf("CacheActiveState(inactive) error = %v", err)
	}
	active, found, err = repo.GetCachedActiveState(ctx, 102)
	if err != nil {
		t.Fatalf("GetCachedActiveState(inactive) error = %v", err)
	}
	if !found || active {
		t.Fatalf("GetCachedActiveState(inactive) = (%v, %v), want (false, true)", active, found)
	}
}

func TestUserRepositoryActiveStateCacheMiss(t *testing.T) {
	repo := newCacheOnlyUserRepository(t, 60, 0)
	active, found, err := repo.GetCachedActiveState(context.Background(), 999)
	if err != nil {
		t.Fatalf("GetCachedActiveState() error = %v", err)
	}
	if found || active {
		t.Fatalf("GetCachedActiveState() = (%v, %v), want (false, false)", active, found)
	}
}

func TestUserRepositoryInvalidActiveStateValueIsTreatedAsMiss(t *testing.T) {
	repo := newCacheOnlyUserRepository(t, 60, 0)
	ctx := context.Background()
	if err := global.Redis.Set(ctx, rediskey.UserActiveStateKey(103), "invalid", time.Minute).Err(); err != nil {
		t.Fatalf("seed invalid cache value error = %v", err)
	}

	active, found, err := repo.GetCachedActiveState(ctx, 103)
	if err != nil {
		t.Fatalf("GetCachedActiveState() error = %v", err)
	}
	if found || active {
		t.Fatalf("GetCachedActiveState() = (%v, %v), want (false, false)", active, found)
	}
}

func TestUserRepositoryActiveStateCacheTTLUsesJitterWindow(t *testing.T) {
	repo := newCacheOnlyUserRepository(t, 60, 10)
	ctx := context.Background()
	if err := repo.CacheActiveState(ctx, 104, true); err != nil {
		t.Fatalf("CacheActiveState() error = %v", err)
	}

	ttl, err := global.Redis.TTL(ctx, rediskey.UserActiveStateKey(104)).Result()
	if err != nil {
		t.Fatalf("TTL() error = %v", err)
	}
	if ttl < 59*time.Second || ttl > 70*time.Second {
		t.Fatalf("TTL() = %v, want in [59s, 70s]", ttl)
	}
}

func newCacheOnlyUserRepository(t *testing.T, ttlSeconds, jitterSeconds int) *UserGormRepository {
	t.Helper()

	srv, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis.Run() error = %v", err)
	}
	client := redis.NewClient(&redis.Options{Addr: srv.Addr()})

	oldRedis := global.Redis
	oldConfig := global.Config
	global.Redis = client
	global.Config = &config.Config{
		Redis: config.Redis{
			ActiveUserStateTTLSeconds:       ttlSeconds,
			ActiveUserStateTTLJitterSeconds: jitterSeconds,
		},
	}

	t.Cleanup(func() {
		global.Redis = oldRedis
		global.Config = oldConfig
		_ = client.Close()
		srv.Close()
	})

	return &UserGormRepository{}
}
