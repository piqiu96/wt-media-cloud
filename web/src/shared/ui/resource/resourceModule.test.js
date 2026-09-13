import { existsSync, readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

const resourceRoot = new URL('./', import.meta.url)
const read = (file) => readFileSync(new URL(file, resourceRoot), 'utf8')

describe('operation resource design primitives', () => {
  it('loads the shared token and resource styles from both application entrypoints', () => {
    const cloudEntry = readFileSync(new URL('../../../apps/cloud/main.ts', import.meta.url), 'utf8')
    const desktopEntry = readFileSync(new URL('../../../apps/desktop/main.ts', import.meta.url), 'utf8')

    expect(cloudEntry).toContain('design-token.css')
    expect(cloudEntry).toContain('resource-module.css')
    expect(desktopEntry).toContain('design-token.css')
    expect(desktopEntry).toContain('resource-module.css')
  })

  it('provides token-backed presentation components for resource pages', () => {
    for (const file of ['ResourcePageHeader.vue', 'ResourceCard.vue', 'ResourceStatGrid.vue', 'ResourceStatusBadge.vue']) {
      expect(existsSync(new URL(file, resourceRoot))).toBe(true)
    }

    expect(read('ResourcePageHeader.vue')).toContain("title: { type: String, required: true }")
    expect(read('ResourceCard.vue')).toContain('<slot />')
    expect(read('ResourceStatGrid.vue')).toContain("items: { type: Array, required: true }")
    expect(read('ResourceStatusBadge.vue')).toContain("tone: { type: String, default: 'neutral' }")
    expect(read('../../../styles/design-token.css')).toContain('--wt-primary: #2563EB')
  })
})
