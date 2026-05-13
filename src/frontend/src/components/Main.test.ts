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

  it('renders total network traffic inside the server probe panel', () => {
    const source = readFileSync(fileURLToPath(new URL('./Main.vue', import.meta.url)), 'utf8')

    expect(source).toContain('class="probe-card__traffic-total"')
    expect(source).toContain("i18n.global.t('main.netTraffic.totalData')")
    expect(source).toContain("i18n.global.t('main.netTraffic.sent')")
    expect(source).toContain("i18n.global.t('main.netTraffic.received')")
    expect(source).toContain('HumanReadable.sizeFormat(tilesData.value.net?.sent)')
    expect(source).toContain('HumanReadable.sizeFormat(tilesData.value.net?.recv)')
  })

  it('fills each desktop overview card with column flex content', () => {
    const source = readFileSync(fileURLToPath(new URL('./Main.vue', import.meta.url)), 'utf8')
    const cssRule = (selector: string) => {
      const escapedSelector = selector.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
      return source.match(new RegExp(`${escapedSelector} \\{[\\s\\S]*?\\n\\}`))?.[0] ?? ''
    }

    expect(cssRule('.home-panel')).toContain('display: flex')
    expect(cssRule('.home-panel')).toContain('flex-direction: column')
    expect(cssRule('.home-panel')).toContain('height: 100%')
    expect(cssRule('.home-panel--map')).toContain('display: flex')
    expect(cssRule('.home-panel--runtime')).toContain('display: flex')
    expect(cssRule('.overview-grid')).toContain('flex: 1 1 auto')
    expect(cssRule('.overview-grid__item')).toContain('flex: 1 1 calc(50% - 5px)')
    expect(cssRule('.probe-card__streams')).toContain('flex: 1 1 auto')
    expect(cssRule('.probe-stream')).toContain('flex: 1 1 0')
  })
})
