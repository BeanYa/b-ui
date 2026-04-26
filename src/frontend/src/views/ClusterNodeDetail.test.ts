import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'

describe('ClusterNodeDetail view source', () => {
  it('can be imported without errors', () => {
    const source = readFileSync(fileURLToPath(new URL('./ClusterNodeDetail.vue', import.meta.url)), 'utf8')
    expect(source).toBeTruthy()
    expect(source).toContain('<template>')
    expect(source).toContain('<script lang="ts" setup>')
  })
})
