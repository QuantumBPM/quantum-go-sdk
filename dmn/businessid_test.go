package dmn

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/QuantumBPM/quantum-go-sdk/generated"
	"github.com/QuantumBPM/quantum-go-sdk/variables"
)

func newTestClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	api, err := generated.NewClientWithResponses(srv.URL)
	if err != nil {
		t.Fatalf("client: %v", err)
	}
	return New(api, openapi_types.UUID{})
}

func TestEvaluate_SendsBusinessID(t *testing.T) {
	var got generated.EvaluateStoredRequest
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &got)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{}`))
	})

	if _, err := c.Evaluate(context.Background(), "def-1", variables.New(), WithBusinessID("ORDER-42")); err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if got.BusinessId == nil || *got.BusinessId != "ORDER-42" {
		t.Fatalf("BusinessId in request = %v, want ORDER-42", got.BusinessId)
	}
}

func TestEvaluateByID_SendsBusinessID(t *testing.T) {
	var got generated.EvaluateStoredRequest
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &got)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{}`))
	})

	if _, err := c.EvaluateByID(context.Background(), openapi_types.UUID{}, variables.New(), WithBusinessID("ORDER-42")); err != nil {
		t.Fatalf("EvaluateByID: %v", err)
	}
	if got.BusinessId == nil || *got.BusinessId != "ORDER-42" {
		t.Fatalf("BusinessId in request = %v, want ORDER-42", got.BusinessId)
	}
}
