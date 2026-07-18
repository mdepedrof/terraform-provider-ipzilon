package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Client struct {
	BaseURL    string
	Token      string
	HTTPClient *http.Client
}

func New(baseURL, token string) *Client {
	base := strings.TrimRight(baseURL, "/")
	return &Client{
		BaseURL:    resolveAPIBase(base),
		Token:      token,
		HTTPClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// resolveAPIBase probes /api/health then /health to find where the API is mounted.
// Handles production deployments (nginx strips /api/ prefix) and local dev (API at root).
// Falls back to /api if neither probe responds within 5 s.
func resolveAPIBase(base string) string {
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
		resp.Body.Close()
		if resp.StatusCode == 200 {
			return candidate
		}
	}
	return base + "/api"
}

type APIError struct {
	Code    int
	Message string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("API error %d: %s", e.Code, e.Message)
}

func IsNotFound(err error) bool {
	if err == nil {
		return false
	}
	apiErr, ok := err.(*APIError)
	return ok && apiErr.Code == 404
}

func (c *Client) do(method, path string, body, out any) error {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal request: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequest(method, c.BaseURL+path, bodyReader)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		var apiErr struct {
			Detail string `json:"detail"`
		}
		_ = json.Unmarshal(respBody, &apiErr)
		msg := apiErr.Detail
		if msg == "" {
			msg = string(respBody)
		}
		return &APIError{Code: resp.StatusCode, Message: msg}
	}

	if out != nil && len(respBody) > 0 {
		if err := json.Unmarshal(respBody, out); err != nil {
			return fmt.Errorf("unmarshal response: %w", err)
		}
	}
	return nil
}

func (c *Client) Get(path string, out any) error {
	return c.do(http.MethodGet, path, nil, out)
}

func (c *Client) Post(path string, body, out any) error {
	return c.do(http.MethodPost, path, body, out)
}

func (c *Client) Patch(path string, body, out any) error {
	return c.do(http.MethodPatch, path, body, out)
}

func (c *Client) Delete(path string) error {
	return c.do(http.MethodDelete, path, nil, nil)
}
