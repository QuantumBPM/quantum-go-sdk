package quantumdmn_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumDMN/dmn-go-sdk/pkg/quantumdmn"
)

func TestNewClientWithToken(t *testing.T) {
	// create a test server that verifies the auth header
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth != "Bearer test-token" {
			t.Errorf("expected 'Bearer test-token', got '%s'", auth)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()

	client, err := quantumdmn.NewClientWithToken(server.URL, "test-token")
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	resp, err := client.GetHealthWithResponse(context.Background())
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}

	if resp.StatusCode() != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode())
	}
}

func TestNewAuthenticatedClient(t *testing.T) {
	callCount := 0
	tokenProvider := func(ctx context.Context) (string, error) {
		callCount++
		return "dynamic-token", nil
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth != "Bearer dynamic-token" {
			t.Errorf("expected 'Bearer dynamic-token', got '%s'", auth)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()

	client, err := quantumdmn.NewAuthenticatedClient(server.URL, tokenProvider)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	_, err = client.GetHealthWithResponse(context.Background())
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}

	if callCount != 1 {
		t.Errorf("expected tokenProvider to be called once, got %d", callCount)
	}
}

func TestNewAuthenticatedClient_MultipleRequests(t *testing.T) {
	callCount := 0
	tokenProvider := func(ctx context.Context) (string, error) {
		callCount++
		return "token", nil
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()

	client, err := quantumdmn.NewAuthenticatedClient(server.URL, tokenProvider)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	// make multiple requests
	for i := 0; i < 3; i++ {
		_, err = client.GetHealthWithResponse(context.Background())
		if err != nil {
			t.Fatalf("request %d failed: %v", i, err)
		}
	}

	// token provider should be called for each request
	if callCount != 3 {
		t.Errorf("expected tokenProvider to be called 3 times, got %d", callCount)
	}
}
