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
	r := router.NewActionRouter()

	// List actions
	r.Register("inbound.list", action.NewListHandler(inboundSvc))
	r.Register("client.list", action.NewListHandler(clientSvc))
	r.Register("tls.list", action.NewListHandler(tlsSvc))
	r.Register("service.list", action.NewListHandler(serviceSvc))
	r.Register("route.list", action.NewListHandler(routeSvc))
	r.Register("outbound.list", action.NewListHandler(outboundSvc))

	return &Runtime{Router: r}
}

// InfoResponse returns the info response with supported actions.
func (rt *Runtime) InfoResponse() clustertypes.InfoResponse {
	return clustertypes.InfoResponse{Actions: rt.Router.Actions()}
}
