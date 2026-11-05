import { describe,expect,it } from 'vitest'
import { nextOverlayFocus } from './overlayFocus.js'
const node=(name,enabled=true,visible=true)=>({name,disabled:!enabled,tabIndex:0,closest:()=>null,getClientRects:()=>visible?[{}]:[]})
describe('modal keyboard focus',()=>{
 it('wraps both ends and skips hidden or disabled controls',()=>{
  const first=node('first'),last=node('last'),root={querySelectorAll:()=>[first,node('disabled',false),node('hidden',true,false),last]}
  expect(nextOverlayFocus(root,last,false)).toBe(first)
  expect(nextOverlayFocus(root,first,true)).toBe(last)
  expect(nextOverlayFocus(root,null,false)).toBe(first)
 })
})
