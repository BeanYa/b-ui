# TLS Template And SNI Candidate Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Align TLS, hysteria2, and reality preset defaults with the approved design and make SNI domain candidate diagnostics render as display-only labels while saving only the selected domain.

**Architecture:** Keep the change localized to the existing frontend TLS preset factory and TLS modal. Add small focused unit tests around preset generation and domain-hint item mapping, then implement the minimal preset and rendering changes needed to satisfy those tests.

**Tech Stack:** Vue 3, Vuetify, TypeScript, Vitest

---

## File Map

- Modify: `src/frontend/src/plugins/tlsTemplates.ts`
- Modify: `src/frontend/src/layouts/modals/Tls.vue`
- Create: `src/frontend/src/plugins/tlsTemplates.test.ts`
- Create: `src/frontend/src/layouts/modals/tlsDomainHints.test.ts`

### Task 1: Lock In Preset Defaults With Tests

**Files:**
- Create: `src/frontend/src/plugins/tlsTemplates.test.ts`
- Modify: `src/frontend/src/plugins/tlsTemplates.ts`

- [ ] **Step 1: Write the failing test**

```ts
import { describe, expect, it } from 'vitest'

import { createTlsPreset } from './tlsTemplates'

describe('createTlsPreset', () => {
  it('keeps only SNI and ALPN enabled for the standard preset and enables insecure', () => {
    const preset = createTlsPreset('standard')

    expect(preset.server.server_name).toBe('')
    expect(preset.server.alpn).toEqual(['h2', 'http/1.1'])
    expect(preset.server.min_version).toBeUndefined()
    expect(preset.server.max_version).toBeUndefined()
    expect(preset.client.insecure).toBe(true)
  })

  it('keeps only SNI enabled for the hysteria2 preset and enables insecure', () => {
    const preset = createTlsPreset('hysteria2')

    expect(preset.server.server_name).toBe('')
    expect(preset.server.alpn).toBeUndefined()
    expect(preset.server.min_version).toBeUndefined()
    expect(preset.server.max_version).toBeUndefined()
    expect(preset.client.insecure).toBe(true)
  })

  it('leaves max time difference disabled for the reality preset', () => {
    const preset = createTlsPreset('reality')

    expect(preset.server.reality?.max_time_difference).toBeUndefined()
  })
})
```

- [ ] **Step 2: Run test to verify it fails**

Run: `npm test -- src/plugins/tlsTemplates.test.ts`
Expected: FAIL because the current presets still include `min_version`, `max_version`, and `max_time_difference`, and do not set `client.insecure`.

- [ ] **Step 3: Write minimal implementation**

```ts
const presets: Record<TlsPresetKey, Omit<tls, 'name' | 'id'>> = {
  standard: {
    server: {
      enabled: true,
      server_name: '',
      alpn: ['h2', 'http/1.1'],
      certificate_path: '',
      key_path: '',
    },
    client: {
      insecure: true,
    },
  },
  hysteria2: {
    server: {
      enabled: true,
      server_name: '',
      certificate_path: '',
      key_path: '',
    },
    client: {
      insecure: true,
    },
  },
  reality: {
    server: {
      enabled: true,
      server_name: 'www.youtube.com',
      reality: {
        enabled: true,
        handshake: {
          server: 'www.youtube.com',
          server_port: 443,
        },
        private_key: '',
        short_id: RandomUtil.randomShortId(),
      },
    },
    client: {
      utls: {
        enabled: true,
        fingerprint: 'chrome',
      },
      reality: {
        enabled: true,
        public_key: '',
        short_id: '',
      },
    },
  },
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `npm test -- src/plugins/tlsTemplates.test.ts`
Expected: PASS

### Task 2: Lock In Domain Hint Value Separation With Tests

**Files:**
- Create: `src/frontend/src/layouts/modals/tlsDomainHints.test.ts`
- Modify: `src/frontend/src/layouts/modals/Tls.vue`

- [ ] **Step 1: Write the failing test**

```ts
import { describe, expect, it } from 'vitest'

import { buildDomainHintItems } from './Tls.vue'

