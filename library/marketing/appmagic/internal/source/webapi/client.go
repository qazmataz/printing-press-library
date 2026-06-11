// Copyright 2026 Hamza Qazi and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored client for the UNOFFICIAL appmagic.rocks web XHR surface
// (https://appmagic.rocks/api/v2). This is a separate surface from the
// official api.appmagic.rocks/v1 API: it authenticates with the Bearer token
// a logged-in browser session stores in localStorage under 'datamagic.token'
// (env var APPMAGIC_WEB_TOKEN) and can change without notice.

package webapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/appmagic/internal/cliutil"
)

const (
	defaultBaseURL = "https://appmagic.rocks/api/v2"
	tokenEnvVar    = "APPMAGIC_WEB_TOKEN" // #nosec G101 -- env var name read at runtime, not a credential value
	maxRetries429  = 3
)

// Client talks to the unofficial web XHR surface. All methods are read-only.
type Client struct {
	baseURL string
	token   string
	http    *http.Client
	limiter *cliutil.AdaptiveLimiter
}

// New builds a web-surface client from APPMAGIC_WEB_TOKEN. The timeout bounds
// each request; callers pass the root --timeout value.
func New(timeout time.Duration) (*Client, error) {
	token := strings.TrimSpace(os.Getenv(tokenEnvVar))
	if token == "" {
		return nil, fmt.Errorf("%s is not set: the 'web' commands use the unofficial appmagic.rocks web surface, which needs the Bearer token from a logged-in browser session (localStorage key 'datamagic.token')", tokenEnvVar)
	}
	base := defaultBaseURL
	if v := strings.TrimSpace(os.Getenv("APPMAGIC_WEB_BASE_URL")); v != "" {
		base = strings.TrimRight(v, "/")
	}
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	return &Client{
		baseURL: base,
		token:   token,
		http:    &http.Client{Timeout: timeout},
		limiter: cliutil.NewAdaptiveLimiter(2),
	}, nil
}

// APIError carries the web surface's HTTP failure details.
type APIError struct {
	StatusCode int
	Method     string
	Path       string
	Body       string
}

func (e *APIError) Error() string {
	hint := ""
	if e.StatusCode == 401 {
		hint = " (APPMAGIC_WEB_TOKEN is invalid or expired; re-copy it from a logged-in appmagic.rocks session, localStorage key 'datamagic.token')"
	}
	return fmt.Sprintf("web surface HTTP %d on %s %s%s: %s", e.StatusCode, e.Method, e.Path, hint, truncateBody(e.Body))
}

func truncateBody(b string) string {
	b = strings.TrimSpace(b)
	if len(b) > 300 {
		return b[:300] + "..."
	}
	return b
}

// Get issues a GET with query params against the web surface.
func (c *Client) Get(ctx context.Context, path string, params map[string]string) (json.RawMessage, error) {
	u := c.baseURL + path
	if len(params) > 0 {
		q := url.Values{}
		for k, v := range params {
			if v != "" {
				q.Set(k, v)
			}
		}
		u += "?" + q.Encode()
	}
	return c.do(ctx, http.MethodGet, u, path, nil)
}

// Post issues a JSON POST against the web surface.
func (c *Client) Post(ctx context.Context, path string, body any) (json.RawMessage, error) {
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("encoding request body: %w", err)
	}
	return c.do(ctx, http.MethodPost, c.baseURL+path, path, payload)
}

func (c *Client) do(ctx context.Context, method, fullURL, path string, body []byte) (json.RawMessage, error) {
	var lastErr error
	for attempt := 0; attempt <= maxRetries429; attempt++ {
		c.limiter.Wait()
		var reader io.Reader
		if body != nil {
			reader = bytes.NewReader(body)
		}
		req, err := http.NewRequestWithContext(ctx, method, fullURL, reader)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+c.token)
		req.Header.Set("Accept", "application/json")
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		resp, err := c.http.Do(req)
		if err != nil {
			return nil, fmt.Errorf("web surface request failed: %w", err)
		}
		data, readErr := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
		closeErr := resp.Body.Close()
		if readErr != nil {
			return nil, fmt.Errorf("reading web surface response: %w", readErr)
		}
		if closeErr != nil {
			return nil, fmt.Errorf("closing web surface response body: %w", closeErr)
		}
		if resp.StatusCode == http.StatusTooManyRequests {
			c.limiter.OnRateLimit()
			retryAfter, _ := time.ParseDuration(strings.TrimSpace(resp.Header.Get("Retry-After")) + "s")
			lastErr = &cliutil.RateLimitError{URL: fullURL, RetryAfter: retryAfter, Body: string(data)}
			// Honor Retry-After (capped at 10s) before the next attempt,
			// bailing out promptly if the context is cancelled meanwhile.
			if retryAfter > 0 {
				wait := retryAfter
				if wait > 10*time.Second {
					wait = 10 * time.Second
				}
				select {
				case <-ctx.Done():
					return nil, ctx.Err()
				case <-time.After(wait):
				}
			}
			continue
		}
		if resp.StatusCode >= 400 {
			return nil, &APIError{StatusCode: resp.StatusCode, Method: method, Path: path, Body: string(data)}
		}
		// The SPA returns 200 + HTML for routes that are not real API
		// endpoints; surface that as a typed error instead of a JSON
		// parse failure downstream.
		trimmed := bytes.TrimLeft(data, " \t\r\n")
		if len(trimmed) > 0 && trimmed[0] == '<' {
			return nil, &APIError{StatusCode: resp.StatusCode, Method: method, Path: path,
				Body: "response is HTML, not JSON: this web-surface route changed or requires a different method"}
		}
		c.limiter.OnSuccess()
		return json.RawMessage(data), nil
	}
	return nil, lastErr
}
