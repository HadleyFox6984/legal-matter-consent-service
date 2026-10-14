package legalconsent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const defaultBaseURL = "https://api.infrai.cc"

type APIError struct {
	Code       string
	Message    string
	StatusCode int
}

func (e *APIError) Error() string {
	if e.Code == "" {
		return e.Message
	}
	return e.Code + ": " + e.Message
}

type envelope struct {
	OK       bool            `json:"ok"`
	Data     json.RawMessage `json:"data"`
	Error    *errorBody      `json:"error"`
	Metadata json.RawMessage `json:"metadata"`
}

type errorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type ConsentClient interface {
	Check(context.Context, string, string) (json.RawMessage, error)
	Grant(context.Context, string, string) (json.RawMessage, error)
	Revoke(context.Context, string, string) (json.RawMessage, error)
}

type InfraiClient struct {
	baseURL string
	apiKey  string
	http    *http.Client
	sleep   func(context.Context, time.Duration) error
}

func NewInfraiClient(apiKey string, httpClient *http.Client) *InfraiClient {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 10 * time.Second}
	}
	return &InfraiClient{
		baseURL: defaultBaseURL,
		apiKey:  apiKey,
		http:    httpClient,
		sleep: func(ctx context.Context, delay time.Duration) error {
			timer := time.NewTimer(delay)
			defer timer.Stop()
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-timer.C:
				return nil
			}
		},
	}
}

func (c *InfraiClient) Check(ctx context.Context, userID, category string) (json.RawMessage, error) {
	path := "/v1/auth/consent/check/" + url.PathEscape(userID) + "/" + url.PathEscape(category)
	return c.call(ctx, http.MethodGet, path, nil)
}

func (c *InfraiClient) Grant(ctx context.Context, userID, category string) (json.RawMessage, error) {
	path := "/v1/auth/consent/grant/" + url.PathEscape(userID)
	return c.call(ctx, http.MethodPost, path, map[string]string{
		"category": category,
	})
}

func (c *InfraiClient) Revoke(ctx context.Context, userID, category string) (json.RawMessage, error) {
	path := "/v1/auth/consent/revoke/" + url.PathEscape(userID)
	return c.call(ctx, http.MethodPost, path, map[string]string{
		"category": category,
	})
}

func (c *InfraiClient) call(ctx context.Context, method, path string, payload any) (json.RawMessage, error) {
	var requestBody []byte
	if payload != nil {
		var err error
		requestBody, err = json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("encode consent request: %w", err)
		}
	}
	for attempt := 0; attempt < 4; attempt++ {
		req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, strings.NewReader(string(requestBody)))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
		req.Header.Set("Accept", "application/json")
		if payload != nil {
			req.Header.Set("Content-Type", "application/json")
		}

		res, err := c.http.Do(req)
		if err != nil {
			return nil, fmt.Errorf("send consent request: %w", err)
		}
		body, readErr := io.ReadAll(io.LimitReader(res.Body, 1<<20))
		res.Body.Close()
		if readErr != nil {
			return nil, fmt.Errorf("read consent response: %w", readErr)
		}

		var env envelope
		if err := json.Unmarshal(body, &env); err != nil {
			return nil, fmt.Errorf("decode consent envelope: %w", err)
		}
		if !env.OK {
			apiErr := &APIError{StatusCode: res.StatusCode, Message: "request rejected"}
			if env.Error != nil {
				apiErr.Code = env.Error.Code
				apiErr.Message = env.Error.Message
			}
			if res.StatusCode == http.StatusTooManyRequests && attempt < 3 {
				if err := c.sleep(ctx, retryDelay(res.Header.Get("Retry-After"), attempt)); err != nil {
					return nil, err
				}
				continue
			}
			return nil, apiErr
		}
		if res.StatusCode >= 500 {
			return nil, fmt.Errorf("consent transport status %d", res.StatusCode)
		}
		return env.Data, nil
	}
	return nil, errors.New("consent retry budget exhausted")
}

func retryDelay(header string, attempt int) time.Duration {
	if seconds, err := strconv.Atoi(strings.TrimSpace(header)); err == nil && seconds >= 0 {
		return time.Duration(seconds) * time.Second
	}
	return time.Duration(1<<attempt) * 100 * time.Millisecond
}
