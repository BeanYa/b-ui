import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'

const source = readFileSync(fileURLToPath(new URL('./Settings.vue', import.meta.url)), 'utf8')

describe('Settings subscription TLS link', () => {
  it('exposes a panel TLS link switch and disables custom paths while linked', () => {
    expect(source).toContain('v-model="subTLSUsesPanel"')
    expect(source).toContain(':disabled="subTLSUsesPanel"')
  })
})

describe('Settings region field', () => {
  it('renders a region display, manual country select, and auto-fetch button', () => {
    expect(source).toContain(':model-value="regionDisplay"')
    expect(source).toContain("$t('setting.regionManual')")
    expect(source).toContain('@click="autoFetchRegion"')
    expect(source).toContain(':loading="regionLoading"')
  })

  it('auto-fetch posts to the region fetch endpoint added in 9a', () => {
    expect(source).toContain("'api/setting/region/fetch'")
  })

  it('builds the region display from a flag and Intl.DisplayNames name', () => {
    expect(source).toContain('countryToFlag(regionCode.value)')
    expect(source).toContain('DisplayNames(')
  })

  it('renders the region caption through i18n instead of a hardcoded string', () => {
    expect(source).toContain("$t('setting.regionCaption')")
    expect(source).not.toContain('Detected or manual region used for naming and routing hints.')
  })
})

describe('Settings manualRegion spurious-save guard', () => {
  // Settings.vue cannot be mounted in this repo's test setup (no jsdom /
  // @vue/test-utils / Vuetify stubs configured). This test pins the contract
  // of the init-guard that prevents the watch(manualRegion) from firing on
  // the initial seed from persisted settings:
  //   1. a regionSeeded flag is declared and starts false
  //   2. the watch returns early until regionSeeded is true
  //   3. setData() flips regionSeeded to true after seeding manualRegion
  // Together: the very first mutation of manualRegion ('' -> persisted code)
  // is treated as a seed (no api/save), while any later user edit saves.
  it('declares regionSeeded starting false', () => {
    expect(source).toMatch(/let regionSeeded = false/)
  })

  it('early-returns from the manualRegion watch before regionSeeded is true', () => {
    expect(source).toMatch(/watch\(manualRegion,[\s\S]*?if \(!regionSeeded\) return/)
  })

  it('seeds manualRegion then sets regionSeeded=true inside setData', () => {
    const idx = source.indexOf('const setData')
    expect(idx).toBeGreaterThan(-1)
    const setDataBody = source.slice(idx, idx + 800)
    expect(setDataBody).toMatch(/if \(!manualRegion\.value\) manualRegion\.value = code/)
    expect(setDataBody).toMatch(/if \(!regionSeeded\) regionSeeded = true/)
    // seed must happen in setData body (after the assignment), not elsewhere
    const assignIdx = setDataBody.indexOf('manualRegion.value = code')
    const flagIdx = setDataBody.indexOf('regionSeeded = true')
    expect(flagIdx).toBeGreaterThan(assignIdx)
  })
})
