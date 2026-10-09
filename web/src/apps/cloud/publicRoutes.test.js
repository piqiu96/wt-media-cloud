import { describe, expect, it } from 'vitest'
import { cloudRoutes } from './router.ts'
import { isPublicCloudRoute } from './publicRoutes.js'

describe('public Cloud routes', () => {
  it('exposes /home and /login without exposing business routes', () => {
    expect(cloudRoutes.some((route) => route.path === '/home')).toBe(true)
    expect(isPublicCloudRoute('/home')).toBe(true)
    expect(isPublicCloudRoute('/login')).toBe(true)
    expect(isPublicCloudRoute('/')).toBe(false)
    expect(isPublicCloudRoute('/users')).toBe(false)
  })
})
