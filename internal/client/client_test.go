package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-log/tflogtest"
)

// fakeSleeper records the waits requested by the retry loop without sleeping.
type fakeSleeper struct {
	mu    sync.Mutex
	waits []time.Duration
}

func (f *fakeSleeper) sleep(ctx context.Context, d time.Duration) error {
	f.mu.Lock()
	f.waits = append(f.waits, d)
	f.mu.Unlock()
	return ctx.Err()
}

// newTestClient builds a Client pointed at srv, bypassing New's /health probe,
// with a fake sleeper so retry tests run instantly.
func newTestClient(srv *httptest.Server) (*Client, *fakeSleeper) {
	fs := &fakeSleeper{}
	return &Client{
		BaseURL:    srv.URL,
		Token:      "test-token",
		HTTPClient: srv.Client(),
		sleep:      fs.sleep,
	}, fs
}

// sequenceServer answers each request with the next response in the list and
// repeats the last one when the list is exhausted.
type response struct {
	code       int
	body       string
	retryAfter string
}

func sequenceServer(t *testing.T, responses []response) (*httptest.Server, *int, *[]string) {
	t.Helper()
	var mu sync.Mutex
	calls := 0
	var bodies []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		b, _ := io.ReadAll(r.Body)
		bodies = append(bodies, string(b))
		resp := responses[min(calls, len(responses)-1)]
		calls++
		if resp.retryAfter != "" {
			w.Header().Set("Retry-After", resp.retryAfter)
		}
		w.WriteHeader(resp.code)
		_, _ = io.WriteString(w, resp.body)
	}))
	t.Cleanup(srv.Close)
	return srv, &calls, &bodies
}

// --- T002: retries ---------------------------------------------------------

func TestDo_RetryAfterHonoured(t *testing.T) {
	srv, calls, _ := sequenceServer(t, []response{
		{code: 429, body: `{"error":"Rate limit exceeded: 60 per 1 minute"}`, retryAfter: "2"},
		{code: 200, body: `{"id":1}`},
	})
	c, fs := newTestClient(srv)

	var out map[string]int
	if err := c.Get(context.Background(), "/x", &out); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if *calls != 2 {
		t.Errorf("calls = %d, want 2", *calls)
	}
	if len(fs.waits) != 1 || fs.waits[0] != 2*time.Second {
		t.Errorf("waits = %v, want [2s]", fs.waits)
	}
	if out["id"] != 1 {
		t.Errorf("out = %v", out)
	}
}

