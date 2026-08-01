// Package mirakurun implements the Mirakurun-only backend used by placidus.
package mirakurun

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// API is the Mirakurun behavior consumed by the application.
type API interface {
	Services(context.Context) ([]Service, error)
	Programs(context.Context) ([]Program, error)
	Tuners(context.Context) ([]Tuner, error)
	OpenServiceStream(context.Context, uint64, int) (io.ReadCloser, error)
}

// Client calls one Mirakurun HTTP API endpoint.
type Client struct {
	baseURL    *url.URL
	httpClient *http.Client
}

// NewClient validates baseURL and returns a Mirakurun client.
func NewClient(baseURL string, httpClient *http.Client) (*Client, error) {
	parsed, err := url.Parse(strings.TrimRight(baseURL, "/"))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return nil, fmt.Errorf("invalid Mirakurun URL %q", baseURL)
	}
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 15 * time.Second}
	}
	return &Client{baseURL: parsed, httpClient: httpClient}, nil
}

// Services returns all services known to Mirakurun.
func (c *Client) Services(ctx context.Context) ([]Service, error) {
	var services []Service
	if err := c.getJSON(ctx, "/api/services", &services); err != nil {
		return nil, err
	}
	return services, nil
}

// Programs returns all current EPG entries known to Mirakurun.
func (c *Client) Programs(ctx context.Context) ([]Program, error) {
	var programs []Program
	if err := c.getJSON(ctx, "/api/programs", &programs); err != nil {
		return nil, err
	}
	return programs, nil
}

// Tuners returns Mirakurun's current tuner state.
func (c *Client) Tuners(ctx context.Context) ([]Tuner, error) {
	var tuners []Tuner
	if err := c.getJSON(ctx, "/api/tuners", &tuners); err != nil {
		return nil, err
	}
	return tuners, nil
}

// OpenServiceStream opens a decoded MPEG-TS service stream at the requested priority.
func (c *Client) OpenServiceStream(ctx context.Context, serviceID uint64, priority int) (io.ReadCloser, error) {
	values := url.Values{}
	values.Set("decode", "1")
	values.Set("priority", strconv.Itoa(priority))
	path := "/api/services/" + strconv.FormatUint(serviceID, 10) + "/stream?" + values.Encode()
	req, err := c.request(ctx, http.MethodGet, path)
	if err != nil {
		return nil, err
	}
	// Streaming responses must not use the metadata client's global timeout.
	streamClient := *c.httpClient
	streamClient.Timeout = 0
	resp, err := streamClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("open Mirakurun service %d stream: %w", serviceID, err)
	}
	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		message, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("Mirakurun stream returned %s: %s", resp.Status, strings.TrimSpace(string(message)))
	}
	return resp.Body, nil
}

func (c *Client) getJSON(ctx context.Context, path string, target any) error {
	req, err := c.request(ctx, http.MethodGet, path)
	if err != nil {
		return err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request Mirakurun %s: %w", path, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		message, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("Mirakurun %s returned %s: %s", path, resp.Status, strings.TrimSpace(string(message)))
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 32<<20)).Decode(target); err != nil {
		return fmt.Errorf("decode Mirakurun %s response: %w", path, err)
	}
	return nil
}

func (c *Client) request(ctx context.Context, method, path string) (*http.Request, error) {
	target, err := c.baseURL.Parse(path)
	if err != nil {
		return nil, fmt.Errorf("build Mirakurun request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, method, target.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("build Mirakurun request: %w", err)
	}
	req.Header.Set("Accept", "application/json, video/MP2T")
	req.Header.Set("User-Agent", "placidus/0.1")
	return req, nil
}
