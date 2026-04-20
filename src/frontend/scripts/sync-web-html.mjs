import { cpSync, existsSync, mkdirSync, readdirSync, rmSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

const scriptDir = dirname(fileURLToPath(import.meta.url))
const frontendRoot = resolve(scriptDir, '..')
const distDir = resolve(frontendRoot, 'dist')

function syncOutputDir(outputDir) {
  mkdirSync(outputDir, { recursive: true })

  for (const entry of readdirSync(outputDir)) {
    rmSync(resolve(outputDir, entry), { recursive: true, force: true })
  }

  for (const entry of readdirSync(distDir)) {
    cpSync(resolve(distDir, entry), resolve(outputDir, entry), { recursive: true })
  }
}

const outputDirs = [
  resolve(frontendRoot, '..', 'backend', 'internal', 'infra', 'web', 'html'),
  resolve(frontendRoot, '..', '..', 'web', 'html'),
]

if (!existsSync(distDir)) {
  throw new Error(`Missing frontend build output: ${distDir}`)
}

for (const outputDir of outputDirs) {
  syncOutputDir(outputDir)
}

console.log(`Synced ${distDir} -> ${outputDirs.join(', ')}`)
