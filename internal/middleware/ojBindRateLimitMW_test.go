package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"personal_assistant/global"
	"personal_assistant/pkg/errors"
	"personal_assistant/pkg/ratelimit"
	"personal_assistant/pkg/response"

	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"go.uber.org/zap"
)

func TestOJBindRateLimitMiddlewareBlocksFourthLeetcodeRequest(t *testing.T) {
	_, router := setupOJBindRateLimitTest(t, http.StatusNoContent)

	for i := 0; i < 3; i++ {
		rec := performOJBindRequest(router, "/oj/bind", `{"platform":"leetcode","identifier":"demo"}`)
		if rec.Code != http.StatusNoContent {
			t.Fatalf("attempt %d status = %d, want %d", i+1, rec.Code, http.StatusNoContent)
		}
	}

	rec := performOJBindRequest(router, "/oj/bind", `{"platform":"leetcode","identifier":"demo"}`)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("blocked status = %d, want %d", rec.Code, http.StatusTooManyRequests)
	}
	if got := rec.Header().Get("Retry-After"); got != "10" {
		t.Fatalf("Retry-After = %q, want %q", got, "10")
	}
	if got := rec.Header().Get("X-RateLimit-Remaining"); got != "0" {
		t.Fatalf("X-RateLimit-Remaining = %q, want %q", got, "0")
	}

	var respBody response.BizResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &respBody); err != nil {
		t.Fatalf("decode response error = %v", err)
	}
	if respBody.Code != errors.CodeTooManyRequests.Int() {
		t.Fatalf("biz code = %d, want %d", respBody.Code, errors.CodeTooManyRequests.Int())
	}
	if respBody.Message != "当前绑定人数过多，请稍后再试。" {
		t.Fatalf("message = %q, want %q", respBody.Message, "当前绑定人数过多，请稍后再试。")
	}
}

func TestOJBindRateLimitMiddlewareSeparatesPlatforms(t *testing.T) {
	_, router := setupOJBindRateLimitTest(t, http.StatusNoContent)

	for i := 0; i < 4; i++ {
		rec := performOJBindRequest(router, "/oj/bind", `{"platform":"leetcode","identifier":"demo"}`)
		if i < 3 && rec.Code != http.StatusNoContent {
			t.Fatalf("leetcode attempt %d status = %d, want %d", i+1, rec.Code, http.StatusNoContent)
		}
		if i == 3 && rec.Code != http.StatusTooManyRequests {
			t.Fatalf("leetcode blocked status = %d, want %d", rec.Code, http.StatusTooManyRequests)
		}
	}

	luoguRec := performOJBindRequest(router, "/oj/bind", `{"platform":"luogu","identifier":"1001"}`)
	if luoguRec.Code != http.StatusNoContent {
		t.Fatalf("luogu status = %d, want %d", luoguRec.Code, http.StatusNoContent)
	}

	for i := 0; i < 4; i++ {
		rec := performOJBindRequest(router, "/oj/lanqiao/bind", `{"phone":"13800000000","password":"secret"}`)
		if i < 3 && rec.Code != http.StatusNoContent {
			t.Fatalf("lanqiao attempt %d status = %d, want %d", i+1, rec.Code, http.StatusNoContent)
		}
		if i == 3 && rec.Code != http.StatusTooManyRequests {
			t.Fatalf("lanqiao blocked status = %d, want %d", rec.Code, http.StatusTooManyRequests)
		}
	}
}

func TestOJBindRateLimitMiddlewarePassesThroughInvalidPayload(t *testing.T) {
	_, router := setupOJBindRateLimitTest(t, http.StatusTeapot)

	cases := []struct {
		name string
		path string
		body string
	}{
		{name: "invalid json", path: "/oj/bind", body: `{"platform":`},
		{name: "missing platform", path: "/oj/bind", body: `{"identifier":"demo"}`},
		{name: "unsupported platform", path: "/oj/bind", body: `{"platform":"codeforces","identifier":"demo"}`},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			rec := performOJBindRequest(router, tc.path, tc.body)
			if rec.Code != http.StatusTeapot {
				t.Fatalf("status = %d, want %d", rec.Code, http.StatusTeapot)
			}
		})
	}
}

func TestOJBindRateLimitMiddlewareAllowsAfterWindowExpires(t *testing.T) {
	srv, router := setupOJBindRateLimitTest(t, http.StatusNoContent)

	for i := 0; i < 4; i++ {
		rec := performOJBindRequest(router, "/oj/bind", `{"platform":"leetcode","identifier":"demo"}`)
		if i < 3 && rec.Code != http.StatusNoContent {
			t.Fatalf("attempt %d status = %d, want %d", i+1, rec.Code, http.StatusNoContent)
		}
		if i == 3 && rec.Code != http.StatusTooManyRequests {
			t.Fatalf("blocked status = %d, want %d", rec.Code, http.StatusTooManyRequests)
		}
	}

	srv.FastForward(10 * time.Second)

	rec := performOJBindRequest(router, "/oj/bind", `{"platform":"leetcode","identifier":"demo"}`)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status after expiry = %d, want %d", rec.Code, http.StatusNoContent)
	}
}

func TestOJBindRateLimitMiddlewareFailsOpenOnRedisError(t *testing.T) {
	_, router := setupOJBindRateLimitTest(t, http.StatusNoContent)
	if err := global.Redis.Close(); err != nil {
		t.Fatalf("close redis client error = %v", err)
	}

	rec := performOJBindRequest(router, "/oj/bind", `{"platform":"leetcode","identifier":"demo"}`)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNoContent)
	}
}

func setupOJBindRateLimitTest(t *testing.T, handlerStatus int) (*miniredis.Miniredis, *gin.Engine) {
	t.Helper()

	gin.SetMode(gin.TestMode)

	srv, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis.Run() error = %v", err)
	}
	client := redis.NewClient(&redis.Options{Addr: srv.Addr()})

	oldRedis := global.Redis
	oldLog := global.Log
	oldLimiters := global.OJBindLimiters
	global.Redis = client
	global.Log = zap.NewNop()
	global.OJBindLimiters = map[string]*ratelimit.SlidingWindowLimiter{
		"leetcode": ratelimit.NewSlidingWindowLimiter(client, "ratelimit:oj_bind:leetcode", 3, 10*time.Second),
		"luogu":    ratelimit.NewSlidingWindowLimiter(client, "ratelimit:oj_bind:luogu", 3, 10*time.Second),
		"lanqiao":  ratelimit.NewSlidingWindowLimiter(client, "ratelimit:oj_bind:lanqiao", 3, 10*time.Second),
	}

	t.Cleanup(func() {
		global.Redis = oldRedis
		global.Log = oldLog
		global.OJBindLimiters = oldLimiters
		if err := client.Close(); err != nil && !strings.Contains(err.Error(), "client is closed") {
			t.Errorf("client.Close() error = %v", err)
		}
		if srv != nil {
			srv.Close()
		}
	})

	router := gin.New()
	mw := OJBindRateLimitMiddleware(global.OJBindLimiters)
	router.POST("/oj/bind", mw, func(c *gin.Context) {
		c.Status(handlerStatus)
	})
	router.POST("/oj/lanqiao/bind", mw, func(c *gin.Context) {
		c.Status(handlerStatus)
	})

	return srv, router
}

func performOJBindRequest(router *gin.Engine, path string, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}
