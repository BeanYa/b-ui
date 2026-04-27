package router

import (
	"context"
	"sort"

	clustertypes "github.com/BeanYa/b-ui/src/backend/internal/domain/services/cluster/types"
)

// HandlerError represents a business-logic error returned by an action handler.
type HandlerError struct {
	Message string
}

func (e HandlerError) Error() string {
	return e.Message
}

// ActionRouter dispatches action requests to registered handlers.
type ActionRouter struct {
	handlers map[string]clustertypes.ActionHandler
}

// NewActionRouter creates a router with an empty handler map.
func NewActionRouter() *ActionRouter {
	return &ActionRouter{
		handlers: make(map[string]clustertypes.ActionHandler),
	}
}

// Register adds a handler for the given action name.
func (r *ActionRouter) Register(action string, handler clustertypes.ActionHandler) {
	r.handlers[action] = handler
}

// Handle looks up the handler for req.Action, dispatches the request, and
// returns the response. Returns an "unsupported" response for unknown actions,
// and an "error" response when the handler returns a HandlerError.
func (r *ActionRouter) Handle(req clustertypes.ActionRequest) clustertypes.ActionResponse {
	handler, ok := r.handlers[req.Action]
	if !ok {
		return clustertypes.ActionResponse{
			Status: "unsupported",
			Action: req.Action,
		}
	}

	resp, err := handler(context.Background(), req)
	if err != nil {
		var handlerErr HandlerError
		if AsHandlerError(err, &handlerErr) {
			return clustertypes.ActionResponse{
				Status:       "error",
				Action:       req.Action,
				ErrorMessage: handlerErr.Message,
			}
		}
		// Unexpected errors are still surfaced as error responses.
		return clustertypes.ActionResponse{
			Status:       "error",
			Action:       req.Action,
			ErrorMessage: err.Error(),
		}
	}

	resp.Action = req.Action
	return resp
}

// Actions returns a sorted list of registered action names.
func (r *ActionRouter) Actions() []string {
	actions := make([]string, 0, len(r.handlers))
	for action := range r.handlers {
		actions = append(actions, action)
	}
	sort.Strings(actions)
	return actions
}

// AsHandlerError checks if the given error is a HandlerError and extracts it.
func AsHandlerError(err error, target *HandlerError) bool {
	if he, ok := err.(HandlerError); ok {
		*target = he
		return true
	}
	return false
}
