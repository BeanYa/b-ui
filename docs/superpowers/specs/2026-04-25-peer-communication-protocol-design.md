# Peer Communication Protocol Design

## Context

This design applies primarily to `b-ui`, with an explicit boundary to `b-cluster-hub`.

The Hub is the authority for domain membership. It owns domain existence, node registration, node exit, node removal, member endpoint metadata, public keys, capabilities, and the current membership version. The Hub does not relay peer messages, schedule peer work, or participate in panel business synchronization.

Nodes communicate directly through their registered endpoints. Each node keeps its own durable message log, idempotency state, workflow state, acknowledgement state, and local schedules. Peer communication must remain compatible with future actions such as node-to-node requests, multicast, broadcast, chained execution, periodic reconciliation, and panel information create/update/delete synchronization.

## Goals

- Separate membership authority from peer communication.
- Keep the Hub as the authoritative source for domain membership and node metadata only.
- Support direct, multicast, broadcast, chained, and scheduled broadcast peer communication.
- Make messages extensible across protocol and payload versions.
- Make peer message handling idempotent, signed, replay-resistant, and observable.
- Allow old nodes to ignore or reject unsupported future actions without blocking unrelated work.
- Provide a foundation for panel information synchronization without requiring the Hub to store panel business data.

## Non-Goals

- The Hub will not forward peer messages.
- The Hub will not execute chained workflows.
- The Hub will not store peer event logs or panel business events.
- This design does not introduce strong global consistency for peer-broadcast data.
- This design does not require CRDT-based concurrent editing in the first version.

## Architecture

The system has two planes.

### Membership Plane

`b-cluster-hub` owns:

- domain records
- domain membership version
- node membership status
- node endpoint
- node public key
- node supported protocol versions
- node supported actions or capabilities
- node last-seen metadata

Nodes register with the Hub, exit through the Hub, and refresh the domain membership table from the Hub. Every successful membership mutation increments the domain membership version.

### Peer Communication Plane

`b-ui` nodes own:

- peer endpoint handlers
- outbound peer delivery
- inbound peer message verification
- local durable peer logs
- idempotency state
- chained workflow execution
- acknowledgement tracking
- retry and dead-letter handling
- scheduled peer jobs

Peer communication uses the Hub-provided membership table only for routing, public-key lookup, capability filtering, and membership validation.

## Message Envelope

All peer messages use one extensible envelope:

```ts
type PeerMessage = {
  message_id: string;
  workflow_id?: string;
  step_id?: string;

  domain_id: string;
  membership_version: number;

  source_node_id: string;
  source_seq: number;

  category: "command" | "event" | "query" | "response";
  action: string;
  protocol_version: "v1";
  schema_version: number;

  route: RoutePlan;

  idempotency_key?: string;
  causation_id?: string;
  correlation_id?: string;

  created_at: number;
  expires_at?: number;

  payload_hash: string;
  payload: Record<string, unknown>;

  signature: string;
};
```

`protocol_version` describes the envelope, signature, routing, acknowledgement, and retry contract.

`schema_version` describes the payload schema for the specific `action`.

`action` is a namespaced string, not a closed enum. Examples:

```text
domain.cluster.changed
domain.node.removed
panel.info.create
panel.info.update
panel.info.delete
panel.info.changed
node.config.apply
node.config.applied
node.log.reconcile
node.health.heartbeat
```

## Route Plan

`target` is replaced by a route plan so the protocol can support multiple targets and chained propagation without special cases.

```ts
type RoutePlan = {
  mode: "direct" | "multicast" | "broadcast" | "chain" | "scheduled_broadcast";

  targets?: string[];

  selector?: TargetSelector;
  chain?: RouteStep[];
  delivery?: DeliveryPolicy;
  schedule?: SchedulePolicy;
};

type TargetSelector = {
  include?: string[];
  exclude?: string[];
  capability_required?: string[];
};

type RouteStep = {
  step_id: string;
  node_id: string;
  action?: string;
  payload_override?: Record<string, unknown>;
  continue_on_failure?: boolean;
};

type DeliveryPolicy = {
  ack: "none" | "node" | "quorum" | "all";
  timeout_ms: number;
  retry: {
    max_attempts: number;
    backoff_ms: number;
  };
  max_hops?: number;
};

type SchedulePolicy = {
  kind: "once" | "interval" | "cron";
  run_at?: number;
  interval_ms?: number;
  cron?: string;
  max_runs?: number;
  expires_at?: number;
};
```

### Direct

`direct` sends one message to one node listed in `targets`.

### Multicast

`multicast` sends the same message to a fixed list of nodes. The target list is materialized before delivery and recorded for audit and retry.

### Broadcast

`broadcast` sends to all current domain members that match the optional selector. The sender calculates recipients from its local membership table. If the table is stale, retries and reconciliation may later reach missed nodes.

### Chain

`chain` executes a workflow across ordered nodes. Each step is idempotent. The current node records step status, signs the result, and forwards to the next step only when the route policy allows it.

### Scheduled Broadcast

`scheduled_broadcast` is a persisted local schedule that creates ordinary broadcast messages at run time. Scheduled broadcasts are for compensation, reconciliation, heartbeat, cache refresh, and drift detection. They are not the only delivery mechanism for critical state changes.

## Message Handling

On receipt, a node must:

