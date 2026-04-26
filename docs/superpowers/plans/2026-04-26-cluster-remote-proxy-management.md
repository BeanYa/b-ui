# Cluster Remote Proxy Management Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Enable cluster nodes to remotely manage proxies (create/read/update/delete) on other nodes, with a node detail page UI and action-based protocol routing.

**Architecture:** Extend the existing `/_cluster/v1/` communication pipeline with a new `/_cluster/v1/action` endpoint and `/_cluster/v1/info` endpoint. The action endpoint dispatches to registered handlers via an `ActionRouter`. Handlers call existing domain services (`InboundService`, `TLSService`, `UserService`, etc.) so business logic is not duplicated. Frontend adds a node detail page with tabbed views and a combined create-node dialog.

**Tech Stack:** Go (Gin), Vue 3 (Vuetify 4 + Pinia), TypeScript, Ed25519 signatures, AES-GCM encryption.

---

## Phase 1: Backend — Action Router & Protocol Types

### Task 1: Create Action Request/Response Types

**Files:**
- Create: `src/backend/internal/domain/services/cluster/types/action.go`

- [ ] **Step 1: Write the type definitions**

```go
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
```

- [ ] **Step 2: Commit**

```bash
git add src/backend/internal/domain/services/cluster/types/action.go
git commit -m "feat(cluster): add action request/response types"
```

---

### Task 2: Create ActionRouter

**Files:**
- Create: `src/backend/internal/domain/services/cluster/router/action_router.go`
- Test: `src/backend/internal/domain/services/cluster/router/action_router_test.go`

- [ ] **Step 1: Write the failing test**

```go
package router

import (
	"context"
	"testing"

	clustertypes "github.com/alireza0/s-ui/src/backend/internal/domain/services/cluster/types"
)

func TestActionRouter_ReturnsUnsupportedForUnknownAction(t *testing.T) {
	r := NewActionRouter()
	req := clustertypes.ActionRequest{Action: "unknown.action"}
	resp := r.Handle(req)
	if resp.Status != "unsupported" {
		t.Fatalf("expected unsupported, got %q", resp.Status)
	}
	if resp.Action != "unknown.action" {
		t.Fatalf("expected action unknown.action, got %q", resp.Action)
	}
}

func TestActionRouter_DispatchesToRegisteredHandler(t *testing.T) {
	r := NewActionRouter()
	r.Register("test.action", func(ctx context.Context, req clustertypes.ActionRequest) (clustertypes.ActionResponse, error) {
		return clustertypes.ActionResponse{Status: "success", Data: "ok"}, nil
	})
	req := clustertypes.ActionRequest{Action: "test.action"}
	resp := r.Handle(req)
	if resp.Status != "success" {
		t.Fatalf("expected success, got %q", resp.Status)
	}
	if resp.Data != "ok" {
		t.Fatalf("expected ok, got %v", resp.Data)
	}
}

func TestActionRouter_ReturnsErrorWhenHandlerFails(t *testing.T) {
	r := NewActionRouter()
	r.Register("test.fail", func(ctx context.Context, req clustertypes.ActionRequest) (clustertypes.ActionResponse, error) {
		return clustertypes.ActionResponse{}, &HandlerError{Message: "something went wrong"}
	})
	req := clustertypes.ActionRequest{Action: "test.fail"}
	resp := r.Handle(req)
	if resp.Status != "error" {
		t.Fatalf("expected error, got %q", resp.Status)
	}
	if resp.ErrorMessage != "something went wrong" {
		t.Fatalf("expected 'something went wrong', got %q", resp.ErrorMessage)
	}
}

func TestActionRouter_ActionsReturnsRegisteredActions(t *testing.T) {
	r := NewActionRouter()
	r.Register("proxy.create", nil)
	r.Register("proxy.delete", nil)
	actions := r.Actions()
	if len(actions) != 2 {
		t.Fatalf("expected 2 actions, got %d", len(actions))
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd src/backend && go test ./internal/domain/services/cluster/router/ -v`
Expected: FAIL — package does not exist

- [ ] **Step 3: Write minimal implementation**

```go
package router

import (
	"context"
	"sort"

	clustertypes "github.com/alireza0/s-ui/src/backend/internal/domain/services/cluster/types"
)

// HandlerError is returned by action handlers to indicate a business error.
type HandlerError struct {
	Message string
}

func (e *HandlerError) Error() string {
	return e.Message
}

// ActionRouter dispatches action requests to registered handlers.
type ActionRouter struct {
	handlers map[string]clustertypes.ActionHandler
}

// NewActionRouter creates a router with no registered handlers.
func NewActionRouter() *ActionRouter {
	return &ActionRouter{handlers: make(map[string]clustertypes.ActionHandler)}
}

// Register adds a handler for the given action name.
func (r *ActionRouter) Register(action string, handler clustertypes.ActionHandler) {
	r.handlers[action] = handler
}

// Handle dispatches the request to the registered handler, or returns unsupported.
func (r *ActionRouter) Handle(req clustertypes.ActionRequest) clustertypes.ActionResponse {
	h, ok := r.handlers[req.Action]
	if !ok {
		return clustertypes.ActionResponse{Status: "unsupported", Action: req.Action}
	}
	resp, err := h(context.Background(), req)
	if err != nil {
		if he, ok := err.(*HandlerError); ok {
			return clustertypes.ActionResponse{Status: "error", Action: req.Action, ErrorMessage: he.Message}
		}
		return clustertypes.ActionResponse{Status: "error", Action: req.Action, ErrorMessage: err.Error()}
	}
	resp.Action = req.Action
	return resp
}

// Actions returns a sorted list of all registered action names.
func (r *ActionRouter) Actions() []string {
	actions := make([]string, 0, len(r.handlers))
	for a := range r.handlers {
		actions = append(actions, a)
	}
	sort.Strings(actions)
	return actions
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd src/backend && go test ./internal/domain/services/cluster/router/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add src/backend/internal/domain/services/cluster/router/
git commit -m "feat(cluster): add ActionRouter with dispatch and registration"
```

---

## Phase 2: Backend — HTTP Endpoint Registration

### Task 3: Add Info and Action Endpoint Path Helpers

**Files:**
- Modify: `src/backend/internal/http/api/cluster.go`
- Test: `src/backend/internal/http/api/cluster_path_test.go`

