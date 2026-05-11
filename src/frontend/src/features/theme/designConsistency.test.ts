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

    expect(login).toContain('login-panel')
    expect(login).toContain('login-brand')
    expect(login).toContain('B-UI Command Surface')
    expect(login).toContain('Route everything. Observe every pulse.')
    expect(login).toContain('brandCapabilities')
    expect(login).toContain('Live control plane preview')
    expect(login).toContain('login-brand__visual')
    expect(login).toContain('login-window__form')
  })
})
