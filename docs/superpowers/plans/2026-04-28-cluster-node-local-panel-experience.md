# Cluster Node Local Panel Experience Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make cluster node detail provide the same CRUD-oriented management workflows as the local panel without exposing peer tokens to the browser.

**Architecture:** Add remote `panel.*` cluster actions that mirror the local panel's `api/load`, `api/save`, partial loads, keypair generation, outbound checks, and stats calls. Put the frontend `Data` Pinia store into a remote-node mode while `ClusterNodeDetail.vue` is mounted, then render the existing local panel pages inside remote tabs so existing cards, modals, presets, validation, bulk actions, and save flows are reused.

**Tech Stack:** Go/Gin backend, existing cluster action router, Vue 3, Pinia, Vuetify, Vitest.

---

## File Structure

- Create `src/backend/internal/domain/services/cluster/handler/action/panel_handler.go`
  - Owns `panel.load`, `panel.partial`, `panel.save`, `panel.keypairs`, `panel.linkConvert`, `panel.checkOutbound`, and `panel.stats` action decoding.
- Create `src/backend/internal/domain/services/cluster/handler/action/panel_handler_test.go`
  - Unit tests for action payload decoding, service calls, and error handling.
- Create `src/backend/internal/domain/services/cluster_panel_service.go`
  - Adapts existing local services into a cluster action service without importing HTTP handlers.
- Modify `src/backend/internal/domain/services/cluster/runtime.go`
  - Register existing list actions plus new `panel.*` actions when a panel service is provided.
- Modify `src/backend/internal/domain/services/cluster_service.go`
  - Add `NewClusterService()` that wires the runtime for cluster protocol routes.
- Modify `src/backend/internal/http/api/apiHandler.go`
  - Use `service.NewClusterService()` for local API cluster proxy service.
- Modify `src/backend/internal/infra/web/web.go`
  - Register cluster protocol routes with a runtime-enabled cluster service.
- Create `src/frontend/src/features/remotePanelApi.ts`
  - Frontend helper for sending `panel.*` actions through `sendAction`.
- Create `src/frontend/src/features/remotePanelApi.test.ts`
  - Vitest coverage for action request construction and action error handling.
- Modify `src/frontend/src/store/modules/data.ts`
  - Add remote-node mode, remote load/save/partial helpers, keypair/link/stats/check helpers.
- Modify `src/frontend/src/views/ClusterNodeDetail.vue`
  - Switch `Data` into remote mode and render local panel views inside tabs with read-only fallback for old peers.
- Modify `src/frontend/src/views/Outbounds.vue`
  - Use `Data().checkOutbound()` instead of local-only `HttpUtils.get('api/checkOutbound')`.
- Modify `src/frontend/src/layouts/modals/Outbound.vue`
  - Use `Data().linkConvert()` instead of local-only `HttpUtils.post('api/linkConvert')`.
- Modify `src/frontend/src/layouts/modals/Tls.vue`
  - Use `Data().keypairs()` instead of local-only `HttpUtils.get('api/keypairs')`.
- Modify `src/frontend/src/layouts/modals/Endpoint.vue`
  - Use `Data().keypairs()` for WireGuard key operations.
- Modify `src/frontend/src/layouts/modals/Stats.vue`
  - Use `Data().stats()` instead of local-only `HttpUtils.get('api/stats')`.

## Task 1: Backend Panel Action Handler

**Files:**
- Create: `src/backend/internal/domain/services/cluster/handler/action/panel_handler_test.go`
- Create: `src/backend/internal/domain/services/cluster/handler/action/panel_handler.go`

- [ ] **Step 1: Write failing tests for panel actions**

Add tests that describe the desired handler contract:

