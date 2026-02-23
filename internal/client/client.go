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
