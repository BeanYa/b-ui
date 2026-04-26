# Panel Auto-Update Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Enforce a unified panel software version across all members in a cluster domain via hub-coordinated claim-based updates with peer broadcasting.

**Architecture:** b-cluster-hub stores `panel_version` and `status` per member plus `update_target_version` per domain. b-ui compares own version against domain max after sync/register, claims an update cycle at the hub to prevent duplicates, broadcasts `domain.panel.update.available` to peers, then self-updates via the existing unattended `StartUpdate()` path.

**Tech Stack:** TypeScript (Cloudflare Workers, D1), Go (Gin, GORM), Vue 3 + Vuetify

---

### Task 1: b-cluster-hub — D1 Migration (members + domains)

**Files:**
- Create: `migrations/0008_member_panel_version.sql`

- [ ] **Step 1: Create the migration file**

```sql
ALTER TABLE members ADD COLUMN panel_version TEXT;
ALTER TABLE members ADD COLUMN status TEXT NOT NULL DEFAULT 'online';
ALTER TABLE domains ADD COLUMN update_target_version TEXT;

UPDATE members
SET status = 'online'
WHERE status IS NULL OR status = '';

UPDATE domains
SET update_target_version = NULL
WHERE update_target_version = '';
```

- [ ] **Step 2: Apply migration locally**

```bash
cd C:/universe/workspace/repo/b-project/b-cluster-hub
npx wrangler d1 execute HUB_DB --local --file=./migrations/0008_member_panel_version.sql
```

- [ ] **Step 3: Apply migration remotely**

```bash
npx wrangler d1 execute HUB_DB --remote --file=./migrations/0008_member_panel_version.sql
```

- [ ] **Step 4: Commit**

```bash
git add migrations/0008_member_panel_version.sql
git commit -m "feat: add panel_version, status to members and update_target_version to domains"
```

---

### Task 2: b-cluster-hub — Update Types

**Files:**
- Modify: `src/types.ts`

- [ ] **Step 1: Add `panel_version` and `status` to `MemberRecord`**

Change `MemberRecord`:
```typescript
export type MemberRecord = {
  member_id: string;
  node_id: string;
  address: string;
  base_url: string;
  public_key: string;
  name?: string;
  display_name?: string;
  panel_version?: string;      // <-- add
  status: 'online' | 'offline'; // <-- add
  peer_token: string;
  joined_at: number;
  updated_at: number;
};
```

- [ ] **Step 2: Add `update_target_version` to `DomainRecord`**

Change `DomainRecord`:
```typescript
export type DomainRecord = {
  domain_id: string;
  domain_token_hash: string;
  domain_token_encrypted: string;
  communication_endpoint_path: string;
  communication_protocol_version: string;
  version: number;
  update_target_version?: string | null; // <-- add
  created_at: number;
  updated_at: number;
};
```

- [ ] **Step 3: Add same fields to `AdminMemberRecord`**

Change `AdminMemberRecord`:
```typescript
export type AdminMemberRecord = {
  member_id: string;
  node_id: string;
  address: string;
  base_url: string;
  public_key: string;
  name?: string;
  display_name?: string;
  panel_version?: string;      // <-- add
  status?: 'online' | 'offline'; // <-- add
  joined_at: number;
  updated_at: number;
};
```

- [ ] **Step 4: Commit**

```bash
git add src/types.ts
git commit -m "feat: add panel_version, status, update_target_version to hub types"
```

---

### Task 3: b-cluster-hub — Update Store Layer

**Files:**
- Modify: `src/store.ts`

- [ ] **Step 1: Update `StoredMemberRecord`**

Add to `StoredMemberRecord`:
```typescript
type StoredMemberRecord = {
  domain_id: string;
  member_id: string;
  node_id: string;
  address: string;
  base_url: string;
  public_key: string;
  name: string | null;
  display_name: string | null;
  panel_version: string | null;   // <-- add
  status: string | null;          // <-- add
  peer_token: string;
  joined_at: number;
  updated_at: number;
};
```

- [ ] **Step 2: Update `getDomain()` SQL SELECT**

Change the SELECT in `getDomain()`:
```typescript
`SELECT domain_id, domain_token_hash, COALESCE(domain_token_encrypted, '') AS domain_token_encrypted,
        COALESCE(communication_endpoint_path, ?2) AS communication_endpoint_path,
        COALESCE(communication_protocol_version, ?3) AS communication_protocol_version,
        version, COALESCE(update_target_version, '') AS update_target_version, created_at, updated_at
 FROM domains
 WHERE domain_id = ?1`,
```

- [ ] **Step 3: Update `setDomain()` SQL**

Change the INSERT/UPDATE in `setDomain()` to include `update_target_version`:
```typescript
`INSERT INTO domains (domain_id, domain_token_hash, domain_token_encrypted, communication_endpoint_path, communication_protocol_version, version, update_target_version, created_at, updated_at)
 VALUES (?1, ?2, ?3, ?4, ?5, ?6, ?7, ?8, ?9)
 ON CONFLICT(domain_id) DO UPDATE SET
   domain_token_hash = excluded.domain_token_hash,
   domain_token_encrypted = excluded.domain_token_encrypted,
   communication_endpoint_path = excluded.communication_endpoint_path,
   communication_protocol_version = excluded.communication_protocol_version,
   version = excluded.version,
   update_target_version = excluded.update_target_version,
   created_at = excluded.created_at,
   updated_at = excluded.updated_at`,
```