```go
func TestPanelHandlerLoadCallsService(t *testing.T) {
    svc := &stubPanelService{}
    h := NewPanelHandler(svc)
    resp, err := h.Load(context.Background(), clustertypes.ActionRequest{
        Action: "panel.load",
        Payload: map[string]interface{}{
            "lu": "123",
            "hostname": "node.example.com",
        },
    })
    if err != nil {
        t.Fatalf("load: %v", err)
    }
    if resp.Status != "success" {
        t.Fatalf("expected success, got %q", resp.Status)
    }
    if svc.loadLU != "123" || svc.loadHostname != "node.example.com" {
        t.Fatalf("unexpected load args: lu=%q hostname=%q", svc.loadLU, svc.loadHostname)
    }
}

func TestPanelHandlerSavePassesRawData(t *testing.T) {
    svc := &stubPanelService{}
    h := NewPanelHandler(svc)
    resp, err := h.Save(context.Background(), clustertypes.ActionRequest{
        Action: "panel.save",
        Payload: map[string]interface{}{
            "object": "inbounds",
            "action": "new",
            "data": map[string]interface{}{"tag": "direct-10000"},
            "initUsers": []interface{}{float64(1), float64(2)},
            "hostname": "node.example.com",
        },
    })
    if err != nil {
        t.Fatalf("save: %v", err)
    }
    if resp.Status != "success" {
        t.Fatalf("expected success, got %q", resp.Status)
    }
    if svc.saveObject != "inbounds" || svc.saveAction != "new" || svc.saveInitUsers != "1,2" {
        t.Fatalf("unexpected save args: %#v", svc)
    }
    if string(svc.saveData) != `{"tag":"direct-10000"}` {
        t.Fatalf("unexpected raw data %s", string(svc.saveData))
    }
}

func TestPanelHandlerReturnsErrorForMissingObject(t *testing.T) {
    h := NewPanelHandler(&stubPanelService{})
    _, err := h.Save(context.Background(), clustertypes.ActionRequest{
        Action: "panel.save",
        Payload: map[string]interface{}{"action": "new", "data": map[string]interface{}{}},
    })
    if err == nil {
        t.Fatal("expected validation error")
    }
}
```

- [ ] **Step 2: Run tests and verify RED**

Run:

```bash
rtk go test ./src/backend/internal/domain/services/cluster/handler/action -run PanelHandler -count=1
```

Expected: FAIL because `NewPanelHandler` and the panel handler types do not exist.

- [ ] **Step 3: Implement the panel handler**

Create `panel_handler.go` with:

```go
package action

import (
    "context"
    "encoding/json"
    "fmt"
    "strconv"
    "strings"

    clustertypes "github.com/BeanYa/b-ui/src/backend/internal/domain/services/cluster/types"
    "github.com/BeanYa/b-ui/src/backend/internal/domain/services/cluster/router"
)

type PanelService interface {
    Load(lu string, hostname string) (map[string]interface{}, error)
    Partial(object string, id string, hostname string) (map[string]interface{}, error)
    Save(object string, act string, data json.RawMessage, initUsers string, hostname string) (map[string]interface{}, error)
    Keypairs(kind string, options string) ([]string, error)
    LinkConvert(link string) (interface{}, error)
    CheckOutbound(tag string, link string) (interface{}, error)
    Stats(resource string, tag string, limit int) (interface{}, error)
}

type PanelHandler struct {
    svc PanelService
}

func NewPanelHandler(svc PanelService) *PanelHandler {
    return &PanelHandler{svc: svc}
}

func (h *PanelHandler) RegisterAll(r *router.ActionRouter) {
    r.Register("panel.load", h.Load)
    r.Register("panel.partial", h.Partial)
    r.Register("panel.save", h.Save)
    r.Register("panel.keypairs", h.Keypairs)
    r.Register("panel.linkConvert", h.LinkConvert)
    r.Register("panel.checkOutbound", h.CheckOutbound)
    r.Register("panel.stats", h.Stats)
}
```

Implement each method with `marshalUnmarshalPayload(req.Payload)`, small payload structs, validation, and `clustertypes.ActionResponse{Status: "success", Data: data}`. Convert `initUsers` arrays to comma-separated IDs so existing `ConfigService.Save` can be reused.

- [ ] **Step 4: Run tests and verify GREEN**

Run:

```bash
rtk go test ./src/backend/internal/domain/services/cluster/handler/action -run PanelHandler -count=1
```

Expected: PASS.

## Task 2: Backend Panel Service Adapter and Runtime Wiring

**Files:**
- Create: `src/backend/internal/domain/services/cluster_panel_service.go`
- Modify: `src/backend/internal/domain/services/cluster/runtime.go`
- Modify: `src/backend/internal/domain/services/cluster_service.go`
- Modify: `src/backend/internal/infra/web/web.go`
- Modify: `src/backend/internal/http/api/apiHandler.go`
- Test: `src/backend/internal/domain/services/cluster/handler/action/panel_handler_test.go`

