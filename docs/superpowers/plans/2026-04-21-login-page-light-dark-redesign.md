# Login Page Light/Dark Redesign Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Rebuild the login page as a single centered desktop-style window that keeps one layout across light and dark themes while preserving existing login, language, and theme behavior.

**Architecture:** Keep the login behavior in `Login.vue`, but split theme-aware presentational decisions into a tiny helper so the new surface model can be unit-tested. Then replace the current split-panel template with a single-window structure and rewrite the scoped styles around shared layout plus theme-specific CSS variables.

**Tech Stack:** Vue 3, TypeScript, Vuetify 4, Vite, Vitest, scoped CSS

---

## File Structure

- Modify: `src/frontend/src/views/Login.vue`
  - Replace the current split-panel markup with a single-window layout.
  - Keep login, locale, and theme-change behavior intact.
  - Consume a small presentational helper for subtitle text and theme labels if needed.
- Create: `src/frontend/src/views/loginWindowTheme.ts`
  - Hold pure helper logic for theme-aware semantic values used by the login view.
  - Keep the output minimal: root modifier class names and subtitle copy.
- Create: `src/frontend/src/views/loginWindowTheme.test.ts`
  - Unit-test the helper so the page's light/dark structure and copy contract are protected.

## Task 1: Add a Testable Theme Helper

**Files:**
- Create: `src/frontend/src/views/loginWindowTheme.ts`
- Test: `src/frontend/src/views/loginWindowTheme.test.ts`

- [ ] **Step 1: Write the failing test**

```ts
import { describe, expect, it } from 'vitest'

import { getLoginWindowThemeModel } from '@/views/loginWindowTheme'

describe('getLoginWindowThemeModel', () => {
  it('returns the light modifier and compact subtitle for light theme', () => {
    expect(getLoginWindowThemeModel('light')).toEqual({
      rootClass: 'login-shell--light',
      surfaceClass: 'login-window--light',
      subtitle: 'Access the B-UI control surface with your administrator account.',
    })
  })

  it('returns the dark modifier and the same subtitle for dark theme', () => {
    expect(getLoginWindowThemeModel('dark')).toEqual({
      rootClass: 'login-shell--dark',
      surfaceClass: 'login-window--dark',
      subtitle: 'Access the B-UI control surface with your administrator account.',
    })
  })
})
```

- [ ] **Step 2: Run test to verify it fails**

Run: `npm run test -- src/views/loginWindowTheme.test.ts`
Expected: FAIL with a module resolution error for `@/views/loginWindowTheme`

- [ ] **Step 3: Write minimal implementation**

```ts
export type LoginWindowThemeName = 'light' | 'dark'

type LoginWindowThemeModel = {
  rootClass: 'login-shell--light' | 'login-shell--dark'
  surfaceClass: 'login-window--light' | 'login-window--dark'
  subtitle: string
}

const LOGIN_SUBTITLE = 'Access the B-UI control surface with your administrator account.'

export const getLoginWindowThemeModel = (
  themeName: LoginWindowThemeName
): LoginWindowThemeModel => {
  if (themeName === 'dark') {
    return {
      rootClass: 'login-shell--dark',
      surfaceClass: 'login-window--dark',
      subtitle: LOGIN_SUBTITLE,
    }
  }

  return {
    rootClass: 'login-shell--light',
    surfaceClass: 'login-window--light',
    subtitle: LOGIN_SUBTITLE,
  }
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `npm run test -- src/views/loginWindowTheme.test.ts`
Expected: PASS with 2 tests passed

- [ ] **Step 5: Commit**

```bash
git add src/frontend/src/views/loginWindowTheme.ts src/frontend/src/views/loginWindowTheme.test.ts
git commit -m "test: add login window theme helper"
```

## Task 2: Rebuild the Login Template Around a Single Window

**Files:**
- Modify: `src/frontend/src/views/Login.vue`
- Create: `src/frontend/src/views/loginWindowTheme.ts`

- [ ] **Step 1: Write the failing test for the helper usage contract**

Update `src/frontend/src/views/loginWindowTheme.test.ts` to assert the helper stays limited to the two supported themes and the stable subtitle string:

```ts
import { describe, expect, it } from 'vitest'

