package legalconsent

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestClientDecodesBusinessRejectionBeforeStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"ok":false,"data":null,"error":{"code":"CONSENT_REJECTED","message":"decision rejected"},"metadata":{}}`))
	}))
	defer server.Close()

	client := NewInfraiClient("test-key", server.Client())
	client.baseURL = server.URL
	_, err := client.Grant(context.Background(), "user-42", "matter-intake")
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.Code != "CONSENT_REJECTED" || apiErr.StatusCode != http.StatusBadRequest {
		t.Fatalf("error=%v", err)
	}
}

func TestClientSendsConsentWritesAsJSON(t *testing.T) {
	tests := []struct {
		name string
		call func(*InfraiClient) error
		path string
	}{
		{"grant", func(c *InfraiClient) error {
			_, err := c.Grant(context.Background(), "user-42", "matter-intake")
			return err
		}, "/v1/auth/consent/grant/user-42"},
		{"revoke", func(c *InfraiClient) error {
			_, err := c.Revoke(context.Background(), "user-42", "matter-intake")
			return err
		}, "/v1/auth/consent/revoke/user-42"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost || r.URL.Path != tt.path || r.Header.Get("Content-Type") != "application/json" {
					t.Errorf("method=%s path=%s content-type=%s", r.Method, r.URL.Path, r.Header.Get("Content-Type"))
				}
				var body map[string]string
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Fatal(err)
				}
				if len(body) != 1 || body["category"] != "matter-intake" {
					t.Errorf("body=%v", body)
				}
				_, _ = w.Write([]byte(`{"ok":true,"data":{},"error":null,"metadata":{}}`))
			}))
			defer server.Close()

			client := NewInfraiClient("test-key", server.Client())
			client.baseURL = server.URL
			if err := tt.call(client); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestClientRetriesRateLimit(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		if calls == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"ok":false,"error":{"code":"RATE_LIMITED","message":"retry later"}}`))
			return
		}
		_, _ = w.Write([]byte(`{"ok":true,"data":{"granted":true},"error":null,"metadata":{}}`))
	}))
	defer server.Close()

	client := NewInfraiClient("test-key", server.Client())
	client.baseURL = server.URL
	client.sleep = func(context.Context, time.Duration) error { return nil }
	if _, err := client.Check(context.Background(), "user-42", "matter-intake"); err != nil {
		t.Fatal(err)
	}
	if calls != 2 {
		t.Fatalf("calls=%d", calls)
	}
}
