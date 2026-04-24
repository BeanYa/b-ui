import { describe, expect, it } from 'vitest'

import { buildPanelUpdateProgressLines, panelUpdateCompletionMessage } from './status'

describe('buildPanelUpdateProgressLines', () => {
  it('shows the target version and log path while the panel update is running', () => {
    expect(buildPanelUpdateProgressLines({
      targetVersion: 'v0.2.0',
      logPath: '/tmp/b-ui-panel-update.log',
    })).toEqual([
      'Target version: v0.2.0',
      'Log file: /tmp/b-ui-panel-update.log',
    ])
  })
})

describe('panelUpdateCompletionMessage', () => {
  it('asks the user to refresh the page after the update completes', () => {
    expect(panelUpdateCompletionMessage('v0.2.0')).toBe('The panel has been updated to v0.2.0. Refresh the page to load the new version.')
  })
})