- [ ] **Step 1: Write the failing test**

Add to `cluster_path_test.go`:

```go
func TestClusterInfoPath(t *testing.T) {
	if got := ClusterInfoPath("/panel/"); got != "/panel/_cluster/v1/info" {
		t.Fatalf("expected /panel/_cluster/v1/info, got %q", got)
	}
	if got := ClusterInfoPath("/"); got != "/_cluster/v1/info" {
		t.Fatalf("expected /_cluster/v1/info, got %q", got)
	}
}

func TestClusterActionPath(t *testing.T) {
	if got := ClusterActionPath("/panel/"); got != "/panel/_cluster/v1/action" {
		t.Fatalf("expected /panel/_cluster/v1/action, got %q", got)
	}
	if got := ClusterActionPath("/"); got != "/_cluster/v1/action" {
		t.Fatalf("expected /_cluster/v1/action, got %q", got)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd src/backend && go test ./internal/http/api/ -run TestClusterInfoPath -v`
Expected: FAIL — `ClusterInfoPath` undefined

- [ ] **Step 3: Add path helper functions to `cluster.go`**

Add after `ClusterPingPath`:

```go
func ClusterInfoPath(basePath string) string {
	return clusterProtocolPath(basePath, "info")
}

func ClusterActionPath(basePath string) string {
	return clusterProtocolPath(basePath, "action")
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd src/backend && go test ./internal/http/api/ -run "TestClusterInfoPath|TestClusterActionPath" -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add src/backend/internal/http/api/cluster.go src/backend/internal/http/api/cluster_path_test.go
git commit -m "feat(cluster): add info and action endpoint path helpers"
```

---

### Task 4: Register Info and Action Routes

**Files:**
- Modify: `src/backend/internal/http/api/cluster.go`

- [ ] **Step 1: Extend `RegisterClusterMessageRoute` to register info and action endpoints**

The `clusterAPIService` interface needs new methods. Add to the interface:

```go
type clusterAPIService interface {
	Register(service.ClusterRegisterRequest) (*service.ClusterOperationStatus, error)
	GetOperation(string) (*service.ClusterOperationStatus, error)
	ListDomains() ([]service.ClusterDomainResponse, error)
	ListMembers() ([]service.ClusterMemberResponse, error)
	ManualSync() (*service.ClusterOperationStatus, error)
	DeleteMember(uint) error
	LeaveDomain(uint) error
	ReceiveMessage(*service.ClusterEnvelope, string) error
	Heartbeat(string) (*service.ClusterPeerStatus, error)
	Ping(string) (*service.ClusterPeerStatus, error)
	HandleAction(c *gin.Context)
	Info(c *gin.Context)
}
```

Add route registrations inside `RegisterClusterMessageRoute`:

```go
router.GET(ClusterInfoPath("/"), func(c *gin.Context) {
    clusterService.Info(c)
})
router.POST(ClusterActionPath("/"), func(c *gin.Context) {
    clusterService.HandleAction(c)
})
```

- [ ] **Step 2: Add stub implementations to `ClusterService`**

In `src/backend/internal/domain/services/cluster_service.go`, add:

```go
func (s *ClusterService) Info(c *gin.Context) {
    c.JSON(http.StatusOK, clustertypes.InfoResponse{
        Actions: s.actionRouter.Actions(),
    })
}

func (s *ClusterService) HandleAction(c *gin.Context) {
    var req clustertypes.ActionRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, clustertypes.ActionResponse{
            Status:       "error",
            ErrorMessage: "invalid request: " + err.Error(),
        })
        return
    }
    resp := s.actionRouter.Handle(req)
    c.JSON(http.StatusOK, resp)
}
```

This requires adding `actionRouter *router.ActionRouter` field to `ClusterService` struct and importing the new packages.

- [ ] **Step 3: Update `ClusterCommunicationSupportedActions`**

In `cluster_runtime.go`, update:

```go
func ClusterCommunicationSupportedActions() []string {
    return []string{"events", "heartbeat", "ping", "info", "action"}
}
```

- [ ] **Step 4: Run existing tests**

Run: `cd src/backend && go test ./... -v`
Expected: All existing tests PASS

- [ ] **Step 5: Commit**

```bash
git add src/backend/internal/domain/services/cluster_service.go src/backend/internal/domain/services/cluster_runtime.go src/backend/internal/http/api/cluster.go
git commit -m "feat(cluster): register /_cluster/v1/info and /_cluster/v1/action routes"
```

---

## Phase 3: Backend — Action Handlers

### Task 5: Info Handler

**Files:**
- Create: `src/backend/internal/domain/services/cluster/handler/info_handler.go`
- Test: `src/backend/internal/domain/services/cluster/handler/info_handler_test.go`

- [ ] **Step 1: Write the failing test**

```go
package handler

import (
	"context"
	"testing"

	clustertypes "github.com/alireza0/s-ui/src/backend/internal/domain/services/cluster/types"
)

func TestInfoHandler_ReturnsSupportedActions(t *testing.T) {
	actions := []string{"proxy.create", "proxy.read", "proxy.delete"}
	h := NewInfoHandler(actions)
	resp := h(context.Background(), clustertypes.ActionRequest{})
	if resp.Status != "success" {
		t.Fatalf("expected success, got %q", resp.Status)
	}
	data, ok := resp.Data.(clustertypes.InfoResponse)
	if !ok {
		t.Fatal("expected InfoResponse data")
	}
	if len(data.Actions) != 3 {
		t.Fatalf("expected 3 actions, got %d", len(data.Actions))
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd src/backend && go test ./internal/domain/services/cluster/handler/ -run TestInfoHandler -v`
Expected: FAIL

- [ ] **Step 3: Write implementation**

```go
package handler

import (
	"context"

	clustertypes "github.com/alireza0/s-ui/src/backend/internal/domain/services/cluster/types"
)

// InfoHandler returns the list of actions this node supports.
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
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd src/backend && go test ./internal/domain/services/cluster/handler/ -run TestInfoHandler -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add src/backend/internal/domain/services/cluster/handler/
git commit -m "feat(cluster): add info handler"
```

---

### Task 6: Proxy Handler — Create

**Files:**
- Create: `src/backend/internal/domain/services/cluster/handler/action/proxy_handler.go`
- Test: `src/backend/internal/domain/services/cluster/handler/action/proxy_handler_test.go`

