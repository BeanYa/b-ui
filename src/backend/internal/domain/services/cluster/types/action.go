package clustertypes

import "context"

// ActionRequest is the unified request body for /_cluster/v1/action.
type ActionRequest struct {
	SchemaVersion int                    `json:"schema_version"`
	SourceNodeID  string                 `json:"sourceNodeId"`
	Domain        string                 `json:"domain"`
	SentAt        int64                  `json:"sentAt"`
	Signature     string                 `json:"signature"`
	Action        string                 `json:"action"`
	Payload       map[string]interface{} `json:"payload"`
}

// ActionResponse is the unified response from action handlers.
type ActionResponse struct {
	Status       string      `json:"status"`       // "success" | "unsupported" | "error"
	Action       string      `json:"action"`
	ErrorMessage string      `json:"error_message,omitempty"`
	Data         interface{} `json:"data,omitempty"`
}

// ActionHandler processes a single action type.
type ActionHandler func(ctx context.Context, req ActionRequest) (ActionResponse, error)

// PaginationRequest is the common pagination payload for list actions.
type PaginationRequest struct {
	Page     int `json:"page"`
	PageSize int `json:"page_size"`
}

// PaginationResponse is the common pagination wrapper for list responses.
type PaginationResponse struct {
	Items    interface{} `json:"items"`
	Total    int64       `json:"total"`
	Page     int         `json:"page"`
	PageSize int         `json:"page_size"`
}

// InfoResponse is the response for GET /_cluster/v1/info.
type InfoResponse struct {
	Actions []string `json:"actions"`
}
