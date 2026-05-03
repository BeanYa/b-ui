package action

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/BeanYa/b-ui/src/backend/internal/domain/services/cluster/router"
	clustertypes "github.com/BeanYa/b-ui/src/backend/internal/domain/services/cluster/types"
)

type DomainCleanupService interface {
	HandleDomainCleanup(context.Context, clustertypes.ActionRequest, clustertypes.DomainCleanupPayload) (map[string]interface{}, error)
}

type DomainCleanupHandler struct {
	svc DomainCleanupService
}

func NewDomainCleanupHandler(svc DomainCleanupService) *DomainCleanupHandler {
	return &DomainCleanupHandler{svc: svc}
}

func (h *DomainCleanupHandler) RegisterAll(r *router.ActionRouter) {
	r.Register("domain.cleanup", h.Cleanup)
}

func (h *DomainCleanupHandler) Cleanup(ctx context.Context, req clustertypes.ActionRequest) (clustertypes.ActionResponse, error) {
	var payload clustertypes.DomainCleanupPayload
	if err := decodeDomainCleanupPayload(req.Payload, &payload); err != nil {
		return clustertypes.ActionResponse{}, router.HandlerError{Message: err.Error()}
	}
	data, err := h.svc.HandleDomainCleanup(ctx, req, payload)
	if err != nil {
		return clustertypes.ActionResponse{}, router.HandlerError{Message: err.Error()}
	}
	return clustertypes.ActionResponse{Status: "success", Action: req.Action, Data: data}, nil
}

func decodeDomainCleanupPayload(rawPayload interface{}, out interface{}) error {
	raw, err := json.Marshal(rawPayload)
	if err != nil {
		return fmt.Errorf("invalid payload: %v", err)
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("invalid payload: %v", err)
	}
	return nil
}