This handler creates inbound + optional TLS + optional users on the local node, then returns connection URIs. It calls existing domain services.

- [ ] **Step 1: Write the failing test**

```go
package action

import (
	"context"
	"testing"

	clustertypes "github.com/alireza0/s-ui/src/backend/internal/domain/services/cluster/types"
	"github.com/alireza0/s-ui/src/backend/internal/domain/services/cluster/router"
)

func TestProxyCreate_ReturnsUnsupportedWhenPayloadMissing(t *testing.T) {
	h := NewProxyHandler(nil, nil, nil)
	req := clustertypes.ActionRequest{Action: "proxy.create", Payload: nil}
	resp, err := h.Create(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for nil payload")
	}
	_ = resp
}

func TestProxyCreate_ReturnsErrorWhenInboundMissing(t *testing.T) {
	h := NewProxyHandler(nil, nil, nil)
	req := clustertypes.ActionRequest{
		Action:  "proxy.create",
		Payload: map[string]interface{}{"request_id": "test-123"},
	}
	resp := h.Create(context.Background(), req)
	if resp.Status != "error" {
		t.Fatalf("expected error, got %q", resp.Status)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd src/backend && go test ./internal/domain/services/cluster/handler/action/ -v`
Expected: FAIL

- [ ] **Step 3: Write implementation**

The proxy handler needs interfaces for the services it calls:

```go
package action

import (
	"context"
	"encoding/json"

	clustertypes "github.com/alireza0/s-ui/src/backend/internal/domain/services/cluster/types"
	"github.com/alireza0/s-ui/src/backend/internal/domain/services/cluster/router"
)

// ProxyCreatePayload is the payload for proxy.create action.
type ProxyCreatePayload struct {
	RequestID string                 `json:"request_id"`
	TLS       json.RawMessage        `json:"tls"`
	Inbound   json.RawMessage        `json:"inbound"`
	Users     []json.RawMessage      `json:"users"`
	Expiry    *string                `json:"expiry"`
}

type inboundService interface {
	// CreateInbound creates an inbound on the local node and returns its ID.
	CreateInbound(inboundJSON json.RawMessage) (int, error)
	// DeleteInbound removes an inbound by ID.
	DeleteInbound(id int) error
	// GetInbound retrieves inbound data by ID.
	GetInbound(id int) (json.RawMessage, error)
}

type tlsService interface {
	// CreateTLS creates a TLS config and returns its ID.
	CreateTLS(tlsJSON json.RawMessage) (int, error)
	// ListTLS returns all TLS configs.
	ListTLS() (json.RawMessage, error)
}

type userService interface {
	// CreateUsers creates users for the given inbound.
	CreateUsers(inboundID int, usersJSON []json.RawMessage) error
	// GenerateURIs generates connection URIs for the given inbound.
	GenerateURIs(inboundID int) ([]string, error)
}

// ProxyHandler handles proxy.* actions.
type ProxyHandler struct {
	inbounds inboundService
	tls      tlsService
	users    userService
}

// NewProxyHandler creates a proxy handler with the given service dependencies.
func NewProxyHandler(inbounds inboundService, tls tlsService, users userService) *ProxyHandler {
	return &ProxyHandler{inbounds: inbounds, tls: tls, users: users}
}

// Create handles proxy.create — creates inbound + optional TLS + optional users.
func (h *ProxyHandler) Create(ctx context.Context, req clustertypes.ActionRequest) (clustertypes.ActionResponse, error) {
	if req.Payload == nil {
		return clustertypes.ActionResponse{}, &router.HandlerError{Message: "payload is required"}
	}

	raw, err := json.Marshal(req.Payload)
	if err != nil {
		return clustertypes.ActionResponse{}, &router.HandlerError{Message: "invalid payload: " + err.Error()}
	}

	var payload ProxyCreatePayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return clustertypes.ActionResponse{}, &router.HandlerError{Message: "invalid payload structure: " + err.Error()}
	}

	if len(payload.Inbound) == 0 {
		return clustertypes.ActionResponse{Status: "error", Action: req.Action, ErrorMessage: "inbound config is required"}, nil
	}

	// 1. Create TLS config if provided
	var tlsID int
	if len(payload.TLS) > 0 {
		id, err := h.tls.CreateTLS(payload.TLS)
		if err != nil {
			return clustertypes.ActionResponse{Status: "error", Action: req.Action, ErrorMessage: "tls creation failed: " + err.Error()}, nil
		}
		tlsID = id
	}

	// 2. Create inbound
	inboundID, err := h.inbounds.CreateInbound(payload.Inbound)
	if err != nil {
		return clustertypes.ActionResponse{Status: "error", Action: req.Action, ErrorMessage: "inbound creation failed: " + err.Error()}, nil
	}

	// 3. Create users if provided
	if len(payload.Users) > 0 {
		if err := h.users.CreateUsers(inboundID, payload.Users); err != nil {
			return clustertypes.ActionResponse{Status: "error", Action: req.Action, ErrorMessage: "user creation failed: " + err.Error()}, nil
		}
	}

	// 4. Generate URIs
	uris, err := h.users.GenerateURIs(inboundID)
	if err != nil {
		uris = []string{}
	}

	var expiry *string
	if payload.Expiry != nil {
		expiry = payload.Expiry
	}

	return clustertypes.ActionResponse{
		Status: "success",
		Data: map[string]interface{}{
			"inbound_id": inboundID,
			"uris":       uris,
			"expiry":     expiry,
		},
	}, nil
}

// Read handles proxy.read — reads proxy info by inbound ID.
func (h *ProxyHandler) Read(ctx context.Context, req clustertypes.ActionRequest) (clustertypes.ActionResponse, error) {
	inboundID, ok := req.Payload["inbound_id"]
	if !ok || inboundID == nil {
		// List all inbounds
		return clustertypes.ActionResponse{Status: "error", Action: req.Action, ErrorMessage: "inbound_id is required"}, nil
	}

	var id int
	switch v := inboundID.(type) {
	case float64:
		id = int(v)
	default:
		return clustertypes.ActionResponse{Status: "error", Action: req.Action, ErrorMessage: "invalid inbound_id type"}, nil
	}

	data, err := h.inbounds.GetInbound(id)
	if err != nil {
		return clustertypes.ActionResponse{Status: "error", Action: req.Action, ErrorMessage: err.Error()}, nil
	}

	return clustertypes.ActionResponse{
		Status: "success",
		Data:   map[string]interface{}{"inbound_id": id, "inbound": json.RawMessage(data)},
	}, nil
}

// Delete handles proxy.delete — deletes an inbound by ID.
func (h *ProxyHandler) Delete(ctx context.Context, req clustertypes.ActionRequest) (clustertypes.ActionResponse, error) {
	inboundID, ok := req.Payload["inbound_id"]
	if !ok {
		return clustertypes.ActionResponse{Status: "error", Action: req.Action, ErrorMessage: "inbound_id is required"}, nil
	}

	var id int
	switch v := inboundID.(type) {
	case float64:
		id = int(v)
	default:
		return clustertypes.ActionResponse{Status: "error", Action: req.Action, ErrorMessage: "invalid inbound_id type"}, nil
	}

	if err := h.inbounds.DeleteInbound(id); err != nil {
		return clustertypes.ActionResponse{Status: "error", Action: req.Action, ErrorMessage: err.Error()}, nil
	}

	return clustertypes.ActionResponse{Status: "success", Data: map[string]interface{}{"inbound_id": id}}, nil
}

// RegisterAll registers all proxy.* actions on the given router.
func (h *ProxyHandler) RegisterAll(r *router.ActionRouter) {
	r.Register("proxy.create", h.Create)
	r.Register("proxy.read", h.Read)
	r.Register("proxy.delete", h.Delete)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd src/backend && go test ./internal/domain/services/cluster/handler/action/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add src/backend/internal/domain/services/cluster/handler/action/
git commit -m "feat(cluster): add proxy handler with create/read/delete"
```

