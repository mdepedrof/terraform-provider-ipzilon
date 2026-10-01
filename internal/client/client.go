package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// Retry defaults for 429 (rate limited) and 503 (server busy) responses.
// Retries stop when the accumulated wait of a request would exceed
// defaultRetryMaxElapsed; defaultRetryMax is only a safety net sized so that
// it never ends a request before that budget (IPzilon may answer Retry-After: 1).
const (
	defaultRetryMax        = 1000
	defaultRetryMaxElapsed = 10 * time.Minute
	defaultRetryBaseWait   = time.Second
	defaultRetryMaxWait    = 60 * time.Second

	// pageSize is the largest page IPzilon accepts; it minimises the number
	// of requests (and rate-limit quota) spent walking a listing.
	pageSize = 1000
)

type Client struct {
	BaseURL    string
	Token      string
	HTTPClient *http.Client
	// APIVersion is the IPzilon version reported by /health ("" if unknown).
	APIVersion string

	// Retry tuning; zero values fall back to the defaults above. Unexported
	// so only tests can override them.
	retryMax        int
	retryMaxElapsed time.Duration
	retryBaseWait   time.Duration
	retryMaxWait    time.Duration
	sleep           func(context.Context, time.Duration) error
}

func New(baseURL, token string) *Client {
	base := strings.TrimRight(baseURL, "/")
	apiBase, version := resolveAPIBase(base)
	return &Client{
		BaseURL:    apiBase,
		Token:      token,
		HTTPClient: &http.Client{Timeout: 30 * time.Second},
		APIVersion: version,
	}
}

// resolveAPIBase probes /api/health then /health to find where the API is mounted.
// Handles production deployments (nginx strips /api/ prefix) and local dev (API at root).
// Falls back to /api if neither probe responds within 5 s. It also returns the
// version IPzilon reports in the health payload ("" if absent).
func resolveAPIBase(base string) (string, string) {
	probe := &http.Client{Timeout: 5 * time.Second}
	for _, candidate := range []string{base + "/api", base} {
		req, err := http.NewRequest("GET", candidate+"/health", nil)
		if err != nil {
			continue
		}
		resp, err := probe.Do(req)
		if err != nil {
			continue
		}
		var health struct {
			Version string `json:"version"`
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode == 200 {
			_ = json.Unmarshal(body, &health)
			return candidate, health.Version
		}
	}
	return base + "/api", ""
}

// ErrUnsupportedAPIVersion is returned when the server is an IPzilon release
// older than the one this provider version supports.
var ErrUnsupportedAPIVersion = errors.New("unsupported IPzilon version")

var semverRe = regexp.MustCompile(`^(\d+)\.(\d+)\.(\d+)$`)

// CheckAPIVersion fails when the server reports a release version older than
// 3.0.0. Unknown or non-release versions (e.g. "0.0.0-dev") are not blocked.
func (c *Client) CheckAPIVersion() error {
	m := semverRe.FindStringSubmatch(c.APIVersion)
	if m == nil {
		return nil
	}
	major, _ := strconv.Atoi(m[1])
	if major >= 1 && major < 3 {
		return fmt.Errorf("%w: IPzilon %s is not supported by this provider version: requires IPzilon >= 3.0.0. Pin the provider to ~> 2.2 to keep using IPzilon 2.x", ErrUnsupportedAPIVersion, c.APIVersion)
	}
	return nil
}

// MinZonesAPIVersion is the first IPzilon release with network zones. Only
// configurations that use zones require it; everything else keeps working
// against IPzilon 3.0.x.
const MinZonesAPIVersion = "3.1.0"

// RequireAPIVersion fails when the server reports a release version older
// than min, naming the feature that needs it. Like CheckAPIVersion it does
// not block unknown or non-release versions, and it makes no request: the
// version was read from /health when the client was created.
func (c *Client) RequireAPIVersion(min, feature string) error {
	have := semverRe.FindStringSubmatch(c.APIVersion)
	want := semverRe.FindStringSubmatch(min)
	if have == nil || want == nil {
		return nil
	}
	for i := 1; i <= 3; i++ {
		h, _ := strconv.Atoi(have[i])
		w, _ := strconv.Atoi(want[i])
		if h != w {
			if h < w {
				return fmt.Errorf("%w: %s requires IPzilon >= %s (server reports %s)", ErrUnsupportedAPIVersion, feature, min, c.APIVersion)
			}
			return nil
		}
	}
	return nil
}

type APIError struct {
	Code    int
	Message string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("API error %d: %s", e.Code, e.Message)
}

func apiErrorCode(err error) int {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr.Code
	}
	return 0
}

func IsNotFound(err error) bool {
	return apiErrorCode(err) == http.StatusNotFound
}

func IsConflict(err error) bool {
	return apiErrorCode(err) == http.StatusConflict
}

// IsSearchTruncated reports whether IPzilon gave up looking for a free block
// because the address space is too fragmented (409 "Search truncated …"), as
// opposed to a 409 meaning there is no free space at all.
func IsSearchTruncated(err error) bool {
	var apiErr *APIError
	return errors.As(err, &apiErr) && apiErr.Code == http.StatusConflict &&
		strings.HasPrefix(apiErr.Message, "Search truncated")
}

