# Operations Console Redesign Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement the approved global B-UI operations-console redesign across the app shell, shared primitives, homepage telemetry, and representative page families while preserving client performance.

**Architecture:** Keep the existing Vue 3 + Vuetify structure, but centralize the redesign in global design tokens and shell primitives so list pages, forms, dialogs, and the homepage inherit a consistent visual language. Use a segmented-workbench shell, self-hosted `Inter` and `GeistMono` typography, and a single homepage server-probe card that merges the current duplicated system/runtime summary areas.

**Tech Stack:** Vue 3, Vuetify 4, TypeScript, Vite, Vitest, Sass

---

## File Map

- Modify: `frontend/src/styles/settings.scss`
  Global design tokens, self-hosted font-face declarations, shared shells, data-table/form/dialog/tab styling, light/dark parity, motion primitives.
- Create: `frontend/src/assets/fonts/InterVariable.woff2`
  Self-hosted primary UI font for constrained network environments.
- Create: `frontend/src/assets/fonts/GeistMonoVariable.woff2`
  Self-hosted monospace font for code, ports, telemetry, and technical metadata.
- Modify: `frontend/src/plugins/theme.ts`
  Theme preference handling and root document synchronization.
- Modify: `frontend/src/layouts/default/Default.vue`
  App shell background, ambient layers, overall framing.
- Modify: `frontend/src/layouts/default/AppBar.vue`
  Global top bar structure and action styling.
- Modify: `frontend/src/layouts/default/Drawer.vue`
  Navigation density, footer, and theme-aware module styling.
- Modify: `frontend/src/layouts/default/View.vue`
  Main viewport spacing, page transition tuning.
- Modify: `frontend/src/components/Main.vue`
  Homepage overview and telemetry layout refinements, including merged server probe card.
- Modify: `frontend/src/components/tiles/Gauge.vue`
  Homepage gauge rendering adjustments for the merged runtime probe card.
- Modify: `frontend/src/components/tiles/History.vue`
  Homepage telemetry styling alignment with the new probe/control-panel treatment.
- Modify: `frontend/src/views/Clients.vue`
  Representative list-page shell, actions, filters, and table presentation.
- Modify: `frontend/src/views/Inbounds.vue`
  Representative card-first catalog page shell and action density.
- Modify: `frontend/src/views/Outbounds.vue`
  Representative card-first catalog page shell, test-state presentation, and action density.
- Modify: `frontend/src/views/Settings.vue`
  Representative control-panel page with sectioned settings presentation.
- Create: `frontend/src/features/theme/theme.test.ts`
  Theme synchronization regression tests.

## Task 1: Theme Sync Foundation

**Files:**
- Create: `frontend/src/features/theme/theme.test.ts`
- Modify: `frontend/src/plugins/theme.ts`

- [ ] **Step 1: Write the failing test**

```ts
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { applyThemePreference, getThemePreference, resolveThemeName, startThemeSync, stopThemeSync } from '@/plugins/theme'

describe('theme preference helpers', () => {
  beforeEach(() => {
    localStorage.clear()
    document.documentElement.removeAttribute('data-theme-preference')
    document.documentElement.removeAttribute('data-theme-name')
  })

  it('stores both preference and resolved theme markers on the document', () => {
    const controller = {
      change: vi.fn(),
      global: { name: { value: 'dark' } },
    }

    applyThemePreference(controller, 'dark')

    expect(controller.change).toHaveBeenCalledWith('dark')
    expect(document.documentElement.dataset.themePreference).toBe('dark')
    expect(document.documentElement.dataset.themeName).toBe('dark')
  })

  it('reacts to system theme changes when preference is system', () => {
    const controller = {
      change: vi.fn(),
      global: { name: { value: 'light' } },
    }
    const listeners: Array<(event: MediaQueryListEvent) => void> = []
    const media = {
      matches: false,
      addEventListener: vi.fn((_: string, cb: (event: MediaQueryListEvent) => void) => listeners.push(cb)),
      removeEventListener: vi.fn(),
    }
    vi.stubGlobal('matchMedia', vi.fn(() => media))

    applyThemePreference(controller, 'system')
    startThemeSync(controller)
    listeners[0]({ matches: true } as MediaQueryListEvent)

    expect(controller.change).toHaveBeenLastCalledWith('dark')
    expect(document.documentElement.dataset.themeName).toBe('dark')

    stopThemeSync()
  })
})
```

