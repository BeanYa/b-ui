package router

import (
	"context"
	"testing"

	clustertypes "github.com/alireza0/b-ui/src/backend/internal/domain/services/cluster/types"
)

func TestHandleUnsupported(t *testing.T) {
	r := NewActionRouter()
	req := clustertypes.ActionRequest{Action: "unknown"}
	resp := r.Handle(req)

	if resp.Status != "unsupported" {
		t.Errorf("expected status unsupported, got %s", resp.Status)
	}
	if resp.Action != "unknown" {
		t.Errorf("expected action unknown, got %s", resp.Action)
	}
}

func TestHandleDispatchesToRegisteredHandler(t *testing.T) {
	r := NewActionRouter()
	r.Register("test_action", func(ctx context.Context, req clustertypes.ActionRequest) (clustertypes.ActionResponse, error) {
		return clustertypes.ActionResponse{
			Status: "success",
			Data:   "ok",
		}, nil
	})

	req := clustertypes.ActionRequest{Action: "test_action"}
	resp := r.Handle(req)

	if resp.Status != "success" {
		t.Errorf("expected status success, got %s", resp.Status)
	}
	if resp.Action != "test_action" {
		t.Errorf("expected action test_action, got %s", resp.Action)
	}
	if resp.Data != "ok" {
		t.Errorf("expected data ok, got %v", resp.Data)
	}
}

func TestHandleHandlerError(t *testing.T) {
	r := NewActionRouter()
	r.Register("fail_action", func(ctx context.Context, req clustertypes.ActionRequest) (clustertypes.ActionResponse, error) {
		return clustertypes.ActionResponse{}, HandlerError{Message: "something went wrong"}
	})

	req := clustertypes.ActionRequest{Action: "fail_action"}
	resp := r.Handle(req)

	if resp.Status != "error" {
		t.Errorf("expected status error, got %s", resp.Status)
	}
	if resp.Action != "fail_action" {
		t.Errorf("expected action fail_action, got %s", resp.Action)
	}
	if resp.ErrorMessage != "something went wrong" {
		t.Errorf("expected error message 'something went wrong', got %s", resp.ErrorMessage)
	}
}

func TestActionsReturnsSortedNames(t *testing.T) {
	r := NewActionRouter()
	r.Register("zebra", func(ctx context.Context, req clustertypes.ActionRequest) (clustertypes.ActionResponse, error) {
		return clustertypes.ActionResponse{}, nil
	})
	r.Register("alpha", func(ctx context.Context, req clustertypes.ActionRequest) (clustertypes.ActionResponse, error) {
		return clustertypes.ActionResponse{}, nil
	})
	r.Register("middle", func(ctx context.Context, req clustertypes.ActionRequest) (clustertypes.ActionResponse, error) {
		return clustertypes.ActionResponse{}, nil
	})

	actions := r.Actions()

	expected := []string{"alpha", "middle", "zebra"}
	if len(actions) != len(expected) {
		t.Fatalf("expected %d actions, got %d", len(expected), len(actions))
	}
	for i, name := range expected {
		if actions[i] != name {
			t.Errorf("expected actions[%d] = %s, got %s", i, name, actions[i])
		}
	}
}
