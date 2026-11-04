import { createSSRApp,h } from 'vue'
import { renderToString } from 'vue/server-renderer'
import { describe,expect,it } from 'vitest'
import ComposeOutputDetail from './ComposeOutputDetail.vue'
const detail={output:{id:42,name:'原素材 · 原设置',file_name:'原素材-42.mp4',file_size_bytes:2048,created_at:'2026-10-15T10:00:00Z',video:{status:'ready'},media:{duration_ms:12500,width:720,height:1280}},trace:{material:{id:157,title:'原素材'},strategy:{id:24,name:'原策略',template_schema:{properties:{insert_seconds:{title:'插入秒数',type:'integer'},guide_video_url:{title:'引导视频',type:'string'}}}},compose_task:{id:252,batch_id:30,setting_name:'原设置',template_code:'insert',template_version:2,platforms:['baidu'],config_snapshot:{insert_seconds:3,guide_video_url:'（下载地址已隐藏）'}},publishing:{available:false,reason:'发布能力尚未交付',platforms:[{platform:'baidu',status:null}]}}}
async function render(props){const app=createSSRApp({render:()=>h(ComposeOutputDetail,props)});for(const name of ['t-button','t-link'])app.component(name,{setup:(_,ctx)=>()=>h('button',ctx.slots.default?.())});return renderToString(app)}
describe('output asset detail',()=>{
 it('renders four expanded sections without a player or cover placeholder',async()=>{
  const html=await render({detail,management:true})
  for(const label of ['基本信息','成片 ID','视频状态','成片名称','负责人','视频信息','制作来源','发布情况'])expect(html).toContain(label)
  expect(html).not.toContain('<video');expect(html).not.toContain('暂时无法预览');expect(html).not.toContain('<details')
  expect(html.indexOf('基本信息')).toBeLessThan(html.indexOf('视频信息'))
 })
 it('opens the real file URL by its fixed filename and compacts measured metadata',async()=>{
  const html=await render({detail,previewUrl:'https://media.test/real.mp4'})
  expect(html).toContain('href="https://media.test/real.mp4"');expect(html).toContain('原素材-42.mp4');expect(html).toContain('target="_blank"')
  expect(html).toContain('12.5');expect(html).toContain('720 × 1280');expect(html).toContain('2.0 KB')
 })
 it('uses original final parameters and exact schema, never current strategy defaults or signed guide URL',async()=>{
  const html=await render({detail,linkable:true,templates:[{code:'insert',version:99,schema:{properties:{wrong:{title:'错误新版默认'}}}}]})
  for(const value of ['插入秒数','3 秒','原设置','30','252','已配置（固定）'])expect(html).toContain(value)
  expect(html).not.toContain('错误新版默认');expect(html).not.toContain('下载地址已隐藏');expect(html).not.toContain('策略当前参数');expect(html).not.toMatch(/#\d/)
 })
 it('does not invent platform publishing state or missing historical names and measurements',async()=>{
  const html=await render({detail:{...detail,output:{id:42,media:{},video:{status:'unknown',reason:'尚未核验'}},trace:detail.trace}})
  expect(html).toContain('发布能力尚未交付');expect(html).not.toContain('未安排');expect(html).not.toContain('已安排');expect(html).not.toContain('待发布')
  expect(html).toContain('文件名未记录');expect(html).toContain('分辨率 —');expect(html).toContain('文件大小 —');expect(html).not.toContain('undefined')
 })
})