describe('buildDomainHintItems', () => {
  it('keeps the saved value as the domain and moves diagnostics into labels', () => {
    const items = buildDomainHintItems([
      {
        domain: 'example.com',
        status: 'recommended',
        tlsVersion: 'TLS 1.3',
        alpn: 'h2',
        latencyMs: 8,
      },
    ])

    expect(items).toEqual([
      {
        value: 'example.com',
        domain: 'example.com',
        metaLabels: ['Recommended', 'TLS 1.3', 'H2', '8ms'],
      },
    ])
  })
})
```

- [ ] **Step 2: Run test to verify it fails**

Run: `npm test -- src/layouts/modals/tlsDomainHints.test.ts`
Expected: FAIL because no export currently exists for the domain-hint transformation.

- [ ] **Step 3: Write minimal implementation**

```ts
export interface DomainHintDisplayItem {
  value: string
  domain: string
  metaLabels: string[]
}

export const buildDomainHintItems = (
  items: DomainHintItem[],
  t: (key: string) => string,
): DomainHintDisplayItem[] => {
  return items.map((item) => {
    const metaLabels = [t(`tls.status.${item.status}`)]
    if (item.tlsVersion) metaLabels.push(item.tlsVersion)
    if (item.alpn) metaLabels.push(item.alpn.toUpperCase())
    if (item.redirect) metaLabels.push(t('tls.redirected'))
    if (item.latencyMs) metaLabels.push(`${item.latencyMs}ms`)

    return {
      value: item.domain,
      domain: item.domain,
      metaLabels,
    }
  })
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `npm test -- src/layouts/modals/tlsDomainHints.test.ts`
Expected: PASS

### Task 3: Render Candidate Diagnostics As Labels

**Files:**
- Modify: `src/frontend/src/layouts/modals/Tls.vue`
- Test: `src/frontend/src/layouts/modals/tlsDomainHints.test.ts`

- [ ] **Step 1: Update the combobox item model and rendering**

```vue
<v-combobox
  :items="domainHintItems"
  item-title="domain"
  item-value="value"
  v-model="inTls.server_name"
>
  <template #item="{ props, item }">
    <v-list-item v-bind="props" :title="item.raw.domain">
      <template #append>
        <div class="tls-domain-hint-labels">
          <v-chip
            v-for="label in item.raw.metaLabels"
            :key="label"
            size="x-small"
            variant="text"
          >
            {{ label }}
          </v-chip>
        </div>
      </template>
    </v-list-item>
  </template>
  <template #selection="{ item }">
    <span>{{ item.raw.domain }}</span>
  </template>
</v-combobox>
```

- [ ] **Step 2: Apply the same rendering to the Reality handshake server combobox**

```vue
<v-combobox
  :items="domainHintItems"
  item-title="domain"
  item-value="value"
  v-model="inTls.reality.handshake.server"
>
  <!-- same item and selection slots -->
</v-combobox>
```

- [ ] **Step 3: Keep manual entry behavior intact**

```ts
domainHintItems(): DomainHintDisplayItem[] {
  return buildDomainHintItems(this.domainHints, (key) => this.$t(key).toString())
}
```

- [ ] **Step 4: Run the focused tests**

Run: `npm test -- src/layouts/modals/tlsDomainHints.test.ts src/plugins/tlsTemplates.test.ts`
Expected: PASS

### Task 4: Verify And Commit

**Files:**
- Modify: `docs/superpowers/specs/2026-04-21-tls-template-sni-candidate-design.md` only if verification finds a mismatch

- [ ] **Step 1: Run all relevant frontend tests**

Run: `npm test -- src/plugins/tlsTemplates.test.ts src/layouts/modals/tlsDomainHints.test.ts`
Expected: PASS with no failing assertions.

- [ ] **Step 2: Run any available targeted frontend quality check**

Run: `npm run build`
Expected: successful frontend build.

- [ ] **Step 3: Commit the implementation**

```bash
git add src/frontend/src/plugins/tlsTemplates.ts src/frontend/src/layouts/modals/Tls.vue src/frontend/src/plugins/tlsTemplates.test.ts src/frontend/src/layouts/modals/tlsDomainHints.test.ts docs/superpowers/specs/2026-04-21-tls-template-sni-candidate-design.md docs/superpowers/plans/2026-04-21-tls-template-sni-candidate.md
git commit -m "fix: align TLS presets and SNI candidate labels"
```

- [ ] **Step 4: Create and push the release tag**

```bash
git tag <new-version-tag>
git push origin HEAD
git push origin <new-version-tag>
```

Use the repository's existing version-tag naming scheme discovered from git tags before creating the new tag.
