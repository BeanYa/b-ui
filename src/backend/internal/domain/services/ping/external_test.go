package ping

import (
	"context"
	"testing"
)

func TestDefaultExternalConfig(t *testing.T) {
	config := defaultExternalConfig()
	if len(config.Sources) != 10 {
		t.Fatalf("expected 10 default sources, got %d", len(config.Sources))
	}
	inCount := 0
	outCount := 0
	for _, src := range config.Sources {
		if src.Direction == "inbound" {
			inCount++
		} else if src.Direction == "outbound" {
			outCount++
		}
	}
	if inCount != 5 {
		t.Fatalf("expected 5 inbound sources, got %d", inCount)
	}
	if outCount != 5 {
		t.Fatalf("expected 5 outbound sources, got %d", outCount)
	}
}

func TestRunExternal_NoSourcesEnabled(t *testing.T) {
	svc := NewExternalService(newStoreWithDir(t.TempDir()))
	_, err := svc.Run(context.Background(), ExternalRunRequest{SourceIDs: []string{}}, nil)
	if err == nil {
		t.Fatal("expected error for no enabled sources")
	}
}
