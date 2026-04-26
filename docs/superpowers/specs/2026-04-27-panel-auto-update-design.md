# Design: Domain-Wide Panel Auto-Update

## Summary

Enforce a unified panel software version across all members in a cluster domain.
Every member must run the latest panel version reported within the domain.
When a newer version is detected, the hub coordinates the update via a
claim-based mechanism to prevent duplicate updates, and the update is
broadcast to all members before execution.

## Data Model

### b-cluster-hub: MemberRecord

```
panel_version: string | null   -- panel software version (e.g. "v0.1.22")
status: 'online' | 'offline'   -- defaults to 'online'
```

### b-cluster-hub: DomainRecord

```
update_target_version: string | null   -- target version for current update cycle;
                                          cleared when all members reach it
```

### b-ui: ClusterMember (Go model)

```
PanelVersion string `json:"panelVersion"`
Status       string `json:"status"`       // "online" | "offline"
```

### b-ui: Frontend types

```typescript
interface ClusterMember {
  // ... existing fields
  panelVersion: string
  status: 'online' | 'offline'
}
```

## Hub API Changes

### POST /v1/domains/:id/claim-update

Request:
```json
{ "request_id": "...", "domain_token": "...", "target_version": "v0.1.23" }
```

Response logic:
- `update_target_version` is null → set it, return `{ proceed: true, target_version }`
- `update_target_version` already set → return `{ proceed: false }`

### POST /v1/domains/:id/member-status

Request:
```json
{ "request_id": "...", "domain_token": "...", "member_id": "...", "status": "offline" }
```

Updates the member's `status` field in the domain.

### Snapshot changes

Each member in GET /v1/domains/:id/snapshot now includes:
```json
{ "panel_version": "v0.1.22", "status": "online", ... }
```

## Claim-Based Dedup Flow

```
Member A detects version gap          Member B detects version gap (simultaneously)
        │                                      │
        ▼                                      ▼
POST /v1/domains/:id/claim-update      POST /v1/domains/:id/claim-update
  { target_version: "v0.1.23" }          { target_version: "v0.1.23" }
        │                                      │
        ▼                                      ▼
Hub: update_target_version == null      Hub: update_target_version == "v0.1.23"
  → set "v0.1.23"                         → { proceed: false }
  → { proceed: true }                      │
        │                                  ▼
        ▼                              Member B waits (will get broadcast)
Member A owns the update cycle
```

## Full Update Flow

### Trigger: sync / register pulls hub snapshot

```
GET /v1/domains/:id/snapshot
        │
        ▼
max_version = max(members[*].panel_version)
        │
        ├── own_version >= max_version → no-op, update local DB
        │
        ▼ own_version < max_version
POST /v1/domains/:id/claim-update
        │
        ├── { proceed: false } → another node owns the cycle, wait for broadcast
        │
        ▼ { proceed: true }
For each other member in domain:
  POST {member_base_url}/_cluster/v1/events
    { action: "domain.panel.update.available",
      payload: { target_version: max_version } }
        │
        ▼
Self: call StartUpdate(target_version, force=true)
  → POST /v1/domains/:id/member-status { status: "offline" }
  → download install.sh
  → systemd-run install.sh --force-update <target_version>
  → stop old binary, install new, restart
        │
        ▼
After restart: trigger sync
  → POST /v1/domains/:id/member-status { status: "online" }
  → hub returns snapshot with updated panel_version
```

### Other members receiving broadcast

```
POST /_cluster/v1/events
  { action: "domain.panel.update.available",
    payload: { target_version } }
        │
        ▼
target_version > own_panel_version?
        ├── no → ignore
        ▼ yes
Call StartUpdate(target_version, force=true)
  → set status offline → download → install → restart → sync → status online
```

### Hub clears update_target_version

After each sync (member status update), hub checks:
- All members' `panel_version >= update_target_version`?
  - Yes → clear `update_target_version`
  - No → keep waiting

## Files Changed

### b-cluster-hub (4 files)

| File | Change |
|------|--------|
| `src/types.ts` | `MemberRecord`: add `panel_version`, `status`. `DomainRecord`: add `update_target_version` |
| `src/index.ts` | Register stores `panel_version`+`status`. Snapshot outputs both. New `/claim-update` and `/member-status` endpoints. Version badge: `ver.N` |
| `src/store.ts` | SQL: `panel_version`, `status` on members; `update_target_version` on domains |
| `src/html.ts` | Badge `version-N` → `ver.N`. Member table: `panel_version` + `status` columns |

### b-ui backend (6 files)

| File | Change |
|------|--------|
| `model.go` | `ClusterMember`: add `PanelVersion`, `Status` |
| `cluster_hub_client.go` | Types: `PanelVersion`, `Status`, `UpdateTargetVersion`. Methods: `ClaimUpdate()`, `SetMemberStatus()` |
| `cluster_service.go` | `SyncDomain()`: compare versions, claim, broadcast, trigger self-update. `HandleUpdateAvailable()`: receive broadcast, trigger update. `ReportStatus()`: set online/offline |
| `cluster_runtime.go` | `BroadcastUpdateAvailable()`: iterate members, send `domain.panel.update.available` |
| `cluster_peer_message.go` | Register action `domain.panel.update.available`; validate payload |
| `panel_update.go` | No structural change — `StartUpdate()` already handles unattended mode |

### b-ui frontend (2 files)

| File | Change |
|------|--------|
| `types/clusters.ts` | `ClusterMember`: add `panelVersion`, `status` |
| `ClusterCenter.vue` | Member table shows `panelVersion` + status indicator |

## Edge Cases

- **Simultaneous claim**: Hub serializes — first one wins, second gets `proceed: false`.
- **Update fails**: Panel stays at current version, status set back to `online` on next sync. Next poll cycle detects gap again, re-attempts.
- **New member joins with old version**: Join flow pulls snapshot → detects gap → `claim-update` → if win, broadcast and self-update.
- **Single member domain**: Broadcast iterates 0 other members, only self-update runs.
- **Multiple domains, different versions**: Each domain's `update_target_version` is independent. No cross-domain interference.
- **Member offline during broadcast**: Delivery is best-effort (existing peer delivery with retry). Member catches up on next sync.

## Hub UI Display

- Domain list: `ver.N` badge (was `version-N`)
- Member table: columns for `panel_version` and `status` (online/offline indicator)
