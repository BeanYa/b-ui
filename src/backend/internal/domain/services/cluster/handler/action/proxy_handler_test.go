package action

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	clustertypes "github.com/alireza0/b-ui/src/backend/internal/domain/services/cluster/types"
)

// --- Mock implementations ---

type mockInboundService struct {
	createFunc func(inboundJSON json.RawMessage) (int, error)
	deleteFunc func(id int) error
	getFunc    func(id int) (json.RawMessage, error)
}

func (m *mockInboundService) CreateInbound(inboundJSON json.RawMessage) (int, error) {
	return m.createFunc(inboundJSON)
}

func (m *mockInboundService) DeleteInbound(id int) error {
	return m.deleteFunc(id)
}

func (m *mockInboundService) GetInbound(id int) (json.RawMessage, error) {
	return m.getFunc(id)
}

type mockTLSService struct {
	createFunc func(tlsJSON json.RawMessage) (int, error)
}

func (m *mockTLSService) CreateTLS(tlsJSON json.RawMessage) (int, error) {
	return m.createFunc(tlsJSON)
}

type mockUserService struct {
	createUsersFunc  func(inboundID int, usersJSON []json.RawMessage) error
	generateURIsFunc func(inboundID int) ([]string, error)
}

func (m *mockUserService) CreateUsers(inboundID int, usersJSON []json.RawMessage) error {
	return m.createUsersFunc(inboundID, usersJSON)
}

func (m *mockUserService) GenerateURIs(inboundID int) ([]string, error) {
	return m.generateURIsFunc(inboundID)
}

// --- Tests ---

func TestProxyCreate_ReturnsErrorWhenInboundMissing(t *testing.T) {
	h := NewProxyHandler(
		&mockInboundService{},
		&mockTLSService{},
		&mockUserService{},
	)

	req := clustertypes.ActionRequest{
		Action:  "proxy.create",
		Payload: map[string]interface{}{},
	}

	_, err := h.Create(context.Background(), req)
	if err == nil {
		t.Fatal("expected error when inbound is missing, got nil")
	}
}

