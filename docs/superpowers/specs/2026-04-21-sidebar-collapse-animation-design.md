# Sidebar Collapse Animation Design

## Goal

Make the left navigation drawer collapse animation behave as the exact reverse of the current expand animation, while removing visible jitter during the transition.

## Current Behavior

The drawer currently uses two visual states driven by separate timers in `src/frontend/src/layouts/default/Drawer.vue`:

- `drawerRail` controls whether the Vuetify drawer is in rail mode.
- `showExpandedContent` controls whether labels and footer content are visible.

Expand and collapse are both implemented as two-step sequences with `setTimeout(140)`, but they are not true inverses:

- Expand: disable rail first, then reveal content after a delay.
- Collapse: hide content first, then enable rail after a delay.

This creates timing mismatches between width changes and content transitions, which can produce jitter during collapse.

## Chosen Approach

Use a single visual animation timeline for desktop drawer transitions.

The implementation should preserve the current broad interaction model:

- Mobile drawer behavior remains unchanged.
- Desktop still ends in a true rail state when collapsed.
- Desktop still ends in a full-width drawer when expanded.

The change is to make all animated desktop elements transition along one mirrored sequence so collapse becomes the reverse of expand.

## Animation Design

### Expand

1. Drawer leaves rail mode and begins expanding from `92px` toward `308px`.
2. Text-bearing content fades in and shifts from a slight left offset to rest.
3. The transition ends with full content visibility and stable full-width layout.

### Collapse

1. Full-width content remains mounted while the reverse transition starts.
2. Text-bearing content fades out and shifts slightly left.
3. Drawer width contracts along the same duration and easing used for expand.
4. Rail mode becomes the resting state only after the mirrored visual sequence completes.

This makes collapse read as a direct reversal of expand instead of a separate, staged animation.

## State Model

Replace the current asymmetric mental model with a mirrored desktop transition model:

- Keep `showDrawer` as the external visibility model.
- Keep mobile logic separate and immediate.
- On desktop, maintain a visual transition state that distinguishes:
  - expanded resting state
  - collapsing in progress
  - collapsed resting state
  - expanding in progress

The exact code shape can stay minimal, but the implementation should avoid using one timer sequence for expand and a different timer sequence for collapse.

The important rule is:

- content visibility, text offset, and drawer width must all derive from the same transition phase
- rail mode should not flip early enough to cause a second layout jump midway through collapse

## Styling Rules

Apply the mirrored motion consistently to the text-bearing regions inside `Drawer.vue`:

- brand titles
- group section labels
- footer note
- logout label

These regions should:

- remain mounted during the transition
- animate `opacity`
- animate a small horizontal `transform`
- be clipped by containers with `overflow: hidden`
- avoid layout-affecting toggles that abruptly remove them before width animation completes

The timing and easing should stay aligned with the app motion tokens already used in the file.

## Constraints

- Do not change mobile drawer behavior.
- Do not redesign the visual look of the drawer.
- Do not add unrelated refactors outside `Drawer.vue` unless required to keep the animation stable.
- Prefer the smallest code change that produces a truly mirrored collapse sequence.

## Error Handling And Edge Cases

- Rapid repeated toggle clicks should clear or supersede any pending transition timer so stale callbacks cannot force the drawer into the wrong visual state.
- Breakpoint switches between mobile and desktop should immediately normalize state and cancel pending desktop animation timers.
- Desktop rail persistence behavior in `Default.vue` should remain unchanged.

## Verification

Verify the implementation with the existing app locally:

- expand from collapsed desktop state and confirm labels enter smoothly
- collapse from expanded desktop state and confirm the motion is the visual reverse of expand
- repeatedly toggle the drawer and confirm there is no jitter or late state snap
- switch between mobile and desktop breakpoints and confirm no stuck intermediate state appears

## Files In Scope

- `src/frontend/src/layouts/default/Drawer.vue`

## Out Of Scope

- App bar redesign
- Drawer information architecture changes
- Global animation token changes
- New persistence behavior
