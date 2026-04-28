package action

import (
	"context"
	"encoding/json"
	"testing"

	clustertypes "github.com/BeanYa/b-ui/src/backend/internal/domain/services/cluster/types"
)

type stubPanelService struct {
	loadLU       string
	loadHostname string
	loadData     map[string]interface{}

	partialObject   string
	partialID       string
	partialHostname string
	partialData     map[string]interface{}

	saveObject    string
	saveAction    string
	saveData      json.RawMessage
	saveInitUsers string
	saveHostname  string
	saveResult    map[string]interface{}

	keypairKind    string
	keypairOptions string

	linkConvertLink string

	checkOutboundTag  string
	checkOutboundLink string

	statsResource string
	statsTag      string
	statsLimit    int
}

func (s *stubPanelService) Load(lu string, hostname string) (map[string]interface{}, error) {
	s.loadLU = lu
	s.loadHostname = hostname
	if s.loadData != nil {
		return s.loadData, nil
	}
	return map[string]interface{}{"inbounds": []interface{}{}}, nil
}

func (s *stubPanelService) Partial(object string, id string, hostname string) (map[string]interface{}, error) {
	s.partialObject = object
	s.partialID = id
	s.partialHostname = hostname
	if s.partialData != nil {
		return s.partialData, nil
	}
	return map[string]interface{}{object: []interface{}{}}, nil
}

func (s *stubPanelService) Save(object string, act string, data json.RawMessage, initUsers string, hostname string) (map[string]interface{}, error) {
	s.saveObject = object
	s.saveAction = act
	s.saveData = append(json.RawMessage{}, data...)
	s.saveInitUsers = initUsers
	s.saveHostname = hostname
	if s.saveResult != nil {
		return s.saveResult, nil
	}
	return map[string]interface{}{"saved": true}, nil
}

func (s *stubPanelService) Keypairs(kind string, options string) ([]string, error) {
	s.keypairKind = kind
	s.keypairOptions = options
	return []string{"PrivateKey: abc"}, nil
}

func (s *stubPanelService) LinkConvert(link string) (interface{}, error) {
	s.linkConvertLink = link
	return map[string]interface{}{"tag": "converted"}, nil
}

func (s *stubPanelService) CheckOutbound(tag string, link string) (interface{}, error) {
	s.checkOutboundTag = tag
	s.checkOutboundLink = link
	return map[string]interface{}{"OK": true}, nil
}

func (s *stubPanelService) Stats(resource string, tag string, limit int) (interface{}, error) {
	s.statsResource = resource
	s.statsTag = tag
	s.statsLimit = limit
	return []interface{}{}, nil
}

func TestPanelHandlerLoadCallsService(t *testing.T) {
	svc := &stubPanelService{}
	h := NewPanelHandler(svc)

	resp, err := h.Load(context.Background(), clustertypes.ActionRequest{
		Action: "panel.load",
		Payload: map[string]interface{}{
			"lu":       "123",
			"hostname": "node.example.com",
		},
	})

	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if resp.Status != "success" {
		t.Fatalf("expected success, got %q", resp.Status)
	}
	if svc.loadLU != "123" || svc.loadHostname != "node.example.com" {
		t.Fatalf("unexpected load args: lu=%q hostname=%q", svc.loadLU, svc.loadHostname)
	}
}

func TestPanelHandlerLoadAcceptsNumericCursor(t *testing.T) {
	svc := &stubPanelService{}
	h := NewPanelHandler(svc)

	resp, err := h.Load(context.Background(), clustertypes.ActionRequest{
		Action: "panel.load",
		Payload: map[string]interface{}{
			"lu":       float64(123),
			"hostname": "node.example.com",
		},
	})

	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if resp.Status != "success" {
		t.Fatalf("expected success, got %q", resp.Status)
	}
	if svc.loadLU != "123" {
		t.Fatalf("expected numeric cursor to be normalized to 123, got %q", svc.loadLU)
	}
}

