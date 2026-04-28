import { describe, expect, it } from 'vitest'

import en from './en'
import zhcn from './zhcn'
import zhtw from './zhtw'

describe('panel update completion copy', () => {
  it('uses direct success copy without a leading punctuation marker', () => {
    expect(zhcn.main.updatePanel.completed).toBe('面板更新成功。')
    expect(zhcn.main.updatePanel.completedWithRefresh).toBe('面板更新成功，请刷新页面以加载新版本。')
    expect(zhtw.main.updatePanel.completed).toBe('面板更新成功。')
    expect(zhtw.main.updatePanel.completedWithRefresh).toBe('面板更新成功，請重新整理頁面以載入新版本。')
    expect(en.main.updatePanel.completed).toBe('Panel update succeeded.')
    expect(en.main.updatePanel.completedWithRefresh).toBe('Panel update succeeded. Refresh the page to load the new version.')
  })
})
