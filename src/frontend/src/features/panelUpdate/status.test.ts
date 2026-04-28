import { describe, expect, it } from 'vitest'

import { buildPanelUpdateProgressLines, panelUpdateCompletionMessage } from './status'

describe('buildPanelUpdateProgressLines', () => {
  it('shows the target version and log path while the panel update is running', () => {
    expect(buildPanelUpdateProgressLines({
      targetVersion: 'v0.2.0',
      logPath: '/tmp/b-ui-panel-update.log',
      logText: '准备更新面板\n下载安装脚本',
    })).toEqual([
      'Target version: v0.2.0',
      'Log file: /tmp/b-ui-panel-update.log',
      '准备更新面板',
      '下载安装脚本',
    ])
  })
})

describe('panelUpdateCompletionMessage', () => {
  it('asks the user to refresh the page after the update completes', () => {
    expect(panelUpdateCompletionMessage('v0.2.0')).toBe('Panel update succeeded. Refresh the page to load the new version.')
  })
})