func TestDo_ExponentialBackoffWithoutRetryAfter(t *testing.T) {
	srv, calls, _ := sequenceServer(t, []response{
		{code: 503, body: `{"detail":"Server busy, retry later"}`},
		{code: 503, body: `{"detail":"Server busy, retry later"}`},
		{code: 200, body: `{}`},
	})
	c, fs := newTestClient(srv)

	if err := c.Get(context.Background(), "/x", nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if *calls != 3 {
		t.Errorf("calls = %d, want 3", *calls)
	}
	want := []time.Duration{time.Second, 2 * time.Second}
	if len(fs.waits) != len(want) {
		t.Fatalf("waits = %v, want ~%v", fs.waits, want)
	}
	for i, w := range want {
		lo, hi := time.Duration(float64(w)*0.8), time.Duration(float64(w)*1.2)
		if fs.waits[i] < lo || fs.waits[i] > hi {
			t.Errorf("wait[%d] = %v, want within [%v, %v]", i, fs.waits[i], lo, hi)
		}
	}
}

func TestDo_GivesUpAfterMaxElapsed(t *testing.T) {
	srv, calls, _ := sequenceServer(t, []response{
		{code: 429, body: `{"error":"Rate limit exceeded"}`, retryAfter: "60"},
	})
	c, _ := newTestClient(srv)

	err := c.Get(context.Background(), "/x", nil)
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.Code != 429 {
		t.Fatalf("err = %v, want APIError 429", err)
	}
	if *calls != 11 {
		t.Errorf("calls = %d, want 11 (10 retries of 60s = 10m)", *calls)
	}
	if !strings.Contains(err.Error(), "(gave up after 10 retries in 10m0s)") {
		t.Errorf("err = %q, want gave-up suffix", err)
	}
}

// TestDo_ShortRetryAfterUsesWholeBudget checks that a sustained Retry-After: 1
// (a saturated sliding window) is retried for the whole 10 minutes: the retry
// count must not end the request first.
func TestDo_ShortRetryAfterUsesWholeBudget(t *testing.T) {
	srv, calls, _ := sequenceServer(t, []response{
		{code: 429, body: `{"error":"Rate limit exceeded"}`, retryAfter: "1"},
	})
	c, _ := newTestClient(srv)

	err := c.Get(context.Background(), "/x", nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if *calls != 601 {
		t.Errorf("calls = %d, want 601 (600 retries of 1s = 10m)", *calls)
	}
	if !strings.Contains(err.Error(), "(gave up after 600 retries in 10m0s)") {
		t.Errorf("err = %q, want gave-up suffix", err)
	}
}

func TestDo_GivesUpAfterMaxRetries(t *testing.T) {
	srv, calls, _ := sequenceServer(t, []response{
		{code: 429, body: `{"error":"Rate limit exceeded"}`, retryAfter: "1"},
	})
	c, _ := newTestClient(srv)
	c.retryMax = 3

	err := c.Get(context.Background(), "/x", nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if *calls != 4 {
		t.Errorf("calls = %d, want 4 (3 retries)", *calls)
	}
	if !strings.Contains(err.Error(), "(gave up after 3 retries in 3s)") {
		t.Errorf("err = %q, want gave-up suffix", err)
	}
}

func TestDo_RetryAfterCapped(t *testing.T) {
	srv, _, _ := sequenceServer(t, []response{
		{code: 429, body: `{}`, retryAfter: "3600"},
		{code: 200, body: `{}`},
	})
	c, fs := newTestClient(srv)

	if err := c.Get(context.Background(), "/x", nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fs.waits) != 1 || fs.waits[0] != 60*time.Second {
		t.Errorf("waits = %v, want [1m0s]", fs.waits)
	}
}

func TestDo_RetryAfterHTTPDate(t *testing.T) {
	future := time.Now().Add(5 * time.Second).UTC().Format(http.TimeFormat)
	srv, _, _ := sequenceServer(t, []response{
		{code: 503, body: `{}`, retryAfter: future},
		{code: 200, body: `{}`},
	})
	c, fs := newTestClient(srv)

	if err := c.Get(context.Background(), "/x", nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fs.waits) != 1 || fs.waits[0] <= 0 || fs.waits[0] > 6*time.Second {
		t.Errorf("waits = %v, want ~5s", fs.waits)
	}
}

func TestDo_NoRetryOnOtherErrors(t *testing.T) {
	for _, code := range []int{400, 404, 409, 422, 500} {
		t.Run(strconv.Itoa(code), func(t *testing.T) {
			srv, calls, _ := sequenceServer(t, []response{{code: code, body: `{"detail":"nope"}`}})
			c, fs := newTestClient(srv)

			err := c.Get(context.Background(), "/x", nil)
			var apiErr *APIError
			if !errors.As(err, &apiErr) || apiErr.Code != code || apiErr.Message != "nope" {
				t.Fatalf("err = %v, want APIError %d nope", err, code)
			}
			if *calls != 1 || len(fs.waits) != 0 {
				t.Errorf("calls = %d waits = %v, want 1 call and no waits", *calls, fs.waits)
			}
		})
	}
}

func TestDo_RetriedPostResendsBody(t *testing.T) {
	srv, _, bodies := sequenceServer(t, []response{
		{code: 429, body: `{}`, retryAfter: "1"},
		{code: 201, body: `{}`},
	})
	c, _ := newTestClient(srv)

	if err := c.Post(context.Background(), "/x", map[string]string{"address": "10.0.0.1"}, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(*bodies) != 2 || (*bodies)[0] != (*bodies)[1] || (*bodies)[1] != `{"address":"10.0.0.1"}` {
		t.Errorf("bodies = %q, want the same JSON twice", *bodies)
	}
}

func TestDo_ContextCancelledDuringWait(t *testing.T) {
	srv, _, _ := sequenceServer(t, []response{{code: 503, body: `{}`, retryAfter: "1"}})
	c, _ := newTestClient(srv)
	ctx, cancel := context.WithCancel(context.Background())
	c.sleep = func(context.Context, time.Duration) error {
		cancel()
		return context.Canceled
	}

	err := c.Get(ctx, "/x", nil)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want context.Canceled", err)
	}
}

func TestDo_ErrorFieldUsedWhenNoDetail(t *testing.T) {
	srv, _, _ := sequenceServer(t, []response{{code: 400, body: `{"error":"Rate limit exceeded: x"}`}})
	c, _ := newTestClient(srv)

	err := c.Get(context.Background(), "/x", nil)
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.Message != "Rate limit exceeded: x" {
		t.Errorf("err = %v, want message from error field", err)
	}
}

// --- 005: warning for IPzilon's invalid-token block ------------------------

// warnLogs returns the warn-level entries written to buf by a tflogtest logger.
func warnLogs(t *testing.T, buf *bytes.Buffer) []map[string]any {
	t.Helper()
	entries, err := tflogtest.MultilineJSONDecode(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("decode logs: %v", err)
	}
	var warns []map[string]any
	for _, e := range entries {
		if e["@level"] == "warn" {
			warns = append(warns, e)
		}
	}
	return warns
}

func TestDo_InvalidTokenBlockWarnsOnce(t *testing.T) {
	blocked := response{code: 429, body: `{"error":"Too many invalid API tokens"}`, retryAfter: "42"}
	srv, calls, _ := sequenceServer(t, []response{blocked, blocked, {code: 200, body: `{"id":1}`}})
	c, fs := newTestClient(srv)
	var buf bytes.Buffer
	ctx := tflogtest.RootLogger(context.Background(), &buf)

	var out map[string]int
	if err := c.Get(ctx, "/x", &out); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if *calls != 3 {
		t.Errorf("calls = %d, want 3", *calls)
	}
	if len(fs.waits) != 2 || fs.waits[0] != 42*time.Second || fs.waits[1] != 42*time.Second {
		t.Errorf("waits = %v, want [42s 42s]", fs.waits)
	}

	warns := warnLogs(t, &buf)
	if len(warns) != 1 {
		t.Fatalf("warnings = %d, want 1: %v", len(warns), warns)
	}
	w := warns[0]
	msg, _ := w["@message"].(string)
	for _, want := range []string{"non-existent API tokens", "not the rate-limit quota of the configured token"} {
		if !strings.Contains(msg, want) {
			t.Errorf("message %q does not contain %q", msg, want)
		}
	}
	if w["method"] != "GET" || w["path"] != "/x" || w["status"] != float64(429) || w["wait"] != "42s" {
		t.Errorf("fields = method:%v path:%v status:%v wait:%v", w["method"], w["path"], w["status"], w["wait"])
	}
	if logs := buf.String(); strings.Contains(logs, "test-token") || strings.Contains(logs, "Authorization") {
		t.Errorf("logs leak credentials: %s", logs)
	}
}

func TestDo_OtherRetriesDoNotWarnInvalidToken(t *testing.T) {
	ok := response{code: 200, body: `{"id":1}`}
	for name, first := range map[string]response{
		"rate limit": {code: 429, body: `{"error":"Rate limit exceeded: 60 per 1 minute"}`, retryAfter: "2"},
		"busy":       {code: 503, retryAfter: "1"},
	} {
		t.Run(name, func(t *testing.T) {
			srv, _, _ := sequenceServer(t, []response{first, ok})
			c, _ := newTestClient(srv)
			var buf bytes.Buffer
			ctx := tflogtest.RootLogger(context.Background(), &buf)

			if err := c.Get(ctx, "/x", nil); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if warns := warnLogs(t, &buf); len(warns) != 0 {
				t.Errorf("warnings = %v, want none", warns)
			}
		})
	}

	t.Run("gives up without retrying", func(t *testing.T) {
		srv, _, _ := sequenceServer(t, []response{
			{code: 429, body: `{"error":"Too many invalid API tokens"}`, retryAfter: "60"},
		})
		c, _ := newTestClient(srv)
		c.retryMaxElapsed = 30 * time.Second
		var buf bytes.Buffer
		ctx := tflogtest.RootLogger(context.Background(), &buf)

		err := c.Get(ctx, "/x", nil)
		var apiErr *APIError
		if !errors.As(err, &apiErr) || apiErr.Code != 429 ||
			!strings.HasPrefix(apiErr.Message, "Too many invalid API tokens (gave up after 0 retries") {
			t.Errorf("err = %v, want 429 giving up after 0 retries", err)
		}
		if warns := warnLogs(t, &buf); len(warns) != 0 {
			t.Errorf("warnings = %v, want none", warns)
		}
	})
}

// --- T003: pagination -------------------------------------------------------

type item struct {
	N int `json:"n"`
}

// pagedServer serves `total` items as {items, total}, honouring limit/offset,
// and records every query string it receives.
func pagedServer(t *testing.T, total int) (*httptest.Server, *[]string) {
	t.Helper()
	var queries []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		queries = append(queries, r.URL.RawQuery)
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
		items := []item{}
		for i := offset; i < min(offset+limit, total); i++ {
			items = append(items, item{N: i})
		}
		_ = json.NewEncoder(w).Encode(Page[item]{Items: items, Total: total})
	}))
	t.Cleanup(srv.Close)
	return srv, &queries
}

