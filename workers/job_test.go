package workers

import (
	"testing"

	"github.com/QuantumBPM/quantum-go-sdk/generated"
	"github.com/QuantumBPM/quantum-go-sdk/variables"
)

func TestJob_ExposesBusinessId(t *testing.T) {
	want := "ORDER-42"
	raw := &generated.ExternalJob{BusinessId: &want}
	job := &Job{ExternalJob: raw, Vars: variables.New()}

	if job.BusinessId == nil {
		t.Fatal("expected BusinessId to be set, got nil")
	}
	if *job.BusinessId != want {
		t.Fatalf("BusinessId = %q, want %q", *job.BusinessId, want)
	}
}

func TestJob_BusinessIdNilWhenAbsent(t *testing.T) {
	job := &Job{ExternalJob: &generated.ExternalJob{}, Vars: variables.New()}

	if job.BusinessId != nil {
		t.Fatalf("expected BusinessId nil, got %q", *job.BusinessId)
	}
}