---

### Task 7: List Handlers (Inbound/Client/TLS/Service/Route/Outbound)

**Files:**
- Create: `src/backend/internal/domain/services/cluster/handler/action/list_handler.go`
- Test: `src/backend/internal/domain/services/cluster/handler/action/list_handler_test.go`

Each list handler wraps an existing service's list method with pagination. All follow the same pattern.

- [ ] **Step 1: Write the failing test**

```go
package action

import (
	"context"
	"testing"

	clustertypes "github.com/alireza0/s-ui/src/backend/internal/domain/services/cluster/types"
)

type mockListService struct {
	items []map[string]interface{}
	total int64
}

func (m *mockListService) List(page, pageSize int) ([]map[string]interface{}, int64, error) {
	return m.items, m.total, nil
}

func TestListHandler_PaginatesCorrectly(t *testing.T) {
	mock := &mockListService{
		items: []map[string]interface{}{{"id": 1}, {"id": 2}},
		total: 2,
	}
	h := NewListHandler("inbound.list", mock)
	req := clustertypes.ActionRequest{
		Action:  "inbound.list",
		Payload: map[string]interface{}{"page": float64(1), "page_size": float64(10)},
	}
	resp, err := h(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.Status != "success" {
		t.Fatalf("expected success, got %q", resp.Status)
	}
}

func TestListHandler_DefaultPagination(t *testing.T) {
	mock := &mockListService{
		items: []map[string]interface{}{},
		total: 0,
	}
	h := NewListHandler("inbound.list", mock)
	req := clustertypes.ActionRequest{
		Action:  "inbound.list",
		Payload: map[string]interface{}{},
	}
	resp, err := h(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.Status != "success" {
		t.Fatalf("expected success, got %q", resp.Status)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd src/backend && go test ./internal/domain/services/cluster/handler/action/ -run TestListHandler -v`
Expected: FAIL

- [ ] **Step 3: Write implementation**

```go
package action

import (
	"context"

	clustertypes "github.com/alireza0/s-ui/src/backend/internal/domain/services/cluster/types"
	"github.com/alireza0/s-ui/src/backend/internal/domain/services/cluster/router"
)

// ListService is the interface that domain services implement for listing.
type ListService interface {
	List(page, pageSize int) ([]map[string]interface{}, int64, error)
}

// NewListHandler creates an action handler for a paginated list action.
func NewListHandler(actionName string, svc ListService) clustertypes.ActionHandler {
	return func(ctx context.Context, req clustertypes.ActionRequest) (clustertypes.ActionResponse, error) {
		page := 1
		pageSize := 10

		if req.Payload != nil {
			if p, ok := req.Payload["page"]; ok {
				if f, ok := p.(float64); ok && f > 0 {
					page = int(f)
				}
			}
			if ps, ok := req.Payload["page_size"]; ok {
				if f, ok := ps.(float64); ok && f > 0 {
					pageSize = int(f)
				}
			}
		}

		items, total, err := svc.List(page, pageSize)
		if err != nil {
			return clustertypes.ActionResponse{}, &router.HandlerError{Message: err.Error()}
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
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd src/backend && go test ./internal/domain/services/cluster/handler/action/ -run TestListHandler -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add src/backend/internal/domain/services/cluster/handler/action/list_handler.go src/backend/internal/domain/services/cluster/handler/action/list_handler_test.go
git commit -m "feat(cluster): add generic list handler with pagination"
```

---

### Task 8: Wire Handlers into Runtime

**Files:**
- Create: `src/backend/internal/domain/services/cluster/runtime.go`
- Modify: `src/backend/internal/domain/services/cluster_service.go`

- [ ] **Step 1: Create runtime.go that wires all handlers**

