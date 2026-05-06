package service

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	database "github.com/BeanYa/b-ui/src/backend/internal/infra/db"
	"github.com/BeanYa/b-ui/src/backend/internal/infra/db/model"
)

func initClusterDomainOperationTestDB(t *testing.T) {
	t.Helper()
	if err := database.InitDB(filepath.Join(t.TempDir(), "cluster-domain-operation.db")); err != nil {
		if strings.Contains(err.Error(), "go-sqlite3 requires cgo") || strings.Contains(err.Error(), "C compiler") {
			t.Skipf("sqlite test database unavailable in this toolchain: %v", err)
		}
		t.Fatalf("init test db: %v", err)
	}
}

func TestClusterDomainOperationStoreAggregatesPartialStatus(t *testing.T) {
	initClusterDomainOperationTestDB(t)
	store := ClusterDomainOperationStore{DB: database.GetDB()}

	op := &model.ClusterDomainOperation{
		OperationID:       "op-1",
		DomainID:          1,
		Domain:            "edge.example.com",
		ResourceKind:      ClusterDomainResourceInbound,
		ResourceID:        "group-1",
		Action:            ClusterDomainOperationCreate,
		Revision:          1,
		CoordinatorNodeID: "node-a",
		Status:            ClusterDomainOperationDispatching,
		DesiredPayload:    []byte(`{"group_id":"group-1"}`),
	}
	if err := store.SaveOperation(op); err != nil {
		t.Fatalf("save operation: %v", err)
	}
	if op.Id == 0 || op.CreatedAt == 0 {
		t.Fatalf("saved operation did not reload id/created_at: id=%d created_at=%d", op.Id, op.CreatedAt)
	}
	createdAt := op.CreatedAt
	op.Status = ClusterDomainOperationReported
	if err := store.SaveOperation(op); err != nil {
		t.Fatalf("update operation: %v", err)
	}
	if op.CreatedAt != createdAt {
		t.Fatalf("operation created_at = %d, want preserved %d", op.CreatedAt, createdAt)
	}

	applied := &model.ClusterDomainOperationInstance{
		OperationID:  "op-1",
		MemberID:     "member-a",
		NodeID:       "node-a",
		DisplayName:  "Node A",
		TargetTag:    "edge",
		Status:       ClusterDomainOperationApplied,
		AttemptCount: 1,
	}
	if err := store.SaveInstance(applied); err != nil {
		t.Fatalf("save applied instance: %v", err)
	}
	if applied.Id == 0 || applied.CreatedAt == 0 {
		t.Fatalf("saved instance did not reload id/created_at: id=%d created_at=%d", applied.Id, applied.CreatedAt)
	}
	appliedCreatedAt := applied.CreatedAt
	applied.AttemptCount = 2
	if err := store.SaveInstance(applied); err != nil {
		t.Fatalf("update applied instance: %v", err)
	}
	if applied.CreatedAt != appliedCreatedAt {
		t.Fatalf("instance created_at = %d, want preserved %d", applied.CreatedAt, appliedCreatedAt)
	}
	if err := store.SaveInstance(&model.ClusterDomainOperationInstance{
		OperationID:  "op-1",
		MemberID:     "member-b",
		NodeID:       "node-b",
		DisplayName:  "Node B",
		TargetTag:    "edge",
		Status:       ClusterDomainOperationFailed,
		AttemptCount: 1,
		Error:        "boom",
	}); err != nil {
		t.Fatalf("save failed instance: %v", err)
	}

	status, summary, err := store.RecomputeStatus("op-1")
	if err != nil {
		t.Fatalf("recompute status: %v", err)
	}

	if status != ClusterDomainOperationPartial {
		t.Fatalf("status = %q, want %q", status, ClusterDomainOperationPartial)
	}
	if summary.Failed != 1 || summary.Applied != 1 {
		t.Fatalf("summary failed/applied = %d/%d, want 1/1", summary.Failed, summary.Applied)
	}

	op, err = store.GetOperation("op-1")
	if err != nil {
		t.Fatalf("get operation: %v", err)
	}
	if op.Status != ClusterDomainOperationPartial {
		t.Fatalf("stored status = %q, want %q", op.Status, ClusterDomainOperationPartial)
	}
}