func TestGetAll_MultiplePages(t *testing.T) {
	srv, queries := pagedServer(t, 1500)
	c, _ := newTestClient(srv)

	got, err := GetAll[item](context.Background(), c, "/cidrs/")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1500 {
		t.Fatalf("len = %d, want 1500", len(got))
	}
	for i, it := range got {
		if it.N != i {
			t.Fatalf("got[%d] = %d, order not preserved", i, it.N)
		}
	}
	want := []string{"limit=1000&offset=0", "limit=1000&offset=1000"}
	if fmt.Sprint(*queries) != fmt.Sprint(want) {
		t.Errorf("queries = %v, want %v", *queries, want)
	}
}

func TestGetAll_KeepsExistingQuery(t *testing.T) {
	srv, queries := pagedServer(t, 3)
	c, _ := newTestClient(srv)

	if _, err := GetAll[item](context.Background(), c, "/hubs/1/networks?name=x"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(*queries) != 1 || (*queries)[0] != "name=x&limit=1000&offset=0" {
		t.Errorf("queries = %v", *queries)
	}
}

func TestGetAll_Empty(t *testing.T) {
	srv, _ := pagedServer(t, 0)
	c, _ := newTestClient(srv)

	got, err := GetAll[item](context.Background(), c, "/x")
	if err != nil || len(got) != 0 {
		t.Errorf("got = %v, err = %v; want empty, nil", got, err)
	}
}

func TestGetAll_EmptyPageStops(t *testing.T) {
	srv, _, _ := sequenceServer(t, []response{
		{code: 200, body: `{"items":[{"n":0}],"total":5}`},
		{code: 200, body: `{"items":[],"total":5}`},
	})
	c, _ := newTestClient(srv)

	got, err := GetAll[item](context.Background(), c, "/x")
	if err != nil || len(got) != 1 {
		t.Errorf("got = %v, err = %v; want 1 item, nil", got, err)
	}
}

func TestGetAll_RunawayPaginationFails(t *testing.T) {
	// Server ignores limit and returns one item per page while claiming 5000:
	// the walk must stop after ceil(5000/1000)+1 pages instead of 5000.
	srv, calls, _ := sequenceServer(t, []response{
		{code: 200, body: `{"items":[{"n":0}],"total":5000}`},
	})
	c, _ := newTestClient(srv)
	t.Cleanup(func() {
		if *calls > 7 {
			t.Errorf("calls = %d, want at most 7", *calls)
		}
	})

	if _, err := GetAll[item](context.Background(), c, "/x"); err == nil {
		t.Error("expected error for runaway pagination")
	}
}

func TestGetAll_BareListIsUnsupportedVersion(t *testing.T) {
	srv, _, _ := sequenceServer(t, []response{{code: 200, body: ` [{"n":0}]`}})
	c, _ := newTestClient(srv)

	_, err := GetAll[item](context.Background(), c, "/x")
	if !errors.Is(err, ErrUnsupportedAPIVersion) || !strings.Contains(err.Error(), ">= 3.0.0") {
		t.Errorf("err = %v, want ErrUnsupportedAPIVersion mentioning >= 3.0.0", err)
	}
}

// --- T004: version detection and error helpers ------------------------------

func TestResolveAPIBase_ReadsVersion(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/health" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = io.WriteString(w, `{"status":"ok","version":"2.3.1"}`)
	}))
	defer srv.Close()

	base, version := resolveAPIBase(srv.URL)
	if base != srv.URL+"/api" || version != "2.3.1" {
		t.Errorf("base, version = %q, %q", base, version)
	}
}