func TestProxyCreate_CreatesInboundAndReturnsURIs(t *testing.T) {
	inbounds := &mockInboundService{
		createFunc: func(inboundJSON json.RawMessage) (int, error) {
			return 1, nil
		},
	}
	users := &mockUserService{
		generateURIsFunc: func(inboundID int) ([]string, error) {
			if inboundID != 1 {
				t.Fatalf("expected inboundID 1, got %d", inboundID)
			}
			return []string{"vless://example.com"}, nil
		},
	}

	h := NewProxyHandler(inbounds, &mockTLSService{}, users)

	req := clustertypes.ActionRequest{
		Action: "proxy.create",
		Payload: map[string]interface{}{
			"inbound": map[string]interface{}{"tag": "test-inbound"},
		},
	}

	resp, err := h.Create(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Status != "success" {
		t.Fatalf("expected success, got %q", resp.Status)
	}

	data, ok := resp.Data.(map[string]interface{})
	if !ok {
		t.Fatal("expected map data")
	}

	if data["inbound_id"] != 1 {
		t.Fatalf("expected inbound_id 1, got %v", data["inbound_id"])
	}

	uris, ok := data["uris"].([]string)
	if !ok {
		t.Fatal("expected uris to be []string")
	}
	if len(uris) != 1 || uris[0] != "vless://example.com" {
		t.Fatalf("expected uris [vless://example.com], got %v", uris)
	}
}

func TestProxyCreate_CreatesTLSAndUsers(t *testing.T) {
	tlsCreated := false
	usersCreated := false

	tls := &mockTLSService{
		createFunc: func(tlsJSON json.RawMessage) (int, error) {
			tlsCreated = true
			return 10, nil
		},
	}
	inbounds := &mockInboundService{
		createFunc: func(inboundJSON json.RawMessage) (int, error) {
			return 1, nil
		},
	}
	users := &mockUserService{
		createUsersFunc: func(inboundID int, usersJSON []json.RawMessage) error {
			usersCreated = true
			if len(usersJSON) != 1 {
				t.Fatalf("expected 1 user, got %d", len(usersJSON))
			}
			return nil
		},
		generateURIsFunc: func(inboundID int) ([]string, error) {
			return []string{"vless://example.com"}, nil
		},
	}

	h := NewProxyHandler(inbounds, tls, users)

	req := clustertypes.ActionRequest{
		Action: "proxy.create",
		Payload: map[string]interface{}{
			"inbound": map[string]interface{}{"tag": "test"},
			"tls":     map[string]interface{}{"enabled": true},
			"users":   []interface{}{map[string]interface{}{"name": "user1"}},
		},
	}

	resp, err := h.Create(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Status != "success" {
		t.Fatalf("expected success, got %q", resp.Status)
	}
	if !tlsCreated {
		t.Fatal("expected TLS to be created")
	}
	if !usersCreated {
		t.Fatal("expected users to be created")
	}
}

func TestProxyRead_ReturnsInboundData(t *testing.T) {
	inboundData := json.RawMessage(`{"tag": "test-inbound", "port": 443}`)

	inbounds := &mockInboundService{
		getFunc: func(id int) (json.RawMessage, error) {
			if id != 5 {
				t.Fatalf("expected inbound_id 5, got %d", id)
			}
			return inboundData, nil
		},
	}

	h := NewProxyHandler(inbounds, &mockTLSService{}, &mockUserService{})

	req := clustertypes.ActionRequest{
		Action: "proxy.read",
		Payload: map[string]interface{}{
			"inbound_id": float64(5),
		},
	}

	resp, err := h.Read(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Status != "success" {
		t.Fatalf("expected success, got %q", resp.Status)
	}

	data, ok := resp.Data.(map[string]interface{})
	if !ok {
		t.Fatal("expected map data")
	}
	if data["inbound_id"] != 5 {
		t.Fatalf("expected inbound_id 5, got %v", data["inbound_id"])
	}
}

func TestProxyRead_ReturnsErrorWhenInboundIDMissing(t *testing.T) {
	h := NewProxyHandler(&mockInboundService{}, &mockTLSService{}, &mockUserService{})

	req := clustertypes.ActionRequest{
		Action:  "proxy.read",
		Payload: map[string]interface{}{},
	}

	_, err := h.Read(context.Background(), req)
	if err == nil {
		t.Fatal("expected error when inbound_id is missing, got nil")
	}
}

func TestProxyRead_ReturnsErrorWhenInboundNotFound(t *testing.T) {
	inbounds := &mockInboundService{
		getFunc: func(id int) (json.RawMessage, error) {
			return nil, errors.New("not found")
		},
	}

	h := NewProxyHandler(inbounds, &mockTLSService{}, &mockUserService{})

	req := clustertypes.ActionRequest{
		Action: "proxy.read",
		Payload: map[string]interface{}{
			"inbound_id": float64(99),
		},
	}

	_, err := h.Read(context.Background(), req)
	if err == nil {
		t.Fatal("expected error when inbound not found, got nil")
	}
}

func TestProxyDelete_DeletesInbound(t *testing.T) {
	deleted := false
	inbounds := &mockInboundService{
		deleteFunc: func(id int) error {
			if id != 3 {
				t.Fatalf("expected inbound_id 3, got %d", id)
			}
			deleted = true
			return nil
		},
	}

	h := NewProxyHandler(inbounds, &mockTLSService{}, &mockUserService{})

	req := clustertypes.ActionRequest{
		Action: "proxy.delete",
		Payload: map[string]interface{}{
			"inbound_id": float64(3),
		},
	}

	resp, err := h.Delete(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Status != "success" {
		t.Fatalf("expected success, got %q", resp.Status)
	}
	if !deleted {
		t.Fatal("expected inbound to be deleted")
	}

	data, ok := resp.Data.(map[string]interface{})
	if !ok {
		t.Fatal("expected map data")
	}
	if data["inbound_id"] != 3 {
		t.Fatalf("expected inbound_id 3, got %v", data["inbound_id"])
	}
}

func TestProxyDelete_ReturnsErrorWhenInboundIDMissing(t *testing.T) {
	h := NewProxyHandler(&mockInboundService{}, &mockTLSService{}, &mockUserService{})

	req := clustertypes.ActionRequest{
		Action:  "proxy.delete",
		Payload: map[string]interface{}{},
	}

	_, err := h.Delete(context.Background(), req)
	if err == nil {
		t.Fatal("expected error when inbound_id is missing, got nil")
	}
}
