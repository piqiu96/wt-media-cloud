import { describe, expect, it } from 'vitest'
import ComposeRelatedDetail from './ComposeRelatedDetail.vue'
import { relatedTarget } from '../relatedTarget.js'
describe('related entity navigation',()=>{
 it('compiles an independent detail instance for every nested entity',()=>expect(ComposeRelatedDetail).toBeTruthy())
 it('returns to an ancestor rather than replacing its context',()=>{
  const path=[{kind:'task',id:12,key:'task-12'},{kind:'batch',id:3,key:'batch-3'}]
  expect(relatedTarget(path,'task',12)).toEqual({back:'task-12'})
  expect(relatedTarget(path,'task',13)).toEqual({kind:'task',id:13,key:'task-13'})
  expect(relatedTarget(path,'task',0)).toBeNull()
 })
})
