export const defaultSettings = {
  webListen: '',
  webDomain: '',
  webPort: '2095',
  webCertFile: '',
  webKeyFile: '',
  webPath: '/app/',
  webURI: '',
  sessionMaxAge: '0',
  trafficAge: '30',
  timeLocation: 'Asia/Tehran',
  tlsDomainHints: '',
  subListen: '',
  subPort: '2096',
  subPath: '/sub/',
  subDomain: '',
  subCertFile: '',
  subKeyFile: '',
  subUpdates: '12',
  subEncode: 'true',
  subShowInfo: 'false',
  subURI: '',
  subJsonExt: '',
  subClashExt: '',
}

const toStringSetting = (value: unknown, fallback: string): string => {
  if (value === undefined || value === null) return fallback
  return typeof value === 'string' ? value : String(value)
}

export const isSubTLSLinked = (value: unknown): boolean => {
  const source = value && typeof value === 'object' ? value as Record<string, unknown> : {}
  const certFile = toStringSetting(source.subCertFile, defaultSettings.subCertFile).trim()
  const keyFile = toStringSetting(source.subKeyFile, defaultSettings.subKeyFile).trim()
  return certFile === '' || keyFile === ''
}

export const normalizeSettings = (value: unknown) => {
  const source = value && typeof value === 'object' ? value as Record<string, unknown> : {}

  return {
    ...defaultSettings,
    ...source,
    webListen: toStringSetting(source.webListen, defaultSettings.webListen),
    webDomain: toStringSetting(source.webDomain, defaultSettings.webDomain),
    webPort: toStringSetting(source.webPort, defaultSettings.webPort),
    webCertFile: toStringSetting(source.webCertFile, defaultSettings.webCertFile),
    webKeyFile: toStringSetting(source.webKeyFile, defaultSettings.webKeyFile),
    webPath: toStringSetting(source.webPath, defaultSettings.webPath),
    webURI: toStringSetting(source.webURI, defaultSettings.webURI),
    sessionMaxAge: toStringSetting(source.sessionMaxAge, defaultSettings.sessionMaxAge),
    trafficAge: toStringSetting(source.trafficAge, defaultSettings.trafficAge),
    timeLocation: toStringSetting(source.timeLocation, defaultSettings.timeLocation),
    tlsDomainHints: toStringSetting(source.tlsDomainHints, defaultSettings.tlsDomainHints),
    subListen: toStringSetting(source.subListen, defaultSettings.subListen),
    subPort: toStringSetting(source.subPort, defaultSettings.subPort),
    subPath: toStringSetting(source.subPath, defaultSettings.subPath),
    subDomain: toStringSetting(source.subDomain, defaultSettings.subDomain),
    subCertFile: toStringSetting(source.subCertFile, defaultSettings.subCertFile),
    subKeyFile: toStringSetting(source.subKeyFile, defaultSettings.subKeyFile),
    subUpdates: toStringSetting(source.subUpdates, defaultSettings.subUpdates),
    subEncode: toStringSetting(source.subEncode, defaultSettings.subEncode),
    subShowInfo: toStringSetting(source.subShowInfo, defaultSettings.subShowInfo),
    subURI: toStringSetting(source.subURI, defaultSettings.subURI),
    subJsonExt: toStringSetting(source.subJsonExt, defaultSettings.subJsonExt),
    subClashExt: toStringSetting(source.subClashExt, defaultSettings.subClashExt),
  }
}

export const toNumberSetting = (value: unknown, fallback: number): number => {
  const normalized = toStringSetting(value, '')
  if (normalized.trim().length === 0) return fallback

  const parsed = Number.parseInt(normalized, 10)
  return Number.isFinite(parsed) ? parsed : fallback
}
