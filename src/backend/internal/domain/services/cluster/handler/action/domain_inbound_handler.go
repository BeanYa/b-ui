package action

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/BeanYa/b-ui/src/backend/internal/domain/services/cluster/router"
	clustertypes "github.com/BeanYa/b-ui/src/backend/internal/domain/services/cluster/types"
)

type DomainInboundCreateService interface {
	HandleDomainInboundCreate(context.Context, clustertypes.ActionRequest, clustertypes.DomainInboundCreatePayload) (map[string]interface{}, error)
}

type DomainInboundHandler struct {
	svc DomainInboundCreateService
}

func NewDomainInboundHandler(svc DomainInboundCreateService) *DomainInboundHandler {
	return &DomainInboundHandler{svc: svc}
}

func (h *DomainInboundHandler) RegisterAll(r *router.ActionRouter) {
	r.Register("domain.inbound.create", h.Create)
}

func (h *DomainInboundHandler) Create(ctx context.Context, req clustertypes.ActionRequest) (clustertypes.ActionResponse, error) {
	var payload clustertypes.DomainInboundCreatePayload
	raw, err := json.Marshal(req.Payload)
	if err != nil {
		return clustertypes.ActionResponse{}, router.HandlerError{Message: fmt.Sprintf("invalid payload: %v", err)}
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return clustertypes.ActionResponse{}, router.HandlerError{Message: fmt.Sprintf("invalid payload: %v", err)}
	}
	data, err := h.svc.HandleDomainInboundCreate(ctx, req, payload)
	if err != nil {
		return clustertypes.ActionResponse{}, router.HandlerError{Message: err.Error()}
	}
	return clustertypes.ActionResponse{Status: "success", Action: req.Action, Data: data}, nil
}
