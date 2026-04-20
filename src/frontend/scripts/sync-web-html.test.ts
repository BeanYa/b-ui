import { existsSync, mkdirSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { dirname, join, resolve } from 'node:path'
import { spawnSync } from 'node:child_process'
import { fileURLToPath } from 'node:url'

import { afterEach, describe, expect, it } from 'vitest'

const tempDirs: string[] = []

afterEach(() => {
  for (const dir of tempDirs.splice(0)) {
    rmSync(dir, { recursive: true, force: true })
  }
})

describe('sync-web-html script', () => {
  it('syncs dist into the repository root web/html after the frontend moves under src', () => {
    const sandboxRoot = mkdtempSync(join(tmpdir(), 'sync-web-html-'))
    tempDirs.push(sandboxRoot)

    const repoRoot = resolve(sandboxRoot, 'repo')
    const frontendRoot = resolve(repoRoot, 'src', 'frontend')
    const scriptsDir = resolve(frontendRoot, 'scripts')
    const distDir = resolve(frontendRoot, 'dist')
    const rootOutputDir = resolve(repoRoot, 'web', 'html')
    const wrongOutputDir = resolve(repoRoot, 'src', 'web', 'html')
    const scriptSource = resolve(dirname(fileURLToPath(import.meta.url)), 'sync-web-html.mjs')
    const scriptTarget = resolve(scriptsDir, 'sync-web-html.mjs')

    mkdirSync(scriptsDir, { recursive: true })
    mkdirSync(distDir, { recursive: true })
    mkdirSync(rootOutputDir, { recursive: true })

    writeFileSync(resolve(distDir, 'index.html'), '<html>fresh</html>')
    writeFileSync(resolve(rootOutputDir, 'stale.txt'), 'stale')
    writeFileSync(scriptTarget, readFileSync(scriptSource, 'utf8'))

    const result = spawnSync(process.execPath, [scriptTarget], {
      cwd: frontendRoot,
      encoding: 'utf8',
    })

    expect(result.status).toBe(0)
    expect(existsSync(resolve(rootOutputDir, 'index.html'))).toBe(true)
    expect(existsSync(resolve(rootOutputDir, 'stale.txt'))).toBe(false)
    expect(existsSync(wrongOutputDir)).toBe(false)
  })
})