func TestPanelHandlerPartialAcceptsNumericID(t *testing.T) {
	svc := &stubPanelService{}
	h := NewPanelHandler(svc)

	resp, err := h.Partial(context.Background(), clustertypes.ActionRequest{
		Action: "panel.partial",
		Payload: map[string]interface{}{
			"object":   "clients",
			"id":       float64(7),
			"hostname": "node.example.com",
		},
	})

	if err != nil {
		t.Fatalf("partial: %v", err)
	}
	if resp.Status != "success" {
		t.Fatalf("expected success, got %q", resp.Status)
	}
	if svc.partialObject != "clients" || svc.partialID != "7" || svc.partialHostname != "node.example.com" {
		t.Fatalf("unexpected partial args: object=%q id=%q hostname=%q", svc.partialObject, svc.partialID, svc.partialHostname)
	}
}

func TestPanelHandlerSavePassesRawDataAndInitUsers(t *testing.T) {
	svc := &stubPanelService{}
	h := NewPanelHandler(svc)

	resp, err := h.Save(context.Background(), clustertypes.ActionRequest{
		Action: "panel.save",
		Payload: map[string]interface{}{
			"object":    "inbounds",
			"action":    "new",
			"data":      map[string]interface{}{"tag": "direct-10000"},
			"initUsers": []interface{}{float64(1), float64(2)},
			"hostname":  "node.example.com",
		},
	})

	if err != nil {
		t.Fatalf("save: %v", err)
	}
	if resp.Status != "success" {
		t.Fatalf("expected success, got %q", resp.Status)
	}
	if svc.saveObject != "inbounds" || svc.saveAction != "new" || svc.saveInitUsers != "1,2" {
		t.Fatalf("unexpected save args: object=%q action=%q initUsers=%q", svc.saveObject, svc.saveAction, svc.saveInitUsers)
	}
	if svc.saveHostname != "node.example.com" {
		t.Fatalf("unexpected hostname %q", svc.saveHostname)
	}
	if string(svc.saveData) != `{"tag":"direct-10000"}` {
		t.Fatalf("unexpected raw data %s", string(svc.saveData))
	}
}

func TestPanelHandlerSaveReturnsErrorForMissingObject(t *testing.T) {
	h := NewPanelHandler(&stubPanelService{})

	_, err := h.Save(context.Background(), clustertypes.ActionRequest{
		Action: "panel.save",
		Payload: map[string]interface{}{
			"action": "new",
			"data":   map[string]interface{}{},
		},
	})

	if err == nil {
		t.Fatal("expected validation error")
	}
}

func TestPanelHandlerUtilityActionsCallService(t *testing.T) {
	svc := &stubPanelService{}
	h := NewPanelHandler(svc)

	if _, err := h.Keypairs(context.Background(), clustertypes.ActionRequest{Payload: map[string]interface{}{"k": "reality", "o": "seed"}}); err != nil {
		t.Fatalf("keypairs: %v", err)
	}
	if svc.keypairKind != "reality" || svc.keypairOptions != "seed" {
		t.Fatalf("unexpected keypair args: %#v", svc)
	}

	if _, err := h.CheckOutbound(context.Background(), clustertypes.ActionRequest{Payload: map[string]interface{}{"tag": "proxy-a", "link": "https://example.com"}}); err != nil {
		t.Fatalf("check outbound: %v", err)
	}
	if svc.checkOutboundTag != "proxy-a" || svc.checkOutboundLink != "https://example.com" {
		t.Fatalf("unexpected check outbound args: %#v", svc)
	}

	if _, err := h.Stats(context.Background(), clustertypes.ActionRequest{Payload: map[string]interface{}{"resource": "user", "tag": "alice", "limit": float64(6)}}); err != nil {
		t.Fatalf("stats: %v", err)
	}
	if svc.statsResource != "user" || svc.statsTag != "alice" || svc.statsLimit != 6 {
		t.Fatalf("unexpected stats args: %#v", svc)
	}
}
