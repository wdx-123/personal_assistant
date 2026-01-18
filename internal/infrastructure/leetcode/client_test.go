package leetcode

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestClient_PublicProfile_OK(t *testing.T) {
	t.Parallel()

	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if r.URL.Path != "/leetcode/public_profile" {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		var req map[string]any
		_ = json.NewDecoder(r.Body).Decode(&req)
		if req["username"] != "u1" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ok": true,
			"data": map[string]any{
				"profile": map[string]any{
					"userSlug":   "u1",
					"realName":   "n1",
					"userAvatar": "a1",
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

	resp, err := c.PublicProfile(ctx, "u1", 0.8)
	if err != nil {
		t.Fatal(err)
	}
	if !resp.OK {
		t.Fatalf("expected OK=true")
	}
	if resp.Data.Profile.UserSlug != "u1" {
		t.Fatalf("unexpected userSlug: %s", resp.Data.Profile.UserSlug)
	}
}

func TestClient_RecentAC_OK(t *testing.T) {
	t.Parallel()

	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if r.URL.Path != "/leetcode/recent_ac" {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ok": true,
			"data": map[string]any{
				"recent_accepted": []map[string]any{
					{
						"title":     "t1",
						"slug":      "s1",
						"timestamp": 1,
						"time":      "2025-04-24 23:00:44",
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

	resp, err := c.RecentAC(ctx, "u1", 2)
	if err != nil {
		t.Fatal(err)
	}
	if !resp.OK {
		t.Fatalf("expected OK=true")
	}
	if len(resp.Data.RecentAccepted) != 1 {
		t.Fatalf("expected 1 item, got %d", len(resp.Data.RecentAccepted))
	}
	if resp.Data.RecentAccepted[0].Slug != "s1" {
		t.Fatalf("unexpected slug: %s", resp.Data.RecentAccepted[0].Slug)
	}
}

func TestClient_RemoteHTTPError(t *testing.T) {
	t.Parallel()

	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte("bad gateway"))
	}))
	defer s.Close()

	c, err := NewClient(s.URL)
	if err != nil {
		t.Fatal(err)
	}

	_, err = c.PublicProfile(context.Background(), "u1", 0)
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
}
