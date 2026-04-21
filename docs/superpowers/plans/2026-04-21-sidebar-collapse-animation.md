# Sidebar Collapse Animation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the desktop left drawer collapse animation behave as the reverse of the expand animation while removing visible jitter.

**Architecture:** Keep the change localized to `Drawer.vue` by replacing the current asymmetric two-stage desktop animation logic with a mirrored transition model. The drawer will continue to use Vuetify rail mode as its resting collapsed state, but content visibility and rail switching will be coordinated from one transition phase so width, opacity, and transform changes stay aligned.

**Tech Stack:** Vue 3 `<script setup>`, TypeScript, Vuetify navigation drawer, scoped CSS, Vitest

---

### Task 1: Add transition-state tests for desktop drawer animation

**Files:**
- Modify: `src/frontend/src/layouts/default/Drawer.vue`
- Create: `src/frontend/src/layouts/default/Drawer.test.ts`

- [ ] **Step 1: Write the failing test**

```ts
import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { nextTick } from 'vue'
import Drawer from './Drawer.vue'

vi.mock('@/router', () => ({
  default: {
    currentRoute: {
      value: {
        path: '/',
      },
    },
  },
}))

vi.mock('@/plugins/httputil', () => ({
  logout: vi.fn(),
}))

const factory = (collapsed: boolean) => mount(Drawer, {
  props: {
    isMobile: false,
    displayDrawer: false,
    collapsed,
  },
  global: {
    stubs: {
      VNavigationDrawer: {
        template: '<div class="v-navigation-drawer"><slot /><slot name="append" /></div>',
      },
      VList: { template: '<div><slot /></div>' },
      VListItem: { template: '<div class="v-list-item"><slot /><slot name="prepend" /></div>' },
      VListItemTitle: { template: '<div class="v-list-item-title"><slot /></div>' },
      VTooltip: { template: '<div><slot /></div>' },
      VBtn: { template: '<button><slot /></button>' },
      VIcon: { template: '<i />' },
      VImg: { template: '<img />' },
    },
    mocks: {
      $t: (value: string) => value,
    },
  },
})

describe('Drawer desktop animation state', () => {
  it('keeps expanded content mounted while collapsing before rail settles', async () => {
    vi.useFakeTimers()
    const wrapper = factory(false)

    await wrapper.setProps({ collapsed: true })
    await nextTick()

    expect(wrapper.classes()).toContain('app-drawer--collapsing')
    expect(wrapper.classes()).not.toContain('app-drawer--rail-state')

    vi.advanceTimersByTime(200)
    await nextTick()

    expect(wrapper.classes()).toContain('app-drawer--rail-state')
    expect(wrapper.classes()).not.toContain('app-drawer--collapsing')

    vi.useRealTimers()
  })

  it('leaves rail immediately when expanding and restores visible content after the mirrored delay', async () => {
    vi.useFakeTimers()
    const wrapper = factory(true)

    await wrapper.setProps({ collapsed: false })
    await nextTick()

    expect(wrapper.classes()).toContain('app-drawer--expanding')
    expect(wrapper.classes()).not.toContain('app-drawer--rail-state')
    expect(wrapper.classes()).toContain('app-drawer--content-hidden')

    vi.advanceTimersByTime(200)
    await nextTick()

    expect(wrapper.classes()).not.toContain('app-drawer--expanding')
    expect(wrapper.classes()).not.toContain('app-drawer--content-hidden')

    vi.useRealTimers()
  })
})
```

- [ ] **Step 2: Run test to verify it fails**

Run: `npm test -- Drawer.test.ts`
Expected: FAIL because `Drawer.vue` does not yet expose mirrored desktop transition classes and timing.

- [ ] **Step 3: Write minimal implementation**

```ts
const DRAWER_TRANSITION_MS = 180
type DrawerPhase = 'expanded' | 'expanding' | 'collapsing' | 'collapsed'

const drawerPhase = ref<DrawerPhase>(props.isMobile ? 'expanded' : (props.collapsed ? 'collapsed' : 'expanded'))

const syncDrawerVisualState = () => {
  clearDrawerContentTimer()

  if (props.isMobile) {
    drawerPhase.value = 'expanded'
    return
  }

  if (props.collapsed) {
    drawerPhase.value = 'collapsing'
    drawerContentTimer = setTimeout(() => {
      drawerPhase.value = 'collapsed'
      drawerContentTimer = null
    }, DRAWER_TRANSITION_MS)
    return
  }

  drawerPhase.value = 'expanding'
  drawerContentTimer = setTimeout(() => {
    drawerPhase.value = 'expanded'
    drawerContentTimer = null
  }, DRAWER_TRANSITION_MS)
}

const isRail = computed((): boolean => !props.isMobile && drawerPhase.value === 'collapsed')
const isContentHidden = computed((): boolean => !props.isMobile && drawerPhase.value !== 'expanded' && drawerPhase.value !== 'collapsing')
const drawerClasses = computed(() => ({
  'app-drawer--rail-state': isRail.value,
  'app-drawer--content-hidden': isContentHidden.value,
  'app-drawer--expanding': drawerPhase.value === 'expanding',
  'app-drawer--collapsing': drawerPhase.value === 'collapsing',
}))
```

```vue
<v-navigation-drawer
  v-model="showDrawer"
  :class="['app-drawer', drawerClasses]"
  :temporary="isMobile"
  :permanent="!isMobile"
  :rail="isRail"
  :rail-width="92"
  :width="308"
>
```

