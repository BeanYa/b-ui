# Cluster Node Local Panel Experience Design

## Summary

Upgrade the cluster node detail page from a read-only inventory view into a
remote management surface that behaves like the local panel. When an admin
opens a node from a cluster domain, they should be able to create, edit, delete,
clone, and inspect the same configuration categories they manage locally:

- Inbounds
- Clients
- TLS
- Services
- Routes or rules
- Outbounds

All remote operations continue to pass through the local panel backend proxy.
The browser must not receive peer tokens or call the remote node directly.

## Goals

- Provide the same day-to-day management experience on a remote node that users
  already have on the local panel.
- Reuse the existing local forms, modal structure, presets, validation, and
  card actions wherever practical.
- Extend the cluster action runtime so remote nodes can expose full CRUD
  capabilities instead of list-only capabilities.
- Keep capability discovery explicit through `/_cluster/v1/info` so the UI can
  hide or disable unsupported operations on older peers.
- Keep remote writes on the same backend code paths used by local saves, so core
  restarts, link refreshes, TLS-in-use checks, and generated outbound JSON stay
  consistent.

## Non-Goals

- No direct iframe, redirect, or browser-side login into the remote panel.
- No cross-domain token exposure in frontend state.
- No broad redesign of local panel cards or forms.
- No changes to cluster registration, hub sync, or membership protocol beyond
  advertised action names.
- No bulk import/export workflow in this iteration.

## Current State

`ClusterNodeDetail.vue` currently loads remote node metadata and paginated list
actions through:

- `api/cluster/member-info`
- `api/cluster/member-action`
- remote actions such as `inbound.list`, `client.list`, and `tls.list`

The page renders generic cards from list responses. It has no add, edit, delete,
clone, or modal workflow.

There is already a `ClusterCreateNode.vue` modal, but it is not mounted from the
node detail page and only covers a narrow proxy creation flow. It does not match
the local panel's complete inbound and TLS editing experience.

The backend contains an action runtime with list handlers and a `ProxyHandler`,
but the runtime currently registers only list actions. Full write capability is
therefore not available to remote node management.

## Selected Approach

Use a remote data adapter plus reusable management components.

Instead of duplicating all local panel forms, introduce a small data access
boundary that can run against either:

- local `Data()` store methods, or
- remote cluster actions via `api/cluster/member-action`

Existing local modals and form components remain the source of truth for inbound,
TLS, client, service, rule, and outbound editing. The remote node page supplies a
remote adapter to those workflows and refreshes the active tab after each
successful mutation.

This approach keeps visual behavior and field coverage aligned with local panel
behavior while avoiding a second independent CRUD UI.

## Backend Action Design

### Capability Discovery

Each node reports supported actions from its runtime router. The node detail UI
uses these names to decide which commands are available.

Action names follow this pattern:

```text
<resource>.list
<resource>.get
<resource>.create
<resource>.update
<resource>.delete
```

Resources:

```text
inbound
client
tls
service
route
outbound
```

For compatibility, existing list action names stay unchanged, such as
`inbound.list` and `tls.list`.

### Request Payloads

List actions keep the current pagination payload:

```json
{
  "page": 1,
  "page_size": 10
}
```

Get actions use:

```json
{
  "id": 12
}
```

Create and update actions use:

```json
{
  "data": {
    "id": 0,
    "tag": "direct-12345"
  },
  "init_users": [1, 2, 3]
}
```

`init_users` is only meaningful for inbound creation. Empty or absent values are
treated as no initial user binding.

Delete actions use:

```json
{
  "id": 12
}
```

If a local service still requires a tag for deletion, the handler resolves the
ID to the current tag before calling the existing save path.

### Execution Path

Remote action handlers should call the same domain services used by local
`api/save` and partial load endpoints. The implementation should avoid adding a
parallel persistence path.

Expected behavior:

- Create/update/delete inbounds reuse `InboundService.Save`.
- Create/update/delete TLS reuse `TlsService.Save`.
- Other resources follow their existing local save services.
- Full single-item reads use the existing partial load helpers where available,
  such as `LoadPartialData` behavior for inbounds and clients.
- Errors are returned as cluster action responses with `status: "error"` and a
  useful `error_message`.

### Runtime Registration

The runtime should register CRUD handlers for all supported resources. Existing
list handlers remain registered. `proxy.create` can remain as a compatibility
shortcut if already used elsewhere, but the local-panel experience should use
resource CRUD actions.

## Frontend Design

### Node Detail Structure

`ClusterNodeDetail.vue` remains the entry route for remote node management.

The top area keeps:

- Node name
- Base URL
- Supported action count
- Back button
- Loading and error states

