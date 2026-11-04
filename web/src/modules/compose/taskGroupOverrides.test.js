import { describe, expect, it } from 'vitest'
import { groupOverridePayload, validateTaskGroup } from './taskGroupOverrides.js'
const fields=[{key:'start_seconds',label:'开始',type:'integer',minimum:0,required:true,editable:true},{key:'end_seconds',label:'结束',type:'integer',minimum:1,maximum:50,required:true,editable:true},{key:'fixed',type:'integer'},{key:'guide_video_url',type:'string',editable:true}]
const group={key:'a',params:{start_seconds:0,end_seconds:30,fixed:3,guide_video_url:'secret'}}
describe('inherited task group parameters',()=>{
  it('excludes fixed values and guide URLs even if supplied by an altered client draft',()=>{
    expect(groupOverridePayload([group],{a:{start_seconds:0,end_seconds:40,fixed:999,guide_video_url:'override'}},fields)).toEqual([{key:'a',params:{end_seconds:40}}])
  })
  it('checks cross-field constraints, cleared required values, and upper bounds',()=>{
    expect(validateTaskGroup(group,{...group.params,end_seconds:0},fields)).toHaveProperty('end_seconds')
    expect(validateTaskGroup(group,{...group.params,end_seconds:51},fields).end_seconds).toContain('50')
    expect(validateTaskGroup(group,{...group.params,start_seconds:null},fields)).toHaveProperty('start_seconds')
  })
})
