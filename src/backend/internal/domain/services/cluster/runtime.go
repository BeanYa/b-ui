package cluster

import (
	"github.com/BeanYa/b-ui/src/backend/internal/domain/services/cluster/handler/action"
	"github.com/BeanYa/b-ui/src/backend/internal/domain/services/cluster/router"
	clustertypes "github.com/BeanYa/b-ui/src/backend/internal/domain/services/cluster/types"
)

// Runtime wires together the action router and all handlers.
type Runtime struct {
	Router *router.ActionRouter
}

// RuntimeListServices groups list-capable services by cluster resource.
type RuntimeListServices struct {
	Inbound  action.ListService
	Client   action.ListService
	TLS      action.ListService
	Service  action.ListService
	Route    action.ListService
	Outbound action.ListService
}

// NewRuntime creates a Runtime with all handlers registered.
// Parameters: service interfaces for each handler category.
func NewRuntime(
	inboundSvc action.ListService,
	clientSvc action.ListService,
	tlsSvc action.ListService,
	serviceSvc action.ListService,
	routeSvc action.ListService,
	outboundSvc action.ListService,
) *Runtime {
	return NewRuntimeWithPanel(RuntimeListServices{
		Inbound:  inboundSvc,
		Client:   clientSvc,
		TLS:      tlsSvc,
		Service:  serviceSvc,
		Route:    routeSvc,
		Outbound: outboundSvc,
	}, nil)
}

// NewRuntimeWithPanel creates a Runtime with list handlers and optional
// panel-compatible management actions.
func NewRuntimeWithPanel(lists RuntimeListServices, panel action.PanelService) *Runtime {
	r := router.NewActionRouter()
	registerListActions(r, lists)

	if panel != nil {
		action.NewPanelHandler(panel).RegisterAll(r)
	}

	return &Runtime{Router: r}
}

func registerListActions(r *router.ActionRouter, lists RuntimeListServices) {
	if lists.Inbound != nil {
		r.Register("inbound.list", action.NewListHandler(lists.Inbound))
	}
	if lists.Client != nil {
		r.Register("client.list", action.NewListHandler(lists.Client))
	}
	if lists.TLS != nil {
		r.Register("tls.list", action.NewListHandler(lists.TLS))
	}
	if lists.Service != nil {
		r.Register("service.list", action.NewListHandler(lists.Service))
	}
	if lists.Route != nil {
		r.Register("route.list", action.NewListHandler(lists.Route))
	}
	if lists.Outbound != nil {
		r.Register("outbound.list", action.NewListHandler(lists.Outbound))
	}
}

// InfoResponse returns the info response with supported actions.
func (rt *Runtime) InfoResponse() clustertypes.InfoResponse {
	return clustertypes.InfoResponse{Actions: rt.Router.Actions()}
}
