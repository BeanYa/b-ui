package handler

import (
	"context"
	"testing"

	clustertypes "github.com/BeanYa/b-ui/src/backend/internal/domain/services/cluster/types"
)

func TestInfoHandler_ReturnsSupportedActions(t *testing.T) {
	actions := []string{"proxy.create", "proxy.read", "proxy.delete"}
	h := NewInfoHandler(actions)
	resp, err := h(context.Background(), clustertypes.ActionRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Status != "success" {
		t.Fatalf("expected success, got %q", resp.Status)
	}
	data, ok := resp.Data.(clustertypes.InfoResponse)
	if !ok {
		t.Fatal("expected InfoResponse data")
	}
	if len(data.Actions) != 3 {
		t.Fatalf("expected 3 actions, got %d", len(data.Actions))
	}
}