func (c *Client) do(ctx context.Context, method, path string, body, out any) error {
	var data []byte
	if body != nil {
		var err error
		if data, err = json.Marshal(body); err != nil {
			return fmt.Errorf("marshal request: %w", err)
		}
	}

	var elapsed time.Duration
	for retries := 0; ; retries++ {
		tflog.Debug(ctx, "IPzilon request", map[string]any{"method": method, "path": path})
		resp, respBody, err := c.send(ctx, method, path, data)
		if err != nil {
			return err
		}

		if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == http.StatusServiceUnavailable {
			wait := c.retryWait(resp, retries)
			if retries >= c.retryMaxOrDefault() || elapsed+wait > c.retryMaxElapsedOrDefault() {
				apiErr := parseAPIError(resp.StatusCode, respBody)
				apiErr.Message += fmt.Sprintf(" (gave up after %d retries in %s)", retries, elapsed)
				return apiErr
			}
			tflog.Debug(ctx, "retrying IPzilon request", map[string]any{
				"method": method, "path": path, "status": resp.StatusCode,
				"attempt": retries + 1, "wait": wait.String(),
			})
			if err := c.sleepOrDefault()(ctx, wait); err != nil {
				return err
			}
			elapsed += wait
			continue
		}

		if resp.StatusCode >= 400 {
			return parseAPIError(resp.StatusCode, respBody)
		}
		if out != nil && len(respBody) > 0 {
			if err := json.Unmarshal(respBody, out); err != nil {
				return fmt.Errorf("unmarshal response: %w", err)
			}
		}
		return nil
	}
}

// send performs a single HTTP attempt and returns the fully read body.
func (c *Client) send(ctx context.Context, method, path string, data []byte) (*http.Response, []byte, error) {
	var bodyReader io.Reader
	if data != nil {
		bodyReader = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.BaseURL+path, bodyReader)
	if err != nil {
		return nil, nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)
	if data != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("read response: %w", err)
	}
	return resp, respBody, nil
}

// parseAPIError extracts the error message: FastAPI's "detail", the rate
// limiter's "error", or the raw body.
func parseAPIError(code int, body []byte) *APIError {
	var payload struct {
		Detail string `json:"detail"`
		Error  string `json:"error"`
	}
	_ = json.Unmarshal(body, &payload)
	msg := payload.Detail
	if msg == "" {
		msg = payload.Error
	}
	if msg == "" {
		msg = string(body)
	}
	return &APIError{Code: code, Message: msg}
}

// retryWait honours Retry-After (seconds or HTTP date) and otherwise backs off
// exponentially with ±20 % jitter; the result is capped at retryMaxWait.
func (c *Client) retryWait(resp *http.Response, retries int) time.Duration {
	maxWait := c.retryMaxWait
	if maxWait == 0 {
		maxWait = defaultRetryMaxWait
	}

	wait := time.Duration(-1)
	if ra := resp.Header.Get("Retry-After"); ra != "" {
		if secs, err := strconv.Atoi(ra); err == nil && secs >= 0 {
			wait = time.Duration(secs) * time.Second
		} else if t, err := http.ParseTime(ra); err == nil {
			wait = max(time.Until(t), 0)
		}
	}
	if wait < 0 {
		base := c.retryBaseWait
		if base == 0 {
			base = defaultRetryBaseWait
		}
		backoff := float64(base) * float64(uint64(1)<<min(retries, 16))
		wait = time.Duration(backoff * (0.8 + 0.4*rand.Float64()))
	}
	return min(wait, maxWait)
}

func (c *Client) retryMaxOrDefault() int {
	if c.retryMax == 0 {
		return defaultRetryMax
	}
	return c.retryMax
}

func (c *Client) retryMaxElapsedOrDefault() time.Duration {
	if c.retryMaxElapsed == 0 {
		return defaultRetryMaxElapsed
	}
	return c.retryMaxElapsed
}

func (c *Client) sleepOrDefault() func(context.Context, time.Duration) error {
	if c.sleep != nil {
		return c.sleep
	}
	return func(ctx context.Context, d time.Duration) error {
		t := time.NewTimer(d)
		defer t.Stop()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-t.C:
			return nil
		}
	}
}

func (c *Client) Get(ctx context.Context, path string, out any) error {
	return c.do(ctx, http.MethodGet, path, nil, out)
}

func (c *Client) Post(ctx context.Context, path string, body, out any) error {
	return c.do(ctx, http.MethodPost, path, body, out)
}

func (c *Client) Patch(ctx context.Context, path string, body, out any) error {
	return c.do(ctx, http.MethodPatch, path, body, out)
}

func (c *Client) Delete(ctx context.Context, path string) error {
	return c.do(ctx, http.MethodDelete, path, nil, nil)
}

// GetAll walks a paginated IPzilon listing ({items, total}) and returns every
// item in server order. path may already carry a query string (filters).
func GetAll[T any](ctx context.Context, c *Client, path string) ([]T, error) {
	sep := "?"
	if strings.Contains(path, "?") {
		sep = "&"
	}

	all := []T{}
	maxPages := -1
	for offset, pages := 0, 0; ; pages++ {
		if maxPages >= 0 && pages >= maxPages {
			return nil, fmt.Errorf("pagination of %s did not converge after %d pages", path, pages)
		}

		var raw json.RawMessage
		if err := c.Get(ctx, fmt.Sprintf("%s%slimit=%d&offset=%d", path, sep, pageSize, offset), &raw); err != nil {
			return nil, err
		}
		if trimmed := bytes.TrimSpace(raw); len(trimmed) > 0 && trimmed[0] == '[' {
			return nil, fmt.Errorf("%w: %s returned a plain list; this provider version requires IPzilon >= 3.0.0. Pin the provider to ~> 2.2 to keep using IPzilon 2.x", ErrUnsupportedAPIVersion, path)
		}

		var page Page[T]
		if err := json.Unmarshal(raw, &page); err != nil {
			return nil, fmt.Errorf("unmarshal page: %w", err)
		}
		if maxPages < 0 {
			maxPages = (page.Total+pageSize-1)/pageSize + 1
		}

		all = append(all, page.Items...)
		offset += len(page.Items)
		if len(page.Items) == 0 || offset >= page.Total {
			return all, nil
		}
	}
}