import { getLoginWindowThemeModel } from '@/views/loginWindowTheme'

describe('getLoginWindowThemeModel', () => {
  it('keeps distinct root and surface classes for each supported theme', () => {
    const lightModel = getLoginWindowThemeModel('light')
    const darkModel = getLoginWindowThemeModel('dark')

    expect(lightModel.rootClass).not.toBe(darkModel.rootClass)
    expect(lightModel.surfaceClass).not.toBe(darkModel.surfaceClass)
    expect(lightModel.subtitle).toBe(darkModel.subtitle)
  })
})
```

- [ ] **Step 2: Run test to verify the current helper still passes before the view refactor**

Run: `npm run test -- src/views/loginWindowTheme.test.ts`
Expected: PASS with 1 additional test passed

- [ ] **Step 3: Rewrite the template and script to use the single-window structure**

Replace the template and the top part of the script in `src/frontend/src/views/Login.vue` with this structure, keeping the existing login logic below it:

```vue
<template>
  <v-container :class="['login-shell', themeModel.rootClass, 'fill-height']" fluid>
    <v-row align="center" class="fill-height login-shell__row" justify="center">
      <v-col cols="12" sm="10" md="8" lg="6" xl="5">
        <section :class="['login-window', themeModel.surfaceClass]">
          <header class="login-window__header">
            <div class="login-window__brand">
              <v-img class="login-window__logo" src="@/assets/logo.svg" width="18" />
              <span>B-UI</span>
            </div>
            <div class="login-window__toolbar">
              <v-select
                v-model="$i18n.locale"
                class="login-window__locale"
                density="comfortable"
                hide-details
                :items="languages"
                menu-icon="mdi-chevron-down"
                variant="outlined"
                @update:modelValue="changeLocale"
              />
              <v-menu>
                <template #activator="{ props }">
                  <v-btn class="login-window__theme" icon variant="outlined" v-bind="props">
                    <v-icon>mdi-theme-light-dark</v-icon>
                  </v-btn>
                </template>
                <v-list>
                  <v-list-item
                    v-for="th in themes"
                    :key="th.value"
                    :active="isActiveTheme(th.value)"
                    :prepend-icon="th.icon"
                    @click="changeTheme(th.value)"
                  >
                    <v-list-item-title>{{ $t(`theme.${th.value}`) }}</v-list-item-title>
                  </v-list-item>
                </v-list>
              </v-menu>
            </div>
          </header>

          <div class="login-window__body">
            <div class="login-window__intro">
              <p class="login-window__eyebrow">B-UI</p>
              <h1 class="login-window__title">{{ $t('login.title') }}</h1>
              <p class="login-window__subtitle">{{ themeModel.subtitle }}</p>
            </div>

            <v-form class="login-window__form" @submit.prevent="login" ref="form">
              <v-text-field
                v-model="username"
                :label="$t('login.username')"
                :rules="usernameRules"
                class="login-window__field"
                prepend-inner-icon="mdi-account"
                required
                variant="outlined"
              />
              <v-text-field
                v-model="password"
                :label="$t('login.password')"
                :rules="passwordRules"
                class="login-window__field"
                prepend-inner-icon="mdi-lock"
                required
                type="password"
                variant="outlined"
              />
              <v-btn
                :loading="loading"
                block
                class="login-window__submit"
                type="submit"
              >
                {{ $t('actions.submit') }}
              </v-btn>
            </v-form>
          </div>
        </section>
      </v-col>
    </v-row>
  </v-container>
</template>

