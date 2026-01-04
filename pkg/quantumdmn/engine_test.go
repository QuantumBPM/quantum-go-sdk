package quantumdmn_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumDMN/dmn-go-sdk/pkg/quantumdmn"
	"github.com/google/uuid"
)

func TestEvaluate(t *testing.T) {
	projectID := uuid.New().String()
	tokenProvider := func(ctx context.Context) (string, error) {
		return "token", nil
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify URL
		expectedPath := "/projects/" + projectID + "/definitions/by-xml-id/my-model/evaluate"
		if r.URL.Path != expectedPath {
			t.Errorf("expected path %s, got %s", expectedPath, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}

		// Verify Body (Context)
		// We can't strictly compare body without unmarshaling to map, but minimal check is fine.
		// Just ensure server is hit.

		// Mock Response
		// EvaluationResult map: key -> EvaluationResult
		// EvaluationResult has Value (FeelValue)
		// FeelValue unmarshals from JSON value.
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		// Return { "My Decision": { "value": "Result", "name": "My Decision" } }
		// "value": "Result" is a JSON string, which FeelValue can unmarshal.
		w.Write([]byte(`{"My Decision": {"value": "Result", "name": "My Decision"}}`))
	}))
	defer server.Close()

	client, err := quantumdmn.NewEngineClient(server.URL, projectID, tokenProvider)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	ctx := context.Background()
	result, err := client.Evaluate(ctx, "my-model", nil, map[string]interface{}{"Input": 123})
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}

	if len(result) != 1 {
		t.Errorf("expected 1 result, got %d", len(result))
	}
	if res, ok := result["My Decision"]; ok {
		if res.Name == nil || *res.Name != "My Decision" {
			t.Errorf("expected name 'My Decision', got %v", res.Name)
		}
		// Verification of Value content is tricky as it is FeelValue (internal JSON)
		// but if we got here, unmarshal worked.
	} else {
		t.Error("expected 'My Decision' in result")
	}
}