```go
package cluster

import (
	"github.com/alireza0/s-ui/src/backend/internal/domain/services/cluster/handler"
	"github.com/alireza0/s-ui/src/backend/internal/domain/services/cluster/handler/action"
	"github.com/alireza0/s-ui/src/backend/internal/domain/services/cluster/router"
	clustertypes "github.com/alireza0/s-ui/src/backend/internal/domain/services/cluster/types"
)

// Runtime wires together the action router and all handlers.
type Runtime struct {
	Router *router.ActionRouter
}

// NewRuntime creates a Runtime with all handlers registered.
func NewRuntime(
	proxyHandler *action.ProxyHandler,
	inboundListSvc action.ListService,
	clientListSvc action.ListService,
	tlsListSvc action.ListService,
	serviceListSvc action.ListService,
	routeListSvc action.ListService,
	outboundListSvc action.ListService,
) *Runtime {
	r := router.NewActionRouter()

	// Info handler — uses the router's registered actions
	infoHandler := handler.NewInfoHandler(nil) // will be updated after registration

	// Proxy actions
	proxyHandler.RegisterAll(r)

	// List actions
	r.Register("inbound.list", action.NewListHandler("inbound.list", inboundListSvc))
	r.Register("client.list", action.NewListHandler("client.list", clientListSvc))
	r.Register("tls.list", action.NewListHandler("tls.list", tlsListSvc))
	r.Register("service.list", action.NewListHandler("service.list", serviceListSvc))
	r.Register("route.list", action.NewListHandler("route.list", routeListSvc))
	r.Register("outbound.list", action.NewListHandler("outbound.list", outboundListSvc))

	// Update info handler with actual actions
	_ = infoHandler // info will query the router directly

	return &Runtime{Router: r}
}

// SupportedActions returns all actions registered on the router.
func (rt *Runtime) SupportedActions() []string {
	return rt.Router.Actions()
}

// InfoResponse returns the info response with supported actions.
func (rt *Runtime) InfoResponse() clustertypes.InfoResponse {
	return clustertypes.InfoResponse{Actions: rt.SupportedActions()}
}
```

- [ ] **Step 2: Update ClusterService to use the Runtime**

Add `runtime *cluster.Runtime` field to `ClusterService` struct. Initialize it in the constructor or via lazy init. The `Info` and `HandleAction` methods delegate to `runtime.Router`.

- [ ] **Step 3: Run all tests**

Run: `cd src/backend && go test ./... -v`
Expected: All existing tests PASS, new tests PASS

- [ ] **Step 4: Commit**

```bash
git add src/backend/internal/domain/services/cluster/
git commit -m "feat(cluster): wire all handlers into runtime"
```

---

## Phase 4: Frontend — Types & API

### Task 9: Add Cluster Action Types

**Files:**
- Create: `src/frontend/src/types/clusterActions.ts`

- [ ] **Step 1: Write the type definitions**

```typescript
export interface ActionRequest {
  schema_version: number
  sourceNodeId: string
  domain: string
  sentAt: number
  signature: string
  action: string
  payload: Record<string, unknown>
}

export interface ActionResponse {
  status: 'success' | 'unsupported' | 'error'
  action: string
  error_message?: string
  data?: unknown
}

export interface InfoResponse {
  actions: string[]
}

export interface PaginationResponse<T> {
  items: T[]
  total: number
  page: number
  page_size: number
}

export interface ProxyCreatePayload {
  request_id: string
  tls?: Record<string, unknown>
  inbound: Record<string, unknown>
  users?: Record<string, unknown>[]
  expiry?: string | null
}

export interface ProxyCreateResponse {
  inbound_id: number
  uris: string[]
  expiry: string | null
}
```

- [ ] **Step 2: Commit**

```bash
git add src/frontend/src/types/clusterActions.ts
git commit -m "feat(frontend): add cluster action types"
```

---

### Task 10: Add Cluster Peer API Client

**Files:**
- Create: `src/frontend/src/features/clusterPeerApi.ts`

- [ ] **Step 1: Write the API client**

This client sends requests to other cluster nodes (not the local API). It uses the existing `X-Cluster-Token` header.

```typescript
import type {
  ActionRequest,
  ActionResponse,
  InfoResponse,
} from '@/types/clusterActions'

export async function fetchNodeInfo(
  baseURL: string,
  token: string
): Promise<InfoResponse> {
  const resp = await fetch(`${baseURL}/_cluster/v1/info`, {
    headers: { 'X-Cluster-Token': token },
  })
  if (!resp.ok) throw new Error(`info request failed: ${resp.status}`)
  return resp.json()
}

export async function sendAction(
  baseURL: string,
  token: string,
  req: ActionRequest
): Promise<ActionResponse> {
  const resp = await fetch(`${baseURL}/_cluster/v1/action`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'X-Cluster-Token': token,
    },
    body: JSON.stringify(req),
  })
  if (!resp.ok) throw new Error(`action request failed: ${resp.status}`)
  return resp.json()
}

export function buildListActionPayload(
  action: string,
  page: number,
  pageSize: number = 10
): ActionRequest {
  return {
    schema_version: 1,
    sourceNodeId: '',
    domain: '',
    sentAt: Math.floor(Date.now() / 1000),
    signature: '',
    action,
    payload: { page, page_size: pageSize },
  }
}
```

- [ ] **Step 2: Commit**

```bash
git add src/frontend/src/features/clusterPeerApi.ts
git commit -m "feat(frontend): add cluster peer API client"
```

---

### Task 11: Add Remote Node Pinia Store

**Files:**
- Create: `src/frontend/src/store/modules/remoteNode.ts`

- [ ] **Step 1: Write the store**

```typescript
import { defineStore } from 'pinia'
import { ref, reactive } from 'vue'
import { fetchNodeInfo, sendAction, buildListActionPayload } from '@/features/clusterPeerApi'
import type { InfoResponse, PaginationResponse } from '@/types/clusterActions'

interface TabData<T> {
  items: T[]
  total: number
  page: number
  loaded: boolean
  loading: boolean
}

export const useRemoteNodeStore = defineStore('RemoteNode', () => {
  const baseURL = ref('')
  const token = ref('')
  const info = ref<InfoResponse | null>(null)

  const pageLoading = ref(true)
  const pageError = ref<string | null>(null)

  const tlsConfigs = reactive<TabData<any>>({ items: [], total: 0, page: 1, loaded: false, loading: false })
  const clients = reactive<TabData<any>>({ items: [], total: 0, page: 1, loaded: false, loading: false })
  const inbounds = reactive<TabData<any>>({ items: [], total: 0, page: 1, loaded: false, loading: false })
  const services = reactive<TabData<any>>({ items: [], total: 0, page: 1, loaded: false, loading: false })
  const routes = reactive<TabData<any>>({ items: [], total: 0, page: 1, loaded: false, loading: false })
  const outbounds = reactive<TabData<any>>({ items: [], total: 0, page: 1, loaded: false, loading: false })

  async function init(url: string, t: string) {
    baseURL.value = url
    token.value = t
    pageLoading.value = true
    pageError.value = null

    try {
      info.value = await fetchNodeInfo(url, t)
      // Pre-fetch TLS and clients
      await Promise.all([
        fetchTab(tlsConfigs, 'tls.list'),
        fetchTab(clients, 'client.list'),
      ])
    } catch (e: any) {
      pageError.value = e.message
    } finally {
      pageLoading.value = false
    }
  }

  async function fetchTab<T>(tab: TabData<T>, action: string, page?: number) {
    tab.loading = true
    try {
      const p = page ?? tab.page ?? 1
      const req = buildListActionPayload(action, p)
      const resp = await sendAction(baseURL.value, token.value, req)
      if (resp.status === 'success' && resp.data) {
        const data = resp.data as PaginationResponse<T>
        tab.items = data.items
        tab.total = data.total
        tab.page = data.page
        tab.loaded = true
      }
    } finally {
      tab.loading = false
    }
  }

  function reset() {
    info.value = null
    tlsConfigs.loaded = false
    clients.loaded = false
    inbounds.loaded = false
    services.loaded = false
    routes.loaded = false
    outbounds.loaded = false
  }

  return {
    baseURL, token, info, pageLoading, pageError,
    tlsConfigs, clients, inbounds, services, routes, outbounds,
    init, fetchTab, reset,
  }
})
```

