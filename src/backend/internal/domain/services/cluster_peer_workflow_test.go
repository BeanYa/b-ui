package service

import (
	"path/filepath"
	"strings"
	"testing"

	database "github.com/alireza0/b-ui/src/backend/internal/infra/db"
	"github.com/alireza0/b-ui/src/backend/internal/infra/db/model"
)

func TestNextChainStepStopsOnFailureByDefault(t *testing.T) {
	route := RoutePlan{Mode: RouteModeChain, Chain: []RouteStep{
		{StepID: "step-a", NodeID: "node-a"},
		{StepID: "step-b", NodeID: "node-b"},
	}}
	next, ok := NextClusterPeerChainStep(route, "step-a", false)
	if ok || next.StepID != "" {
		t.Fatalf("expected chain to stop on failure, got %#v %v", next, ok)
	}
}

func TestNextChainStepContinuesOnSuccess(t *testing.T) {
	route := RoutePlan{Mode: RouteModeChain, Chain: []RouteStep{
		{StepID: "step-a", NodeID: "node-a"},
		{StepID: "step-b", NodeID: "node-b"},
	}}
	next, ok := NextClusterPeerChainStep(route, "step-a", true)
	if !ok || next.StepID != "step-b" {
		t.Fatalf("expected step-b, got %#v %v", next, ok)
	}
}

func TestNextChainStepContinuesOnFailureWhenAllowed(t *testing.T) {
	route := RoutePlan{Mode: RouteModeChain, Chain: []RouteStep{
		{StepID: "step-a", NodeID: "node-a", ContinueOnFailure: true},
		{StepID: "step-b", NodeID: "node-b"},
	}}
	next, ok := NextClusterPeerChainStep(route, "step-a", false)
	if !ok || next.StepID != "step-b" {
		t.Fatalf("expected step-b after allowed failure, got %#v %v", next, ok)
	}
}

func TestNextChainStepReturnsFalseForNonChainRoute(t *testing.T) {
	route := RoutePlan{Mode: RouteModeDirect, Chain: []RouteStep{
		{StepID: "step-a", NodeID: "node-a"},
		{StepID: "step-b", NodeID: "node-b"},
	}}
	next, ok := NextClusterPeerChainStep(route, "step-a", true)
	if ok || next.StepID != "" {
		t.Fatalf("expected no next step for non-chain route, got %#v %v", next, ok)
	}
}

func TestNextChainStepReturnsFalseForLastOrMissingStep(t *testing.T) {
	route := RoutePlan{Mode: RouteModeChain, Chain: []RouteStep{
		{StepID: "step-a", NodeID: "node-a"},
		{StepID: "step-b", NodeID: "node-b"},
	}}

	next, ok := NextClusterPeerChainStep(route, "step-b", true)
	if ok || next.StepID != "" {
		t.Fatalf("expected no next step after last step, got %#v %v", next, ok)
	}

	next, ok = NextClusterPeerChainStep(route, "step-missing", true)
	if ok || next.StepID != "" {
		t.Fatalf("expected no next step for missing step, got %#v %v", next, ok)
	}
}

