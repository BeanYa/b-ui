import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'

const readSource = (path: string) => readFileSync(fileURLToPath(new URL(path, import.meta.url)), 'utf8')

describe('shared design consistency', () => {
  it('uses shared form card styling in common rule and dial editors', () => {
    const sources = [
      readSource('../../components/Dial.vue'),
      readSource('../../components/Rule.vue'),
      readSource('../../components/DnsRule.vue'),
    ].join('\n')

    expect(sources).toContain('class="app-form-card')
    expect(sources).not.toContain('style="background-color: inherit;"')
  })

  it('uses shared nested section styling in service editors', () => {
    const sources = [
      readSource('../../components/services/Ccm.vue'),
      readSource('../../components/services/Derp.vue'),
      readSource('../../components/services/Ocm.vue'),
      readSource('../../components/services/SSMAPI.vue'),
    ].join('\n')

    expect(sources).toContain('app-nested-card')
    expect(sources).not.toContain('style="margin: 4px; padding: 8px;"')
    expect(sources).not.toContain('style="padding: 8px;"')
  })

  it('uses shared modal body, code, and QR styling for utility dialogs', () => {
    const sources = [
      readSource('../../components/Editor.vue'),
      readSource('../../layouts/modals/Changes.vue'),
      readSource('../../layouts/modals/Logs.vue'),
      readSource('../../layouts/modals/QrCode.vue'),
      readSource('../../layouts/modals/Stats.vue'),
    ].join('\n')

    expect(sources).toContain('app-dialog__body')
    expect(sources).toContain('app-code-block')
    expect(sources).toContain('app-qr-panel')
    expect(sources).not.toContain('background-color: background')
  })

  it('uses shared form styling in TLS option editors and rule modals', () => {
    const sources = [
      readSource('../../components/tls/Acme.vue'),
      readSource('../../components/tls/Ech.vue'),
      readSource('../../layouts/modals/Rule.vue'),
      readSource('../../layouts/modals/DnsRule.vue'),
    ].join('\n')

    expect(sources).toContain('app-form-card')
    expect(sources).toContain('app-nested-card')
    expect(sources).not.toContain('style="background-color: inherit;')
    expect(sources).not.toContain('style="padding: 0;')
  })

  it('uses atmospheric console effects without grid backgrounds', () => {
    const settings = readSource('../../styles/settings.scss')
    const shell = readSource('../../layouts/default/Default.vue')
    const login = readSource('../../views/Login.vue')
    const backgroundSources = `${settings}\n${shell}\n${login}`

    expect(backgroundSources).not.toContain('linear-gradient(var(--app-bg-grid)')
    expect(backgroundSources).not.toContain('linear-gradient(90deg, var(--app-bg-grid)')
    expect(backgroundSources).not.toContain('--login-page-grid')
    expect(backgroundSources).not.toContain('linear-gradient(var(--login-page-grid)')
    expect(backgroundSources).not.toContain('linear-gradient(90deg, var(--login-page-grid)')
    expect(shell).toContain('shell-app__aurora')
    expect(shell).toContain('shell-app__scan')
    expect(login).toContain('--login-page-noise')
    expect(login).toContain('login-aurora-sweep')
    expect(login).toContain('login-scan-pass')
    expect(settings).toContain('--app-bg-noise')
    expect(settings).toContain('animation: app-panel-glow')
    expect(settings).toContain('@media (prefers-reduced-motion: reduce)')
  })

  it('presents the password login view as a B-UI branding page', () => {
    const login = readSource('../../views/Login.vue')

    expect(login).toContain('login-brand-bg')
    expect(login).not.toContain('<section :class="[\'login-panel\'')
    expect(login).toContain('B-UI Command Surface')
    expect(login).toContain('Route everything. Observe every pulse.')
    expect(login).toContain('brandCapabilities')
    expect(login).toContain('Live control plane preview')
    expect(login).toContain('login-brand__visual')
    expect(login).toContain('login-console--ambient')
    expect(login).toContain('border: 0')
    expect(login).toContain('login-window__form')
  })

  it('maps the shared shell to the Clay-inspired design tokens with light and dark themes', () => {
    const settings = readSource('../../styles/settings.scss')
    const shell = readSource('../../layouts/default/Default.vue')
    const appBar = readSource('../../layouts/default/AppBar.vue')

    expect(settings).toContain('--app-canvas: #fffaf0')
    expect(settings).toContain('--app-brand-pink: #ff4d8b')
    expect(settings).toContain('--app-brand-teal: #1a3a3a')
    expect(settings).toContain('--app-brand-lavender: #b8a4ed')
    expect(settings).toContain(':root[data-theme-name=\'light\']')
    expect(settings).toContain(':root[data-theme-name=\'dark\']')
    expect(settings).toContain('app-panel-float 8s')
    expect(shell).toContain('shell-app__clay-scene')
    expect(appBar).toContain('app-bar-shell__theme-toggle')
    expect(appBar).toContain('theme.global.name.value')
  })

  it('keeps custom button motion inside the Vuetify overlay layer', () => {
    const settings = readSource('../../styles/settings.scss')

    expect(settings).not.toContain('.v-btn::after')
    expect(settings).not.toContain('.v-btn:hover::after')
    expect(settings).toContain('.v-btn > .v-btn__overlay')
    expect(settings).toContain('.v-btn > .v-btn__overlay::after')
    expect(settings).toContain('.v-btn:hover > .v-btn__overlay::after')
  })

  it('uses a single shared shell frame for page title and routed content', () => {
    const defaultLayout = readSource('../../layouts/default/Default.vue')
    const view = readSource('../../layouts/default/View.vue')
    const main = readSource('../../components/Main.vue')
    const appBar = readSource('../../layouts/default/AppBar.vue')

    expect(defaultLayout).toContain('shell-frame')
    expect(defaultLayout).toContain('shell-frame__header')
    expect(defaultLayout).toContain('shell-frame__body')
    expect(defaultLayout).toContain('shell-app__workspace--expanded-nav')
    expect(defaultLayout).toContain('--shell-drawer-offset: 104px')
    expect(defaultLayout).toContain('--shell-drawer-offset: 320px')
    expect(appBar).not.toContain('<v-app-bar')
    expect(appBar).toContain('<header class="app-bar-shell"')
    expect(view).toContain('<main class="shell-main">')
    expect(main).toContain('background: transparent')
    expect(main).not.toContain('box-shadow: var(--app-shadow-device)')
    expect(main).not.toContain('margin-top: -10px')
  })

  it('keeps routed pages padded inside the shared frame when the desktop nav expands', () => {
    const settings = readSource('../../styles/settings.scss')
    const appPageRule = settings.match(/\.app-page \{[\s\S]*?\n\}/)?.[0] ?? ''
    const desktopCompactionRule = settings.match(/@media \(max-width: 1280px\) \{[\s\S]*?\.app-page \{[\s\S]*?\n  \}/)?.[0] ?? ''
    const mobileCompactionRule = settings.match(/@media \(max-width: 960px\) \{[\s\S]*?\.app-page \{[\s\S]*?\n  \}/)?.[0] ?? ''

    expect(appPageRule).toContain('padding: clamp(16px, 1.8vw, 26px)')
    expect(appPageRule).not.toContain('padding-top')
    expect(desktopCompactionRule).toContain('padding: 18px')
    expect(mobileCompactionRule).toContain('padding: 12px')
  })

  it('keeps catalog page controls compact instead of letting rows stretch through empty workspace', () => {
    const settings = readSource('../../styles/settings.scss')
    const catalogSources = [
      readSource('../../views/Admins.vue'),
      readSource('../../views/Basics.vue'),
      readSource('../../views/Dns.vue'),
      readSource('../../views/Endpoints.vue'),
      readSource('../../views/Rules.vue'),
      readSource('../../views/Services.vue'),
      readSource('../../views/Tls.vue'),
    ].join('\n')
    const appPageRowsRule = settings.match(/\.app-page > \.v-row \{[\s\S]*?\n\}/)?.[0] ?? ''
    const toolbarRule = settings.match(/\.app-page__toolbar \{[\s\S]*?\n\}/)?.[0] ?? ''
    const toolbarActionsRule = settings.match(/\.app-page__toolbar-actions \{[\s\S]*?\n\}/)?.[0] ?? ''
    const toolbarClusterRule = settings.match(/\.app-toolbar-cluster \{[\s\S]*?\n\}/)?.[0] ?? ''

    expect(appPageRowsRule).toContain('flex: 0 0 auto')
    expect(toolbarRule).toContain('flex: 0 0 auto')
    expect(toolbarActionsRule).toContain('gap: 8px')
    expect(toolbarActionsRule).toContain('width: auto')
    expect(toolbarActionsRule).toContain('max-width: 100%')
    expect(toolbarClusterRule).toContain('display: inline-flex')
    expect(toolbarClusterRule).toContain('padding: 6px')
    expect(toolbarClusterRule).toContain('width: fit-content')
    expect(catalogSources).toContain('app-page__toolbar-actions app-toolbar-cluster')
    expect(catalogSources).not.toContain('class="app-page__toolbar-actions"')
  })

  it('stacks the home overview above full-width telemetry instead of splitting the page into side columns', () => {
    const main = readSource('../../components/Main.vue')

    expect(main).toContain('grid-template-areas:')
    expect(main).toContain('\'hero map runtime\'')
    expect(main).toContain('grid-template-columns: minmax(380px, 1.15fr) minmax(260px, 0.75fr) minmax(330px, 0.95fr)')
    expect(main).toContain('.dashboard-shell__tiles')
    expect(main).toContain('padding: 0 clamp(18px, 2.2vw, 30px) clamp(18px, 2.2vw, 30px)')
    expect(main).not.toContain('display: flex;\n  flex: 1 1 auto;\n  padding: 0;')
  })
})
