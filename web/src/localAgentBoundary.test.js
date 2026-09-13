import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

const source = readFileSync(new URL('./apps/desktop/features/local-agent/init.js', import.meta.url), 'utf8')

describe('Desktop Local Agent boundary', () => {
  it('does not let Vue connect to the Local Agent loopback port', () => {
    expect(source).not.toMatch(/127\.0\.0\.1:8765/)
    expect(source).not.toMatch(/fetch\([^)]*api\/v1\/status/)
  })
})
