interface PanelUpdateProgressDetails {
  targetVersion?: string
  logPath?: string
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
  return lines
}

export const panelUpdateCompletionMessage = (targetVersion: string) => {
  return `The panel has been updated to ${targetVersion}. Refresh the page to load the new version.`
}
