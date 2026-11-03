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
	"time"

	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/QuantumBPM/quantum-go-sdk/v2/generated"
	"github.com/QuantumBPM/quantum-go-sdk/v2/variables"
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
		_, _ = w.Write([]byte(`{"instanceID":"wf-1"}`))
	})

	if _, err := c.StartInstance(context.Background(), "order", variables.New(), WithStartBusinessID("ORDER-42")); err != nil {
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
		_, _ = w.Write([]byte(`{"instanceID":"wf-1"}`))
	})

	if _, err := c.StartInstance(context.Background(), "order", variables.New()); err != nil {
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
		_, _ = w.Write([]byte(`{"data":[],"pagination":{"total":0}}`))
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
		_, _ = w.Write([]byte(`{"data":[],"pagination":{"total":0}}`))
	})

	if _, err := c.ListUserTasks(context.Background(), WithUserTaskBusinessID("ORDER-42")); err != nil {
		t.Fatalf("ListUserTasks: %v", err)
	}
	if got := query.Get("businessId"); got != "ORDER-42" {
		t.Fatalf("businessId query = %q, want ORDER-42", got)
	}
}

// Both user task lists send the due date bounds and the sort.
func TestListUserTasks_PassDueRangeAndSort(t *testing.T) {
	var query url.Values
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		query = r.URL.Query()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[],"pagination":{"total":0}}`))
	})
	now := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)

	if _, err := c.ListUserTasks(context.Background(), WithUserTaskDueBefore(now), WithUserTaskSort("dueDate")); err != nil {
		t.Fatalf("ListUserTasks: %v", err)
	}
	if query.Get("dueBefore") != "2026-11-10T12:00:00Z" || query.Get("dueAfter") != "" || query.Get("sort") != "dueDate" {
		t.Fatalf("ListUserTasks query = %v", query)
	}
	if _, err := c.ListUserTasksForCaller(context.Background(), WithUserTaskDueAfter(now), WithUserTaskSort("priority")); err != nil {
		t.Fatalf("ListUserTasksForCaller: %v", err)
	}
	if query.Get("dueAfter") != "2026-11-10T12:00:00Z" || query.Get("dueBefore") != "" || query.Get("sort") != "priority" {
		t.Fatalf("ListUserTasksForCaller query = %v", query)
	}
}

// Each cursor list sends its cursor and limit, and hands back the next
// page's cursor.
func TestCursorListsPassCursorAndLimit(t *testing.T) {
	var query url.Values
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		query = r.URL.Query()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[],"pagination":{"nextCursor":"next","total":3}}`))
	})
	ctx := context.Background()
	lists := map[string]func() (*generated.CursorPage, error){
		"ListInstances": func() (*generated.CursorPage, error) {
			p, err := c.ListInstances(ctx, WithInstanceCursor("prev"), WithInstanceLimit(7))
			if err != nil {
				return nil, err
			}
			return &p.Pagination, nil
		},
		"ListUserTasks": func() (*generated.CursorPage, error) {
			p, err := c.ListUserTasks(ctx, WithUserTaskCursor("prev"), WithUserTaskLimit(7))
			if err != nil {
				return nil, err
			}
			return &p.Pagination, nil
		},
		"ListUserTasksForCaller": func() (*generated.CursorPage, error) {
			p, err := c.ListUserTasksForCaller(ctx, WithUserTaskCursor("prev"), WithUserTaskLimit(7))
			if err != nil {
				return nil, err
			}
			return &p.Pagination, nil
		},
	}
	for name, list := range lists {
		page, err := list()
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if query.Get("cursor") != "prev" || query.Get("limit") != "7" {
			t.Errorf("%s sent cursor %q and limit %q, want prev and 7", name, query.Get("cursor"), query.Get("limit"))
		}
		if page.NextCursor == nil || *page.NextCursor != "next" {
			t.Errorf("%s NextCursor = %v, want next", name, page.NextCursor)
		}
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
		_, _ = w.Write([]byte(`{"instanceID":"wf-1"}`))
	})

	resID := openapi_types.UUID{}
	if _, err := c.StartTestInstance(context.Background(), resID, nil, variables.New(), WithStartBusinessID("TEST-1")); err != nil {
		t.Fatalf("StartTestInstance: %v", err)
	}
	if got.BusinessId == nil || *got.BusinessId != "TEST-1" {
		t.Fatalf("BusinessId in request = %v, want TEST-1", got.BusinessId)
	}
}