- [ ] **Step 1: Write failing runtime tests**

Add a test in `panel_handler_test.go` or a new runtime test that asserts:

```go
func TestRuntimeRegistersPanelActions(t *testing.T) {
    rt := cluster.NewRuntimeWithPanel(nil, &stubPanelService{})
    actions := rt.InfoResponse().Actions
    expected := []string{"panel.load", "panel.save", "panel.partial", "panel.keypairs", "panel.linkConvert", "panel.checkOutbound", "panel.stats"}
    for _, want := range expected {
        if !slices.Contains(actions, want) {
            t.Fatalf("expected action %s in %#v", want, actions)
        }
    }
}
```

- [ ] **Step 2: Run tests and verify RED**

Run:

```bash
rtk go test ./src/backend/internal/domain/services/cluster/... ./src/backend/internal/domain/services/cluster/handler/action -run 'RuntimeRegistersPanelActions|PanelHandler' -count=1
```

Expected: FAIL because `NewRuntimeWithPanel` does not exist.

- [ ] **Step 3: Implement service adapter**

Create `cluster_panel_service.go` in package `service`:

```go
type ClusterPanelActionService struct {
    ConfigService
    ServerService
    StatsService
}
```

Implement:

- `Load(lu, hostname string)` by copying the data assembly from `ApiService.getData` without Gin dependencies.
- `Partial(object, id, hostname string)` by using existing service methods for `inbounds`, `clients`, `tls`, `outbounds`, `services`, `endpoints`, and `config`.
- `Save(object, act string, data json.RawMessage, initUsers, hostname string)` by calling `ConfigService.Save(object, act, data, initUsers, "ClusterRemotePanel", hostname)` and then returning `Load("", hostname)`.
- `Keypairs` through `ServerService.GenKeypair`.
- `LinkConvert` through `util.GetOutbound`.
- `CheckOutbound` through `ConfigService.CheckOutbound`.
- `Stats` through `StatsService.GetStats`.

- [ ] **Step 4: Register runtime actions**

In `cluster/runtime.go`, add a new constructor:

```go
type RuntimePanelServices struct {
    Panel action.PanelService
}

func NewRuntimeWithPanel(
    lists RuntimeListServices,
    panel action.PanelService,
) *Runtime {
    r := router.NewActionRouter()
    registerListActions(r, lists)
    if panel != nil {
        action.NewPanelHandler(panel).RegisterAll(r)
    }
    return &Runtime{Router: r}
}
```

Keep the existing `NewRuntime(...)` constructor by having it call `registerListActions`.

- [ ] **Step 5: Wire cluster services**

Add to `cluster_service.go`:

```go
func NewClusterService() *ClusterService {
    svc := &ClusterService{}
    svc.SetRuntime(cluster.NewRuntimeWithPanel(cluster.RuntimeListServices{}, &ClusterPanelActionService{}))
    return svc
}
```

Modify:

- `apiHandler.go`: `clusterService: service.NewClusterService()`
- `web.go`: `api.RegisterClusterMessageRoute(engine.Group(base_url), service.NewClusterService())`

- [ ] **Step 6: Run tests and verify GREEN**

Run:

```bash
rtk go test ./src/backend/internal/domain/services/cluster/... ./src/backend/internal/domain/services -run 'RuntimeRegistersPanelActions|PanelHandler' -count=1
```

Expected: PASS.

## Task 3: Frontend Remote Panel API

**Files:**
- Create: `src/frontend/src/features/remotePanelApi.test.ts`
- Create: `src/frontend/src/features/remotePanelApi.ts`

- [ ] **Step 1: Write failing tests**

Add tests:

```ts
it('sends panel.load with hostname through cluster member action', async () => {
  vi.mocked(api.post).mockResolvedValue({
    data: { success: true, obj: { status: 'success', action: 'panel.load', data: { inbounds: [] } } },
  })
  const data = await remotePanelLoad('node-a', { lu: 123, hostname: 'node.example.com' })
  expect(data).toEqual({ inbounds: [] })
  expect(api.post).toHaveBeenCalledWith('api/cluster/member-action', expect.objectContaining({
    node_id: 'node-a',
    request: expect.objectContaining({
      action: 'panel.load',
      payload: { lu: 123, hostname: 'node.example.com' },
    }),
  }), expect.any(Object))
})

it('throws action error messages', async () => {
  vi.mocked(api.post).mockResolvedValue({
    data: { success: true, obj: { status: 'error', action: 'panel.save', error_message: 'bad tag' } },
  })
  await expect(remotePanelSave('node-a', { object: 'inbounds', action: 'new', data: {}, hostname: 'node.example.com' }))
    .rejects.toThrow('bad tag')
})
```

- [ ] **Step 2: Run tests and verify RED**

Run:

```bash
cd src/frontend
rtk npm test -- remotePanelApi.test.ts
```

Expected: FAIL because `remotePanelApi.ts` does not exist.

- [ ] **Step 3: Implement remote panel API**

Create helpers:

```ts
export async function remotePanelLoad(nodeId: string, payload: RemotePanelLoadPayload): Promise<any>
export async function remotePanelPartial(nodeId: string, payload: RemotePanelPartialPayload): Promise<any>
export async function remotePanelSave(nodeId: string, payload: RemotePanelSavePayload): Promise<any>
export async function remotePanelKeypairs(nodeId: string, payload: RemotePanelKeypairPayload): Promise<string[]>
export async function remotePanelLinkConvert(nodeId: string, payload: { link: string }): Promise<any>
export async function remotePanelCheckOutbound(nodeId: string, payload: { tag: string; link?: string }): Promise<any>
export async function remotePanelStats(nodeId: string, payload: { resource: string; tag: string; limit: number }): Promise<any>
```

All helpers call `sendAction` and unwrap only `status === 'success'`.

- [ ] **Step 4: Run tests and verify GREEN**

Run:

```bash
cd src/frontend
rtk npm test -- remotePanelApi.test.ts
```

Expected: PASS.

## Task 4: Data Store Remote Mode

**Files:**
- Modify: `src/frontend/src/store/modules/data.ts`
- Test: `src/frontend/src/store/modules/data.test.ts` if present, otherwise create `src/frontend/src/store/modules/data.remote.test.ts`

- [ ] **Step 1: Write failing tests**

Test that:

- `enterRemoteNode('node-a', 'https://node.example.com:8443/base')` stores node ID and hostname `node.example.com`.
- `loadData()` calls `remotePanelLoad` instead of local `HttpUtils.get('api/load')`.
- `save('inbounds', 'new', data, [1])` calls `remotePanelSave` and refreshes store data.
- `exitRemoteNode()` returns subsequent `loadData()` calls to local behavior.

- [ ] **Step 2: Run tests and verify RED**

Run:

```bash
cd src/frontend
rtk npm test -- data.remote.test.ts
```

Expected: FAIL because remote mode methods do not exist.

- [ ] **Step 3: Implement remote mode**

Add state:

```ts
remoteNodeId: '',
remoteBaseUrl: '',
remoteHostname: '',
```

Add actions:

```ts
enterRemoteNode(nodeId: string, baseUrl: string): void
exitRemoteNode(): void
isRemote(): boolean
keypairs(kind: string, options?: string): Promise<string[]>
linkConvert(link: string): Promise<any>
checkOutbound(tag: string, link?: string): Promise<any>
stats(resource: string, tag: string, limit: number): Promise<any>
```

Change existing `loadData`, `loadInbounds`, `loadClients`, and `save` to use
remote panel helpers when `remoteNodeId` is set. Keep the local behavior
unchanged when not in remote mode.

- [ ] **Step 4: Run tests and verify GREEN**

Run:

```bash
cd src/frontend
rtk npm test -- data.remote.test.ts remotePanelApi.test.ts
```

Expected: PASS.

## Task 5: Local Components Use Data Helper Methods

**Files:**
- Modify: `src/frontend/src/views/Outbounds.vue`
- Modify: `src/frontend/src/layouts/modals/Outbound.vue`
- Modify: `src/frontend/src/layouts/modals/Tls.vue`
- Modify: `src/frontend/src/layouts/modals/Endpoint.vue`
- Modify: `src/frontend/src/layouts/modals/Stats.vue`

- [ ] **Step 1: Write or extend tests where practical**