func TestClusterDomainOperationSummaryCountsPendingStatusesAsQueued(t *testing.T) {
	summary := summarizeClusterDomainOperationInstances([]model.ClusterDomainOperationInstance{
		{Status: ClusterDomainOperationApplied},
		{Status: ClusterDomainOperationFailed},
		{Status: ClusterDomainOperationDispatching},
		{Status: ClusterDomainOperationReported},
	})

	if summary.Total != 4 || summary.Applied != 1 || summary.Failed != 1 || summary.Queued != 2 {
		t.Fatalf("summary = %+v, want total=4 applied=1 failed=1 queued=2", summary)
	}
	if status := statusForClusterDomainOperationSummary(summary, ClusterDomainOperationDispatching); status != ClusterDomainOperationPartial {
		t.Fatalf("status = %q, want %q", status, ClusterDomainOperationPartial)
	}
}

func TestClusterDomainOperationStatusPreservesActiveZeroInstanceStatus(t *testing.T) {
	summary := ClusterDomainOperationSummary{}

	for _, existing := range []string{
		ClusterDomainOperationQueued,
		ClusterDomainOperationDispatching,
		ClusterDomainOperationPartial,
		ClusterDomainOperationReported,
	} {
		if status := statusForClusterDomainOperationSummary(summary, existing); status != existing {
			t.Fatalf("status with existing %q = %q, want preserved", existing, status)
		}
	}
	if status := statusForClusterDomainOperationSummary(summary, ClusterDomainOperationFailed); status != ClusterDomainOperationQueued {
		t.Fatalf("status with failed existing = %q, want %q", status, ClusterDomainOperationQueued)
	}
}

func TestClusterDomainOperationStatusClassifiesAllSkipped(t *testing.T) {
	summary := ClusterDomainOperationSummary{Skipped: 2, Total: 2}

	if status := statusForClusterDomainOperationSummary(summary, ClusterDomainOperationDispatching); status != ClusterDomainOperationSkipped {
		t.Fatalf("status = %q, want %q", status, ClusterDomainOperationSkipped)
	}
}

func TestClusterDomainOperationStatusClassifiesAllFailed(t *testing.T) {
	summary := ClusterDomainOperationSummary{Failed: 2, Total: 2}

	if status := statusForClusterDomainOperationSummary(summary, ClusterDomainOperationDispatching); status != ClusterDomainOperationFailed {
		t.Fatalf("status = %q, want %q", status, ClusterDomainOperationFailed)
	}
}

func TestClusterDomainOperationStatusClassifiesAllTimeout(t *testing.T) {
	summary := ClusterDomainOperationSummary{Timeout: 2, Total: 2}

	if status := statusForClusterDomainOperationSummary(summary, ClusterDomainOperationDispatching); status != ClusterDomainOperationTimeout {
		t.Fatalf("status = %q, want %q", status, ClusterDomainOperationTimeout)
	}
}

func TestClusterDomainOperationStatusClassifiesMixedFailedTimeoutAsPartial(t *testing.T) {
	summary := ClusterDomainOperationSummary{Failed: 1, Timeout: 1, Total: 2}

	if status := statusForClusterDomainOperationSummary(summary, ClusterDomainOperationDispatching); status != ClusterDomainOperationPartial {
		t.Fatalf("status = %q, want %q", status, ClusterDomainOperationPartial)
	}
}

func TestClusterDomainOperationViewJSONShape(t *testing.T) {
	view := ClusterDomainOperationView{
		OperationID:  "op-1",
		DomainID:     1,
		Domain:       "edge.example.com",
		ResourceKind: ClusterDomainResourceInbound,
		ResourceID:   "group-1",
		Action:       ClusterDomainOperationCreate,
		Revision:     1,
		Status:       ClusterDomainOperationPartial,
		Summary: ClusterDomainOperationSummary{
			Queued:  1,
			Applied: 1,
			Failed:  1,
			Timeout: 1,
			Skipped: 1,
			Total:   5,
		},
		Instances: []ClusterDomainOperationInstanceView{{
			MemberID:        "member-a",
			NodeID:          "node-a",
			DisplayName:     "Node A",
			TargetTag:       "edge",
			Status:          ClusterDomainOperationApplied,
			AttemptCount:    2,
			LocalResourceID: 10,
			Error:           "",
			UpdatedAt:       123,
		}},
		UpdatedAt: 456,
	}

	data, err := json.Marshal(view)
	if err != nil {
		t.Fatalf("marshal view: %v", err)
	}
	jsonText := string(data)
	for _, key := range []string{
		`"operationId"`,
		`"domainId"`,
		`"resourceKind"`,
		`"resourceId"`,
		`"attemptCount"`,
		`"localResourceId"`,
		`"updatedAt"`,
	} {
		if !strings.Contains(jsonText, key) {
			t.Fatalf("json %s missing key %s", jsonText, key)
		}
	}
	if strings.Contains(jsonText, `"OperationID"`) || strings.Contains(jsonText, `"AttemptCount"`) {
		t.Fatalf("json used Go field names: %s", jsonText)
	}
}
