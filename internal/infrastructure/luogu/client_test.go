package luogu

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestClient_GetPractice_OK(t *testing.T) {
	t.Parallel()

	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if r.URL.Path != "/luogu/practice" {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		var req map[string]any
		_ = json.NewDecoder(r.Body).Decode(&req)
		// 注意：JSON数字默认解析为float64
		if uid, ok := req["uid"].(float64); !ok || int(uid) != 12345 {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ok": true,
			"data": map[string]any{
				"user": map[string]any{
					"uid":    12345,
					"name":   "user1",
					"avatar": "http://avatar.com/1.png",
				},
				"passed": []map[string]any{
					{
						"pid":        "P1001",
						"title":      "A+B Problem",
						"difficulty": 1,
						"type":       "P",
					},
				},
			},
		})
	}))
	defer s.Close()

	c, err := NewClient(s.URL)
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	resp, err := c.GetPractice(ctx, 12345, 0.5)
	if err != nil {
		t.Fatal(err)
	}
	if !resp.OK {
		t.Fatalf("expected OK=true")
	}
	if resp.Data.User.UID != 12345 {
		t.Fatalf("unexpected uid: %d", resp.Data.User.UID)
	}
	if len(resp.Data.Passed) != 1 {
		t.Fatalf("expected 1 passed problem")
	}
	if resp.Data.Passed[0].PID != "P1001" {
		t.Fatalf("unexpected pid: %s", resp.Data.Passed[0].PID)
	}
}

func TestClient_RemoteHTTPError(t *testing.T) {
	t.Parallel()

	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("internal server error"))
	}))
	defer s.Close()

	c, err := NewClient(s.URL)
	if err != nil {
		t.Fatal(err)
	}

	_, err = c.GetPractice(context.Background(), 12345, 0)
	if err == nil {
		t.Fatal("expected error")
	}
	var re *RemoteHTTPError
	if !As(err, &re) {
		t.Fatalf("expected RemoteHTTPError, got %T", err)
	}
	if re.StatusCode != http.StatusInternalServerError {
		t.Fatalf("unexpected status: %d", re.StatusCode)
	}
}

func TestClient_StructuredError(t *testing.T) {
	t.Parallel()

	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Header().Set("Content-Type", "application/json")
		// 模拟 Python 服务返回的 JSON 错误
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ok":    false,
			"error": "403 Client Error: Forbidden for url: https://www.luogu.com.cn/user/123/practice",
		})
	}))
	defer s.Close()

	c, err := NewClient(s.URL)
	if err != nil {
		t.Fatal(err)
	}

	_, err = c.GetPractice(context.Background(), 12345, 0)
	if err == nil {
		t.Fatal("expected error")
	}

	var re *RemoteHTTPError
	if !errors.As(err, &re) {
		t.Fatalf("expected RemoteHTTPError, got %T", err)
	}

	// 验证是否成功提取了 clean message
	expected := "403 Client Error: Forbidden for url: https://www.luogu.com.cn/user/123/practice"
	if re.Message != expected {
		t.Fatalf("expected message %q, got %q", expected, re.Message)
	}

	// 验证 Error() 输出格式
	expectedErrorStr := "luogu remote error: 403 Client Error: Forbidden for url: https://www.luogu.com.cn/user/123/practice (status=500)"
	if err.Error() != expectedErrorStr {
		t.Fatalf("expected error string %q, got %q", expectedErrorStr, err.Error())
	}
}

func As(err error, target any) bool {
	return errors.As(err, target)
}
