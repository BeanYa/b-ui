import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'

describe('default shell responsive layout', () => {
  const source = readFileSync(fileURLToPath(new URL('./Default.vue', import.meta.url)), 'utf8')

  it('clears the desktop drawer offset whenever the drawer switches to mobile mode', () => {
    expect(source).toContain("'shell-app__workspace--mobile-nav': isMobile")
    expect(source).toMatch(/\.shell-app__workspace--mobile-nav\s*\{[\s\S]*?--shell-drawer-offset:\s*0px;[\s\S]*?margin-left:\s*0;/)
  })
})