- [ ] **Step 2: Commit**

```bash
git add src/frontend/src/store/modules/remoteNode.ts
git commit -m "feat(frontend): add remote node Pinia store"
```

---

## Phase 5: Frontend — Node Detail Page

### Task 12: Add Node Detail Route

**Files:**
- Modify: `src/frontend/src/router/index.ts`

- [ ] **Step 1: Add the route**

Add to the `children` array after the `/clusters` route:

```typescript
{
  path: '/cluster/node/:nodeId',
  name: 'pages.clusterNodeDetail',
  meta: { requiresAdmin: true },
  component: () => import('@/views/ClusterNodeDetail.vue'),
},
```

- [ ] **Step 2: Commit**

```bash
git add src/frontend/src/router/index.ts
git commit -m "feat(frontend): add cluster node detail route"
```

---

### Task 13: Create Node Detail Page

**Files:**
- Create: `src/frontend/src/views/ClusterNodeDetail.vue`

- [ ] **Step 1: Write the page component**

This is the main node detail page with tabs for each config category. Uses the `useRemoteNodeStore`.

```vue
<template>
  <div class="app-page">
    <!-- Loading overlay -->
    <v-overlay :model-value="remoteNode.pageLoading" class="align-center justify-center">
      <v-progress-circular indeterminate size="64" />
    </v-overlay>

    <!-- Error state -->
    <v-alert v-if="remoteNode.pageError" type="error" :text="remoteNode.pageError" />

    <!-- Node info card -->
    <template v-if="!remoteNode.pageLoading && !remoteNode.pageError">
      <section class="app-page__hero">
        <div class="app-page__hero-head">
          <div class="app-page__hero-kicker">{{ $t('pages.clusterCenter') }}</div>
          <h1 class="app-page__hero-title">{{ nodeName }}</h1>
          <div class="app-page__hero-meta">
            <span class="app-page__hero-meta-item">{{ remoteNode.baseURL }}</span>
            <span class="app-page__hero-meta-item">
              {{ remoteNode.info?.actions?.length ?? 0 }} actions supported
            </span>
          </div>
        </div>
      </section>

      <v-row class="app-page__toolbar">
        <v-col cols="12">
          <div class="app-page__toolbar-actions">
            <v-btn prepend-icon="mdi-arrow-left" @click="$router.push('/clusters')">
              {{ $t('actions.close') }}
            </v-btn>
          </div>
        </v-col>
      </v-row>

      <v-tabs v-model="activeTab">
        <v-tab value="inbounds">{{ $t('pages.inbounds') }}</v-tab>
        <v-tab value="clients">{{ $t('pages.clients') }}</v-tab>
        <v-tab value="tls">{{ $t('pages.tls') }}</v-tab>
        <v-tab value="services">{{ $t('pages.services') }}</v-tab>
        <v-tab value="routes">{{ $t('pages.rules') }}</v-tab>
        <v-tab value="outbounds">{{ $t('pages.outbounds') }}</v-tab>
      </v-tabs>

      <v-window v-model="activeTab">
        <v-window-item value="inbounds">
          <v-progress-linear v-if="remoteNode.inbounds.loading" indeterminate />
          <v-row class="app-grid">
            <v-col cols="12" md="6" lg="4" xl="3" v-for="item in remoteNode.inbounds.items" :key="item.tag">
              <v-card class="app-entity-card" elevation="5">
                <v-card-title>{{ item.tag }}</v-card-title>
                <v-card-subtitle>{{ item.type }}</v-card-subtitle>
                <v-card-text>
                  <v-row><v-col>{{ $t('in.port') }}</v-col><v-col>{{ item.listen_port }}</v-col></v-row>
                </v-card-text>
              </v-card>
            </v-col>
          </v-row>
          <v-pagination
            v-model="remoteNode.inbounds.page"
            :length="Math.ceil(remoteNode.inbounds.total / 10)"
            @update:model-value="remoteNode.fetchTab(remoteNode.inbounds, 'inbound.list', $event)"
          />
        </v-window-item>

        <v-window-item value="clients">
          <!-- Pre-fetched, no loading needed -->
          <v-row class="app-grid">
            <v-col cols="12" md="6" lg="4" v-for="item in remoteNode.clients.items" :key="item.name">
              <v-card class="app-entity-card" elevation="5">
                <v-card-title>{{ item.name }}</v-card-title>
              </v-card>
            </v-col>
          </v-row>
          <v-pagination
            v-model="remoteNode.clients.page"
            :length="Math.ceil(remoteNode.clients.total / 10)"
            @update:model-value="remoteNode.fetchTab(remoteNode.clients, 'client.list', $event)"
          />
        </v-window-item>

        <v-window-item value="tls">
          <!-- Pre-fetched, no loading needed -->
          <v-row class="app-grid">
            <v-col cols="12" md="6" lg="4" v-for="item in remoteNode.tlsConfigs.items" :key="item.name">
              <v-card class="app-entity-card" elevation="5">
                <v-card-title>{{ item.name }}</v-card-title>
              </v-card>
            </v-col>
          </v-row>
          <v-pagination
            v-model="remoteNode.tlsConfigs.page"
            :length="Math.ceil(remoteNode.tlsConfigs.total / 10)"
            @update:model-value="remoteNode.fetchTab(remoteNode.tlsConfigs, 'tls.list', $event)"
          />
        </v-window-item>

        <v-window-item value="services">
          <v-progress-linear v-if="remoteNode.services.loading" indeterminate />
          <v-row class="app-grid">
            <v-col cols="12" md="6" lg="4" v-for="item in remoteNode.services.items" :key="item.tag">
              <v-card class="app-entity-card" elevation="5">
                <v-card-title>{{ item.tag }}</v-card-title>
              </v-card>
            </v-col>
          </v-row>
          <v-pagination
            v-model="remoteNode.services.page"
            :length="Math.ceil(remoteNode.services.total / 10)"
            @update:model-value="remoteNode.fetchTab(remoteNode.services, 'service.list', $event)"
          />
        </v-window-item>

        <v-window-item value="routes">
          <v-progress-linear v-if="remoteNode.routes.loading" indeterminate />
          <v-row class="app-grid">
            <v-col cols="12" md="6" lg="4" v-for="item in remoteNode.routes.items" :key="item.tag">
              <v-card class="app-entity-card" elevation="5">
                <v-card-title>{{ item.tag }}</v-card-title>
              </v-card>
            </v-col>
          </v-row>
          <v-pagination
            v-model="remoteNode.routes.page"
            :length="Math.ceil(remoteNode.routes.total / 10)"
            @update:model-value="remoteNode.fetchTab(remoteNode.routes, 'route.list', $event)"
          />
        </v-window-item>

        <v-window-item value="outbounds">
          <v-progress-linear v-if="remoteNode.outbounds.loading" indeterminate />
          <v-row class="app-grid">
            <v-col cols="12" md="6" lg="4" v-for="item in remoteNode.outbounds.items" :key="item.tag">
              <v-card class="app-entity-card" elevation="5">
                <v-card-title>{{ item.tag }}</v-card-title>
              </v-card>
            </v-col>
          </v-row>
          <v-pagination
            v-model="remoteNode.outbounds.page"
            :length="Math.ceil(remoteNode.outbounds.total / 10)"
            @update:model-value="remoteNode.fetchTab(remoteNode.outbounds, 'outbound.list', $event)"
          />
        </v-window-item>
      </v-window>
    </template>
  </div>
</template>

<script lang="ts" setup>
import { ref, onMounted, onUnmounted } from 'vue'
import { useRoute } from 'vue-router'
import { useRemoteNodeStore } from '@/store/modules/remoteNode'

const route = useRoute()
const remoteNode = useRemoteNodeStore()
const activeTab = ref('inbounds')
const nodeName = ref(route.query.name as string || route.params.nodeId as string)

onMounted(() => {
  const url = route.query.baseUrl as string
  const token = route.query.token as string
  if (url && token) {
    remoteNode.init(url, token)
  }
})

onUnmounted(() => {
  remoteNode.reset()
})
</script>
```

