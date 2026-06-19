# Primary Cluster Nodes Design

## Summary

Cluster Center will move away from the Cloudflare Worker hub dependency. The new model keeps the `domain` concept, but changes its meaning: a domain is a business or management scope owned by the fixed primary server. The cluster is the primary server plus its child servers, and child servers are represented as nodes.

The first release only implements node reporting from child servers to the primary server. It does not implement resource distribution, remote configuration sync, automatic failover, or domain-scoped node assignment beyond what is needed to display nodes under the primary server.

## Concepts

- Primary server: the fixed, globally authoritative server. It owns cluster state, domains, join tokens, and the nodes list.
- Child server: a server that joins the primary server and periodically reports its own information.
- Cluster: the whole primary-managed server set.
- Domain: a primary-managed business or management scope. This keeps the existing domain vocabulary available for future features, but no longer means a Cloudflare Worker domain.
- Node: a child server known to the primary server. Existing `ClusterMember` storage can be reused initially, with UI copy changed to nodes where appropriate.
- Token: a primary-generated secret used for primary-child authentication. The implementation can reuse the current domain token generation/encryption/checking primitives, but token meaning changes to primary-child cluster auth.

## First Release Scope

The first release includes:

- Primary mode APIs to create or rotate the cluster join token.
- Child mode configuration for primary server URL and token.
- Child-to-primary server info report.
- Primary-side storage and display of nodes.
- Complete authentication for report traffic.
- Basic reliability for reporting through retries and idempotent upsert behavior.

The first release excludes:

- Cloudflare Worker compatibility.
- Resource distribution from primary to children.
- Domain resource operations across nodes.
- Scatter task changes beyond keeping existing code compiling.
- Primary failover or leader election.
- Complex domain-node assignment policy.

## Backend Design

### Data Model

Reuse `ClusterLocalNode` for local identity. Keep `ClusterDomain` as the future business-domain model, but stop treating `HubURL` as a Worker URL in the new primary-child path.

Add or extend storage for node reporting:

- Node ID.
- Display name.
- Base URL.
- Address.
- Panel version.
- Status.
- Capabilities.
- Last report time.
- Last accepted sequence.
- Public key.
- Auth status or sync status.

The first implementation will add a dedicated `ClusterNode` model and adapt the new UI/API path to read from it. `ClusterMember` remains available for existing domain-oriented code until a later cleanup or migration.

### Authentication

Reports must use two layers:

- `X-Cluster-Token` carries the primary-generated join/auth token.
- Ed25519 signatures use the existing local node identity keypair.

Each report includes:

- `messageId`
- `nodeId`
- `sequence`
- `reportedAt`
- `payloadHash`
- `signature`

The primary server rejects missing token, invalid token, invalid signature, stale timestamp, and replayed or regressed sequence. Duplicate `messageId` or same sequence should be idempotent.

### Reporting Flow

Child server:

1. Loads primary URL and token from settings.
2. Ensures local node identity exists.
3. Builds a server info payload.
4. Signs the payload.
5. Sends it to the primary.
6. Retries transient failures with bounded backoff.

Primary server:

1. Validates token.
2. Validates signature against stored or first-seen public key policy.
3. Validates replay fields.
4. Upserts the node record.
5. Returns accepted state and primary time/version.

For first join, the primary can trust the token plus supplied public key, then bind that node ID to the key. Later reports for that node must use the same key unless the node is reset by an admin.

### APIs

Add primary-child APIs separately from Worker hub concepts:

- `POST /api/cluster/primary/token/rotate`
- `GET /api/cluster/primary/token`
- `PUT /api/cluster/child/config`
- `POST /api/cluster/child/report-now`
- `GET /api/cluster/nodes`
- `POST /_cluster/v1/nodes/report`

These route names are the first-release contract. They make primary/child semantics explicit and avoid reusing Worker hub route names for new behavior.

### Existing Code Impact

Update hub-dependent paths instead of deleting Cluster Center:

- `cluster_hub_client.go`: introduce a primary client or split Worker hub client behind a deprecated compatibility boundary.
- `cluster_service.go`: add primary/child service methods and make Cluster Center use the new path.
- `cluster_runtime.go` and `cluster_sync.go`: avoid Worker snapshot polling in the first-release primary-child path.
- `cluster_identity.go`: reuse local identity.
- `apiHandler.go` and `cluster.go`: add primary/child routes.
- `model.go` and `db.go`: add migration fields/models.

Keep peer message and action routing code available for future expansion, but do not make first-release node reporting depend on domain resource dispatch.

## Frontend Design

Cluster Center becomes a primary/child cluster management page.

Primary server view:

- Shows this server as primary.
- Shows join token controls.
- Shows primary join URL.
- Shows reported nodes with status, version, address, base URL, last report time, and capabilities.
- Allows manual token rotation.

Child server view:

- Shows this server as child when primary URL/token are configured.
- Provides fields for primary URL and token.
- Provides test/report-now action.
- Shows last report status and last error.

Existing domain/member wording should be updated carefully:

- Use "domain" only for primary-managed business scopes.
- Use "node" for child servers.
- Avoid "hub" and Worker wording in new flows.

## Reliability

Node reports are idempotent upserts. The child server retries transient network failures with capped backoff. The primary records last accepted sequence and last report time. A node becomes stale/offline if no report arrives within a configurable timeout.

The first release does not need durable delivery queues for all report attempts, but tests must cover duplicate report handling, sequence regression rejection, and retry behavior at the service boundary.

## Testing

Backend tests:

- Token generation, storage, rotation, and validation.
- Report signature verification.
- Replay and stale timestamp rejection.
- First report binds node public key.
- Later report with different public key is rejected.
- Idempotent duplicate report.
- Node list returns current primary-side state.

Frontend tests:

- Cluster Center renders primary mode controls.
- Cluster Center renders child mode config.
- Report-now calls the expected API.
- Locale keys no longer refer to Worker hub in the new flow.

Integration smoke:

- Configure one server as primary.
- Configure one server as child.
- Trigger report-now.
- Verify primary nodes list updates.

## Migration Notes

Existing installations with Worker hub domains should not be silently converted. The first release can leave old data untouched and route the new UI through primary/child settings. A later migration can decide whether to archive, hide, or transform old Worker-domain records.

## Implementation Decisions

- First release creates a new `ClusterNode` model instead of extending `ClusterMember`.
- Child reports run through a new dedicated report job instead of being folded into existing domain sync jobs.
- Default report interval is 60 seconds.
- A node is marked stale after three missed report intervals.
- A stale node is displayed as offline after five missed report intervals.
