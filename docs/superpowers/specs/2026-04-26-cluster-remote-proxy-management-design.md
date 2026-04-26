# Cluster Remote Proxy Management Design

## Overview

Enable nodes within a domain to remotely manage proxies on other nodes. A requesting node sends configuration (inbound + user + TLS) to a target node via the existing cluster communication pipeline. The target node creates the proxy and returns connection URIs.

**v1 scope**: Remote proxy creation and full CRUD management of all node config categories.
**v2 scope**: Node-to-node relay/transit (architecture reserved).

## 1. Protocol Communication Layer

### Endpoints

Fixed endpoints under `/_cluster/v1/`:

| Endpoint | Method | Purpose |
|----------|--------|---------|
| `/_cluster/v1/heartbeat` | GET | Health check (existing) |
| `/_cluster/v1/info` | GET | Node info and capability query |
| `/_cluster/v1/ping` | GET | Connectivity test (existing) |
| `/_cluster/v1/events` | POST | Event notifications (existing) |
| `/_cluster/v1/action` | POST | Unified business operation entry |

### Routing

All business operations go through `POST /_cluster/v1/action`. The `action` field in the request body determines which handler processes the request. This avoids HTTP status code confusion (e.g., 404 for unknown routes vs. unsupported operations).

### Request Envelope

```jsonc
{
  "schema_version": 1,
  "source_node_id": "node-A",
  "domain": "my-domain",
  "sent_at": 1714000000,
  "signature": "ed25519-sig",
  "action": "proxy.create",
  "payload": {
    // action-specific data
  }
}
```

### Response Format

```jsonc
{
  "status": "success" | "unsupported" | "error",
  "action": "proxy.create",
  "error_message": "optional error detail",
  "data": {
    // action-specific response
  }
}
```

- `unsupported`: No handler registered for the requested action.
- `error`: Handler found, but execution failed (validation, conflict, etc.).
- `success`: Operation completed.

### Capability Discovery

`GET /_cluster/v1/info` returns:

```jsonc
{
  "actions": [
    "proxy.create", "proxy.read", "proxy.update", "proxy.delete",
    "inbound.list", "inbound.read", "inbound.create", "inbound.update", "inbound.delete",
    "client.list", "client.read", "client.create", "client.update", "client.delete",
    "tls.list", "tls.read", "tls.create", "tls.update", "tls.delete",
    "service.list", "service.read", "service.update",
    "route.list", "route.read", "route.update",
    "outbound.list", "outbound.read", "outbound.create", "outbound.update", "outbound.delete"
  ]
}
```

## 2. Action Payloads

### proxy.create

Creates inbound + optional TLS + optional users on the target node. Returns connection URIs.

```jsonc
{
  "action": "proxy.create",
  "payload": {
    "request_id": "uuid-for-idempotency",
    "tls": {
      "name": "my-tls-config",
      "preset": "reality",          // standard | hysteria2 | reality
      "config": { ... }             // full TLS config, same structure as local panel
    },
    "inbound": {
      "listen": "0.0.0.0",
      "port": 443,
      "protocol": "vless",
      "settings": { ... },          // protocol-specific config
      "transport": { ... },         // transport config (TCP/WS/gRPC/etc.)
      "tls_ref": "my-tls-config"    // references tls.name above
    },
    "users": [
      {
        "username": "user1",
        "uuid": "auto-generate-if-empty",
        "settings": { ... }
      }
    ],
    "expiry": null                  // ISO8601 timestamp or null for persistent
  }
}
```

**Response**:

```jsonc
{
  "status": "success",
  "data": {
    "inbound_id": 123,
    "uris": ["vless://uuid@target-host:443?..."],
    "expiry": null
  }
}
```

No protocol/transport whitelist filtering. The payload passes through the same config structures used by the local panel. Xray-core validates protocol legality.

### proxy.read

```jsonc
{
  "action": "proxy.read",
  "payload": {
    "inbound_id": 123   // null to list all
  }
}
```

### proxy.update

```jsonc
{
  "action": "proxy.update",
  "payload": {
    "inbound_id": 123,
    "patch": {
      "inbound": { ... },
      "users": [ ... ],
      "tls": { ... }
    }
  }
}
```

### proxy.delete

```jsonc
{
  "action": "proxy.delete",
  "payload": {
    "inbound_id": 123
  }
}
```

### List Actions (inbound/client/tls/service/route/outbound)

All list actions follow the same pattern with pagination:

```jsonc
{
  "action": "inbound.list",
  "payload": {
    "page": 1,
    "page_size": 10
  }
}
```

**Response**:

```jsonc
{
  "status": "success",
  "data": {
    "items": [ ... ],
    "total": 42,
    "page": 1,
    "page_size": 10
  }
}
```