- [ ] **Step 2: Run test to verify it fails**

Run: `npm test -- src/features/theme/theme.test.ts`
Expected: FAIL because `startThemeSync`, `stopThemeSync`, or document sync behavior does not exist yet.

- [ ] **Step 3: Write minimal implementation**

Implement:
- a `syncDocumentTheme(preference)` helper in `frontend/src/plugins/theme.ts`
- `startThemeSync(theme)` and `stopThemeSync()` helpers that listen to `prefers-color-scheme`
- document dataset updates for both preference and resolved theme name

- [ ] **Step 4: Run test to verify it passes**

Run: `npm test -- src/features/theme/theme.test.ts`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add frontend/src/features/theme/theme.test.ts frontend/src/plugins/theme.ts
git commit -m "feat: sync document theme state"
```

## Task 2: Global Design Tokens and Shared Primitives

**Files:**
- Create: `frontend/src/assets/fonts/InterVariable.woff2`
- Create: `frontend/src/assets/fonts/GeistMonoVariable.woff2`
- Modify: `frontend/src/styles/settings.scss`

- [ ] **Step 1: Write the failing test**

No additional automated test for Sass tokens. Use build verification in red/green style for this styling task.

- [ ] **Step 2: Run baseline build**

Run: `npm run build`
Expected: PASS before the styling refactor starts, establishing a clean baseline.

- [ ] **Step 3: Implement global token and primitive refresh**

Update `frontend/src/styles/settings.scss` to:
- split tokens into clearer groups for background, surface, text, border, radius, state, and motion
- improve dual-theme parity with distinct dark/light ambient systems
- declare self-hosted `Inter` and `GeistMono` font faces with multilingual fallbacks
- standardize `.app-page`, `.app-card-shell`, `.app-entity-card`, `.app-dialog`, `.app-data-table`, tabs, fields, chips, menus, and toolbar layouts
- add reusable page title, section, and panel utility classes
- tune shadows and blur usage to stay restrained on dense screens
- add reduced-motion handling for non-essential transitions

- [ ] **Step 4: Run build to verify styles compile**

Run: `npm run build`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add frontend/src/assets/fonts/InterVariable.woff2 frontend/src/assets/fonts/GeistMonoVariable.woff2 frontend/src/styles/settings.scss
git commit -m "feat: refresh global operations console styles"
```

## Task 3: Shell Layer Redesign

**Files:**
- Modify: `frontend/src/layouts/default/Default.vue`
- Modify: `frontend/src/layouts/default/AppBar.vue`
- Modify: `frontend/src/layouts/default/Drawer.vue`
- Modify: `frontend/src/layouts/default/View.vue`
- Modify: `frontend/src/plugins/theme.ts`

- [ ] **Step 1: Write the failing test**

No isolated component tests exist for the shell. Use typecheck/build verification for this layout task.

- [ ] **Step 2: Run baseline build before shell edits**

Run: `npm run build`
Expected: PASS

- [ ] **Step 3: Implement shell refresh**

Apply the redesign to the shell by:
- refining the global background atmosphere in `Default.vue`
- making `AppBar.vue` read like a control rail with stronger page hierarchy and cleaner action grouping
- improving `Drawer.vue` navigation modules, branding, and rail behavior for both themes
- tuning `View.vue` content width, spacing, and page transitions for lower visual noise
- wiring shell theme sync to the updated theme helpers

- [ ] **Step 4: Run build after shell changes**