And bind call:
```typescript
).bind(
  normalizedDomain.domain_id,
  normalizedDomain.domain_token_hash,
  storedDomainToken,
  normalizedDomain.communication_endpoint_path,
  normalizedDomain.communication_protocol_version,
  normalizedDomain.version,
  normalizedDomain.update_target_version ?? null,  // <-- add
  normalizedDomain.created_at,
  normalizedDomain.updated_at,
).run();
```

- [ ] **Step 4: Update `listDomains()` SQL SELECT**

Add `update_target_version` to the SELECT:
```typescript
`SELECT domain_id, domain_token_hash, COALESCE(domain_token_encrypted, '') AS domain_token_encrypted,
        COALESCE(communication_endpoint_path, ?1) AS communication_endpoint_path,
        COALESCE(communication_protocol_version, ?2) AS communication_protocol_version,
        version, COALESCE(update_target_version, '') AS update_target_version, created_at, updated_at
 FROM domains
 ORDER BY domain_id ASC`,
```

- [ ] **Step 5: Update `getMember()` and `listMembers()` SQL SELECT**

Change the SELECT in `getMember()` and `listMembers()`:
```typescript
`SELECT domain_id, member_id, node_id, address, base_url, public_key, name, display_name, panel_version, status, peer_token, joined_at, updated_at
   FROM members
   ...`,
```

- [ ] **Step 6: Update `setMember()` SQL**

Change `setMember()`:
```typescript
`INSERT INTO members (domain_id, member_id, node_id, address, base_url, public_key, name, display_name, panel_version, status, peer_token, joined_at, updated_at)
  VALUES (?1, ?2, ?3, ?4, ?5, ?6, ?7, ?8, ?9, ?10, ?11, ?12, ?13)
  ON CONFLICT(domain_id, member_id) DO UPDATE SET
    node_id = excluded.node_id,
    address = excluded.address,
    base_url = excluded.base_url,
    public_key = excluded.public_key,
    name = excluded.name,
    display_name = excluded.display_name,
    panel_version = excluded.panel_version,
    status = excluded.status,
    peer_token = excluded.peer_token,
    joined_at = excluded.joined_at,
    updated_at = excluded.updated_at`,
```

And bind call:
```typescript
).bind(
  domainId,
  member.member_id,
  member.node_id,
  member.address,
  member.base_url,
  member.public_key,
  member.name ?? null,
  member.display_name ?? null,
  member.panel_version ?? null,   // <-- add
  member.status ?? 'online',      // <-- add
  storedPeerToken,
  member.joined_at,
  member.updated_at,
).run();
```

- [ ] **Step 7: Update `toMemberRecord()`**

Add `panel_version` and `status` to the returned object in `toMemberRecord()`:
```typescript
return {
  member_id: member.member_id,
  node_id: member.node_id,
  address: member.address,
  base_url: member.base_url,
  public_key: member.public_key,
  name: member.name ?? undefined,
  display_name: member.display_name ?? undefined,
  panel_version: member.panel_version ?? undefined,  // <-- add
  status: (member.status as 'online' | 'offline') ?? 'online', // <-- add
  peer_token: peerToken,
  joined_at: member.joined_at,
  updated_at: member.updated_at,
};
```

- [ ] **Step 8: Update `toAdminMemberRecord()`**

Add new fields in `toAdminMemberRecord()`:
```typescript
function toAdminMemberRecord(member: MemberRecord): AdminMemberRecord {
  return {
    member_id: member.member_id,
    node_id: member.node_id,
    address: member.address,
    base_url: member.base_url,
    public_key: member.public_key,
    name: member.name,
    display_name: member.display_name,
    panel_version: member.panel_version,     // <-- add
    status: member.status,                   // <-- add
    joined_at: member.joined_at,
    updated_at: member.updated_at,
  };
}
```

- [ ] **Step 9: Run tests**

```bash
cd C:/universe/workspace/repo/b-project/b-cluster-hub && npx vitest run
```

- [ ] **Step 10: Commit**

```bash
git add src/store.ts
git commit -m "feat: add panel_version, status, update_target_version to hub store"
```

---

### Task 4: b-cluster-hub — Update API Endpoints (index.ts)

**Files:**
- Modify: `src/index.ts`

- [ ] **Step 1: Update `isRegisterMember()` to accept `panel_version`**

Add `panel_version?: string` validation:
```typescript
function isRegisterMember(value: unknown): value is {
  member_id: string;
  node_id?: string;
  address: string;
  base_url?: string;
  public_key?: string;
  name?: string;
  display_name?: string;
  panel_version?: string;   // <-- add
} {
  return (
    isRecord(value)
    && isNonEmptyString(value.member_id)
    && isNonEmptyString(value.address)
    && isOptionalString(value.node_id)
    && isOptionalString(value.base_url)
    && isOptionalString(value.public_key)
    && isOptionalString(value.name)
    && isOptionalString(value.display_name)
    && isOptionalString(value.panel_version)  // <-- add
  );
}
```

- [ ] **Step 2: Update `normalizeRegisterMember()` return type to include `panel_version`** and pass it through:

```typescript
function normalizeRegisterMember(member: {
  member_id: string;
  node_id?: string;
  address: string;
  base_url?: string;
  public_key?: string;
  name?: string;
  display_name?: string;
  panel_version?: string;   // <-- add
}): Omit<MemberRecord, 'peer_token' | 'joined_at' | 'updated_at'> {
  const baseURL = member.base_url?.trim() ?? '';
  if (baseURL && !isUsablePeerBaseURL(baseURL)) {
    throw new Error('invalid_member_base_url');
  }
  const resolvedDisplayName = member.display_name?.trim() || member.name?.trim() || deriveDisplayNameFromBaseURL(baseURL);
  return {
    member_id: member.member_id,
    node_id: member.node_id || member.member_id,
    address: member.address,
    base_url: baseURL,
    public_key: member.public_key || '',
    display_name: resolvedDisplayName,
    panel_version: member.panel_version?.trim() || undefined,  // <-- add
    status: 'online',                                          // <-- add (default)
    ...(member.name ? { name: member.name } : {}),
  };
}
```