<script lang="ts" setup>
import { computed, ref } from 'vue'
import { useLocale, useTheme } from 'vuetify'
import { i18n, languages } from '@/locales'
import { useRouter } from 'vue-router'
import HttpUtil from '@/plugins/httputil'
import { applyThemePreference, getThemePreference, resolveThemeName, type ThemePreference } from '@/plugins/theme'
import { getLoginWindowThemeModel } from '@/views/loginWindowTheme'

const theme = useTheme()
const locale = useLocale()

const themeModel = computed(() => getLoginWindowThemeModel(resolveThemeName(getThemePreference())))

const themes = [
  { value: 'light', icon: 'mdi-white-balance-sunny' },
  { value: 'dark', icon: 'mdi-moon-waning-crescent' },
  { value: 'system', icon: 'mdi-laptop' },
]
```

Keep the existing `username`, `password`, `loading`, `login`, `changeLocale`, `changeTheme`, and `isActiveTheme` logic below this block.

- [ ] **Step 4: Run the helper test to verify the refactor did not break imports**

Run: `npm run test -- src/views/loginWindowTheme.test.ts`
Expected: PASS with all helper tests still green

- [ ] **Step 5: Commit**

```bash
git add src/frontend/src/views/Login.vue src/frontend/src/views/loginWindowTheme.ts src/frontend/src/views/loginWindowTheme.test.ts
git commit -m "feat: convert login page to single window layout"
```

## Task 3: Rebuild the Scoped Styles for Light/Dark Window Materials

**Files:**
- Modify: `src/frontend/src/views/Login.vue:style scoped`

- [ ] **Step 1: Write the failing visual-structure test by extending the helper test with exact class outputs**

Append this case to `src/frontend/src/views/loginWindowTheme.test.ts`:

```ts
it('exposes the exact window surface modifiers used by Login.vue styles', () => {
  expect(getLoginWindowThemeModel('light').surfaceClass).toBe('login-window--light')
  expect(getLoginWindowThemeModel('dark').surfaceClass).toBe('login-window--dark')
})
```

- [ ] **Step 2: Run test to verify it passes before touching CSS**

Run: `npm run test -- src/views/loginWindowTheme.test.ts`
Expected: PASS

- [ ] **Step 3: Replace the current scoped styles with the single-window theme-aware CSS**

Replace the entire `<style scoped>` block in `src/frontend/src/views/Login.vue` with:

```css
.login-shell {
  --login-page-bg: #eef3f8;
  --login-page-grid: rgba(110, 138, 170, 0.08);
  --login-surface: rgba(255, 255, 255, 0.88);
  --login-surface-strong: #ffffff;
  --login-border: rgba(145, 164, 187, 0.28);
  --login-border-strong: rgba(112, 134, 162, 0.4);
  --login-shadow:
    0 24px 60px rgba(44, 65, 91, 0.12),
    0 1px 0 rgba(255, 255, 255, 0.7) inset,
    0 0 0 1px rgba(255, 255, 255, 0.6) inset;
  --login-text: #142132;
  --login-text-muted: #5f7086;
  --login-toolbar: rgba(246, 249, 252, 0.92);
  --login-field-bg: rgba(247, 250, 252, 0.96);
  --login-field-border: rgba(145, 164, 187, 0.28);
  --login-accent: #55b3ff;
  --login-button-bg: #142132;
  --login-button-text: #f8fbff;
  align-items: center;
  background:
    linear-gradient(180deg, rgba(255, 255, 255, 0.75), rgba(238, 243, 248, 0.92)),
    linear-gradient(rgba(110, 138, 170, 0.08) 1px, transparent 1px),
    linear-gradient(90deg, rgba(110, 138, 170, 0.08) 1px, transparent 1px),
    var(--login-page-bg);
  background-size: auto, 28px 28px, 28px 28px, auto;
  display: flex;
  min-height: 100vh;
  padding: 28px;
  position: relative;
}

