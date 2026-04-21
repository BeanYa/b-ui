# B-UI Global Operations Console Design

Date: 2026-04-20
Status: Draft approved in conversation, updated for review
Scope: `frontend/` global design, layout, motion, and theming refresh

## 1. Goal

Refresh B-UI into a cohesive operations console with a calm, precise control-room aesthetic across the entire product, while preserving strong runtime performance on modest deployment hardware.

The redesign must:
- Treat dark and light themes as first-class experiences with equal polish.
- Make navigation, hierarchy, status, and actions easier to parse at a glance.
- Add motion and visual refinement without relying on heavy rendering techniques.
- Keep all rendering client-side and avoid introducing server-side performance costs.
- Consolidate homepage runtime monitoring into a single, high-signal server probe card instead of repeating overlapping system-summary cards.

## 2. Product Context

B-UI is not a marketing site. It is an administrative and monitoring surface for an infrastructure product. The visual language therefore needs to prioritize:
- reliability over novelty
- information hierarchy over decoration
- quick scanning over expressive storytelling
- stable interactions over flashy effects

The intended feel is a professional operations control surface rather than a cyberpunk dashboard.

## 3. Design Direction

### 3.1 Visual Personality

Primary direction: cold, precise operations console with segmented workbench structure.

Characteristics:
- clean structural lines
- layered but restrained surfaces
- subtle instrument-panel depth
- selective use of status color
- motion that clarifies state change rather than drawing attention to itself
- explicit separation between stable runtime status and editable page content

Avoid:
- oversized glow treatments
- neon-heavy cyber styling
- long-running ambient animations
- decorative effects that compete with content

### 3.2 Theme Strategy

Dark and light modes must be designed independently, not by simple inversion.

Dark mode:
- deep neutral-blue background
- low-glare raised surfaces
- cool separators and restrained accent illumination
- strong contrast for telemetry and status

Light mode:
- fog-white control-room background
- pale blue-gray surfaces
- crisp structural borders
- subtle depth from layered shadows rather than blur-heavy glass

Both themes share the same spatial system, density, typography, and interaction behavior, while using distinct token values for contrast and atmosphere.

## 4. Success Criteria

The redesign is successful when:
- users can identify page purpose, primary actions, and current system state within seconds
- pages feel visually related across dashboard, entity lists, forms, and dialogs
- theme switching does not feel like one theme is secondary
- hover, focus, loading, and panel transitions feel modern but lightweight
- page responsiveness and chart updates remain smooth on ordinary client hardware

## 5. Constraints

### 5.1 Technical Constraints

- No heavy animation frameworks are required.
- Prefer CSS transitions and lightweight keyframes.
- Prefer `transform` and `opacity` animation properties.
- Avoid continuous GPU-heavy effects such as animated blur fields, particles, canvas overlays, or WebGL scenes.
- Existing Vuetify structure should be reused where practical.
- Client rendering cost is acceptable within reason; server-side cost must remain unaffected.

### 5.2 Product Constraints

- This is a data-dense admin UI, so readability must outrank novelty.
- Existing navigation model, route structure, and data flows should remain intact.
- The redesign should be applied globally, not only to the homepage.

## 6. Design System Specification

### 6.1 Tokens

Introduce or normalize a global token system for:
- background layers
- surface layers
- borders
- text hierarchy
- state colors
- shadows
- radii
- spacing
- motion duration and easing

Required token groups:
- `--app-bg-*`
- `--app-surface-*`
- `--app-border-*`
- `--app-text-*`
- `--app-state-*`
- `--app-shadow-*`
- `--app-radius-*`
- `--app-motion-*`

Token intent:
- separate atmospheric styling from component styling
- allow true parity between light and dark themes
- reduce page-specific hardcoded colors

### 6.2 Typography

Tone: technical, calm, compact, legible.

Rules:
- use `Inter` for interface typography and `GeistMono` for code-like and numeric telemetry contexts
- self-host both font families inside the frontend bundle or vendor them through local package assets
- do not rely on Google-hosted font CDNs because deployments may run behind restricted networks
- preserve multilingual fallback chains for CJK, Persian, and other non-Latin locales
- use a consistent hierarchy for shell title, page title, section title, card title, metadata, and captions
- tighten uppercase utility labels into a shared eyebrow style
- preserve legibility in dense cards and tables
- avoid oversized hero typography outside the homepage overview

### 6.3 Color Semantics

Status colors must become semantic rather than decorative:
- blue: information, selected state, contextual emphasis
- green: healthy, active, success
- yellow: warning, transitional attention
- red: danger, failure, destructive actions

