import { describe,it,expect } from 'vitest'
import { settingFields,initialParams,validateSettings } from './strategySettings.js'
import { validateTaskGroup } from './taskGroupOverrides.js'

describe('dynamic composition defaults and bounds',()=>{
 const schema={properties:{start_seconds:{type:'number',minimum:0,editable:true},end_seconds:{type:'number',minimum:0,auto_zero:true,editable:true},max_duration_seconds:{type:'number',minimum:1,maximum_from_strategy:true,editable:true}},required:['start_seconds','end_seconds','max_duration_seconds']}
 const fields=settingFields(schema,{start_seconds:0,end_seconds:0,max_duration_seconds:180})
 it('prefills deployment defaults rather than lower bounds',()=>expect(initialParams(fields)).toEqual({start_seconds:0,end_seconds:0,max_duration_seconds:180}))
 it('accepts automatic end and rejects increasing the strategy cap',()=>{
  const group={params:{start_seconds:0,end_seconds:0,max_duration_seconds:180}}
  expect(validateTaskGroup(group,{...group.params,start_seconds:5},fields)).toEqual({})
  expect(validateTaskGroup(group,{...group.params,max_duration_seconds:200},fields)).toHaveProperty('max_duration_seconds')
 })
 it('accepts platform file URLs for required video parameters',()=>{
  const fields=[{key:'intro_video_url',format:'uri',type:'string',required:true,label:'开头'}]
  const group={key:'one',name:'成片',platforms:['baidu'],copies:1,params:{intro_video_url:'http://localhost:8188/api/v1/file-resources/'+ 'a'.repeat(64)}}
  expect(validateSettings([group],fields)).toEqual({})
  expect(validateSettings([{...group,params:{intro_video_url:'file:///tmp/video.mp4'}}],fields)).toHaveProperty('one')
 })
})