.login-shell--dark {
  --login-page-bg: #07080a;
  --login-page-grid: rgba(255, 255, 255, 0.04);
  --login-surface: rgba(16, 17, 17, 0.92);
  --login-surface-strong: #101111;
  --login-border: rgba(255, 255, 255, 0.08);
  --login-border-strong: rgba(255, 255, 255, 0.14);
  --login-shadow:
    0 28px 70px rgba(0, 0, 0, 0.45),
    0 1px 0 rgba(255, 255, 255, 0.05) inset,
    0 0 0 1px rgba(7, 8, 10, 0.9) inset;
  --login-text: #f9f9f9;
  --login-text-muted: #9c9c9d;
  --login-toolbar: rgba(18, 18, 18, 0.9);
  --login-field-bg: rgba(7, 8, 10, 0.92);
  --login-field-border: rgba(255, 255, 255, 0.1);
  --login-accent: #55b3ff;
  --login-button-bg: rgba(255, 255, 255, 0.92);
  --login-button-text: #18191a;
  background:
    radial-gradient(circle at top left, rgba(255, 99, 99, 0.08), transparent 24%),
    radial-gradient(circle at bottom right, rgba(85, 179, 255, 0.08), transparent 24%),
    linear-gradient(rgba(255, 255, 255, 0.04) 1px, transparent 1px),
    linear-gradient(90deg, rgba(255, 255, 255, 0.04) 1px, transparent 1px),
    var(--login-page-bg);
  background-size: auto, auto, 28px 28px, 28px 28px, auto;
}

.login-shell__row {
  min-height: calc(100vh - 56px);
}

.login-window {
  background: var(--login-surface);
  border: 1px solid var(--login-border);
  border-radius: 24px;
  box-shadow: var(--login-shadow);
  color: var(--login-text);
  overflow: hidden;
  position: relative;
  backdrop-filter: blur(18px);
}

.login-window__header {
  align-items: center;
  background: var(--login-toolbar);
  border-bottom: 1px solid var(--login-border);
  display: flex;
  gap: 16px;
  justify-content: space-between;
  padding: 16px 18px;
}

.login-window__brand {
  align-items: center;
  color: var(--login-text);
  display: inline-flex;
  font-size: 13px;
  font-weight: 700;
  gap: 10px;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}

.login-window__logo {
  flex: none;
}

.login-window__toolbar {
  align-items: center;
  display: inline-grid;
  gap: 10px;
  grid-template-columns: minmax(0, 156px) auto;
}

.login-window__body {
  display: grid;
  gap: 24px;
  padding: 28px;
}

.login-window__intro {
  display: grid;
  gap: 8px;
}

.login-window__eyebrow {
  color: var(--login-text-muted);
  font-size: 12px;
  font-weight: 700;
  letter-spacing: 0.12em;
  margin: 0;
  text-transform: uppercase;
}

.login-window__title {
  color: var(--login-text);
  font-size: clamp(30px, 4vw, 40px);
  font-weight: 700;
  letter-spacing: -0.03em;
  line-height: 1.02;
  margin: 0;
}

.login-window__subtitle {
  color: var(--login-text-muted);
  font-size: 15px;
  line-height: 1.6;
  margin: 0;
  max-width: 34ch;
}

.login-window__form {
  display: grid;
  gap: 14px;
}

.login-window__submit {
  background: var(--login-button-bg);
  color: var(--login-button-text);
  min-height: 48px;
  margin-top: 6px;
}

:deep(.login-window__field .v-field),
:deep(.login-window__locale .v-field),
.login-window__theme {
  background: var(--login-field-bg);
  border: 1px solid var(--login-field-border);
  box-shadow: 0 1px 0 rgba(255, 255, 255, 0.04) inset;
}

:deep(.login-window__field .v-field),
:deep(.login-window__locale .v-field) {
  border-radius: 14px;
}

:deep(.login-window__field .v-field__input),
:deep(.login-window__field .v-label),
:deep(.login-window__locale .v-field__input),
:deep(.login-window__locale .v-select__selection-text) {
  color: var(--login-text);
}

