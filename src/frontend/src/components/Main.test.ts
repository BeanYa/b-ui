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

  it('opens the panel update dialog before waiting for release metadata', () => {
    const source = readFileSync(fileURLToPath(new URL('./Main.vue', import.meta.url)), 'utf8')
    const openDialogStart = source.indexOf('const openPanelUpdateDialog = async () => {')
    const visibleIndex = source.indexOf('panelUpdateDialog.value.visible = true', openDialogStart)
    const loadingIndex = source.indexOf('panelUpdateDialog.value.loading = true', openDialogStart)
    const requestIndex = source.indexOf("await HttpUtils.get('api/panelUpdate')", openDialogStart)

    expect(openDialogStart).toBeGreaterThan(-1)
    expect(visibleIndex).toBeGreaterThan(openDialogStart)
    expect(loadingIndex).toBeGreaterThan(openDialogStart)
    expect(requestIndex).toBeGreaterThan(openDialogStart)
    expect(visibleIndex).toBeLessThan(requestIndex)
    expect(loadingIndex).toBeLessThan(requestIndex)
  })
})