- [ ] **Step 3: Update register handler to store `panel_version` and `status`**

In the `setMember()` call inside the register handler:
```typescript
await store.setMember(domainId, {
  member_id: normalizedMember.member_id,
  node_id: normalizedMember.node_id,
  address: normalizedMember.address,
  base_url: normalizedMember.base_url,
  public_key: normalizedMember.public_key,
  display_name: normalizedMember.display_name,
  name: normalizedMember.name,
  panel_version: normalizedMember.panel_version,  // <-- add
  status: 'online',                                 // <-- add
  peer_token: currentMember?.peer_token || generatePeerToken(),
  joined_at: currentMember?.joined_at ?? now,
  updated_at: now,
});
```

- [ ] **Step 4: Update `hasMemberChanges()` to compare `panel_version`**

```typescript
function hasMemberChanges(
  currentMember: MemberRecord,
  nextMember: Omit<MemberRecord, 'peer_token' | 'joined_at' | 'updated_at'>,
): boolean {
  return currentMember.node_id !== nextMember.node_id
    || currentMember.address !== nextMember.address
    || currentMember.base_url !== nextMember.base_url
    || currentMember.public_key !== nextMember.public_key
    || currentMember.name !== nextMember.name
    || currentMember.display_name !== nextMember.display_name
    || currentMember.panel_version !== nextMember.panel_version   // <-- add
    || currentMember.status !== nextMember.status;                // <-- add
}
```

- [ ] **Step 5: Update `toPublicSnapshot()` to output `panel_version` and `status`**

```typescript
function toPublicSnapshot(domain: DomainRecord, members: MemberRecord[]) {
  return {
    domain_id: domain.domain_id,
    version: domain.version,
    update_target_version: domain.update_target_version || undefined,  // <-- add
    communication: {
      endpoint_path: domain.communication_endpoint_path || CLUSTER_COMMUNICATION_ENDPOINT_PATH,
      protocol_version: domain.communication_protocol_version || CLUSTER_COMMUNICATION_PROTOCOL_VERSION,
    },
    members: members.map((member) => {
      const publicMember: Record<string, string> = {
        member_id: member.member_id,
        node_id: member.node_id,
        address: member.address,
        base_url: member.base_url,
        public_key: member.public_key,
        peer_token: member.peer_token,
        display_name: member.display_name || member.name || member.node_id,
        panel_version: member.panel_version || '',   // <-- add
        status: member.status || 'online',            // <-- add
      };
      if (member.name) {
        publicMember.name = member.name;
      }
      return publicMember;
    }),
  };
}
```

- [ ] **Step 6: Add `/claim-update` route in HubRegistry.fetch()**

After the existing snapshot route, add:
```typescript
const claimUpdateMatch = matchPath(url.pathname, '/v1/domains/:domainId/claim-update');
if (claimUpdateMatch && request.method === 'POST') {
  const parsed = await parseJsonRequest(request);
  if ('error' in parsed) {
    return parsed.error;
  }

  return this.withHubLock(async () => {
    const body = parsed.value;
    if (!isClaimUpdateRequestBody(body)) {
      return jsonResponse({ error: 'invalid_request' }, 400);
    }

    const store = getHubStore(this.env);
    const domainId = normalizeDomainId(claimUpdateMatch.domainId);
    const domain = await store.getDomain(domainId);
    if (!domain) {
      return jsonResponse({ error: 'domain_not_found' }, 404);
    }

    const tokenHash = await hashValue(body.domain_token);
    if (domain.domain_token_hash !== tokenHash) {
      return jsonResponse({ error: 'invalid_domain_token' }, 403);
    }

    if (domain.update_target_version) {
      return jsonResponse({ proceed: false });
    }

    const now = Date.now();
    await store.setDomain({
      ...domain,
      update_target_version: body.target_version,
      updated_at: now,
    });

    return jsonResponse({ proceed: true, target_version: body.target_version });
  });
}
```

- [ ] **Step 7: Add `/member-status` route in HubRegistry.fetch()**

After the claim-update route, add:
```typescript
const memberStatusMatch = matchPath(url.pathname, '/v1/domains/:domainId/member-status');
if (memberStatusMatch && request.method === 'POST') {
  const parsed = await parseJsonRequest(request);
  if ('error' in parsed) {
    return parsed.error;
  }

  return this.withHubLock(async () => {
    const body = parsed.value;
    if (!isMemberStatusRequestBody(body)) {
      return jsonResponse({ error: 'invalid_request' }, 400);
    }

    const store = getHubStore(this.env);
    const domainId = normalizeDomainId(memberStatusMatch.domainId);
    const domain = await store.getDomain(domainId);
    if (!domain) {
      return jsonResponse({ error: 'domain_not_found' }, 404);
    }

    const tokenHash = await hashValue(body.domain_token);
    if (domain.domain_token_hash !== tokenHash) {
      return jsonResponse({ error: 'invalid_domain_token' }, 403);
    }

    const member = await store.getMember(domainId, body.member_id);
    if (!member) {
      return jsonResponse({ error: 'member_not_found' }, 404);
    }

    member.status = body.status;
    if (body.panel_version) {
      member.panel_version = body.panel_version;
    }
    member.updated_at = Date.now();
    await store.setMember(domainId, member);

    // Clear update_target_version when all members are at or above target
    if (domain.update_target_version) {
      const members = await store.listMembers(domainId);
      const allUpToDate = members.every((m) => {
        const memberVer = normalizeReleaseVersionForCompare(m.panel_version || '');
        const targetVer = normalizeReleaseVersionForCompare(domain.update_target_version!);
        if (!memberVer || !targetVer) return false;
        return compareReleaseTags(memberVer, targetVer) >= 0;
      });
      if (allUpToDate) {
        await store.setDomain({
          ...domain,
          update_target_version: null,
        });
      }
    }

    return jsonResponse({ ok: true });
  });
}
```