The tab body changes from generic read-only cards to resource-specific manager
sections. Each section mirrors the local page controls:

- Toolbar with Add where `<resource>.create` is supported
- Cards or tables matching the local resource view
- Edit button where `<resource>.update` and `<resource>.get` are supported
- Delete button where `<resource>.delete` is supported
- Clone button where create and get are supported
- Stats or resource-specific secondary actions only where the remote data and
  action support exist

Unsupported actions are hidden when the peer does not advertise them. If a peer
advertises the tab list action but no write actions, the tab remains read-only.

### Remote Data Adapter

Add a frontend module that exposes operations like:

```ts
list(resource, page, pageSize)
get(resource, id)
save(resource, action, data, options)
delete(resource, id)
```

The adapter builds cluster action payloads and calls `sendAction(nodeId, req)`.
It unwraps successful responses and normalizes errors into exceptions or toast
messages that match the local panel's handling style.

The remote node store should keep per-resource state:

- items
- total
- page
- page size
- loaded
- loading
- error

After a successful mutation, the store refreshes the affected resource. If the
mutation can affect related resources, it refreshes those too. Examples:

- TLS update refreshes TLS and inbounds because inbound link material can change.
- Inbound create/update/delete refreshes inbounds and clients if user bindings
  were affected.

### Modal Reuse

Existing local modal components should be made data-source aware. The preferred
interface is a small adapter prop rather than hard-coding remote conditionals in
each component.

Example responsibilities:

- `Inbound.vue` asks the adapter to load the full inbound for edit.
- `Inbound.vue` asks the adapter to save `inbounds` with `new` or `edit`.
- `Tls.vue` emits save data as it already does, and the page adapter persists it.
- Preset and key generation behavior stays local to the modal unless it requires
  target-node execution.

Where a modal currently depends on global local lists such as clients, TLS
configs, or tags, the remote page passes the remote equivalents.

### Existing ClusterCreateNode Modal

`ClusterCreateNode.vue` should not become the main management modal. It can be
removed, simplified, or kept only as a compatibility helper after the full
resource CRUD flow exists. The node detail page should prefer the same inbound
and TLS modals used by the local panel.

## Data Flow

Create inbound on remote node:

1. User opens remote node detail.
2. UI loads `inbound.list`, `client.list`, and `tls.list`.
3. User clicks Add on the Inbounds tab.
4. The reused inbound modal builds the same inbound object as local creation.
5. Remote adapter sends `inbound.create` through local
   `api/cluster/member-action`.
6. Local backend resolves the target node and forwards to
   `/_cluster/v1/action` with the peer token.
7. Target backend calls the same save service used by local panel saves.
8. Response returns to the browser without exposing the peer token.
9. UI refreshes inbounds and any affected related resources.

Edit TLS on remote node:

1. User opens the TLS tab and clicks edit.
2. UI sends `tls.get` for the full config.
3. Existing TLS modal edits the object.
4. Remote adapter sends `tls.update`.
5. Target backend saves TLS and restarts or refreshes dependent resources through
   existing local service behavior.
6. UI refreshes TLS and inbounds.

## Error Handling

- Missing peer connection or token: local API returns the existing cluster member
  error and the UI shows a failure toast.
- Peer does not support an action: UI hides the command when possible; if the
  action still returns `unsupported`, show a concise unsupported-operation toast.
- Validation errors from target services: return `status: "error"` with
  `error_message`; show that message unchanged when safe.
- Stale IDs during edit/delete: refresh the active resource after the error so
  the page reflects current remote state.
- Partial refresh failures after a successful mutation: keep the success toast,
  show a refresh error, and leave the tab marked stale.

## Testing

Backend tests:

- Runtime advertises CRUD actions for supported resources.
- CRUD handlers call the expected service path with normalized action names.
- Inbound delete by ID resolves to the current tag before using the existing
  local delete path.
- Unsupported or invalid payloads return action errors rather than panics.
- Existing list handler tests continue to pass.

Frontend tests:

- Remote adapter builds the expected action request for list/get/create/update
  and delete.
- Node detail hides write buttons when peer capabilities do not include the
  required actions.
- Successful save refreshes the affected resource state.
- Action error responses surface a useful failure message.

Manual verification:

- Build frontend with type checking.
- Run targeted Go tests for cluster action handlers and API proxying.
- In a dev panel, open a remote cluster node and create/edit/delete at least one
  inbound and one TLS config.

## Rollout and Compatibility

Older peers that only support list actions remain usable in read-only mode.
Newer peers advertise CRUD actions and get full management controls.

This keeps the node detail page safe across mixed-version clusters and avoids
requiring all nodes to upgrade before the UI can be deployed.
