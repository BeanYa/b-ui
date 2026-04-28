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
