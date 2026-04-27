package ping

import (
	"path/filepath"
	"testing"
)

func TestSaveAndLoadMeshResult(t *testing.T) {
	dir := t.TempDir()
	domainID := "test.example.com"
	result := &MeshResult{
		DomainID: domainID,
		TestedAt: nowUnix(),
		Results: []MeshPairResult{
			{
				SourceMemberID: "n1", SourceName: "node-1",
				TargetMemberID: "n2", TargetName: "node-2",
				Method: methodPtr("http"), LatencyMs: latencyPtr(12.5),
				Success: true, Error: nil,
			},
		},
	}

	store := newStoreWithDir(dir)
	if err := store.SaveMeshResult(result); err != nil {
		t.Fatalf("SaveMeshResult: %v", err)
	}

	loaded, err := store.LoadMeshResult(domainID)
	if err != nil {
		t.Fatalf("LoadMeshResult: %v", err)
	}
	if loaded.DomainID != domainID {
		t.Fatalf("expected domain %q, got %q", domainID, loaded.DomainID)
	}
	if len(loaded.Results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(loaded.Results))
	}
	if *loaded.Results[0].LatencyMs != 12.5 {
		t.Fatalf("expected 12.5ms, got %f", *loaded.Results[0].LatencyMs)
	}
}

func TestLoadMeshResultNotFound(t *testing.T) {
	dir := t.TempDir()
	store := newStoreWithDir(dir)
	_, err := store.LoadMeshResult("nonexistent")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestSaveAndLoadExternalConfig(t *testing.T) {
	dir := t.TempDir()
	store := newStoreWithDir(dir)
	config := &ExternalConfig{
		Sources: []ExternalSource{
			{ID: "ripe_atlas", Name: "RIPE Atlas", Direction: "inbound", Enabled: true},
		},
	}

	if err := store.SaveExternalConfig(config); err != nil {
		t.Fatalf("SaveExternalConfig: %v", err)
	}

	loaded, err := store.LoadExternalConfig()
	if err != nil {
		t.Fatalf("LoadExternalConfig: %v", err)
	}
	if len(loaded.Sources) != 1 {
		t.Fatalf("expected 1 source, got %d", len(loaded.Sources))
	}
}

func TestSaveAndLoadExternalResults(t *testing.T) {
	dir := t.TempDir()
	store := newStoreWithDir(dir)
	data := &ExternalResultData{
		TestedAt: nowUnix(),
		Results: []ExternalTestResult{
			{SourceLabel: "RIPE - FRA", Direction: "inbound", Success: true},
		},
	}

	if err := store.SaveExternalResults(data); err != nil {
		t.Fatalf("SaveExternalResults: %v", err)
	}

	loaded, err := store.LoadExternalResults()
	if err != nil {
		t.Fatalf("LoadExternalResults: %v", err)
	}
	if len(loaded.Results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(loaded.Results))
	}
}

func newStoreWithDir(dir string) *Store {
	return &Store{dataDir: filepath.Join(dir, DataDir)}
}