Add focused tests for helper functions if component tests are unavailable. The behavior to lock down is that helper calls route through `Data()` so remote mode can intercept them.

- [ ] **Step 2: Replace local-only API calls**

Make these mechanical edits:

- `Outbounds.vue`: `HttpUtils.get('api/checkOutbound', { tag })` -> `Data().checkOutbound(tag)`
- `Outbound.vue`: `HttpUtils.post('api/linkConvert', { link: this.link })` -> `Data().linkConvert(this.link)`
- `Tls.vue`: `HttpUtils.get('api/keypairs', { k: 'tls', o: ... })` -> `Data().keypairs('tls', ...)`; reality likewise.
- `Endpoint.vue`: all `api/keypairs` calls -> `Data().keypairs('wireguard', options)`.
- `Stats.vue`: `HttpUtils.get('api/stats', ...)` -> `Data().stats(...)`.

- [ ] **Step 3: Run focused frontend tests**

Run:

```bash
cd src/frontend
rtk npm test -- remotePanelApi.test.ts data.remote.test.ts
```

Expected: PASS.

## Task 6: Cluster Node Detail Renders Local Panel Tabs

**Files:**
- Modify: `src/frontend/src/views/ClusterNodeDetail.vue`
- Test: `src/frontend/src/views/ClusterNodeDetail.test.ts`

- [ ] **Step 1: Write failing tests**

Add tests that assert:

- When info actions include `panel.load` and `panel.save`, the page enters remote mode and renders management tabs.
- When `panel.load` is absent, the existing read-only card grid fallback remains available.
- Unmount exits remote mode.

- [ ] **Step 2: Run tests and verify RED**

Run:

```bash
cd src/frontend
rtk npm test -- ClusterNodeDetail.test.ts
```

Expected: FAIL because the page does not use remote `Data` mode or local panel components.

- [ ] **Step 3: Implement remote management tabs**

Import local views:

```ts
import InboundsView from '@/views/Inbounds.vue'
import ClientsView from '@/views/Clients.vue'
import TlsView from '@/views/Tls.vue'
import ServicesView from '@/views/Services.vue'
import RulesView from '@/views/Rules.vue'
import OutboundsView from '@/views/Outbounds.vue'
import EndpointsView from '@/views/Endpoints.vue'
import Data from '@/store/modules/data'
```

Add tabs:

```text
inbounds, clients, tls, services, endpoints, routes, outbounds
```

When `panel.load` is supported:

- call `Data().enterRemoteNode(nodeId, nodeConnection.baseUrl)`
- call `Data().loadData()`
- render the imported local view for each tab
- on unmount call `Data().exitRemoteNode()`

When not supported:

- keep the current generic read-only list fallback.

- [ ] **Step 4: Run tests and verify GREEN**

Run:

```bash
cd src/frontend
rtk npm test -- ClusterNodeDetail.test.ts remotePanelApi.test.ts data.remote.test.ts
```

Expected: PASS.

## Task 7: Full Verification

**Files:**
- No production edits unless verification exposes a bug.

- [ ] **Step 1: Run backend targeted tests**

```bash
rtk go test ./src/backend/internal/domain/services/cluster/... ./src/backend/internal/domain/services -run 'PanelHandler|Runtime|Cluster' -count=1
```

Expected: PASS.

- [ ] **Step 2: Run frontend targeted tests**

```bash
cd src/frontend
rtk npm test -- remotePanelApi.test.ts data.remote.test.ts ClusterNodeDetail.test.ts
```

Expected: PASS.

- [ ] **Step 3: Run frontend build**

```bash
cd src/frontend
rtk npm run build:dist
```

Expected: PASS.

- [ ] **Step 4: Check final diff**

```bash
rtk proxy git status --short
rtk proxy git diff --stat
```

Expected: only intended backend, frontend, tests, and plan files are changed.

## Self-Review

- Spec coverage: remote local-panel experience is covered by `panel.*` actions, remote `Data` mode, and rendering existing local views. Older peers remain supported through the read-only fallback.
- Placeholder scan: no incomplete implementation markers are intentionally present.
- Type consistency: backend uses `PanelService`, frontend uses `remotePanel*` helpers, and Data store remote mode is the single integration point for reused local components.
