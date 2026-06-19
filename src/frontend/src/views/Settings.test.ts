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
})
