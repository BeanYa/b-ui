package service

import "context"

// ScatterGatherHandler is the interface each task type must implement.
type ScatterGatherHandler interface {
	TaskType() string
	ExecuteLocal(ctx context.Context, domainID string, params map[string]any) (any, error)
	Aggregate(ctx context.Context, results []NodeResult) (any, error)
}

// NodeResult represents a single node's response in a scatter-gather task.
type NodeResult struct {
	NodeID string
	Status string // "completed", "failed", "timeout"
	Result any
	Error  string
}

var scatterHandlers = map[string]ScatterGatherHandler{}

// RegisterScatterHandler registers a task type handler. Call during init.
func RegisterScatterHandler(h ScatterGatherHandler) {
	scatterHandlers[h.TaskType()] = h
}

// GetScatterHandler returns the registered handler for a task type.
func GetScatterHandler(taskType string) (ScatterGatherHandler, bool) {
	h, ok := scatterHandlers[taskType]
	return h, ok
}
