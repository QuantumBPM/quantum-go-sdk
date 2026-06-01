package bpmn

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/QuantumBPM/quantum-go-sdk/generated"
	"github.com/QuantumBPM/quantum-go-sdk/variables"
)

func newTestClient(t *testing.T, handler http.HandlerFunc) (*Client, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	api, err := generated.NewClientWithResponses(srv.URL)
	if err != nil {
		t.Fatalf("client: %v", err)
	}
	projectID := openapi_types.UUID{}
	return New(api, projectID), srv
}

func TestStartInstance_SendsBusinessID(t *testing.T) {
	var got generated.StartBpmnInstanceRequest
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(body, &got); err != nil {
			t.Fatalf("decode: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"workflowID":"wf-1"}`))
	})

	defID := openapi_types.UUID{}
	if _, err := c.StartInstance(context.Background(), defID, variables.New(), WithStartBusinessID("ORDER-42")); err != nil {
		t.Fatalf("StartInstance: %v", err)
	}
	if got.BusinessId == nil || *got.BusinessId != "ORDER-42" {
		t.Fatalf("BusinessId in request = %v, want ORDER-42", got.BusinessId)
	}
}

func TestStartInstance_OmitsBusinessIDWhenAbsent(t *testing.T) {
	var got generated.StartBpmnInstanceRequest
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &got)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"workflowID":"wf-1"}`))
	})

	defID := openapi_types.UUID{}
	if _, err := c.StartInstance(context.Background(), defID, variables.New()); err != nil {
		t.Fatalf("StartInstance: %v", err)
	}
	if got.BusinessId != nil {
		t.Fatalf("BusinessId = %q, want nil", *got.BusinessId)
	}
}

func TestListInstances_PassesBusinessIDFilter(t *testing.T) {
	var query url.Values
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		query = r.URL.Query()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[],"pagination":{"page":1,"pageSize":20,"totalCount":0,"totalPages":0}}`))
	})

	if _, err := c.ListInstances(context.Background(), WithInstanceBusinessID("ORDER-42")); err != nil {
		t.Fatalf("ListInstances: %v", err)
	}
	if got := query.Get("businessId"); got != "ORDER-42" {
		t.Fatalf("businessId query = %q, want ORDER-42", got)
	}
}

func TestListUserTasks_PassesBusinessIDFilter(t *testing.T) {
	var query url.Values
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		query = r.URL.Query()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[],"pagination":{"page":1,"pageSize":20,"totalCount":0,"totalPages":0}}`))
	})

	if _, err := c.ListUserTasks(context.Background(), WithUserTaskBusinessID("ORDER-42")); err != nil {
		t.Fatalf("ListUserTasks: %v", err)
	}
	if got := query.Get("businessId"); got != "ORDER-42" {
		t.Fatalf("businessId query = %q, want ORDER-42", got)
	}
}

func TestStartTestInstance_SendsBusinessID(t *testing.T) {
	var got generated.StartBpmnTestInstanceRequest
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/test") {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &got)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"workflowID":"wf-1"}`))
	})

	resID := openapi_types.UUID{}
	if _, err := c.StartTestInstance(context.Background(), resID, nil, variables.New(), WithStartBusinessID("TEST-1")); err != nil {
		t.Fatalf("StartTestInstance: %v", err)
	}
	if got.BusinessId == nil || *got.BusinessId != "TEST-1" {
		t.Fatalf("BusinessId in request = %v, want TEST-1", got.BusinessId)
	}
}