Status color usage must be sparse and intentional. Neutral surfaces should carry most of the visual system.

## 7. Layout Architecture

### 7.1 Shell Layer

The global shell consists of:
- drawer
- app bar
- content viewport
- background atmosphere

Shell requirements:
- clearer separation between global navigation and local page content
- consistent top spacing and page width rhythm
- reduced visual clutter in the background
- lighter decorative layering on mobile
- follow a segmented workbench model: left navigation, top context rail, central work area, and stable runtime-status zone pattern
- allow pages to express the stable runtime zone as a side panel, top status strip, or compact status capsule cluster depending on page density

### 7.2 Page Layer

Every page should use a consistent frame:
- page header
- optional page summary or status rail
- action toolbar
- primary content blocks

Each page family should feel structurally consistent:
- dashboard pages
- entity list pages
- entity edit or create pages
- settings pages
- modal-driven workflows

Approved page-family mapping:
- `Overview Workspace`: homepage runtime overview and telemetry
- `Catalog Workspace`: inbounds, outbounds, endpoints, services, rules, TLS, admins
- `Inventory Table Workspace`: clients and other dense inventories that benefit from tabular scanning
- `Configuration Workspace`: settings, basics, DNS, and other multi-section system controls

### 7.3 Block Layer

All reusable blocks should conform to the same design language:
- summary cards
- data tables
- filter bars
- forms
- dialogs
- menus
- drawers
- chips
- buttons
- empty states

## 8. Page Family Design

### 8.1 Homepage

Homepage should become the strongest expression of the console identity.

Requirements:
- preserve the improved telemetry readability work already completed
- separate dashboard overview, primary runtime probe, and secondary telemetry/detail charts into clearer bands
- create stronger scanning order from left to right and top to bottom
- emphasize state summaries and operational shortcuts without oversized marketing-style hero treatment
- maintain fixed data panels as a stable control surface
- remove the current duplicated system-information cards
- replace them with one combined runtime probe card that merges:
  - CPU usage
  - RAM usage
  - Disk I/O activity
  - core system information
  - sing-box runtime information
- style the combined runtime probe card closer to a server control panel or server probe
- prefer live occupancy rings plus dynamic stream bars to show current status at a glance

Homepage structure:
- left or primary area: overview, counts, and action shortcuts
- right or secondary area: one consolidated server probe card
- lower bands: secondary charts and detail modules

Combined probe-card behavior:
- display CPU, RAM, and Disk I/O as continuously refreshed circular occupancy indicators
- show runtime state, memory, threads, active users, uptime, host, and address inside the same card
- avoid splitting the same operational story across multiple separate cards
- keep the card visually dense but internally segmented so scanning remains easy

Motion:
- entry fade and slight lift for cards
- gentle emphasis on refreshed or changed status blocks
- no continuous background animation behind charts

### 8.2 Entity List Pages

Pages like clients, inbounds, outbounds, endpoints, services, admins, rules, and DNS should share one common structure.

Requirements:
- unify page toolbars and action alignment
- standardize card and table presentation
- improve row density without feeling cramped
- create consistent placement for quick actions
- improve empty states and secondary metadata treatment

Approved distinctions:
- `Clients` remains table-first because it is inventory-like and benefits from scanning expiry, volume, group, and online state in rows
- `Inbounds`, `Outbounds`, and similar object pages should stay card-first because they are action-heavy and have fewer but more semantic fields
- `Settings` and similar pages should use grouped control panels instead of generic stacked form blocks

Visual style:
- card/table shells should resemble instrument modules rather than generic content boxes
- hover states should improve affordance without large movement

### 8.3 Forms and Editors

Create/edit screens should feel more deliberate and less like default form stacks.

Requirements:
- group inputs into clearly named sections
- improve label, helper, validation, and focus hierarchy
- make advanced settings feel contained rather than noisy
- preserve fast scan paths for common configuration tasks

Motion:
- focus rings and field transitions only
- no large animated form rearrangements

### 8.4 Settings

Settings should read as system control panels.

Requirements:
- split dense sections into clearer groups
- highlight destructive or security-sensitive settings distinctly
- improve readability of toggles, selectors, and explanatory text

### 8.5 Dialogs and Overlays

Dialogs should feel more premium and more legible in both themes.

Requirements:
- standardized widths by dialog intent
- clear visual separation of title, body, and actions
- stronger scroll affordances for long content
- lower visual noise in overlay scrims and menu surfaces

## 9. Motion System

### 9.1 Principles