- [ ] **Step 8: Add `isClaimUpdateRequestBody()` and `isMemberStatusRequestBody()` validators**

```typescript
function isClaimUpdateRequestBody(value: unknown): value is {
  request_id: string;
  domain_token: string;
  target_version: string;
} {
  return (
    isRecord(value)
    && isNonEmptyString(value.request_id)
    && isNonEmptyString(value.domain_token)
    && isNonEmptyString(value.target_version)
  );
}

function isMemberStatusRequestBody(value: unknown): value is {
  request_id: string;
  domain_token: string;
  member_id: string;
  status: 'online' | 'offline';
  panel_version?: string;
} {
  return (
    isRecord(value)
    && isNonEmptyString(value.request_id)
    && isNonEmptyString(value.domain_token)
    && isNonEmptyString(value.member_id)
    && (value.status === 'online' || value.status === 'offline')
    && isOptionalString(value.panel_version)
  );
}
```

- [ ] **Step 9: Add `normalizeReleaseVersionForCompare()` and `compareReleaseTags()` helpers**

```typescript
function normalizeReleaseVersionForCompare(version: string): string {
  const trimmed = version.trim();
  return trimmed.startsWith('v') ? trimmed.slice(1) : trimmed;
}

function compareReleaseTags(a: string, b: string): number {
  const aParts = a.split('.').map(Number);
  const bParts = b.split('.').map(Number);
  for (let i = 0; i < Math.max(aParts.length, bParts.length); i++) {
    const av = aParts[i] || 0;
    const bv = bParts[i] || 0;
    if (av < bv) return -1;
    if (av > bv) return 1;
  }
  return 0;
}
```

- [ ] **Step 10: Add `/claim-update` and `/member-status` to the worker proxy routing**

In the outer `fetch()`:
```typescript
const claimUpdateMatch = matchPath(url.pathname, '/v1/domains/:domainId/claim-update');
if (claimUpdateMatch && request.method === 'POST') {
  return proxyToHub(env, `/claim-update/${claimUpdateMatch.domainId}`, request);
}

const memberStatusMatch = matchPath(url.pathname, '/v1/domains/:domainId/member-status');
if (memberStatusMatch && request.method === 'POST') {
  return proxyToHub(env, `/member-status/${memberStatusMatch.domainId}`, request);
}
```

- [ ] **Step 11: Run tests**

```bash
cd C:/universe/workspace/repo/b-project/b-cluster-hub && npx vitest run
```

- [ ] **Step 12: Commit**

```bash
git add src/index.ts
git commit -m "feat: add claim-update and member-status endpoints to hub API"
```

---

### Task 5: b-cluster-hub — Update Admin UI (html.ts)

**Files:**
- Modify: `src/html.ts`

- [ ] **Step 1: Update local `MemberRecord` type at top of html.ts**

```typescript
type MemberRecord = {
  member_id: string;
  node_id?: string;
  address: string;
  base_url?: string;
  name?: string;
  display_name?: string;
  panel_version?: string;  // <-- add
  status?: 'online' | 'offline'; // <-- add
  updated_at?: number;
};
```

- [ ] **Step 2: Update `formatVersionLabel()`**

```typescript
function formatVersionLabel(version: number): string {
  return `ver.${version}`;
}
```

- [ ] **Step 3: Update `renderMemberTable()` to show `panel_version` and `status`**

Add columns to the table head:
```html
<th>Member</th>
<th>Panel Ver.</th>
<th>Status</th>
<th>Node</th>
<th>Base URL</th>
<th>Action</th>
```

Add table cells:
```typescript
const panelVerLabel = member.panel_version || '—';
const statusClass = member.status === 'offline' ? 'status-badge status-badge--offline' : 'status-badge status-badge--online';
const statusLabel = member.status === 'offline' ? 'Offline' : 'Online';

return `<tr>
  <td data-label="Member">
    <div class="member-cell__primary">${escapeHtml(memberPrimary)}</div>
    ${memberSecondary}
  </td>
  <td data-label="Panel Ver.">
    <span class="mono-copy">${escapeHtml(panelVerLabel)}</span>
  </td>
  <td data-label="Status">
    <span class="${statusClass}">${statusLabel}</span>
  </td>
  <td data-label="Node">
    <div class="mono-copy">${escapeHtml(nodeLabel)}</div>
  </td>
  ...`;
```

- [ ] **Step 4: Add CSS for status badges** at the end of the stylesheet:

```css
.status-badge {
  display: inline-block;
  padding: 2px 8px;
  border-radius: 10px;
  font-size: 0.75rem;
  font-weight: 600;
}
.status-badge--online {
  background: #dcfce7;
  color: #166534;
}
.status-badge--offline {
  background: #fee2e2;
  color: #991b1b;
}
```

- [ ] **Step 5: Commit**

```bash
git add src/html.ts
git commit -m "feat: show panel_version and status in hub admin UI, change version-N to ver.N"
```