1. Parse the envelope.
2. Reject expired messages.
3. Verify `domain_id` exists locally.
4. Check whether `source_node_id` is in the current membership table.
5. If the sender is unknown or has a newer `membership_version`, refresh membership from the Hub before deciding.
6. Verify the sender public key and message signature.
7. Verify `payload_hash`.
8. Check idempotency by `message_id`, `idempotency_key`, and `source_node_id + source_seq`.
9. Dispatch by `category` and `action`.
10. Persist final state and send acknowledgement when required.

Unsupported actions must not block unrelated queue processing:

```text
command -> return unsupported_action
event -> mark unsupported or ignored
query -> return unsupported_action
response -> attach to correlation state if recognized, otherwise ignore
```

## Local Persistence

Each node stores peer communication state locally.

### peer_event_log

Stores inbound and outbound envelopes, payload hashes, signatures, direction, timestamps, and raw status.

### peer_event_state

Tracks idempotent processing state:

```text
received
processing
succeeded
failed
ignored
unsupported
dead
```

This replaces a simple processed-event list. A duplicate `message_id` only means the message was seen before; the state determines whether the message can be ignored, resumed, retried, or rejected.

### peer_workflow_state

Tracks chain workflows:

- `workflow_id`
- current step
- completed steps
- failed steps
- step result hashes
- continuation decision
- terminal status

### peer_ack_state

Tracks delivery attempts, target status, acknowledgement responses, retry count, next retry time, and final failure.

### peer_schedule

Tracks local scheduled broadcasts:

- `schedule_id`
- `domain_id`
- `owner_node_id`
- `action`
- route plan
- next run time
- last run time
- run count
- max runs
- enabled state

## Queueing

Queues should be scoped instead of globally serialized.

Recommended queue scopes:

```text
domain:{domain_id}
panel-info:{domain_id}
node-message:{domain_id}:{node_id}
workflow:{domain_id}:{workflow_id}
schedule:{domain_id}
```

Domain membership refresh and security-sensitive membership handling must not be blocked by unrelated panel information events.

## Chained Workflow Rules

Chain messages require:

- `workflow_id`
- unique `step_id`
- fixed chain list
- max hop protection
- timeout
- retry policy
- idempotency key
- payload hash
- signed step result

Each step:

1. Checks whether the step is already complete.
2. Applies the action idempotently.
3. Records step result.
4. Signs the result.
5. Forwards to the next step when the route allows it.
6. Stops the workflow or continues based on `continue_on_failure`.

If a chain stalls, scheduled reconciliation may inspect or retry it, but the workflow must eventually enter a terminal state instead of remaining permanently active.

## Scheduled Broadcast Rules

Scheduled broadcast is allowed for:

- peer log reconciliation
- node health heartbeat
- membership refresh hints
- panel snapshot refresh
- configuration drift detection
- cache warming or invalidation

Scheduled broadcast must not be the only path for critical changes. A critical change should immediately emit a peer command or event; scheduled jobs only compensate for missed or failed delivery.

## Panel Information Synchronization

The first version should use single-owner writes.

Each panel information record has an `owner_node_id`. Only the owner emits authoritative create/update/delete events for that record. Other nodes that need a mutation send a command to the owner. After the owner commits the mutation, it broadcasts `panel.info.changed`.

This avoids concurrent multi-writer conflict handling in the first version. If multi-writer editing is required later, it should be introduced explicitly with a conflict strategy such as last-write-wins, vector clocks, or CRDTs.

## Member Removal and Security

When the Hub removes a node:

1. Hub updates the membership table.
2. Hub increments the domain membership version.
3. Remaining nodes refresh membership through normal refresh, direct notification, or scheduled reconciliation.
4. Nodes reject messages from the removed node after their local membership table is updated.

Because peer communication bypasses the Hub, removal is not an instant network kill switch. Peer messages therefore need short TTLs. High-risk actions should force a membership refresh before execution.

## Compatibility

Compatibility is handled at three levels:

- `protocol_version` for the envelope and delivery contract.
- `schema_version` for the payload of one action.
- node capabilities in the Hub membership table.

Nodes advertise supported protocol versions and actions to the Hub. Senders can filter broadcast targets by capability. Receivers must safely ignore or reject unsupported actions without failing unrelated work.

## Error Handling

Common error responses:

```text
unsupported_protocol_version
unsupported_action
unknown_source_node
invalid_signature
stale_membership
message_expired
payload_hash_mismatch
duplicate_in_progress
duplicate_succeeded
workflow_step_failed
delivery_timeout
```

Errors should be logged in local peer state and returned to the sender when the delivery policy requires acknowledgement.

## Testing Strategy

Unit tests should cover:

- message signature validation
- payload hash validation
- duplicate message handling by state
- unsupported action behavior
- membership version refresh decisions
- direct route recipient selection
- multicast recipient materialization
- broadcast selector filtering
- chain step success and failure behavior
- scheduled broadcast materialization into ordinary peer messages

Integration tests should cover:

- register nodes through Hub and then peer-send directly
- remove a node from Hub and verify refreshed peers reject it
- panel owner write followed by peer broadcast synchronization
- missed event repaired by scheduled reconciliation
- old-version node ignores unsupported event without blocking supported events

## Success Criteria

The design is complete when:

- Hub responsibilities are limited to domain membership authority and node metadata.
- Peer messages use a single versioned envelope.
- Routing supports direct, multicast, broadcast, chain, and scheduled broadcast.
- Nodes persist logs, idempotency state, workflow state, acknowledgements, and schedules.
- Unknown future actions do not block current supported actions.
- Panel information synchronization has a clear first-version owner model.
- Membership removal behavior and its non-instant peer-communication consequences are explicit.
