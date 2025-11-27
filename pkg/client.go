package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"golang.org/x/time/rate"
	"net/http"
	"sync"
	"time"
)

type Response struct {
	Components map[string]interface{} `json:"components"`
	Paths      map[string]interface{} `json:"paths"`
}

type ResponseError struct {
	Error string `json:"error"`
}

type OllamaResult struct {
	Response interface{} `json:"response"`
}

type OllamaRequestOptions struct {
	Handler    string `json:"handler"`
	Components string `json:"components"`
	Security   string `json:"security"`
}

type OllamaRequest struct {
	Model   string               `json:"model"`
	Prompt  string               `json:"prompt"`
	Options OllamaRequestOptions `json:"options"`
}

type Provider interface {
	GeneratePathDef(ctx context.Context, handlerCode, components, security string) (*Response, error)
}

type Client struct {
	endpoint     string
	client       *http.Client
	limiter      *rate.Limiter
	maxRetries   int
	backoffBase  time.Duration
	workerPool   chan struct{}
	mu           sync.Mutex
	failureCount int
}

func NewClient(endpoint string, maxConcurrent int) *Client {
	return &Client{
		endpoint:     endpoint,
		client:       &http.Client{Timeout: 30 * time.Second},
		limiter:      rate.NewLimiter(rate.Every(100*time.Millisecond), 1),
		maxRetries:   5,
		backoffBase:  time.Second,
		workerPool:   make(chan struct{}, maxConcurrent),
		failureCount: 0,
	}
}

func (c *Client) GeneratePathDef(ctx context.Context, handlerCode, components, security string) (*Response, error) {
	select {
	case c.workerPool <- struct{}{}:
		defer func() { <-c.workerPool }()
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	var lastErr error
	for attempt := 0; attempt < c.maxRetries; attempt++ {
		if err := c.limiter.Wait(ctx); err != nil {
			return nil, err
		}

		data, err := c.makeRequest(ctx, handlerCode, components, security)
		if err == nil {
			c.resetFailureCount()
			return data, nil
		}

		lastErr = err
		c.incrementFailureCount()

		backoff := c.calculateBackoff(attempt)
		timer := time.NewTimer(backoff)
		select {
		case <-timer.C:
			continue
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		}
	}

	return nil, fmt.Errorf("max retries exceeded: %v", lastErr)
}

func (c *Client) makeRequest(ctx context.Context, handlerCode, components, security string) (*Response, error) {
	reqBody := OllamaRequest{
		Model:  "",
		Prompt: "",
		Options: OllamaRequestOptions{
			Handler:    handlerCode,
			Components: components,
			Security:   security,
		},
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.endpoint+"/api/generate?format=json", bytes.NewReader(jsonBody))
	if err != nil {
		return nil, err
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("AI API error: %d", resp.StatusCode)
	}

	var result OllamaResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	responseBytes, err := json.Marshal(result.Response)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal intermediate response: %v", err)
	}

	// Try to unmarshal as error response first
	var errorResponse ResponseError
	if err = json.Unmarshal(responseBytes, &errorResponse); err == nil && errorResponse.Error != "" {
		return nil, errors.New(errorResponse.Error)
	}

	// If not an error, try to unmarshal as success response
	var response Response
	if err = json.Unmarshal(responseBytes, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %v", err)
	}

	return &response, nil
}

func (c *Client) incrementFailureCount() {
	c.mu.Lock()
	c.failureCount++
	c.mu.Unlock()
}

func (c *Client) resetFailureCount() {
	c.mu.Lock()
	c.failureCount = 0
	c.mu.Unlock()
}

func (c *Client) calculateBackoff(attempt int) time.Duration {
	c.mu.Lock()
	defer c.mu.Unlock()

	multiplier := time.Duration(1 << uint(c.failureCount+attempt))
	return c.backoffBase * multiplier
}