---

### Task 6: b-ui — Add PanelVersion and Status to Go Model

**Files:**
- Modify: `src/backend/internal/infra/db/model/model.go`

- [ ] **Step 1: Add fields to `ClusterMember`**

```go
type ClusterMember struct {
	Id                 uint           `json:"id" gorm:"primaryKey;autoIncrement"`
	NodeID             string         `json:"nodeId" gorm:"uniqueIndex:idx_cluster_domain_node"`
	Name               string         `json:"name"`
	DisplayName        string         `json:"displayName"`
	PanelVersion       string         `json:"panelVersion"`            // <-- add
	Status             string         `json:"status" gorm:"default:online"` // <-- add
	BaseURL            string         `json:"baseUrl"`
	PublicKey          string         `json:"publicKey"`
	PeerTokenEncrypted string         `json:"-"`
	DomainID           uint           `json:"domainId" gorm:"uniqueIndex:idx_cluster_domain_node"`
	LastVersion        int64          `json:"lastVersion" gorm:"default:0"`
	LastNotifiedAt     int64          `json:"lastNotifiedAt" gorm:"default:0"`
	LastNotifiedValue  int64          `json:"lastNotifiedValue" gorm:"default:0"`
	Domain             *ClusterDomain `json:"domain,omitempty" gorm:"foreignKey:DomainID;references:Id"`
}
```

- [ ] **Step 2: Verify Go compilation**

```bash
cd C:/universe/workspace/repo/b-project/b-ui && go build ./...
```

- [ ] **Step 3: Commit**

```bash
git add src/backend/internal/infra/db/model/model.go
git commit -m "feat: add PanelVersion and Status fields to ClusterMember model"
```

---

### Task 7: b-ui — Update Hub Client

**Files:**
- Modify: `src/backend/internal/domain/services/cluster_hub_client.go`

- [ ] **Step 1: Add `PanelVersion` and `Status` to `ClusterHubMemberRegister`**

```go
type ClusterHubMemberRegister struct {
	MemberID    string `json:"member_id"`
	NodeID      string `json:"node_id"`
	Address     string `json:"address"`
	BaseURL     string `json:"base_url"`
	PublicKey   string `json:"public_key"`
	Name        string `json:"name,omitempty"`
	DisplayName string `json:"display_name,omitempty"`
	PanelVersion string `json:"panel_version,omitempty"`  // <-- add
	Status      string `json:"status,omitempty"`           // <-- add
}
```

- [ ] **Step 2: Add `PanelVersion` and `Status` to `ClusterHubMemberResponse`**

```go
type ClusterHubMemberResponse struct {
	MemberID        string `json:"member_id"`
	NodeID          string `json:"nodeId"`
	NodeIDAlt       string `json:"node_id"`
	Name            string `json:"name"`
	DisplayName     string `json:"displayName"`
	DisplayNameAlt  string `json:"display_name"`
	BaseURL         string `json:"baseUrl"`
	BaseURLAlt      string `json:"base_url"`
	PublicKey       string `json:"publicKey"`
	PublicKeyAlt    string `json:"public_key"`
	PeerToken       string `json:"peerToken"`
	PeerTokenAlt    string `json:"peer_token"`
	Address         string `json:"address"`
	PanelVersion    string `json:"panel_version"`   // <-- add (hub always uses snake_case in snapshot)
	Status          string `json:"status"`           // <-- add
}
```

- [ ] **Step 3: Add `EffectivePanelVersion()` and `EffectiveStatus()` methods**

```go
func (m ClusterHubMemberResponse) EffectivePanelVersion() string {
	return m.PanelVersion
}

func (m ClusterHubMemberResponse) EffectiveStatus() string {
	if m.Status != "" {
		return m.Status
	}
	return "online"
}
```

- [ ] **Step 4: Add `UpdateTargetVersion` to `ClusterHubSnapshotResponse` and `SetMemberStatus()` + `ClaimUpdate()` client methods**

Add to `ClusterHubSnapshotResponse`:
```go
type ClusterHubSnapshotResponse struct {
	DomainID             string                          `json:"domain_id"`
	Version              int64                           `json:"version"`
	UpdateTargetVersion  string                          `json:"update_target_version,omitempty"` // <-- add
	Communication        ClusterHubCommunicationResponse `json:"communication"`
	Members              []ClusterHubMemberResponse      `json:"members"`
}
```

Add `ClaimUpdate()` and `SetMemberStatus()` to the `clusterHubClient` interface:
```go
type clusterHubClient interface {
	RegisterNode(context.Context, string, ClusterHubRegisterNodeRequest) (*ClusterHubOperationResponse, error)
	GetLatestVersion(context.Context, string, string, string) (*ClusterHubVersionResponse, error)
	GetSnapshot(context.Context, string, string, string) (*ClusterHubSnapshotResponse, error)
	DeleteMember(context.Context, string, string, string, string) (*ClusterHubOperationResponse, error)
	ClaimUpdate(context.Context, string, string, string, string, string) (*ClusterHubClaimUpdateResponse, error)    // <-- add
	SetMemberStatus(context.Context, string, string, string, string, string, string) (*ClusterHubMemberStatusResponse, error) // <-- add
}
```

Add response types and methods:

```go
type ClusterHubClaimUpdateResponse struct {
	Proceed       bool   `json:"proceed"`
	TargetVersion string `json:"target_version,omitempty"`
}

type ClusterHubMemberStatusResponse struct {
	OK bool `json:"ok"`
}
```

