package dmn

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/QuantumBPM/quantum-go-sdk/v2/generated"
)

// definitionJSON takes isDeployed and any trailing fields.
const definitionJSON = `{"id":"7f6c4a2e-0b1d-4c3e-9f8a-1b2c3d4e5f60","name":"rating.dmn","definitionsID":"rating","xml":"<definitions/>","isDeployed":%s,"createdAt":"2026-11-18T10:00:00Z"%s}`

func TestCreateDefinition_PostsNameAndXML(t *testing.T) {
	var path string
	var got generated.CreateDefinitionRequest
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		path = r.Method + " " + r.URL.Path
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &got)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = fmt.Fprintf(w, definitionJSON, "false", "")
	})

	def, err := c.CreateDefinition(context.Background(), "rating.dmn", "<definitions/>")
	if err != nil {
		t.Fatalf("CreateDefinition: %v", err)
	}
	if want := "POST /projects/00000000-0000-0000-0000-000000000000/dmn/definitions"; path != want {
		t.Fatalf("request = %q, want %q", path, want)
	}
	if got.Name != "rating.dmn" || got.Xml != "<definitions/>" {
		t.Fatalf("body = %+v", got)
	}
	if def.DefinitionsID != "rating" || def.IsDeployed {
		t.Fatalf("definition = %+v", def)
	}
}

func TestDeployDefinition_ReturnsVersion(t *testing.T) {
	id := uuid.MustParse("7f6c4a2e-0b1d-4c3e-9f8a-1b2c3d4e5f60")
	var path string
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		path = r.Method + " " + r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, definitionJSON, "true", `,"version":3`)
	})

	def, err := c.DeployDefinition(context.Background(), id)
	if err != nil {
		t.Fatalf("DeployDefinition: %v", err)
	}
	if want := "POST /projects/00000000-0000-0000-0000-000000000000/dmn/definitions/" + id.String() + "/deploy"; path != want {
		t.Fatalf("request = %q, want %q", path, want)
	}
	if !def.IsDeployed || def.Version == nil || *def.Version != 3 {
		t.Fatalf("definition = %+v", def)
	}
}

func TestCreateDefinition_ConflictIsAnError(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		_, _ = io.WriteString(w, `{"code":"CONFLICT","message":"the document is identical to the latest deployed version"}`)
	})

	_, err := c.CreateDefinition(context.Background(), "rating.dmn", "<definitions/>")
	if err == nil || !strings.Contains(err.Error(), "conflict: the document is identical") {
		t.Fatalf("err = %v, want the conflict message", err)
	}
}
