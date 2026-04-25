package action

import (
	"context"
	"fmt"

	clustertypes "github.com/alireza0/s-ui/src/backend/internal/domain/services/cluster/types"
	"github.com/alireza0/s-ui/src/backend/internal/domain/services/cluster/router"
)

// ListService is the interface that domain services implement for listing.
type ListService interface {
	List(page, pageSize int) ([]map[string]interface{}, int64, error)
}

// NewListHandler creates an ActionHandler for a paginated list action.
// It extracts page and page_size from the payload (defaults: page=1, page_size=10),
// calls the ListService, and returns a PaginationResponse.
func NewListHandler(svc ListService) clustertypes.ActionHandler {
	return func(ctx context.Context, req clustertypes.ActionRequest) (clustertypes.ActionResponse, error) {
		page := 1
		pageSize := 10

		if req.Payload != nil {
			if v, ok := req.Payload["page"]; ok {
				switch val := v.(type) {
				case float64:
					page = int(val)
				case int:
					page = val
				}
			}
			if v, ok := req.Payload["page_size"]; ok {
				switch val := v.(type) {
				case float64:
					pageSize = int(val)
				case int:
					pageSize = val
				}
			}
		}

		items, total, err := svc.List(page, pageSize)
		if err != nil {
			return clustertypes.ActionResponse{}, router.HandlerError{Message: fmt.Sprintf("list failed: %v", err)}
		}

		return clustertypes.ActionResponse{
			Status: "success",
			Data: clustertypes.PaginationResponse{
				Items:    items,
				Total:    total,
				Page:     page,
				PageSize: pageSize,
			},
		}, nil
	}
}