Add `ClaimUpdate()`:
```go
func (c *ClusterHubClient) ClaimUpdate(ctx context.Context, hubURL string, domain string, domainToken string, requestID string, targetVersion string) (*ClusterHubClaimUpdateResponse, error) {
	if err := validateClusterHubURL(hubURL); err != nil {
		return nil, err
	}
	payload := map[string]string{
		"request_id":     requestID,
		"domain_token":   domainToken,
		"target_version": targetVersion,
	}
	response := &ClusterHubClaimUpdateResponse{}
	if err := c.postJSON(ctx, strings.TrimRight(hubURL, "/")+"/v1/domains/"+domain+"/claim-update", payload, response); err != nil {
		return nil, err
	}
	return response, nil
}
```

Add `SetMemberStatus()`:
```go
func (c *ClusterHubClient) SetMemberStatus(ctx context.Context, hubURL string, domain string, domainToken string, requestID string, memberID string, status string, panelVersion string) (*ClusterHubMemberStatusResponse, error) {
	if err := validateClusterHubURL(hubURL); err != nil {
		return nil, err
	}
	payload := map[string]string{
		"request_id":   requestID,
		"domain_token": domainToken,
		"member_id":    memberID,
		"status":       status,
	}
	if panelVersion != "" {
		payload["panel_version"] = panelVersion
	}
	response := &ClusterHubMemberStatusResponse{}
	if err := c.postJSON(ctx, strings.TrimRight(hubURL, "/")+"/v1/domains/"+domain+"/member-status", payload, response); err != nil {
		return nil, err
	}
	return response, nil
}
```

- [ ] **Step 5: Verify Go compilation**

```bash
cd C:/universe/workspace/repo/b-project/b-ui && go build ./...
```

- [ ] **Step 6: Commit**

```bash
git add src/backend/internal/domain/services/cluster_hub_client.go
git commit -m "feat: add panel_version, status to hub client types and ClaimUpdate/SetMemberStatus methods"
```

---

### Task 8: b-ui — Add BroadcastUpdateAvailable to Broadcaster

**Files:**
- Modify: `src/backend/internal/domain/services/cluster_runtime.go`

- [ ] **Step 1: Add `BroadcastUpdateAvailable()` method to `ClusterHTTPBroadcaster`**

```go
func (b *ClusterHTTPBroadcaster) BroadcastUpdateAvailable(ctx context.Context, domainID uint, domainName string, targetVersion string, excludeNodeID string) error {
	identity, err := b.identity.GetOrCreate()
	if err != nil {
		return err
	}
	secret, err := b.getSecretProvider().GetSecret()
	if err != nil {
		return err
	}
	members, err := b.getStore().ListMembersWithDomain()
	if err != nil {
		return err
	}
	reachability := b.getReachability()
	for _, member := range members {
		if member.Domain == nil || member.Domain.Id != domainID {
			continue
		}
		if member.NodeID == excludeNodeID || member.NodeID == identity.NodeID || member.BaseURL == "" {
			continue
		}
		entry, err := reachability.load(member.DomainID, member.NodeID)
		if err != nil {
			continue
		}
		if entry.State == ClusterReachabilityUnreachable {
			shouldRetry, err := reachability.shouldProbeWithError(entry)
			if err != nil || !shouldRetry {
				continue
			}
		}
		token, err := DecryptClusterDomainToken(secret, member.PeerTokenEncrypted)
		if err != nil {
			continue
		}
		message, err := NewClusterPeerMessage(domainName, 0, identity.NodeID, 0, PeerCategoryEvent, "domain.panel.update.available", map[string]interface{}{
			"target_version": targetVersion,
		})
		if err != nil {
			continue
		}
		message.Route = RoutePlan{
			Mode: RouteModeBroadcast,
			Delivery: &DeliveryPolicy{
				Ack:       DeliveryAckNone,
				TimeoutMs: 10000,
				Retry: &RetryPolicy{
					MaxAttempts: 1,
					BackoffMs:   1000,
				},
			},
		}
		if err := SignClusterPeerMessage(identity, message); err != nil {
			continue
		}
		delivery := &ClusterPeerDeliveryService{HTTPClient: b.httpClient(), saveAckAttempt: b.getAckAttemptSaver()}
		_ = delivery.Send(ctx, message, member, token)
	}
	return nil
}
```

- [ ] **Step 2: Add `BroadcastUpdateAvailable` to the `clusterBroadcaster` interface in `cluster_sync.go`**

```go
type clusterBroadcaster interface {
	BroadcastNotifyVersion(context.Context, int64, string) error
	BroadcastUpdateAvailable(context.Context, uint, string, string, string) error  // <-- add
}
```

- [ ] **Step 3: Verify Go compilation**

```bash
cd C:/universe/workspace/repo/b-project/b-ui && go build ./...
```

- [ ] **Step 4: Commit**

```bash
git add src/backend/internal/domain/services/cluster_runtime.go src/backend/internal/domain/services/cluster_sync.go
git commit -m "feat: add BroadcastUpdateAvailable to cluster broadcaster"
```

---

### Task 9: b-ui — Handle Update Available in Peer Dispatcher

**Files:**
- Modify: `src/backend/internal/domain/services/cluster_peer_dispatcher.go`
- Modify: `src/backend/internal/domain/services/cluster_peer_message.go` (check action validation)

- [ ] **Step 1: Add action constant**

In `cluster_peer_dispatcher.go`:
```go
const PeerActionDomainPanelUpdateAvailable = "domain.panel.update.available"
```

- [ ] **Step 2: Add handler dispatch in `Dispatch()`**