Individual CRUD actions (read/create/update/delete) follow the same pattern as proxy.* actions.

## 3. Backend Architecture

### Directory Structure

```
internal/domain/services/cluster/
├── runtime.go                        # Main entry, registers all routes and handlers
├── router/
│   └── action_router.go              # ActionRouter - handler registration and dispatch
└── handler/
    ├── info_handler.go               # GET /_cluster/v1/info
    ├── heartbeat_handler.go          # GET /_cluster/v1/heartbeat
    ├── event_handler.go              # POST /_cluster/v1/events
    ├── ping_handler.go               # GET /_cluster/v1/ping
    └── action/
        ├── proxy_handler.go          # proxy.create/read/update/delete
        ├── inbound_handler.go        # inbound.list/read/create/update/delete
        ├── client_handler.go         # client.list/read/create/update/delete
        ├── tls_handler.go            # tls.list/read/create/update/delete
        ├── service_handler.go        # service.list/read/update
        ├── route_handler.go          # route.list/read/update
        └── outbound_handler.go       # outbound.list/read/create/update/delete
```

### Handler Registration

```go
type ActionHandler func(ctx context.Context, req ActionRequest) (ActionResponse, error)

type ActionRouter struct {
    handlers map[string]ActionHandler
}

func (r *ActionRouter) Register(action string, handler ActionHandler) {
    r.handlers[action] = handler
}

func (r *ActionRouter) Handle(req ActionRequest) ActionResponse {
    h, ok := r.handlers[req.Action]
    if !ok {
        return ActionResponse{Status: "unsupported", Action: req.Action}
    }
    resp, err := h(context.Background(), req)
    if err != nil {
        return ActionResponse{Status: "error", Action: req.Action, ErrorMessage: err.Error()}
    }
    return resp
}
```

### Request Processing Flow

```
Requester UI → POST /_cluster/v1/action
  → ClusterService.ReceiveAction()
    → ActionRouter.Handle()
      → Handler found: proxy.create handler
        → Calls existing InboundService / TLSService / UserService
        → Calls existing URI generation logic
        → Returns {status: "success", data: {uris, inbound_id}}
      → Handler not found → {status: "unsupported"}
```

Handlers do not directly access the database. They call existing domain services (`InboundService`, `TLSService`, `UserService`, etc.), sharing the same business logic as local UI operations.

## 4. Frontend UI

### Entry Point

In `ClusterCenter.vue`, each member row has a "管理" button. Clicking it navigates to the node detail page.

### Node Detail Page

New route page (not a drawer). Layout:

- **Top**: Node info card (name, address, reachability status)
- **Below**: Tab-based content

**Loading behavior**:

1. **Entering detail page**: Show full-page loading overlay → fetch TLS list + client list (needed for creation form) → dismiss loading
2. **Switching tabs**:
   - If data not yet fetched: show tab loading → fetch data → dismiss loading
   - TLS tab and Users tab: no loading needed (pre-fetched on page entry)
3. **After creating a node**: Refresh current tab data

**Pagination**: All list views use pagination with 10 items per page.

**Tabs**:

| Tab | Data Source Action | Description |
|-----|-------------------|-------------|
| 入站 | `inbound.list` | Inbound list, expandable to show users/transport/TLS |
| 用户 | `client.list` | All users (pre-fetched) |
| TLS | `tls.list` | TLS configs (pre-fetched) |
| 服务 | `service.list` | Service configs |
| 路由 | `route.list` | Routing rules |
| 出站 | `outbound.list` | Outbound rules |

**Excluded**: Admin, Cluster Center, WebTerminal, DNS, Settings pages.

### Create Node Dialog

Triggered by "创建节点" button in the 入站 tab. Opens a large card/dialog with three sections:

- **TLS 设置** (optional, collapsible) — Reuses existing TLS config components. Pre-populated with available TLS templates and existing configs fetched on page entry.
- **入站配置** (required) — Reuses existing inbound config components.
- **用户管理** (optional, collapsible) — Reuses existing user config components. Pre-populated with existing user data fetched on page entry.

On submit: serializes to `proxy.create` payload, sends via `/_cluster/v1/action`.

### State Management

New Pinia store `remoteNodeStore` managing:
- Target node info and capability cache
- Pre-fetched TLS and client lists
- Per-tab data and pagination state
- Loading states (page-level and per-tab)
- Create/update/delete operation states

## 5. Version Compatibility

- API path manages major version: `/_cluster/v1/` vs `/_cluster/v2/`
- No minor version numbers — simplification per user decision
- Action field in body manages specific capability discovery
- Target node returns `unsupported` for unrecognized actions
- Requester UI checks `/_cluster/v1/info` to discover available actions before showing management UI
