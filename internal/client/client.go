package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client communicates with the jaimito HTTP API.
type Client struct {
	serverURL string
	apiKey    string
	http      *http.Client
}

// New creates a Client targeting the given server address with the provided API key.
// The address should be host:port (e.g., "127.0.0.1:8080") — http:// is prepended automatically.
func New(address, apiKey string) *Client {
	return &Client{
		serverURL: "http://" + address,
		apiKey:    apiKey,
		http:      &http.Client{Timeout: 10 * time.Second},
	}
}

// NotifyRequest is the JSON body sent to POST /api/v1/notify.
// Mirrors api.NotifyRequest.
type NotifyRequest struct {
	Title    *string  `json:"title,omitempty"`
	Body     string   `json:"body"`
	Channel  string   `json:"channel,omitempty"`
	Priority string   `json:"priority,omitempty"`
	Tags     []string `json:"tags,omitempty"`
}

type notifyResponse struct {
	ID string `json:"id"`
}

type errorResponse struct {
	Error string `json:"error"`
}

// MetricRow represents a metric as returned by GET /api/v1/metrics.
type MetricRow struct {
	Name       string      `json:"name"`
	Category   string      `json:"category"`
	Type       string      `json:"type"`
	LastValue  *float64    `json:"last_value"`
	LastStatus string      `json:"last_status"`
	UpdatedAt  string      `json:"updated_at"`
	Thresholds *Thresholds `json:"thresholds,omitempty"`
}

// Thresholds holds warning/critical threshold values.
type Thresholds struct {
	Warning  *float64 `json:"warning,omitempty"`
	Critical *float64 `json:"critical,omitempty"`
}

// PostMetricRequest is the body for POST /api/v1/metrics.
type PostMetricRequest struct {
	Name  string  `json:"name"`
	Value float64 `json:"value"`
}

// PostMetricResponse is returned by POST /api/v1/metrics on success.
type PostMetricResponse struct {
	Name       string  `json:"name"`
	Value      float64 `json:"value"`
	RecordedAt string  `json:"recorded_at"`
}

// GetMetrics calls GET /api/v1/metrics. No authentication required.
func (c *Client) GetMetrics(ctx context.Context) ([]MetricRow, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		c.serverURL+"/api/v1/metrics", nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	// No Authorization header — public endpoint.

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var errResp errorResponse
		if json.Unmarshal(respBody, &errResp) == nil && errResp.Error != "" {
			return nil, fmt.Errorf("server error (%d): %s", resp.StatusCode, errResp.Error)
		}
		return nil, fmt.Errorf("server error (%d): %s", resp.StatusCode, string(respBody))
	}

	var result []MetricRow
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}
	return result, nil
}

// PostMetric sends a manual reading via POST /api/v1/metrics. Requires auth.
func (c *Client) PostMetric(ctx context.Context, req PostMetricRequest) (*PostMetricResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.serverURL+"/api/v1/metrics", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusCreated {
		var errResp errorResponse
		if json.Unmarshal(respBody, &errResp) == nil && errResp.Error != "" {
			return nil, fmt.Errorf("server error (%d): %s", resp.StatusCode, errResp.Error)
		}
		return nil, fmt.Errorf("server error (%d): %s", resp.StatusCode, string(respBody))
	}

	var result PostMetricResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}
	return &result, nil
}

// Notify sends a notification and returns the message ID on success.
func (c *Client) Notify(ctx context.Context, req NotifyRequest) (string, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.serverURL+"/api/v1/notify", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusAccepted {
		var errResp errorResponse
		if json.Unmarshal(respBody, &errResp) == nil && errResp.Error != "" {
			return "", fmt.Errorf("server error (%d): %s", resp.StatusCode, errResp.Error)
		}
		return "", fmt.Errorf("server error (%d): %s", resp.StatusCode, string(respBody))
	}

	var result notifyResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("parse response: %w", err)
	}
	return result.ID, nil
}
