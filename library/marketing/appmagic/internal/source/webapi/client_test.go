// Copyright 2026 Hamza Qazi and contributors. Licensed under Apache-2.0. See LICENSE.

package webapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/appmagic/internal/cliutil"
)

func newTestClient(t *testing.T, baseURL string) *Client {
	t.Helper()
	t.Setenv("APPMAGIC_WEB_TOKEN", "test-token")
	t.Setenv("APPMAGIC_WEB_BASE_URL", baseURL)
	c, err := New(5 * time.Second)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return c
}

func TestNewRequiresToken(t *testing.T) {
	t.Setenv("APPMAGIC_WEB_TOKEN", "")
	if _, err := New(time.Second); err == nil {
		t.Fatal("expected error when APPMAGIC_WEB_TOKEN is unset")
	} else if !strings.Contains(err.Error(), "APPMAGIC_WEB_TOKEN") {
		t.Fatalf("error should name the env var, got: %v", err)
	}
}

func TestGetSendsBearerAndParams(t *testing.T) {
	var gotAuth, gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotQuery = r.URL.RawQuery
		_ = json.NewEncoder(w).Encode(map[string]string{"ok": "yes"})
	}))
	defer srv.Close()
	c := newTestClient(t, srv.URL)
	data, err := c.Get(context.Background(), "/top/hourly-apps", map[string]string{"store": "2", "empty": ""})
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if gotAuth != "Bearer test-token" {
		t.Fatalf("Authorization = %q, want Bearer test-token", gotAuth)
	}
	if !strings.Contains(gotQuery, "store=2") || strings.Contains(gotQuery, "empty") {
		t.Fatalf("query = %q: want store=2 present and empty param stripped", gotQuery)
	}
	if !strings.Contains(string(data), "yes") {
		t.Fatalf("unexpected body: %s", data)
	}
}

func TestErrorShapes(t *testing.T) {
	cases := []struct {
		name       string
		status     int
		body       string
		wantInErr  string
		wantStatus int
	}{
		{"401 names token expiry", 401, `{"message":"unauthorized"}`, "APPMAGIC_WEB_TOKEN is invalid or expired", 401},
		{"403 plain api error", 403, `{"message":"forbidden"}`, "HTTP 403", 403},
		{"html shell detected", 200, "<!doctype html><html></html>", "HTML, not JSON", 200},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer srv.Close()
			c := newTestClient(t, srv.URL)
			_, err := c.Get(context.Background(), "/x", nil)
			if err == nil {
				t.Fatal("expected error")
			}
			var apiErr *APIError
			if !errors.As(err, &apiErr) {
				t.Fatalf("want *APIError, got %T: %v", err, err)
			}
			if apiErr.StatusCode != tc.wantStatus {
				t.Fatalf("status = %d, want %d", apiErr.StatusCode, tc.wantStatus)
			}
			if !strings.Contains(err.Error(), tc.wantInErr) {
				t.Fatalf("error %q should contain %q", err.Error(), tc.wantInErr)
			}
		})
	}
}

func TestRateLimitExhaustionReturnsTypedError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "1")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()
	c := newTestClient(t, srv.URL)
	_, err := c.Post(context.Background(), "/x", map[string]int{"store": 1})
	if err == nil {
		t.Fatal("expected rate-limit error after retries exhausted")
	}
	var rl *cliutil.RateLimitError
	if !errors.As(err, &rl) {
		t.Fatalf("want *cliutil.RateLimitError (never empty-on-throttle), got %T: %v", err, err)
	}
}

func TestTruncateBody(t *testing.T) {
	if got := truncateBody("  short  "); got != "short" {
		t.Fatalf("truncateBody trims, got %q", got)
	}
	long := strings.Repeat("a", 400)
	if got := truncateBody(long); len(got) != 303 || !strings.HasSuffix(got, "...") {
		t.Fatalf("long body should cap at 300+ellipsis, got len %d", len(got))
	}
}