Motion exists to reinforce:
- state change
- focus change
- navigation continuity
- action confirmation

Motion must not become ambient decoration.

### 9.2 Allowed Motion

- page enter transitions
- drawer expand/collapse transitions
- button hover and press response
- card hover elevation changes
- focus ring transitions
- dialog/menu open transitions
- subtle status pulse when values refresh

### 9.3 Forbidden or Discouraged Motion

- infinite large-area shimmer
- continuous parallax backgrounds
- particle systems
- animated blur blobs
- frequent count-up animations on every refresh tick
- chart animations that replay on normal data updates

### 9.4 Timing

Suggested ranges:
- micro-interactions: 120ms to 180ms
- panel open/close: 180ms to 240ms
- page enter: 220ms to 300ms

Easing should feel direct and controlled, not bouncy.

## 10. Performance Strategy

Performance is a first-order design requirement.

Implementation rules:
- prefer static gradients and pseudo-elements over animated decorative layers
- keep blur limited to shell surfaces and critical overlays
- reduce layered box-shadows on dense repeated elements if profiling shows paint cost
- use CSS variables to avoid repeated per-component style recalculation
- keep charts on minimal animation settings
- ensure mobile and tablet layouts remove non-essential decorative density

Client-side performance budget guidance:
- avoid effects that trigger frequent paints across the full viewport
- avoid deep nesting of translucent layers in list-heavy pages
- do not introduce runtime dependencies solely for animation polish

## 11. Accessibility and Usability

The redesign must preserve or improve:
- keyboard navigation visibility
- focus contrast in both themes
- text/background contrast for metadata and muted states
- button discoverability
- readable density in tables and forms
- touch targets on smaller devices

Motion should remain subtle enough that reduced-motion handling can disable most non-essential animation without harming comprehension.

## 12. Implementation Plan Shape

This design should be implemented in four phases.

### Phase 1: Foundation

- normalize global design tokens in `frontend/src/styles/settings.scss`
- add locally served `Inter` and `GeistMono` assets or equivalent vendored files under the frontend bundle
- refine theme switching support in the theme plugin
- define shared utility classes for page shells, sections, and content blocks

### Phase 2: Shell

- redesign `Default.vue`, `AppBar.vue`, `Drawer.vue`, and `View.vue`
- improve background atmosphere and content framing
- ensure equal quality in light and dark themes

### Phase 3: Shared Components

- standardize buttons, cards, chips, lists, tables, fields, overlays, and dialogs
- reduce repeated page-specific styling where a common primitive can replace it

### Phase 4: Page Families

- homepage
- list pages
- form/editor pages
- settings pages

Representative priorities:
- homepage first, including the merged server probe card
- clients as the table-first inventory template
- inbounds and outbounds as card-first catalog templates
- settings as the grouped configuration-workspace template

## 13. Testing and Verification

Verification must cover both visual quality and technical safety.

Required checks:
- theme switching across representative pages
- responsive behavior at desktop, tablet, and mobile widths
- keyboard focus visibility
- `npm test`
- `npm run build`
- spot-check for layout regressions on homepage, one list page, one form page, and one modal-heavy flow

Recommended manual review pages:
- `/`
- `/clients`
- `/inbounds`
- `/outbounds`
- `/settings`
- at least one create/edit modal workflow

## 14. Non-Goals

This redesign does not include:
- backend changes
- route restructuring
- data model changes
- new feature development unrelated to presentation or interaction clarity
- heavy real-time visualization beyond current product needs

## 15. Risks

Main risks:
- visual inconsistency if page-level overrides remain after token updates
- light theme feeling weaker if only dark surfaces are tuned carefully
- performance regressions from excessive blur or shadows in repeated elements
- form pages becoming over-styled and less scannable
- the merged homepage probe card becoming visually impressive but less readable if too many metrics compete inside one panel

Mitigations:
- foundation-first rollout
- representative page audits per phase
- explicit light-theme review at every milestone
- restrained motion defaults
- keep the merged probe card internally segmented with fixed metric zones and limit it to current-state information only

## 16. Recommendation

Proceed with a system-wide operations-console redesign centered on:
- dual first-class themes
- self-hosted `Inter` + `GeistMono` typography with multilingual fallbacks
- unified shell and page structure
- segmented workbench shell layout
- a single homepage server probe card replacing duplicated system-summary cards
- restrained, high-signal motion
- reusable global primitives
- performance-aware client-side rendering

This is the highest-leverage path because it improves the entire product surface without requiring backend work and without leaning on expensive visual effects.