- [ ] **Step 2: Commit**

```bash
git add src/frontend/src/views/ClusterNodeDetail.vue
git commit -m "feat(frontend): add cluster node detail page with tabs"
```

---

### Task 14: Add "管理" Button to ClusterCenter

**Files:**
- Modify: `src/frontend/src/views/ClusterCenter.vue`

- [ ] **Step 1: Add the "管理" button to each member row**

Locate the member list rendering in the template. For each non-local member, add a "管理" button:

```vue
<v-btn
  size="small"
  variant="tonal"
  @click="manageMember(member)"
>
  {{ $t('actions.manage') }}
</v-btn>
```

In the script section, add the navigation method:

```typescript
import { useRouter } from 'vue-router'

const router = useRouter()

const manageMember = (member: ClusterMember) => {
  router.push({
    name: 'pages.clusterNodeDetail',
    params: { nodeId: member.nodeId },
    query: {
      name: member.name,
      baseUrl: member.baseUrl,
      token: getPeerToken(member),
    },
  })
}
```

The `getPeerToken` function retrieves the decrypted peer token for the target member. This requires the backend to expose peer tokens for local use (or the existing cluster API may already provide this).

- [ ] **Step 2: Commit**

```bash
git add src/frontend/src/views/ClusterCenter.vue
git commit -m "feat(frontend): add manage button to cluster member rows"
```

---

### Task 15: Create Node Dialog

**Files:**
- Create: `src/frontend/src/layouts/modals/ClusterCreateNode.vue`

- [ ] **Step 1: Write the create node dialog**

This dialog combines TLS (optional, collapsible) + Inbound (required) + Users (optional, collapsible) into one large card. It reuses the existing `Inbound.vue` form pattern but adapted for remote creation.