:deep(.login-window__field .v-field__prepend-inner),
:deep(.login-window__locale .v-field__append-inner),
:deep(.login-window__locale .v-field__clearable) {
  color: var(--login-text-muted);
}

:deep(.login-window__field .v-field--focused),
:deep(.login-window__locale .v-field--focused),
.login-window__theme:hover,
.login-window__theme:focus-visible {
  border-color: color-mix(in srgb, var(--login-accent) 48%, var(--login-field-border));
  box-shadow:
    0 0 0 3px color-mix(in srgb, var(--login-accent) 18%, transparent),
    0 1px 0 rgba(255, 255, 255, 0.04) inset;
}

.login-window__theme {
  border-radius: 14px;
  min-height: 44px;
  min-width: 44px;
}

@media (max-width: 960px) {
  .login-shell {
    padding: 16px;
  }

  .login-window__body {
    padding: 22px;
  }
}

@media (max-width: 600px) {
  .login-window {
    border-radius: 20px;
  }

  .login-window__header {
    align-items: stretch;
    flex-direction: column;
  }

  .login-window__toolbar {
    grid-template-columns: minmax(0, 1fr) auto;
  }

  .login-window__body {
    gap: 20px;
    padding: 18px;
  }
}
```

- [ ] **Step 4: Run the helper test and frontend build**

Run: `npm run test -- src/views/loginWindowTheme.test.ts && npm run build`
Expected: PASS for the helper test, then successful frontend build output from Vite and `sync-web-html.mjs`

- [ ] **Step 5: Commit**

```bash
git add src/frontend/src/views/Login.vue src/frontend/src/views/loginWindowTheme.test.ts
git commit -m "feat: restyle login window for light and dark themes"
```

## Task 4: Manual Verification And Cleanup

**Files:**
- Modify: `src/frontend/src/views/Login.vue` (only if verification exposes small polish issues)

- [ ] **Step 1: Run targeted manual verification in the browser**

Run: `npm run dev`
Expected: Vite dev server starts and exposes a local URL.

Check these cases manually:

```text
1. Light theme shows a single centered window with no left-side brand column.
2. Dark theme shows the same structure with near-black surfaces and clean contrast.
3. Theme switching changes materials only; the layout does not jump.
4. Language selector remains in the window header and still updates locale.
5. Mobile width keeps the same one-card layout.
```

- [ ] **Step 2: Apply minimal polish only if one of the above checks fails**

If spacing or contrast needs a small fix, keep the patch inside `Login.vue` and limit it to values like these:

```css
.login-window__subtitle {
  max-width: 32ch;
}

.login-window__toolbar {
  grid-template-columns: minmax(0, 148px) auto;
}
```

- [ ] **Step 3: Re-run the full verification command**

Run: `npm run test -- src/views/loginWindowTheme.test.ts && npm run build`
Expected: PASS and successful build after any polish adjustment

- [ ] **Step 4: Commit**

```bash
git add src/frontend/src/views/Login.vue src/frontend/src/views/loginWindowTheme.ts src/frontend/src/views/loginWindowTheme.test.ts
git commit -m "refactor: polish responsive login window presentation"
```

## Self-Review

### Spec Coverage

- Single centered window in both themes: Task 2 and Task 3
- Remove left-side brand panel: Task 2
- Keep language and theme controls: Task 2 and Task 4
- Keep existing login behavior: Task 2
- Keep one structure across desktop/mobile/themes: Task 2, Task 3, and Task 4
- Make light theme intentional and dark theme aligned with `DESIGN.md`: Task 3

### Placeholder Scan

- No `TODO`/`TBD` placeholders remain
- Each code step includes concrete code
- Each verification step includes exact commands and expected outputs

### Type Consistency

- `LoginWindowThemeName` uses only `'light' | 'dark'`
- `getLoginWindowThemeModel()` returns the same class names used in `Login.vue` and CSS
- The subtitle string is kept stable across tests and the view contract
