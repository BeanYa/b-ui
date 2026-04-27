package action

import (
	"context"
	"encoding/json"
	"fmt"

	clustertypes "github.com/alireza0/b-ui/src/backend/internal/domain/services/cluster/types"
	"github.com/alireza0/b-ui/src/backend/internal/domain/services/cluster/router"
)

// ProxyCreatePayload is the payload for proxy.create action.
type ProxyCreatePayload struct {
	RequestID string            `json:"request_id"`
	TLS       json.RawMessage   `json:"tls"`
	Inbound   json.RawMessage   `json:"inbound"`
	Users     []json.RawMessage `json:"users"`
	Expiry    *string           `json:"expiry"`
}

// Service interfaces that the domain services will implement.
type inboundService interface {
	CreateInbound(inboundJSON json.RawMessage) (int, error)
	DeleteInbound(id int) error
	GetInbound(id int) (json.RawMessage, error)
}

type tlsService interface {
	CreateTLS(tlsJSON json.RawMessage) (int, error)
}

type userService interface {
	CreateUsers(inboundID int, usersJSON []json.RawMessage) error
	GenerateURIs(inboundID int) ([]string, error)
}

// ProxyHandler handles proxy.* actions.
type ProxyHandler struct {
	inbounds inboundService
	tls      tlsService
	users    userService
}

func NewProxyHandler(inbounds inboundService, tls tlsService, users userService) *ProxyHandler {
	return &ProxyHandler{inbounds: inbounds, tls: tls, users: users}
}

// Create handles proxy.create — creates TLS (if provided), inbound, users (if provided),
// and generates URIs.
func (h *ProxyHandler) Create(ctx context.Context, req clustertypes.ActionRequest) (clustertypes.ActionResponse, error) {
	payload, err := marshalUnmarshalPayload(req.Payload)
	if err != nil {
		return clustertypes.ActionResponse{}, router.HandlerError{Message: fmt.Sprintf("invalid payload: %v", err)}
	}

	var p ProxyCreatePayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return clustertypes.ActionResponse{}, router.HandlerError{Message: fmt.Sprintf("invalid payload: %v", err)}
	}

	if len(p.Inbound) == 0 {
		return clustertypes.ActionResponse{}, router.HandlerError{Message: "inbound is required"}
	}

	// Create TLS if provided.
	if len(p.TLS) > 0 {
		if _, err := h.tls.CreateTLS(p.TLS); err != nil {
			return clustertypes.ActionResponse{}, router.HandlerError{Message: fmt.Sprintf("tls creation failed: %v", err)}
		}
	}

	// Create inbound.
	inboundID, err := h.inbounds.CreateInbound(p.Inbound)
	if err != nil {
		return clustertypes.ActionResponse{}, router.HandlerError{Message: fmt.Sprintf("inbound creation failed: %v", err)}
	}

	// Create users if provided.
	if len(p.Users) > 0 {
		if err := h.users.CreateUsers(inboundID, p.Users); err != nil {
			return clustertypes.ActionResponse{}, router.HandlerError{Message: fmt.Sprintf("user creation failed: %v", err)}
		}
	}

	// Generate URIs.
	uris, err := h.users.GenerateURIs(inboundID)
	if err != nil {
		return clustertypes.ActionResponse{}, router.HandlerError{Message: fmt.Sprintf("uri generation failed: %v", err)}
	}

	data := map[string]interface{}{
		"inbound_id": inboundID,
		"uris":       uris,
	}
	if p.Expiry != nil {
		data["expiry"] = *p.Expiry
	}

	return clustertypes.ActionResponse{
		Status: "success",
		Data:   data,
	}, nil
}

// Read handles proxy.read — returns inbound data for a given inbound_id.
func (h *ProxyHandler) Read(ctx context.Context, req clustertypes.ActionRequest) (clustertypes.ActionResponse, error) {
	inboundID, err := getInboundID(req.Payload)
	if err != nil {
		return clustertypes.ActionResponse{}, router.HandlerError{Message: err.Error()}
	}

	inbound, err := h.inbounds.GetInbound(inboundID)
	if err != nil {
		return clustertypes.ActionResponse{}, router.HandlerError{Message: fmt.Sprintf("failed to get inbound: %v", err)}
	}

	return clustertypes.ActionResponse{
		Status: "success",
		Data: map[string]interface{}{
			"inbound_id": inboundID,
			"inbound":    json.RawMessage(inbound),
		},
	}, nil
}

// Delete handles proxy.delete — deletes an inbound by ID.
func (h *ProxyHandler) Delete(ctx context.Context, req clustertypes.ActionRequest) (clustertypes.ActionResponse, error) {
	inboundID, err := getInboundID(req.Payload)
	if err != nil {
		return clustertypes.ActionResponse{}, router.HandlerError{Message: err.Error()}
	}

	if err := h.inbounds.DeleteInbound(inboundID); err != nil {
		return clustertypes.ActionResponse{}, router.HandlerError{Message: fmt.Sprintf("failed to delete inbound: %v", err)}
	}

	return clustertypes.ActionResponse{
		Status: "success",
		Data: map[string]interface{}{
			"inbound_id": inboundID,
		},
	}, nil
}

// RegisterAll registers all proxy.* actions on the given router.
func (h *ProxyHandler) RegisterAll(r *router.ActionRouter) {
	r.Register("proxy.create", h.Create)
	r.Register("proxy.read", h.Read)
	r.Register("proxy.delete", h.Delete)
}

// marshalUnmarshalPayload converts a map[string]interface{} to json.RawMessage
// and back to ensure consistent typing.
func marshalUnmarshalPayload(payload map[string]interface{}) (json.RawMessage, error) {
	b, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return b, nil
}

// getInboundID extracts inbound_id from the payload map.
func getInboundID(payload map[string]interface{}) (int, error) {
	raw, ok := payload["inbound_id"]
	if !ok {
		return 0, fmt.Errorf("inbound_id is required")
	}

	switch v := raw.(type) {
	case float64:
		return int(v), nil
	case int:
		return v, nil
	case json.Number:
		i, err := v.Int64()
		if err != nil {
			return 0, fmt.Errorf("invalid inbound_id: %v", err)
		}
		return int(i), nil
	default:
		return 0, fmt.Errorf("invalid inbound_id type")
	}
}
