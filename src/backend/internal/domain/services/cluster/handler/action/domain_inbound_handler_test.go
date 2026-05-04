package action

import (
	"context"
	"encoding/json"
	"testing"

	clustertypes "github.com/BeanYa/b-ui/src/backend/internal/domain/services/cluster/types"
)

type stubDomainInboundHandlerService struct {
	updatePayload clustertypes.DomainInboundUpdatePayload
	deletePayload clustertypes.DomainInboundDeletePayload
}

func (s *stubDomainInboundHandlerService) HandleDomainInboundCreate(context.Context, clustertypes.ActionRequest, clustertypes.DomainInboundCreatePayload) (map[string]interface{}, error) {
	return map[string]interface{}{"ok": true}, nil
}

func (s *stubDomainInboundHandlerService) HandleDomainInboundUpdate(_ context.Context, _ clustertypes.ActionRequest, payload clustertypes.DomainInboundUpdatePayload) (map[string]interface{}, error) {
	s.updatePayload = payload
	return map[string]interface{}{"request_id": payload.RequestID}, nil
}

func (s *stubDomainInboundHandlerService) HandleDomainInboundDelete(_ context.Context, _ clustertypes.ActionRequest, payload clustertypes.DomainInboundDeletePayload) (map[string]interface{}, error) {
	s.deletePayload = payload
	return map[string]interface{}{"request_id": payload.RequestID}, nil
}

func TestDomainInboundHandlerUpdateDecodesPayload(t *testing.T) {
	svc := &stubDomainInboundHandlerService{}
	h := NewDomainInboundHandler(svc)

	resp, err := h.Update(context.Background(), clustertypes.ActionRequest{
		Action: "domain.inbound.update",
		Payload: map[string]interface{}{
			"request_id": "req-update",
			"domain_id":  "edge.example.com",
			"group_id":   "group-1",
			"inbound": map[string]interface{}{
				"type": "vless",
				"tag":  "main",
			},
		},
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if resp.Status != "success" || svc.updatePayload.RequestID != "req-update" || svc.updatePayload.GroupID != "group-1" {
		t.Fatalf("unexpected update response=%#v payload=%#v", resp, svc.updatePayload)
	}
	var inbound map[string]interface{}
	if err := json.Unmarshal(svc.updatePayload.Inbound, &inbound); err != nil {
		t.Fatalf("decode inbound: %v", err)
	}
	if inbound["type"] != "vless" {
		t.Fatalf("expected inbound type vless, got %#v", inbound)
	}
}

func TestDomainInboundHandlerDeleteDecodesPayload(t *testing.T) {
	svc := &stubDomainInboundHandlerService{}
	h := NewDomainInboundHandler(svc)

	resp, err := h.Delete(context.Background(), clustertypes.ActionRequest{
		Action: "domain.inbound.delete",
		Payload: map[string]interface{}{
			"request_id": "req-delete",
			"domain_id":  "edge.example.com",
			"group_id":   "group-1",
			"target_members": []interface{}{
				map[string]interface{}{"member_id": "member-a", "node_id": "node-a", "display_name": "DE"},
			},
		},
	})
	if err != nil {
		t.Fatalf("delete: %v", err)
	}
	if resp.Status != "success" || svc.deletePayload.RequestID != "req-delete" || svc.deletePayload.DomainID != "edge.example.com" {
		t.Fatalf("unexpected delete response=%#v payload=%#v", resp, svc.deletePayload)
	}
	if len(svc.deletePayload.TargetMembers) != 1 || svc.deletePayload.TargetMembers[0].NodeID != "node-a" {
		t.Fatalf("unexpected target members: %#v", svc.deletePayload.TargetMembers)
	}
}