func TestCheckAPIVersion(t *testing.T) {
	cases := map[string]bool{ // version -> want error
		"2.3.1":     true,
		"1.0.0":     true,
		"3.0.0":     false,
		"3.1.2":     false,
		"0.0.0-dev": false,
		"3.0.0-rc1": false,
		"":          false,
		"garbage":   false,
	}
	for v, wantErr := range cases {
		err := (&Client{APIVersion: v}).CheckAPIVersion()
		if (err != nil) != wantErr {
			t.Errorf("CheckAPIVersion(%q) = %v, wantErr %v", v, err, wantErr)
		}
		if err != nil && (!errors.Is(err, ErrUnsupportedAPIVersion) || !strings.Contains(err.Error(), "~> 2.2")) {
			t.Errorf("CheckAPIVersion(%q) = %v, want ErrUnsupportedAPIVersion mentioning ~> 2.2", v, err)
		}
	}
}

func TestErrorHelpers(t *testing.T) {
	truncated := &APIError{Code: 409, Message: "Search truncated after 4096 steps without finding a free /28 block in 10.0.0.0/12: the space is too fragmented; request a specific CIDR instead"}
	noSpace := &APIError{Code: 409, Message: "No free /24 block available in 10.0.0.0/16"}
	notFound := &APIError{Code: 404, Message: "IP address not found"}

	if !IsConflict(truncated) || !IsConflict(noSpace) || IsConflict(notFound) || IsConflict(nil) {
		t.Error("IsConflict mismatch")
	}
	if !IsSearchTruncated(truncated) || IsSearchTruncated(noSpace) || IsSearchTruncated(notFound) {
		t.Error("IsSearchTruncated mismatch")
	}
	if !IsNotFound(fmt.Errorf("wrapped: %w", notFound)) || IsNotFound(noSpace) {
		t.Error("IsNotFound mismatch")
	}
}

