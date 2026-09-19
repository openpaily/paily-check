package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

const apiBodyTimeout = 7 * time.Minute

// ---------------------------------------------------------------------------
// Domain types
// ---------------------------------------------------------------------------

// Node is a proxy node returned by GET /api/v1/check/nodes.
type Node struct {
	ID       string                 `json:"id"`
	Hash     string                 `json:"hash"`
	Server   string                 `json:"server"`
	Protocol string                 `json:"protocol"`
	Raw      map[string]interface{} `json:"raw"`
}

// InitialResult carries the outcome of the initial latency check.
// LatencyMS == -1 means every ping attempt timed out.
type InitialResult struct {
	LatencyMS int `json:"latency_ms"`
}

// DeepResult carries the outcome of the detailed checks.
// SpeedKbps is a pointer so it is omitted from JSON when the speed test is
// disabled (nil pointer → omitempty).
type DeepResult struct {
	AvgLatencyMS int             `json:"avg_latency_ms"`
	JitterMS     int             `json:"jitter_ms"`
	SpeedKbps    *int            `json:"speed_kbps,omitempty"`
	Region       string          `json:"region,omitempty"`
	Streaming    map[string]bool `json:"streaming,omitempty"`
	Metadata     map[string]any  `json:"metadata,omitempty"`
}

// CheckResult is one entry in the POST /api/v1/check/results payload.
// Deep is omitted when the node failed the initial latency check.
type CheckResult struct {
	NodeID  string        `json:"node_id"`
	Initial InitialResult `json:"initial"`
	Deep    *DeepResult   `json:"deep,omitempty"`
}

// ---------------------------------------------------------------------------
// API wire types
// ---------------------------------------------------------------------------

type checkNodesResponse struct {
	Nodes []Node `json:"nodes"`
}

type checkResultsRequest struct {
	Results []CheckResult `json:"results"`
}

// ---------------------------------------------------------------------------
// Client
// ---------------------------------------------------------------------------

// Client communicates with the core API over HTTP.
// All requests carry Authorization: Bearer <Secret>.
type Client struct {
	baseURL string
	secret  string
	http    *http.Client
}

// New creates a Client and validates the core API base URL.
func New(baseURL, secret string) (*Client, error) {
	parsed, err := url.ParseRequestURI(baseURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return nil, fmt.Errorf("client: invalid base URL %q", baseURL)
	}
	return &Client{
		baseURL: parsed.String(),
		secret:  secret,
		http: &http.Client{
			Timeout: apiBodyTimeout,
		},
	}, nil
}

func (c *Client) endpoint(parts ...string) (string, error) {
	endpoint, err := url.JoinPath(c.baseURL, parts...)
	if err != nil {
		return "", fmt.Errorf("client: join endpoint: %w", err)
	}
	return endpoint, nil
}

// GetNodes fetches all node metadata from paily-core.
// On HTTP or parse error the error is returned and the caller should retry
// on the next cron tick.
func (c *Client) GetNodes() ([]Node, error) {
	endpoint, err := c.endpoint("api", "v1", "check", "nodes")
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("client: build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.secret)
	ctx, cancel := context.WithTimeout(context.Background(), apiBodyTimeout)
	defer cancel()
	req = req.WithContext(ctx)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("client: GET /api/v1/check/nodes: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("client: GET /api/v1/check/nodes status %d: %s", resp.StatusCode, body)
	}

	var out checkNodesResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("client: decode nodes: %w", err)
	}
	return out.Nodes, nil
}

// PostResults uploads all check results to paily-core.
// Non-2xx responses are returned to the caller for retry handling.
func (c *Client) PostResults(results []CheckResult) error {
	payload := checkResultsRequest{Results: results}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("client: marshal results: %w", err)
	}

	endpoint, err := c.endpoint("api", "v1", "check", "results")
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("client: build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.secret)
	req.Header.Set("Content-Type", "application/json")
	ctx, cancel := context.WithTimeout(context.Background(), apiBodyTimeout)
	defer cancel()
	req = req.WithContext(ctx)

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("client: POST /api/v1/check/results: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body2, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("client: POST /api/v1/check/results status %d: %s", resp.StatusCode, body2)
	}
	return nil
}