```css
.app-drawer,
.app-drawer__titles,
.app-drawer__section,
.app-drawer__footer-note,
.app-drawer__logout,
.app-drawer__logout-label,
:deep(.app-drawer__item > .v-list-item__content) {
  transition-duration: 180ms;
}

.app-drawer--collapsing .app-drawer__titles,
.app-drawer--collapsing .app-drawer__section,
.app-drawer--collapsing .app-drawer__footer-note,
.app-drawer--collapsing .app-drawer__logout-label,
.app-drawer--collapsing :deep(.app-drawer__item > .v-list-item__content) {
  opacity: 0;
  transform: translateX(-8px);
}

.app-drawer--expanding .app-drawer__titles,
.app-drawer--expanding .app-drawer__section,
.app-drawer--expanding .app-drawer__footer-note,
.app-drawer--expanding .app-drawer__logout-label,
.app-drawer--expanding :deep(.app-drawer__item > .v-list-item__content) {
  opacity: 0;
  transform: translateX(-8px);
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `npm test -- Drawer.test.ts`
Expected: PASS with both transition-state tests green.

- [ ] **Step 5: Commit**

```bash
git add src/frontend/src/layouts/default/Drawer.vue src/frontend/src/layouts/default/Drawer.test.ts
git commit -m "fix: mirror drawer collapse animation"
```

### Task 2: Refine the CSS motion so collapse reads as a reverse of expand

**Files:**
- Modify: `src/frontend/src/layouts/default/Drawer.vue`
- Test: `src/frontend/src/layouts/default/Drawer.test.ts`

- [ ] **Step 1: Extend the test with rapid-toggle coverage**

```ts
it('cancels stale timers when toggled quickly between collapse and expand', async () => {
  vi.useFakeTimers()
  const wrapper = factory(false)

  await wrapper.setProps({ collapsed: true })
  await nextTick()
  await wrapper.setProps({ collapsed: false })
  await nextTick()

  vi.advanceTimersByTime(200)
  await nextTick()

  expect(wrapper.classes()).not.toContain('app-drawer--rail-state')
  expect(wrapper.classes()).not.toContain('app-drawer--content-hidden')
  expect(wrapper.classes()).not.toContain('app-drawer--collapsing')
  expect(wrapper.classes()).not.toContain('app-drawer--expanding')

  vi.useRealTimers()
})
```

- [ ] **Step 2: Run test to verify it fails**

Run: `npm test -- Drawer.test.ts`
Expected: FAIL if stale timers are still able to leave the drawer in the wrong end state.

- [ ] **Step 3: Write the minimal implementation and styling cleanup**

```ts
const clearDrawerContentTimer = () => {
  if (!drawerContentTimer) return
  clearTimeout(drawerContentTimer)
  drawerContentTimer = null
}

watch(() => [props.isMobile, props.collapsed], syncDrawerVisualState, { immediate: true })
onBeforeUnmount(clearDrawerContentTimer)
```

```css
.app-drawer__titles,
.app-drawer__section,
.app-drawer__footer-note,
.app-drawer__logout-label,
:deep(.app-drawer__item > .v-list-item__content) {
  overflow: hidden;
  transform-origin: left center;
  will-change: opacity, transform;
}

.app-drawer--content-hidden .app-drawer__titles,
.app-drawer--content-hidden .app-drawer__section,
.app-drawer--content-hidden .app-drawer__footer-note,
.app-drawer--content-hidden .app-drawer__logout-label,
.app-drawer--content-hidden :deep(.app-drawer__item > .v-list-item__content) {
  opacity: 0;
  pointer-events: none;
  transform: translateX(-8px);
}

.app-drawer--rail-state .app-drawer__logout {
  justify-content: center;
}
```

Remove duplicated transition blocks and any content-hiding rules that collapse layout too early during the transition.

- [ ] **Step 4: Run test to verify it passes**

Run: `npm test -- Drawer.test.ts`
Expected: PASS with the rapid-toggle test green alongside the earlier transition tests.

- [ ] **Step 5: Commit**

```bash
git add src/frontend/src/layouts/default/Drawer.vue src/frontend/src/layouts/default/Drawer.test.ts
git commit -m "fix: stabilize mirrored drawer transition"
```

### Task 3: Verify the drawer behavior in the real app

**Files:**
- Modify: `src/frontend/src/layouts/default/Drawer.vue` (only if verification exposes a bug)
- Test: `src/frontend/src/layouts/default/Drawer.test.ts`

- [ ] **Step 1: Run the focused unit test suite**

Run: `npm test -- Drawer.test.ts`
Expected: PASS

- [ ] **Step 2: Run the frontend test suite if available**

Run: `npm test`
Expected: PASS, or document unrelated pre-existing failures if they block the full run.

- [ ] **Step 3: Run the local app for manual verification**

Run: `npm run dev`
Expected: the app starts successfully and the desktop drawer can be toggled without jitter.

Manual checks:

- Collapse from expanded desktop state and verify labels fade/shift out while width contracts.
- Expand from collapsed desktop state and verify the motion is the reverse of collapse.
- Toggle repeatedly and verify no late snap to rail state.
- Cross the mobile breakpoint and verify the drawer does not get stuck in an intermediate state.

- [ ] **Step 4: Make the smallest fix if verification reveals a defect**

```ts
// Example of acceptable follow-up change shape inside Drawer.vue:
if (props.isMobile) {
  clearDrawerContentTimer()
  drawerPhase.value = 'expanded'
  return
}
```

- [ ] **Step 5: Commit**

```bash
git add src/frontend/src/layouts/default/Drawer.vue src/frontend/src/layouts/default/Drawer.test.ts
git commit -m "test: verify drawer animation behavior"
```
