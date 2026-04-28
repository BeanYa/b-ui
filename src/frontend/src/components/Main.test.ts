import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'

describe('Main dashboard source', () => {
  it('renders the app version as a first-class system fact beside the IP summary', () => {
    const source = readFileSync(fileURLToPath(new URL('./Main.vue', import.meta.url)), 'utf8')

    expect(source).toContain('v-for="item in systemFacts"')
    expect(source).toContain('class="probe-cluster__fact"')
    expect(source).toContain("label: 'Version'")
    expect(source).toContain('value: appVersion.value')
    expect(source).not.toContain('<span>{{ appVersion }}</span>')
  })
})
