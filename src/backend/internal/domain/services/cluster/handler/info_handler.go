package handler

import (
	"context"

	clustertypes "github.com/alireza0/s-ui/src/backend/internal/domain/services/cluster/types"
)

// NewInfoHandler returns an ActionHandler that responds with the given supported actions.
func NewInfoHandler(actions []string) clustertypes.ActionHandler {
	return func(ctx context.Context, req clustertypes.ActionRequest) (clustertypes.ActionResponse, error) {
		return clustertypes.ActionResponse{
			Status: "success",
			Data: clustertypes.InfoResponse{
				Actions: actions,
			},
		}, nil
	}
}
