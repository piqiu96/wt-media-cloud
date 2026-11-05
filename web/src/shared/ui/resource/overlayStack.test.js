import { describe,it,expect } from 'vitest'
import { createOverlayStack } from './overlayStack.js'
describe('shared overlay order',()=>{
 it('keeps the newest open overlay on top, then returns to its parent',()=>{
  const stack=createOverlayStack();stack.open('parent');stack.open('child');stack.open('child')
  expect(stack.items.value).toEqual(['parent','child']);expect(stack.isTop('parent')).toBe(false)
  stack.close('child');expect(stack.isTop('parent')).toBe(true)
  stack.close('parent');expect(stack.items.value).toEqual([])
 })
})
