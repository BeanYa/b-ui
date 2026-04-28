import { describe, expect, it } from 'vitest'

import { normalizePromptMessage } from './prompt'

describe('normalizePromptMessage', () => {
  it('removes leading punctuation markers from notification messages', () => {
    expect(normalizePromptMessage('。面板更新成功，请刷新面板。')).toBe('面板更新成功，请刷新面板。')
    expect(normalizePromptMessage('• 集群节点已加入域并完成信息刷新')).toBe('集群节点已加入域并完成信息刷新')
  })

  it('keeps normal sentence punctuation intact', () => {
    expect(normalizePromptMessage('面板已更新到 v0.2.0，请刷新页面以加载新版本。')).toBe('面板已更新到 v0.2.0，请刷新页面以加载新版本。')
  })
})
