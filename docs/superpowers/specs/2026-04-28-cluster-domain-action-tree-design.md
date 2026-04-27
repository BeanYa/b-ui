# Design: Cluster Domain Action Tree in Domain Detail

## Summary

Restructure the cluster domain detail card in `ClusterCenter.vue` so the
domain metadata and supported actions can scale independently.

The selected design keeps the existing card shell and title treatment, but
replaces the current metadata grid with a denser two-column layout inside the
card:

- Left: compact definition-style rows for core domain metadata
- Right: a fixed-height supported-actions tree with internal scrolling

The supported actions view must be hierarchical, collapsible, and future-proof
as action names grow over time. Actions are grouped by `.`-delimited prefixes.
Only top-level nodes are visible by default; all child branches start collapsed.

## Goals

- Prevent long `supportedActions` lists from stretching the domain detail card
  horizontally or vertically.
- Preserve fast scanning of core domain metadata.
- Make action growth manageable as more nested action names are introduced.
- Keep the layout visually consistent with the existing cluster center control
  surface instead of turning it into a generic data table.
- Limit implementation scope to the frontend domain detail view and local UI
  logic.

## Non-Goals

- No backend or API contract changes.
- No action-name localization or alias mapping in this iteration.
- No global "expand all" / "collapse all" control.
- No search, filtering, copy affordances, or analytics around action usage.

## Current Problem

The current domain detail card renders `supportedActions` as a flat joined
string inside the same metadata grid as the other fields. This has two scaling
problems:

- Long action lists make the field visually dominant and hard to scan.
- Nested action names like `domain.cluster.changed` are displayed as raw text,
  so shared prefixes are repeated instead of grouped.

This makes the domain card less readable as protocol capabilities expand.

## Selected Layout

### Card Structure

Keep the existing selected-domain card container and title row:

- Domain name as the main title
- Version badge inline with the title

Replace the current `cluster-center__info-grid` contents with a split detail
panel:

- `cluster-center__domain-meta`: left column, compact metadata rows
- `cluster-center__actions-tree`: right column, fixed-width action tree rail

### Left Column: Compact Metadata Rows

Render the following fields as dense label/value rows:

1. Domain ID
2. Hub URL
3. Version
4. Communication protocol
5. In-domain endpoint
6. Mirrored members

Layout rules:

- Labels use a fixed narrow column width.
- Values occupy the remaining width and may wrap.
- Rows use subtle separators or compact surfaces instead of large standalone
  cards.
- The left column remains optimized for quick scanning, not visual emphasis.

### Right Column: Supported Actions Tree Rail

Render supported actions in a dedicated side rail inside the same domain card.

Layout rules:

- Fixed width on desktop.
- Fixed height with internal scrolling.
- The rail does not increase the overall card height once content exceeds the
  visible tree area.
- Tree content is visually separated from the metadata rows so the user can
  understand that actions are a distinct capability surface.

## Tree Data Model

Transform `supportedActions: string[]` into a hierarchical tree by splitting on
`.`.

Example:

```text
domain.cluster.changed
domain.panel.update.available
events
heartbeat
ping
info
action
```

Becomes:

```text
domain
  cluster
    changed
  panel
    update
      available
events
heartbeat
ping
info
action
```

Transformation rules:

- Shared prefixes must be merged into a single branch.
- Leaf order should follow the original input order as much as possible.
- Empty, null, or invalid action values are ignored.
- A top-level node may be either a leaf or a parent branch.

## Tree Interaction

### Default State

- Only top-level nodes are shown by default.
- Every node with children starts collapsed.
- The initial expanded-node set is empty.

### Expansion Behavior

- Clicking a parent row toggles only that node's expanded state.
- Expanded children reveal the next level of the hierarchy.
- Leaf nodes do not toggle.
- No cascade expansion is required.

### Empty State

If there are no supported actions:

- Do not render an empty tree scaffold.
- Show a simple fallback value or empty-state line inside the action rail.

## Component Boundaries

