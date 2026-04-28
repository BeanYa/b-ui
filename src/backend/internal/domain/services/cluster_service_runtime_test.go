package service

import (
	"slices"
	"testing"
)

func TestNewClusterServiceAdvertisesPanelActions(t *testing.T) {
	service := NewClusterService()
	actions := service.resolvedRouter().Actions()

	for _, want := range []string{"panel.load", "panel.save", "panel.partial"} {
		if !slices.Contains(actions, want) {
			t.Fatalf("expected action %s in %#v", want, actions)
		}
	}
}
