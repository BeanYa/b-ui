import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'

describe('router admin-only terminal route', () => {
  it('defines a guarded WebTerminal route inside the authenticated shell', () => {
    const source = readFileSync(fileURLToPath(new URL('./index.ts', import.meta.url)), 'utf8')

    expect(source).toMatch(/path:\s*['"]\/webterminal['"],[\s\S]*name:\s*['"]pages\.webTerminal['"]/)
    expect(source).toMatch(/meta:\s*\{[\s\S]*requiresAdmin:\s*true[\s\S]*\}/)
    expect(source).toContain("@/views/WebTerminal.vue")
  })

  it('loads auth state before checking admin access and redirects non-admin users away', () => {
    const source = readFileSync(fileURLToPath(new URL('./index.ts', import.meta.url)), 'utf8')

    expect(source).toContain("@/store/modules/auth")
    expect(source).toMatch(/if \(to\.meta\.requiresAdmin\) \{[\s\S]*if \(!auth\.loaded\) \{[\s\S]*await auth\.loadAuthState\(\)[\s\S]*\}[\s\S]*if \(!auth\.isAdmin\) \{[\s\S]*return ['"]\/['"][\s\S]*\}[\s\S]*\}/)
  })
})