Run: `npm run build`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add frontend/src/layouts/default/Default.vue frontend/src/layouts/default/AppBar.vue frontend/src/layouts/default/Drawer.vue frontend/src/layouts/default/View.vue frontend/src/plugins/theme.ts
git commit -m "feat: redesign application shell"
```

## Task 4: Homepage Control Surface Refinement

**Files:**
- Modify: `frontend/src/components/Main.vue`
- Modify: `frontend/src/components/tiles/Gauge.vue`
- Modify: `frontend/src/components/tiles/History.vue`

- [ ] **Step 1: Write the failing test**

No new homepage logic is required beyond current layout helpers. Existing homepage layout helper tests should remain green.

- [ ] **Step 2: Run targeted tests before homepage edits**

Run: `npm test -- src/features/dashboard/layout.test.ts src/features/dashboard/persistence.test.ts`
Expected: PASS

- [ ] **Step 3: Implement homepage visual refinements**

Update `frontend/src/components/Main.vue` to:
- strengthen separation between overview, primary runtime probe, metric, and detail sections
- remove the duplicated system-information cards
- replace them with one combined server probe card that merges CPU, RAM, Disk I/O, system information, and runtime information
- style the merged probe card closer to a server control panel, using occupancy rings plus dynamic stream bars for current status
- keep host, IP, uptime, app version, running status, memory, goroutines, and active users inside the same card
- reduce hero-like marketing copy feel and reinforce operational summaries
- add restrained motion staging and more consistent module surfaces
- improve light-theme readability and dark-theme hierarchy without increasing render cost

Update `frontend/src/components/tiles/Gauge.vue` and `frontend/src/components/tiles/History.vue` to:
- align typography and chart treatment with the new probe-card visual system
- support the merged-card presentation without reintroducing duplicated metric blocks

- [ ] **Step 4: Re-run targeted tests and build**

Run: `npm test -- src/features/dashboard/layout.test.ts src/features/dashboard/persistence.test.ts`
Expected: PASS

Run: `npm run build`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add frontend/src/components/Main.vue frontend/src/components/tiles/Gauge.vue frontend/src/components/tiles/History.vue
git commit -m "feat: refine homepage operations dashboard"
```

## Task 5: Representative Catalog, Inventory, and Control Pages

**Files:**
- Modify: `frontend/src/views/Clients.vue`
- Modify: `frontend/src/views/Inbounds.vue`
- Modify: `frontend/src/views/Outbounds.vue`
- Modify: `frontend/src/views/Settings.vue`

- [ ] **Step 1: Write the failing test**

No dedicated view tests exist for these pages. Preserve current functional behavior and validate through build plus existing normalization tests.

- [ ] **Step 2: Run existing related tests**

Run: `npm test -- src/features/settings/normalize.test.ts`
Expected: PASS

- [ ] **Step 3: Implement page-family refresh**

Update `frontend/src/views/Clients.vue` to:
- add a stronger page header block and summary framing
- style filters and tools as integrated control modules
- improve data-table affordance, mobile behavior, and action density

Update `frontend/src/views/Inbounds.vue` and `frontend/src/views/Outbounds.vue` to:
- align them to the approved card-first catalog workspace
- improve quick-action grouping, status presentation, and metadata hierarchy
- make them visually consistent with the new shell and homepage modules while preserving existing actions and modals

Update `frontend/src/views/Settings.vue` to:
- reorganize the page as a system control panel
- add section-level framing for interface, subscription, and export settings
- improve tab readability, action placement, and form grouping in both themes

- [ ] **Step 4: Run tests and build**

Run: `npm test -- src/features/settings/normalize.test.ts`
Expected: PASS

Run: `npm run build`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add frontend/src/views/Clients.vue frontend/src/views/Inbounds.vue frontend/src/views/Outbounds.vue frontend/src/views/Settings.vue
git commit -m "feat: refresh representative page families"
```

## Task 6: Final Verification

**Files:**
- Modify: none expected unless issues are found during verification

- [ ] **Step 1: Run full test suite**

Run: `npm test`
Expected: PASS

- [ ] **Step 2: Run production build**

Run: `npm run build`
Expected: PASS

- [ ] **Step 3: Perform manual review checklist**

Manually review:
- `/`
- `/clients`
- `/settings`
- one modal workflow
- light theme and dark theme
- desktop and mobile widths

- [ ] **Step 4: Commit any verification fixes if needed**

```bash
git add frontend/src
git commit -m "fix: polish operations console verification issues"
```

## Self-Review

Spec coverage:
- foundation tokens, themes, motion, and self-hosted typography: Tasks 1-3
- shell layer: Task 3
- homepage merged server probe card: Task 4
- clients table-first inventory, card-first catalog pages, and settings control panels: Task 5
- verification across themes and responsive layouts: Task 6

Placeholder scan:
- no TODO/TBD placeholders remain
- commands and concrete files are included for each task

Type consistency:
- theme sync API is consistently named `startThemeSync` and `stopThemeSync`
- page-family scope matches the spec and current representative pages
