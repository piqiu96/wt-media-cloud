import { describe,it,expect } from 'vitest'
import { changedTaskParams } from './effectiveTaskParams.js'
describe('effective task params',()=>{
 it('shows final values only for opened, actually changed template parameters',()=>{
  const group={params:{start:0,end:90,locked:3,guide_video_url:'private'}}
  const fields=[{key:'start',label:'起点',editable:true},{key:'end',label:'终点',editable:true},{key:'locked',editable:false},{key:'guide_video_url',editable:true}]
  expect(changedTaskParams(group,{start:0,end:45,locked:6,guide_video_url:'other'},fields)).toEqual([{key:'end',label:'终点',value:45}])
  expect(changedTaskParams(group,{},fields)).toEqual([])
 })
})