After the existing `domain.cluster.changed` handler block, add:
```go
if message.Category == PeerCategoryEvent && message.Action == PeerActionDomainPanelUpdateAvailable {
    if err := d.handleDomainPanelUpdateAvailable(ctx, domain, source, message); err != nil {
        if _, chainErr := d.completeChainStep(ctx, domain, message, PeerEventStatusFailed, err.Error()); chainErr != nil {
            _ = store.MarkEventState(state.MessageID, PeerEventStatusFailed, chainErr.Error())
            return chainErr
        }
        if markErr := store.MarkEventState(state.MessageID, PeerEventStatusFailed, err.Error()); markErr != nil {
            return markErr
        }
        return err
    }
    if _, err := d.completeChainStep(ctx, domain, message, PeerEventStatusSucceeded, ""); err != nil {
        _ = store.MarkEventState(state.MessageID, PeerEventStatusFailed, err.Error())
        return err
    }
    return store.MarkEventState(state.MessageID, PeerEventStatusSucceeded, "")
}
```

- [ ] **Step 3: Add handler function**

```go
func (d *ClusterPeerDispatcher) handleDomainPanelUpdateAvailable(ctx context.Context, domain *model.ClusterDomain, source *model.ClusterMember, message *PeerMessage) error {
	targetVersion, ok := message.Payload["target_version"].(string)
	if !ok || targetVersion == "" {
		return errors.New("invalid_payload_target_version")
	}

	currentVersion := config.GetVersion()
	if compareReleaseTags(currentVersion, targetVersion) != "older" {
		return nil // already at or above target, ignore
	}

	// Trigger self-update via the PanelService
	panelSvc := &PanelService{}
	_, err := panelSvc.StartUpdate(targetVersion, true)
	return err
}
```

- [ ] **Step 4: Verify Go compilation** (note: will need the `config` import)

```bash
cd C:/universe/workspace/repo/b-project/b-ui && go build ./...
```

- [ ] **Step 5: Commit**

```bash
git add src/backend/internal/domain/services/cluster_peer_dispatcher.go
git commit -m "feat: handle domain.panel.update.available in peer dispatcher"
```

---

### Task 10: b-ui — Core Update Logic in ClusterService

**Files:**
- Modify: `src/backend/internal/domain/services/cluster_service.go`

- [ ] **Step 1: Add `broadcaster` and `panelService` fields to `ClusterSyncService`**

```go
type ClusterSyncService struct {
	store        clusterSyncStore
	hubSyncer    clusterHubSyncer
	broadcaster  clusterBroadcaster
	panelService *PanelService  // <-- add
}
```

- [ ] **Step 2: Update `NewRuntimeClusterSyncService()`**

```go
func NewRuntimeClusterSyncService() ClusterSyncService {
	return ClusterSyncService{
		store:        &dbClusterSyncStore{},
		hubSyncer:    &ClusterHubSyncer{},
		panelService: &PanelService{},  // <-- add
	}
}
```

- [ ] **Step 3: Add `CheckAndBroadcastUpdate()` method**

This is called after `SyncDomain()` in the poll/sync path:

```go
func (s *ClusterSyncService) CheckAndBroadcastUpdate(ctx context.Context, domain *model.ClusterDomain) error {
	members, err := s.store.GetMembers(domain.Id)
	if err != nil {
		return err
	}

	currentVersion := canonicalizeReleaseTag(config.GetVersion())
	maxVersion := currentVersion
	for _, member := range members {
		mv := canonicalizeReleaseTag(member.PanelVersion)
		if compareReleaseTags(mv, maxVersion) == "newer" {
			maxVersion = mv
		}
	}

	if compareReleaseTags(currentVersion, maxVersion) != "older" {
		return nil
	}

	// Claim the update cycle
	hubClient := &ClusterHubClient{}
	secret, err := (&SettingService{}).GetSecret()
	if err != nil {
		return err
	}
	domainToken, err := DecryptClusterDomainToken(secret, domain.TokenEncrypted)
	if err != nil {
		return err
	}
	requestID := fmt.Sprintf("update-%d", time.Now().UnixNano())
	claimResp, err := hubClient.ClaimUpdate(ctx, domain.HubURL, domain.Domain, domainToken, requestID, maxVersion)
	if err != nil {
		return err
	}
	if !claimResp.Proceed {
		return nil // another node claimed the cycle
	}

	// Set ourselves offline
	local, err := (&ClusterLocalIdentityService{}).GetOrCreate()
	if err != nil {
		return err
	}
	_ = hubClient.SetMemberStatus(ctx, domain.HubURL, domain.Domain, domainToken, requestID+"-status", local.NodeID, "offline", "")

	// Broadcast to all other members
	if s.broadcaster != nil {
		_ = s.broadcaster.BroadcastUpdateAvailable(ctx, domain.Id, domain.Domain, maxVersion, local.NodeID)
	}

	// Trigger self-update
	_, err = s.panelService.StartUpdate(maxVersion, true)
	return err
}
```

- [ ] **Step 4: Update `PollAndNotifyVersion()` to call `CheckAndBroadcastUpdate()` after sync**

After `s.hubSyncer.SyncDomain(ctx, &domain, version)`:
```go
// Check for panel version updates after sync
_ = s.CheckAndBroadcastUpdate(ctx, &domain)
```

- [ ] **Step 5: Update `SyncDomain()` in `ClusterHubSyncer` to store `PanelVersion` and `Status`**

