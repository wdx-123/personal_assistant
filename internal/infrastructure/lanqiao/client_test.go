package lanqiao

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"personal_assistant/internal/model/config"
)

func TestClient_SolveStats_StatsOnly_OK(t *testing.T) {
	t.Parallel()

	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if r.URL.Path != "/v2/lanqiao/solve_stats" {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		var req map[string]any
		_ = json.NewDecoder(r.Body).Decode(&req)
		if req["sync_num"] != float64(-1) {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ok": true,
			"data": map[string]any{
				"stats": map[string]any{
					"total_passed": 5,
					"total_failed": 2,
				},
			},
		})
	}))
	defer s.Close()

	c, err := NewFromConfig(config.LanqiaoCrawler{BaseURL: s.URL, APIPrefix: "/v2"})
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	resp, err := c.SolveStats(ctx, "13800000000", "pwd", -1)
	if err != nil {
		t.Fatal(err)
	}
	if !resp.OK {
		t.Fatalf("expected OK=true")
	}
	if resp.Data.Stats == nil || resp.Data.Stats.TotalPassed != 5 {
		t.Fatalf("unexpected stats: %+v", resp.Data.Stats)
	}
	if len(resp.Data.Problems) != 0 {
		t.Fatalf("expected no problems")
	}
}

func TestClient_SolveStats_FullSync_OK(t *testing.T) {
	t.Parallel()

	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v2/lanqiao/solve_stats" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ok": true,
			"data": map[string]any{
				"stats": map[string]any{
					"total_passed": 8,
					"total_failed": 3,
				},
				"problems": []map[string]any{
					{
						"problem_name": "带分数",
						"problem_id":   208,
						"created_at":   "2025-02-09T10:24:00+08:00",
						"is_passed":    true,
					},
				},
			},
		})
	}))
	defer s.Close()

	c, err := NewFromConfig(config.LanqiaoCrawler{BaseURL: s.URL, APIPrefix: "/v2"})
	if err != nil {
		t.Fatal(err)
	}

	resp, err := c.SolveStats(context.Background(), "13800000000", "pwd", 0)
	if err != nil {
		t.Fatal(err)
	}
	if resp.Data.Stats == nil || resp.Data.Stats.TotalFailed != 3 {
		t.Fatalf("unexpected stats: %+v", resp.Data.Stats)
	}
	if len(resp.Data.Problems) != 1 {
		t.Fatalf("expected 1 problem")
	}
}

func TestClient_SolveStats_Incremental_OK(t *testing.T) {
	t.Parallel()

	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v2/lanqiao/solve_stats" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ok": true,
			"data": map[string]any{
				"problems": []map[string]any{
					{
						"problem_name": "A+B",
						"problem_id":   1001,
						"created_at":   "2025-02-10T09:00:00+08:00",
						"is_passed":    true,
					},
				},
			},
		})
	}))
	defer s.Close()

	c, err := NewFromConfig(config.LanqiaoCrawler{BaseURL: s.URL, APIPrefix: "/v2"})
	if err != nil {
		t.Fatal(err)
	}

	resp, err := c.SolveStats(context.Background(), "13800000000", "pwd", 10)
	if err != nil {
		t.Fatal(err)
	}
	if resp.Data.Stats != nil {
		t.Fatalf("expected stats nil for incremental mode")
	}
	if len(resp.Data.Problems) != 1 {
		t.Fatalf("expected 1 problem")
	}
}

func TestClient_RemoteHTTPError(t *testing.T) {
	t.Parallel()

	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ok":    false,
			"error": "upstream timeout",
		})
	}))
	defer s.Close()

	c, err := NewFromConfig(config.LanqiaoCrawler{BaseURL: s.URL, APIPrefix: "/v2"})
	if err != nil {
		t.Fatal(err)
	}

	_, err = c.SolveStats(context.Background(), "13800000000", "pwd", 0)
	if err == nil {
		t.Fatal("expected error")
	}
	var re *RemoteHTTPError
	if !errors.As(err, &re) {
		t.Fatalf("expected RemoteHTTPError, got %T", err)
	}
	if re.StatusCode != http.StatusBadGateway {
		t.Fatalf("unexpected status: %d", re.StatusCode)
	}
	if re.Message != "upstream timeout" {
		t.Fatalf("unexpected message: %s", re.Message)
	}
}
