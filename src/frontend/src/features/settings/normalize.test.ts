import { describe, expect, it } from 'vitest'

import { defaultSettings, isSubTLSLinked, normalizeSettings, toNumberSetting } from './normalize'

describe('normalizeSettings', () => {
  it('falls back to defaults and coerces numeric fields to strings', () => {
    const settings = normalizeSettings({
      webPort: 8443,
      subUpdates: undefined,
      webListen: '0.0.0.0',
    })

    expect(settings.webListen).toBe('0.0.0.0')
    expect(settings.webPort).toBe('8443')
    expect(settings.subUpdates).toBe(defaultSettings.subUpdates)
    expect(settings.tlsDomainHints).toBe('')
  })
})

describe('toNumberSetting', () => {
  it('returns a fallback when the source value is empty', () => {
    expect(toNumberSetting(undefined, 2095)).toBe(2095)
    expect(toNumberSetting('', 2095)).toBe(2095)
  })
})

describe('isSubTLSLinked', () => {
  it('uses linked panel TLS unless both subscription cert and key are set', () => {
    expect(isSubTLSLinked({ subCertFile: '', subKeyFile: '' })).toBe(true)
    expect(isSubTLSLinked({ subCertFile: '/cert.pem', subKeyFile: '' })).toBe(true)
    expect(isSubTLSLinked({ subCertFile: '', subKeyFile: '/key.pem' })).toBe(true)
    expect(isSubTLSLinked({ subCertFile: '/cert.pem', subKeyFile: '/key.pem' })).toBe(false)
  })
})
