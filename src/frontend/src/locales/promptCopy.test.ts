import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'

import en from './en'
import fa from './fa'
import ru from './ru'
import vi from './vi'
import zhcn from './zhcn'
import zhtw from './zhtw'

describe('prompt and dialog copy', () => {
  const locales = [en, fa, ru, vi, zhcn, zhtw]

  it('defines localized settings save and web terminal dialog copy', () => {
    const webTerminalKeys = [
      'connectionStatus',
      'connect',
      'disconnect',
      'transcript',
      'placeholder',
      'activationTitle',
      'activationCopy',
      'connectTitle',
      'connectCopy',
      'stay',
      'leaveTitle',
      'leaveCopy',
      'leaveAndAbort',
      'terminalOpening',
      'terminalConnected',
      'terminalDisconnected',
      'terminalConnectionError',
      'terminalMalformedMessage',
      'terminalAborted',
      'terminalStartPrompt',
      'beforeUnload',
    ] as const

    for (const messages of locales) {
      expect(messages.setting.saved).toBeTruthy()
      for (const key of webTerminalKeys) {
        expect(messages.webTerminal[key]).toBeTruthy()
        expect(messages.webTerminal[key]).not.toBe(`webTerminal.${key}`)
      }
    }
  })

  it('keeps editor validation toasts split into localized title and message fields', () => {
    const subClashExtSource = readFileSync(fileURLToPath(new URL('../components/SubClashExt.vue', import.meta.url)), 'utf8')
    const subJsonExtSource = readFileSync(fileURLToPath(new URL('../components/SubJsonExt.vue', import.meta.url)), 'utf8')

    for (const source of [subClashExtSource, subJsonExtSource]) {
      expect(source).toContain("title: i18n.global.t('failed')")
      expect(source).toContain("message: i18n.global.t('error.invalidData')")
      expect(source).not.toContain("i18n.global.t('failed') + \": \" + i18n.global.t('error.invalidData')")
    }
  })

  it('uses a localized sentence for the settings save toast', () => {
    const source = readFileSync(fileURLToPath(new URL('../views/Settings.vue', import.meta.url)), 'utf8')

    expect(source).toContain("message: i18n.global.t('setting.saved')")
    expect(source).not.toContain("i18n.global.t('actions.set') + \" \" + i18n.global.t('pages.settings')")
  })

  it('renders web terminal dialogs from locale keys instead of fixed English copy', () => {
    const source = readFileSync(fileURLToPath(new URL('../views/WebTerminal.vue', import.meta.url)), 'utf8')

    expect(source).toContain("$t('webTerminal.connectTitle')")
    expect(source).toContain("$t('webTerminal.leaveTitle')")
    expect(source).toContain("$t('webTerminal.leaveAndAbort')")
    expect(source).not.toContain('Connect web terminal?')
    expect(source).not.toContain('Leave WebTerminal?')
    expect(source).not.toContain('Leave and abort')
  })
})