In `SyncDomain()` when building members:
```go
members = append(members, model.ClusterMember{
	NodeID:             item.EffectiveNodeID(),
	Name:               item.Name,
	DisplayName:        item.EffectiveDisplayName(),
	PanelVersion:       item.EffectivePanelVersion(),  // <-- add
	Status:             item.EffectiveStatus(),         // <-- add
	BaseURL:            item.EffectiveBaseURL(),
	PublicKey:          item.EffectivePublicKey(),
	PeerTokenEncrypted: peerTokenEncrypted,
	DomainID:           domain.Id,
	LastVersion:        snapshot.Version,
})
```

- [ ] **Step 6: Update `Register()` to pass `PanelVersion` and `Status` to hub**

In the `Register()` method where `ClusterHubMemberRegister` is constructed:
```go
member := ClusterHubMemberRegister{
	MemberID:     local.NodeID,
	NodeID:       local.NodeID,
	Address:      resolvePanelBaseUrl(),
	BaseURL:      resolvePanelBaseUrl(),
	PublicKey:    local.PublicKey,
	Name:         request.Name,
	DisplayName:  request.DisplayName,
	PanelVersion: canonicalizeReleaseTag(config.GetVersion()),  // <-- add
	Status:       "online",                                      // <-- add
}
```

- [ ] **Step 7: Verify Go compilation**

```bash
cd C:/universe/workspace/repo/b-project/b-ui && go build ./...
```

- [ ] **Step 8: Commit**

```bash
git add src/backend/internal/domain/services/cluster_service.go src/backend/internal/domain/services/cluster_runtime.go
git commit -m "feat: add CheckAndBroadcastUpdate logic to cluster sync service"
```

---

### Task 11: b-ui — Add `GetMembers` to sync store

**Files:**
- Modify: `src/backend/internal/domain/services/cluster_sync.go`

- [ ] **Step 1: Add `GetMembers` to `clusterSyncStore` interface**

```go
type clusterSyncStore interface {
	GetMember(domainID uint, nodeID string) (*model.ClusterMember, error)
	GetMembers(domainID uint) ([]model.ClusterMember, error)  // <-- add
	SaveMember(*model.ClusterMember) error
	ListMembers() ([]model.ClusterMember, error)
	GetDomain(id uint) (*model.ClusterDomain, error)
	SaveDomain(*model.ClusterDomain) error
	ListDomains() ([]model.ClusterDomain, error)
}
```

- [ ] **Step 2: Implement `GetMembers()` on `dbClusterSyncStore`**

At the bottom of `cluster_sync.go`, find or create the `dbClusterSyncStore` section and add:
```go
func (s *dbClusterSyncStore) GetMembers(domainID uint) ([]model.ClusterMember, error) {
	var members []model.ClusterMember
	if err := database.GetDB().Where("domain_id = ?", domainID).Find(&members).Error; err != nil {
		return nil, err
	}
	return members, nil
}
```

- [ ] **Step 3: Verify Go compilation**

```bash
cd C:/universe/workspace/repo/b-project/b-ui && go build ./...
```

- [ ] **Step 4: Commit**

```bash
git add src/backend/internal/domain/services/cluster_sync.go
git commit -m "feat: add GetMembers to cluster sync store"
```

---

### Task 12: b-ui — Frontend Types and UI

**Files:**
- Modify: `src/frontend/src/types/clusters.ts`
- Modify: `src/frontend/src/views/ClusterCenter.vue`

- [ ] **Step 1: Update `ClusterMember` interface**

```typescript
export interface ClusterMember {
  id: number
  domainId: number
  nodeId: string
  name: string
  displayName: string
  panelVersion: string    // <-- add
  status: string          // <-- add ("online" | "offline")
  baseUrl: string
  lastVersion: number
  isLocal: boolean
}
```

- [ ] **Step 2: Update `ClusterCenter.vue` member table to show `panelVersion` and `status`**

Add columns to the member table:
```html
<template v-slot:item.panelVersion="{ item }">
  <span class="mono-copy">{{ item.panelVersion || '-' }}</span>
</template>

<template v-slot:item.status="{ item }">
  <v-chip
    :color="item.status === 'offline' ? 'red' : 'green'"
    size="small"
    variant="flat"
  >
    {{ item.status === 'offline' ? $t('common.offline') : $t('common.online') }}
  </v-chip>
</template>
```

- [ ] **Step 3: Add i18n keys**

In `src/frontend/src/locales/en.ts`:
```typescript
common: {
  // ... existing
  online: "Online",
  offline: "Offline",
},
clusterCenter: {
  // ... existing
  panelVersion: "Panel Version",
  status: "Status",
},
```

- [ ] **Step 4: Build frontend**

```bash
cd C:/universe/workspace/repo/b-project/b-ui/src/frontend && npm run build
```

- [ ] **Step 5: Commit**

```bash
git add src/frontend/src/types/clusters.ts src/frontend/src/views/ClusterCenter.vue src/frontend/src/locales/en.ts
git commit -m "feat: show panel version and status in cluster center UI"
```

---

### Task 13: Integration Verification

- [ ] **Step 1: Run b-cluster-hub tests**

```bash
cd C:/universe/workspace/repo/b-project/b-cluster-hub && npx vitest run
```

- [ ] **Step 2: Run b-ui Go tests**

```bash
cd C:/universe/workspace/repo/b-project/b-ui && go test ./src/backend/...
```

- [ ] **Step 3: Verify b-ui frontend builds**

```bash
cd C:/universe/workspace/repo/b-project/b-ui/src/frontend && npm run build
```

- [ ] **Step 4: Push to remotes**

```bash
cd C:/universe/workspace/repo/b-project/b-cluster-hub && git push origin main
cd C:/universe/workspace/repo/b-project/b-ui && git push origin main
```