func TestRequireAPIVersion(t *testing.T) {
	cases := map[string]bool{ // version -> want error
		"3.0.1":     true,
		"3.0.0":     true,
		"2.3.1":     true,
		"3.1.0":     false,
		"3.1.5":     false,
		"3.2.0":     false,
		"4.0.0":     false,
		"":          false,
		"0.0.0-dev": false,
	}
	for v, wantErr := range cases {
		err := (&Client{APIVersion: v}).RequireAPIVersion(MinZonesAPIVersion, "network zones")
		if (err != nil) != wantErr {
			t.Errorf("RequireAPIVersion(%q) = %v, wantErr %v", v, err, wantErr)
		}
		want := "network zones requires IPzilon >= 3.1.0 (server reports " + v + ")"
		if err != nil && (!errors.Is(err, ErrUnsupportedAPIVersion) || !strings.Contains(err.Error(), want)) {
			t.Errorf("RequireAPIVersion(%q) = %v, want ErrUnsupportedAPIVersion containing %q", v, err, want)
		}
	}
}

func TestIsMethodNotAllowed(t *testing.T) {
	if !IsMethodNotAllowed(&APIError{Code: 405, Message: "Method Not Allowed"}) {
		t.Error("405 not detected")
	}
	if !IsMethodNotAllowed(fmt.Errorf("wrapped: %w", &APIError{Code: 405})) {
		t.Error("wrapped 405 not detected")
	}
	for _, err := range []error{&APIError{Code: 404}, &APIError{Code: 409}, errors.New("boom"), nil} {
		if IsMethodNotAllowed(err) {
			t.Errorf("IsMethodNotAllowed(%v) = true", err)
		}
	}
}

func TestRequireAPIVersionGlobalLists(t *testing.T) {
	cases := map[string]bool{ // version -> want error
		"3.1.0":     true,
		"3.1.9":     true,
		"3.2.0":     false,
		"3.3.1":     false,
		"":          false,
		"0.0.0-dev": false,
	}
	for v, wantErr := range cases {
		err := (&Client{APIVersion: v}).RequireAPIVersion(MinGlobalListsAPIVersion, "x")
		if (err != nil) != wantErr {
			t.Errorf("RequireAPIVersion(%q) = %v, wantErr %v", v, err, wantErr)
		}
		if err != nil && (!errors.Is(err, ErrUnsupportedAPIVersion) || !strings.Contains(err.Error(), "requires IPzilon >= 3.2.0")) {
			t.Errorf("RequireAPIVersion(%q) = %v", v, err)
		}
	}
}
