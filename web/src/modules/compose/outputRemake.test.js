import { describe, expect, it, vi } from 'vitest'
import { outputRemakeBody, outputRemakeSeed, submitOutputRemake, remakeTaskName } from './outputRemake.js'
const schema={properties:{start:{type:'integer',editable:true},fixed:{type:'integer'},guide_video_url:{type:'string',editable:true}}}
const detail={output:{id:8,material_id:3,actions:{remake:true}},trace:{material:{title:'原素材'},compose_task:{setting_key:'old',setting_name:'原组',platforms:['baidu'],template_code:'cut',template_version:1,config_snapshot:{start:2,fixed:5,guide_video_url:'（下载地址已隐藏）'}}}}
const templates=[{code:'cut',version:2,schema:{properties:{wrong:{}}}},{code:'cut',version:1,schema}]
describe('output remake source',()=>{
 it('uses exactly the original version, group, final parameters and one copy',()=>{
  const seed=outputRemakeSeed(detail,templates)
  expect(seed.template.version).toBe(1)
  expect(seed.strategy.output_settings).toEqual([{key:'old',name:'原组',platforms:['baidu'],copies:1,params:detail.trace.compose_task.config_snapshot}])
  expect(outputRemakeBody(detail,' 新制作 ',seed,{old:{start:4,fixed:20,guide_video_url:'unsafe'}})).toEqual({name:'新制作',source_output_id:8,remake_confirmed:true,setting_overrides:[{key:'old',params:{start:4}}]})
 })
 it('refuses missing snapshots, disabled actions or a missing exact template without choosing latest',()=>{
  expect(outputRemakeSeed(detail,[templates[0]]).reason).toContain('原模板版本')
  expect(outputRemakeSeed({...detail,trace:{}},templates).reason).toContain('不完整')
  expect(outputRemakeSeed({...detail,output:{...detail.output,actions:{remake:false,remake_reason:'策略已停用'}}},templates).reason).toBe('策略已停用')
 })
 it('confirms the fixed source material before task creation and stops after a failed confirmation',async()=>{
  const calls=[]
  const source={output:{material_id:3,compose_strategy_id:4,operator_id:5}}
  const body={source_output_id:8,request_key:'stable-key'}
  const addUsage=vi.fn(async()=>{calls.push('usage')})
  const createComposeTasks=vi.fn(async()=>{calls.push('create');return{batch:{id:9}}})
  const clients={materialsClient:{addUsage},composeClient:{createComposeTasks}}
  await expect(submitOutputRemake(source,body,clients)).resolves.toEqual({batch:{id:9}})
  expect(calls).toEqual(['usage','create'])
  expect(addUsage).toHaveBeenCalledWith(3,{strategy_id:4,operator_id:5})
  expect(createComposeTasks).toHaveBeenCalledWith(body)
  createComposeTasks.mockClear();addUsage.mockRejectedValueOnce(new Error('素材准备未受理'))
  await expect(submitOutputRemake(source,body,clients)).rejects.toThrow('素材准备未受理')
  expect(createComposeTasks).not.toHaveBeenCalled()
 })

 it('replays the same accepted submission after a lost response without rejoining under changed strategy state',async()=>{
  const body={source_output_id:8,request_key:'same-key'},submission={key:'same-key'}
  const addUsage=vi.fn().mockResolvedValueOnce({}).mockRejectedValue(new Error('策略已停用'))
  const createComposeTasks=vi.fn().mockRejectedValueOnce(new Error('响应丢失')).mockResolvedValue({replayed:true,batch:{id:9}})
  const clients={materialsClient:{addUsage},composeClient:{createComposeTasks},submission}
  await expect(submitOutputRemake(detail,body,clients)).rejects.toThrow('响应丢失')
  expect(submission.materialAccepted).toBe(true)
  await expect(submitOutputRemake(detail,body,clients)).resolves.toMatchObject({replayed:true})
  expect(addUsage).toHaveBeenCalledTimes(1)
  await expect(submitOutputRemake(detail,{...body,request_key:'different'},clients)).rejects.toThrow('策略已停用')
  expect(createComposeTasks).toHaveBeenCalledTimes(2)
 })

 it('limits the seeded task name to 60 Unicode characters while retaining its purpose suffix',()=>{
  const name=remakeTaskName('😀长标题'.repeat(30))
  expect(Array.from(name)).toHaveLength(60)
  expect(name.endsWith(' · 再次制作')).toBe(true)
  expect(name).not.toContain('\uFFFD')
 })

})
