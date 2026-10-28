import { describe, expect, it } from 'vitest'
import { classifyObservedAssignment, isUncertainAssignmentError } from './proxyAssignmentOutcome.js'

describe('proxy assignment uncertainty', () => {
 const target = {proxy_protocol:'socks5',host:'127.0.0.1',port:1080}
 it('keeps a missing observation or matching address unconfirmed because credentials cannot be checked', () => {
  expect(classifyObservedAssignment(null,target)).toBe('unconfirmed')
  expect(classifyObservedAssignment({proxy_type:'socks5',proxy_host:'127.0.0.1',proxy_port:1080},target)).toBe('unconfirmed')
 })
 it('allows explicit retry only after observing a different address', () => {
  expect(classifyObservedAssignment({proxy_type:'http',proxy_host:'127.0.0.1',proxy_port:1080},target)).toBe('different')
 })
 it('treats response loss as uncertain while explicit rejections remain failed', () => {
  expect(isUncertainAssignmentError(new TypeError('Failed to fetch'))).toBe(true)
  expect(isUncertainAssignmentError({errcode:30008})).toBe(true)
  expect(isUncertainAssignmentError({errcode:10001,message:'服务器响应格式错误'})).toBe(true)
  expect(isUncertainAssignmentError({errcode:23101})).toBe(false)
 })
})
