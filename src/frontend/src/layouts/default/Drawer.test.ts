import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'
import en from '@/locales/en'
import fa from '@/locales/fa'
import ru from '@/locales/ru'
import vi from '@/locales/vi'
import zhcn from '@/locales/zhcn'
import zhtw from '@/locales/zhtw'

describe('drawer admin-only terminal menu entry', () => {
  it('shows the WebTerminal entry only for admins', () => {
    const source = readFileSync(fileURLToPath(new URL('./Drawer.vue', import.meta.url)), 'utf8')

    expect(source).toContain("@/store/modules/auth")
    expect(source).toMatch(/const auth = useAuthStore\(\)/)
    expect(source).toMatch(/auth\.isAdmin\s*\?\s*\[\s*\{\s*title:\s*['"]pages\.webTerminal['"],[\s\S]*path:\s*['"]\/webterminal['"]/)
  })

  it('provides a pages.webTerminal label in every locale', () => {
    expect(en.pages.webTerminal).toBeTruthy()
    expect(fa.pages.webTerminal).toBeTruthy()
    expect(ru.pages.webTerminal).toBeTruthy()
    expect(vi.pages.webTerminal).toBeTruthy()
    expect(zhcn.pages.webTerminal).toBeTruthy()
    expect(zhtw.pages.webTerminal).toBeTruthy()
  })
})
