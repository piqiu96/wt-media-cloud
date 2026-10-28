import { describe, expect, it } from 'vitest'
import { classifyObservedAssignment } from './proxyAssignmentOutcome.js'

describe('proxy assignment observation', () => {
  const target = { proxy_protocol: 'socks5', host: '127.0.0.1', port: 1080 }

  it('accepts only an observed address that matches the target', () => {
    expect(classifyObservedAssignment(null, target)).toBe('different')
    expect(classifyObservedAssignment({ proxy_type: 'socks5', proxy_host: '127.0.0.1', proxy_port: 1080 }, target)).toBe('matches')
    expect(classifyObservedAssignment({ proxy_type: 'http', proxy_host: '127.0.0.1', proxy_port: 1080 }, target)).toBe('different')
  })
})