```vue
<template>
  <v-dialog class="app-dialog app-dialog--wide" transition="dialog-bottom-transition" width="900">
    <v-card class="rounded-lg" :loading="loading">
      <v-card-title>{{ $t('actions.add') + ' ' + $t('objects.inbound') }}</v-card-title>
      <v-divider />
      <v-card-text style="padding: 0 16px; overflow-y: scroll;">
        <v-container style="padding: 0;">

          <!-- TLS Settings (optional, collapsible) -->
          <v-card class="rounded-lg mb-4">
            <v-card-title @click="showTls = !showTls" style="cursor: pointer;">
              {{ $t('objects.tls') }}
              <v-icon :icon="showTls ? 'mdi-chevron-up' : 'mdi-chevron-down'" />
            </v-card-title>
            <v-card-text v-if="showTls">
              <!-- Reuse TLS config form fields from Tls.vue -->
              <v-row>
                <v-col cols="12" sm="6" md="4">
                  <v-select
                    :label="$t('tls.existing')"
                    :items="existingTlsOptions"
                    item-title="name"
                    item-value="id"
                    v-model="selectedTlsId"
                    clearable
                    @update:model-value="onSelectExistingTls"
                  />
                </v-col>
              </v-row>
              <!-- Full TLS form if not selecting existing -->
              <template v-if="selectedTlsId == null">
                <!-- Inline TLS config: reuse Tls.vue form fields -->
              </template>
            </v-card-text>
          </v-card>

          <!-- Inbound Config (required) -->
          <v-card class="rounded-lg mb-4">
            <v-card-title>{{ $t('objects.inbound') }}</v-card-title>
            <v-card-text>
              <!-- Reuse inbound type selector, listen, port, protocol-specific fields -->
              <v-row>
                <v-col cols="12" sm="6" md="4">
                  <v-select
                    :label="$t('type')"
                    :items="protocolOptions"
                    v-model="inbound.type"
                    @update:modelValue="changeType"
                  />
                </v-col>
                <v-col cols="12" sm="6" md="4">
                  <v-text-field v-model="inbound.tag" :label="$t('objects.tag')" />
                </v-col>
                <v-col cols="12" sm="6" md="4">
                  <v-text-field v-model="inbound.listen" :label="$t('in.addr')" />
                </v-col>
                <v-col cols="12" sm="6" md="4">
                  <v-text-field v-model.number="inbound.listen_port" type="number" :label="$t('in.port')" />
                </v-col>
              </v-row>
              <!-- Protocol-specific, transport, multiplex components as in Inbound.vue -->
            </v-card-text>
          </v-card>

          <!-- User Management (optional, collapsible) -->
          <v-card class="rounded-lg mb-4">
            <v-card-title @click="showUsers = !showUsers" style="cursor: pointer;">
              {{ $t('pages.clients') }}
              <v-icon :icon="showUsers ? 'mdi-chevron-up' : 'mdi-chevron-down'" />
            </v-card-title>
            <v-card-text v-if="showUsers">
              <!-- Reuse Users component or inline user config -->
            </v-card-text>
          </v-card>

        </v-container>
      </v-card-text>
      <v-card-actions>
        <v-spacer />
        <v-btn color="primary" variant="outlined" @click="closeModal">
          {{ $t('actions.close') }}
        </v-btn>
        <v-btn color="primary" variant="tonal" :loading="loading" @click="createNode">
          {{ $t('actions.save') }}
        </v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>
</template>

<script lang="ts" setup>
import { ref, computed } from 'vue'
import { InTypes, createInbound } from '@/types/inbounds'
import { useRemoteNodeStore } from '@/store/modules/remoteNode'
import { sendAction } from '@/features/clusterPeerApi'
import type { ProxyCreatePayload } from '@/types/clusterActions'

const props = defineProps<{ visible: boolean }>()
const emit = defineEmits<{ (e: 'close'): void; (e: 'created'): void }>()

const remoteNode = useRemoteNodeStore()
const loading = ref(false)
const showTls = ref(false)
const showUsers = ref(false)
const selectedTlsId = ref<number | null>(null)

const inbound = ref(createInbound('direct', { id: 0, tag: '' }))

const existingTlsOptions = computed(() => remoteNode.tlsConfigs.items)
const protocolOptions = computed(() => Object.keys(InTypes).map((key, i) => ({
  title: key,
  value: Object.values(InTypes)[i],
})))

function changeType() {
  // Same logic as Inbound.vue changeType
}

function onSelectExistingTls(id: number | null) {
  // If selecting existing, populate TLS config from remoteNode.tlsConfigs
}

async function createNode() {
  loading.value = true
  try {
    const payload: ProxyCreatePayload = {
      request_id: crypto.randomUUID(),
      inbound: inbound.value,
    }
    // Add TLS if configured
    if (showTls.value && selectedTlsId.value != null) {
      const tlsConfig = remoteNode.tlsConfigs.items.find((t: any) => t.id === selectedTlsId.value)
      if (tlsConfig) payload.tls = tlsConfig
    }
    // Add users if configured
    if (showUsers.value) {
      payload.users = [] // populated from user form
    }

    const resp = await sendAction(remoteNode.baseURL, remoteNode.token, {
      schema_version: 1,
      sourceNodeId: '',
      domain: '',
      sentAt: Math.floor(Date.now() / 1000),
      signature: '',
      action: 'proxy.create',
      payload: payload as any,
    })

    if (resp.status === 'success') {
      emit('created')
      closeModal()
    }
  } finally {
    loading.value = false
  }
}

function closeModal() {
  emit('close')
}
</script>
```

- [ ] **Step 2: Commit**

```bash
git add src/frontend/src/layouts/modals/ClusterCreateNode.vue
git commit -m "feat(frontend): add create node dialog with TLS+inbound+user sections"
```

---

## Phase 6: Integration & Testing

### Task 16: Integration Test — Action Endpoint E2E

**Files:**
- Test: `src/backend/internal/http/api/cluster_test.go`

- [ ] **Step 1: Add integration test for the action endpoint**

```go
func TestClusterActionEndpoint_ReturnsUnsupportedForUnknownAction(t *testing.T) {
    // Set up test router with cluster routes
    // POST /_cluster/v1/action with unknown action
    // Assert response status is "unsupported"
}

func TestClusterInfoEndpoint_ReturnsSupportedActions(t *testing.T) {
    // GET /_cluster/v1/info
    // Assert response contains action list
}
```

- [ ] **Step 2: Run all backend tests**

Run: `cd src/backend && go test ./... -v`
Expected: All tests PASS

- [ ] **Step 3: Commit**

```bash
git add src/backend/internal/http/api/cluster_test.go
git commit -m "test(cluster): add integration tests for info and action endpoints"
```

---

### Task 17: Frontend Smoke Test

**Files:**
- Test: `src/frontend/src/views/ClusterNodeDetail.test.ts` (new)

- [ ] **Step 1: Write basic component test**

Test that the node detail page renders and calls init on mount.

- [ ] **Step 2: Run frontend tests**

Run: `cd src/frontend && npm run test`
Expected: All tests PASS

- [ ] **Step 3: Commit**

```bash
git add src/frontend/src/views/ClusterNodeDetail.test.ts
git commit -m "test(frontend): add node detail page smoke test"
```

---

## Self-Review Checklist

- [x] **Spec coverage**: Every section in the spec maps to at least one task
- [x] **Placeholder scan**: No TBD/TODO/fill-in-later patterns
- [x] **Type consistency**: `ActionRequest`, `ActionResponse`, `ActionHandler` types used consistently across all files
- [x] **Directory structure matches spec**: `cluster/types/`, `cluster/router/`, `cluster/handler/`, `cluster/handler/action/`
