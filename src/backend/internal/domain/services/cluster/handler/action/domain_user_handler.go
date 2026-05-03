package action

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/BeanYa/b-ui/src/backend/internal/domain/services/cluster/router"
	clustertypes "github.com/BeanYa/b-ui/src/backend/internal/domain/services/cluster/types"
)

type DomainUserService interface {
	HandleDomainUserUpsert(context.Context, clustertypes.ActionRequest, clustertypes.DomainUserUpsertPayload) (map[string]interface{}, error)
	HandleDomainUserDelete(context.Context, clustertypes.ActionRequest, clustertypes.DomainUserDeletePayload) (map[string]interface{}, error)
}

type DomainUserHandler struct {
	svc DomainUserService
}

func NewDomainUserHandler(svc DomainUserService) *DomainUserHandler {
	return &DomainUserHandler{svc: svc}
}

func (h *DomainUserHandler) RegisterAll(r *router.ActionRouter) {
	r.Register("domain.user.upsert", h.Upsert)
	r.Register("domain.user.delete", h.Delete)
}

func (h *DomainUserHandler) Upsert(ctx context.Context, req clustertypes.ActionRequest) (clustertypes.ActionResponse, error) {
	var payload clustertypes.DomainUserUpsertPayload
	if err := decodeDomainUserPayload(req.Payload, &payload); err != nil {
		return clustertypes.ActionResponse{}, router.HandlerError{Message: err.Error()}
	}
	data, err := h.svc.HandleDomainUserUpsert(ctx, req, payload)
	if err != nil {
		return clustertypes.ActionResponse{}, router.HandlerError{Message: err.Error()}
	}
	return clustertypes.ActionResponse{Status: "success", Action: req.Action, Data: data}, nil
}

func (h *DomainUserHandler) Delete(ctx context.Context, req clustertypes.ActionRequest) (clustertypes.ActionResponse, error) {
	var payload clustertypes.DomainUserDeletePayload
	if err := decodeDomainUserPayload(req.Payload, &payload); err != nil {
		return clustertypes.ActionResponse{}, router.HandlerError{Message: err.Error()}
	}
	data, err := h.svc.HandleDomainUserDelete(ctx, req, payload)
	if err != nil {
		return clustertypes.ActionResponse{}, router.HandlerError{Message: err.Error()}
	}
	return clustertypes.ActionResponse{Status: "success", Action: req.Action, Data: data}, nil
}

func decodeDomainUserPayload(rawPayload interface{}, out interface{}) error {
	raw, err := json.Marshal(rawPayload)
	if err != nil {
		return fmt.Errorf("invalid payload: %v", err)
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("invalid payload: %v", err)
	}
	return nil
}