func TestSaveClusterPeerWorkflowStepUpsertsByWorkflowStep(t *testing.T) {
	if err := database.InitDB(filepath.Join(t.TempDir(), "workflow-state.db")); err != nil {
		if strings.Contains(err.Error(), "go-sqlite3 requires cgo") || strings.Contains(err.Error(), "C compiler") {
			t.Skipf("sqlite test database unavailable in this toolchain: %v", err)
		}
		t.Fatalf("init test db: %v", err)
	}

	if err := database.GetDB().Create(&model.ClusterPeerWorkflowState{
		WorkflowID: "workflow-direct-duplicate",
		StepID:     "step-a",
		DomainID:   "edge.example.com",
		NodeID:     "node-a",
		Status:     PeerEventStatusProcessing,
		CreatedAt:  100,
		UpdatedAt:  100,
	}).Error; err != nil {
		t.Fatalf("seed direct duplicate baseline: %v", err)
	}
	if err := database.GetDB().Create(&model.ClusterPeerWorkflowState{
		WorkflowID: "workflow-direct-duplicate",
		StepID:     "step-a",
		DomainID:   "edge.example.com",
		NodeID:     "node-b",
		Status:     PeerEventStatusSucceeded,
		CreatedAt:  101,
		UpdatedAt:  101,
	}).Error; err == nil {
		t.Fatal("expected workflow_id and step_id to reject direct duplicates")
	}

	if err := SaveClusterPeerWorkflowStep("workflow-1", "step-a", "edge-a.example.com", "node-a", PeerEventStatusProcessing, "hash-a", ""); err != nil {
		t.Fatalf("save first workflow step: %v", err)
	}
	var first model.ClusterPeerWorkflowState
	if err := database.GetDB().Where("workflow_id = ? AND step_id = ?", "workflow-1", "step-a").First(&first).Error; err != nil {
		t.Fatalf("load first workflow step: %v", err)
	}
	if first.CreatedAt == 0 {
		t.Fatal("expected first save to populate created_at")
	}

	if err := SaveClusterPeerWorkflowStep("workflow-1", "step-a", "edge-b.example.com", "node-b", PeerEventStatusFailed, "hash-b", "step failed"); err != nil {
		t.Fatalf("save updated workflow step: %v", err)
	}

	var count int64
	if err := database.GetDB().Model(&model.ClusterPeerWorkflowState{}).Where("workflow_id = ? AND step_id = ?", "workflow-1", "step-a").Count(&count).Error; err != nil {
		t.Fatalf("count workflow steps: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected one workflow step row, got %d", count)
	}

	var updated model.ClusterPeerWorkflowState
	if err := database.GetDB().Where("workflow_id = ? AND step_id = ?", "workflow-1", "step-a").First(&updated).Error; err != nil {
		t.Fatalf("load updated workflow step: %v", err)
	}
	if updated.DomainID != "edge-b.example.com" {
		t.Fatalf("expected updated domain, got %q", updated.DomainID)
	}
	if updated.NodeID != "node-b" {
		t.Fatalf("expected updated node, got %q", updated.NodeID)
	}
	if updated.Status != PeerEventStatusFailed {
		t.Fatalf("expected updated status, got %q", updated.Status)
	}
	if updated.ResultHash != "hash-b" {
		t.Fatalf("expected updated result hash, got %q", updated.ResultHash)
	}
	if updated.Error != "step failed" {
		t.Fatalf("expected updated error, got %q", updated.Error)
	}
	if updated.CreatedAt != first.CreatedAt {
		t.Fatalf("expected created_at to remain %d, got %d", first.CreatedAt, updated.CreatedAt)
	}
	if updated.UpdatedAt == 0 {
		t.Fatal("expected updated_at to be populated")
	}
}

func TestInitDBDedupesClusterPeerWorkflowStateBeforeUniqueIndex(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "workflow-dedupe.db")
	if err := database.OpenDB(dbPath); err != nil {
		if strings.Contains(err.Error(), "go-sqlite3 requires cgo") || strings.Contains(err.Error(), "C compiler") {
			t.Skipf("sqlite test database unavailable in this toolchain: %v", err)
		}
		t.Fatalf("open test db: %v", err)
	}
	if err := database.GetDB().Exec(`
		CREATE TABLE cluster_peer_workflow_states (
			id integer primary key autoincrement,
			workflow_id text,
			step_id text,
			domain_id text,
			node_id text,
			status text,
			result_hash text,
			error text,
			created_at integer,
			updated_at integer
		)
	`).Error; err != nil {
		t.Fatalf("create legacy workflow state table: %v", err)
	}
	if err := database.GetDB().Exec(`
		INSERT INTO cluster_peer_workflow_states
			(workflow_id, step_id, domain_id, node_id, status, result_hash, error, created_at, updated_at)
		VALUES
			('workflow-legacy', 'step-a', 'edge.example.com', 'node-a', 'processing', 'old-hash', '', 100, 100),
			('workflow-legacy', 'step-a', 'edge.example.com', 'node-a', 'succeeded', 'new-hash', '', 101, 101),
			('workflow-legacy', 'step-b', 'edge.example.com', 'node-b', 'succeeded', 'other-hash', '', 102, 102)
	`).Error; err != nil {
		t.Fatalf("seed duplicate legacy workflow states: %v", err)
	}

	if err := database.InitDB(dbPath); err != nil {
		t.Fatalf("init db with duplicate legacy workflow states: %v", err)
	}

	var count int64
	if err := database.GetDB().Model(&model.ClusterPeerWorkflowState{}).Where("workflow_id = ? AND step_id = ?", "workflow-legacy", "step-a").Count(&count).Error; err != nil {
		t.Fatalf("count deduped workflow states: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected duplicate workflow step rows to be deduped, got %d", count)
	}

	var kept model.ClusterPeerWorkflowState
	if err := database.GetDB().Where("workflow_id = ? AND step_id = ?", "workflow-legacy", "step-a").First(&kept).Error; err != nil {
		t.Fatalf("load deduped workflow state: %v", err)
	}
	if kept.Status != PeerEventStatusSucceeded || kept.ResultHash != "new-hash" {
		t.Fatalf("expected newest duplicate row to be kept, got %#v", kept)
	}

	err := database.GetDB().Create(&model.ClusterPeerWorkflowState{
		WorkflowID: "workflow-legacy",
		StepID:     "step-a",
		DomainID:   "edge.example.com",
		NodeID:     "node-c",
		Status:     PeerEventStatusFailed,
		CreatedAt:  200,
		UpdatedAt:  200,
	}).Error
	if err == nil {
		t.Fatal("expected unique workflow-step index to reject new duplicates after migration")
	}
}
