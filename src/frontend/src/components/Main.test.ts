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

  it('keeps the summary map and runtime sections as flexible columns inside the home summary grid', () => {
    const source = readFileSync(fileURLToPath(new URL('./Main.vue', import.meta.url)), 'utf8')
    const cssRules = (selector: string) => {
      const escapedSelector = selector.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
      const rulePattern = new RegExp(`(?:^|\\n)([^{}]*${escapedSelector}[^{}]*) \\{[\\s\\S]*?\\n\\}`, 'g')
      const rules = [...source.matchAll(rulePattern)].map(match => match[0])
      return rules.filter((rule) => rule
        .slice(0, rule.indexOf('{'))
        .split(',')
        .some(part => part.trim() === selector))
    }
    const cssRule = (selector: string) => cssRules(selector)[0] ?? ''
    const selectorHasDeclaration = (selector: string, declaration: string) =>
      cssRules(selector).some(rule => rule.includes(declaration))

    expect(source).toContain('<div class="home-summary">')
    expect(source).toContain('<section class="home-summary__map">')
    expect(source).toContain('<section class="home-summary__runtime">')
    expect(cssRule('.home-panel')).toContain('display: flex')
    expect(cssRule('.home-panel')).toContain('flex-direction: column')
    expect(cssRule('.home-panel')).toContain('height: 100%')
    expect(cssRule('.home-summary')).toContain('display: grid')
    expect(cssRule('.home-summary')).toContain('grid-template-columns: minmax(320px, 1.08fr) minmax(300px, 0.9fr) minmax(430px, 1.28fr)')
    expect(cssRule('.home-summary__map')).toContain('display: flex')
    expect(cssRule('.home-summary__map')).toContain('flex-direction: column')
    expect(cssRule('.home-summary__runtime')).toContain('display: flex')
    expect(cssRule('.home-summary__runtime')).toContain('flex-direction: column')
    expect(cssRule('.overview-grid')).toContain('display: grid')
    expect(cssRule('.overview-grid')).toContain('grid-template-columns: repeat(auto-fit, minmax(min(100%, 128px), 1fr))')
    expect(cssRule('.overview-grid__item')).toContain('display: flex')
    expect(cssRule('.overview-grid__item')).toContain('flex-direction: column')
    expect(selectorHasDeclaration('.probe-card__streams', 'flex: 1 1 auto')).toBe(true)
    expect(cssRule('.probe-stream')).toContain('flex: 1 1 0')
  })
})
