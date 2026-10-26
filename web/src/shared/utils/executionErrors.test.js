import { describe, expect, it } from 'vitest'
import { executionErrorMessage } from './executionErrors.js'

describe('local browser failure feedback', () => {
  it('tells the operator how to recover when BitBrowser is offline', () => {
    const message = executionErrorMessage({
      code: 'bit_browser_offline',
      message: 'BitBrowser Local API request failed: connection refused',
    })

    expect(message).toContain('启动比特浏览器')
    expect(message).not.toContain('connection refused')
  })

  it('does not expose an upstream diagnostic for other BitBrowser response failures', () => {
    const message = executionErrorMessage({
      code: 'bitbrowser_response_error',
      message: '502 Bad Gateway {"error":"raw diagnostic"}',
    })

    expect(message).toContain('比特浏览器')
    expect(message).not.toContain('502')
    expect(message).not.toContain('raw diagnostic')
  })
})