Introduce a small dedicated frontend component for the supported-actions tree.

Recommended responsibilities:

- `ClusterCenter.vue`
  - Own the domain detail layout
  - Pass `selectedDomain.supportedActions` into the tree component
  - Keep existing domain-level data loading unchanged

- `ClusterDomainActionTree.vue` (new)
  - Accept `supportedActions`
  - Build the hierarchical view model
  - Manage local expanded state
  - Render the collapsible tree UI

- Tree utility or local pure function
  - Convert `string[]` to tree nodes
  - Stay framework-light and easy to unit test

This avoids adding more layout and interaction complexity directly into the
already large `ClusterCenter.vue`.

## Responsive Behavior

### Desktop

- Two-column card interior
- Dense metadata on the left
- Fixed-width tree rail on the right

### Tablet / Mobile

- Stack the columns vertically
- Metadata block first
- Action tree below it
- Tree remains fixed-height with internal scrolling

Responsive adaptation should reuse the existing breakpoint style already present
in `ClusterCenter.vue` rather than introducing a separate layout system.

## Visual Direction

The chosen layout intentionally moves the metadata area slightly toward a
definition-list aesthetic without making the page feel like a plain admin form.

Visual cues:

- Preserve the existing card shell and dark surface styling
- Reduce padding and vertical spacing in the metadata area
- Use restrained separators and compact typography
- Keep the action rail visually distinct with its own inset surface
- Use the tree hierarchy, indentation, and disclosure arrows as the primary
  affordances instead of heavy decoration

## Files in Scope

### Frontend

- `src/frontend/src/views/ClusterCenter.vue`
  - Replace the flat supported-actions field with the new split layout
  - Integrate the action tree component
  - Update responsive styles for the detail card

- `src/frontend/src/views/ClusterCenter.test.ts`
  - Update source-based assertions to reflect the new layout and component usage

- `src/frontend/src/components/ClusterDomainActionTree.vue`
  - New component for tree rendering and expand/collapse behavior

- `src/frontend/src/utils/clusterDomainActionTree.ts`
  - Pure transform helpers for converting `supportedActions` into tree nodes
  - Exported separately so tree-building behavior can be unit tested without
    mounting the Vue component

- `src/frontend/src/utils/clusterDomainActionTree.test.ts`
  - Unit tests for tree construction and default collapsed behavior

## Test Strategy

### Source Assertions

Keep the existing lightweight source-based test style for `ClusterCenter.vue`,
but update coverage so the test guards the new structure:

- The domain detail view uses a dedicated action-tree container/component
- Supported actions are no longer rendered as a flat joined string in the
  metadata grid
- The detail card layout includes separate metadata and action areas

### Tree Transformation Tests

Add focused unit coverage for the action-tree data transform:

- Single-level actions
- Multi-level actions
- Shared-prefix merging
- Empty input
- Stable default collapsed state

### Scope Control

Do not expand this task into end-to-end browser automation or highly coupled
component interaction tests. The goal is to verify layout structure and tree
construction logic without widening the change set.

## Risks and Mitigations

- **Risk: `ClusterCenter.vue` becomes harder to maintain**
  - Mitigation: extract the tree into a dedicated component and keep layout
    concerns separated from tree state.

- **Risk: Long action labels still create overflow pressure**
  - Mitigation: allow wrapping inside tree rows and keep the rail scrollable.

- **Risk: Dense metadata rows feel too form-like**
  - Mitigation: preserve the existing card shell, title treatment, and surface
    styling while only compressing the inner information layout.

- **Risk: Mobile layout becomes cramped**
  - Mitigation: stack the layout below the existing tablet breakpoint and keep
    the tree height explicitly bounded.

## Implementation Notes

- Use the raw action segment names as display text in this iteration.
- Avoid premature abstraction beyond one focused tree component and one pure
  transform function.
- Preserve the current data-fetching and selected-domain state flow.
- Do not couple the tree to localized field labels or backend protocol enums.
