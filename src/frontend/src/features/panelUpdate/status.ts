interface PanelUpdateProgressDetails {
  targetVersion?: string
  logPath?: string
  logText?: string
}

interface PanelUpdateProgressLabels {
  targetVersion: string
  logPath: string
}

const defaultProgressLabels: PanelUpdateProgressLabels = {
  targetVersion: 'Target version',
  logPath: 'Log file',
}

export const buildPanelUpdateProgressLines = (
  details: PanelUpdateProgressDetails,
  labels: PanelUpdateProgressLabels = defaultProgressLabels,
): string[] => {
  const lines: string[] = []
  if (details.targetVersion) {
    lines.push(`${labels.targetVersion}: ${details.targetVersion}`)
  }
  if (details.logPath) {
    lines.push(`${labels.logPath}: ${details.logPath}`)
  }
  const logLines = details.logText
    ?.split(/\r?\n/)
    .map(line => line.trimEnd())
    .filter(line => line.length > 0) ?? []
  lines.push(...logLines)
  return lines
}

export const panelUpdateCompletionMessage = (_targetVersion: string) => {
  return 'Panel update succeeded. Refresh the page to load the new version.'
}
