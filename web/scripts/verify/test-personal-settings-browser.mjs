import assert from 'node:assert/strict'
import { createServer } from 'vite'
import { chromium } from 'playwright'
import { mkdtemp, mkdir } from 'node:fs/promises'
import { tmpdir } from 'node:os'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
const root=fileURLToPath(new URL('../../',import.meta.url))
const output=process.env.SCREENSHOT_DIR || await mkdtemp(path.join(tmpdir(),'wt-settings-browser-'))
await mkdir(output,{recursive:true})
const server=await createServer({configFile:path.join(root,'vite.config.desktop.js'),cacheDir:path.join(output,'.vite'),server:{host:'127.0.0.1',port:0,strictPort:false,open:false}})
let browser
try {
 await server.listen()
 const base=server.resolvedUrls.local[0].replace(/\/$/,'')
 browser=await chromium.launch({channel:'chrome',headless:true})
 const page=await browser.newPage({viewport:{width:1440,height:1100}})
 page.setDefaultTimeout(10000)
 const errors=[]
 page.on('pageerror',e=>errors.push(e.message))
 let user={id:3,username:'operator01',nickname:'pika',avatar_id:'sun',role:'operator',team_name:'默认运营组',category_ids:[1]}
 let profileFail=false
 await page.route('**/api/v1/**',async route=>{
  const req=route.request(), name=new URL(req.url()).pathname.replace('/api/v1','')
  let data=[], errcode=0, message='success'
  if(name==='/auth/me') {
   if(req.method()==='PATCH') {
    if(profileFail){errcode=50001;message='保存失败测试'}
    else user={...user,...req.postDataJSON()}
   }
   data=user
  } else if(name==='/categories') data=[{id:1,name:'火影忍者'}]
  else if(name==='/local-agent/bit-account') data={bound:true,main_user_id_masked:'2c9b****619f',bound_at:'2026-07-22T13:43:50Z',last_verified_at:'2026-11-22T12:30:00Z'}
  else if(name==='/file-transfer-tasks') data=[{execution_scope:'local_agent',file_name:'material.mp4'}]
  await route.fulfill({json:{errcode,message,data,logid:'test'}})
 })
 await page.addInitScript(({base})=>{
  const settings={save_dir:'/test/current',file:'/test/settings.toml',known_save_dirs:['/test/current','/test/old']}
  window.__test={calls:[],pick:null,pushFail:false,slow:false,settings}
  window.__TAURI_INTERNALS__={invoke:async (cmd,args={})=>{
   const test=window.__test;test.calls.push({cmd,args})
   if(cmd==='get_public_config') return {cloud_base_url:base}
   if(cmd==='plugin:app|version') return '0.1.0'
   if(cmd==='local_settings_get') return structuredClone(settings)
   if(cmd==='local_pick_save_directory') {
    if(!test.pick) return null
    settings.save_dir=test.pick;settings.known_save_dirs=[...new Set([test.pick,...settings.known_save_dirs])]
    return structuredClone(settings)
   }
   if(cmd==='local_push_save_directory') {if(test.pushFail)throw 'Agent暂不可达';return {save_dir:settings.save_dir,writable:true,free_bytes:1000}}
   if(cmd==='local_remove_search_directory') {settings.known_save_dirs=settings.known_save_dirs.filter(x=>x!==args.directory);return structuredClone(settings)}
   if(cmd==='local_pick_search_directory') {settings.known_save_dirs.push('/test/restored');return {picked:'/test/restored',added:true,dropped:null,searched:settings.known_save_dirs.length}}
   if(cmd==='local_open_save_directory') return args.directory
   if(cmd==='local_storage_usage') return {available_bytes:47*1024**3,cache_bytes:0,logs:[{source:'desktop',bytes:1024},{source:'agent',bytes:2048}]}
   if(cmd==='local_device_id') return 'WT-TEST-DEVICE'
   if(cmd==='local_agent_status') return {state:'ready',main_user_id_masked:'2c9b****619f',agent_version:'0.3.0',operating_system:'macos',cpu_architecture:'arm64'}
   if(cmd==='local_log_files') return {trees:[{source:'desktop',directory:'/test/log',files:[{name:'desktop.log',kind:'live',bytes:1000}]},{source:'agent',directory:'/test/agent',files:[{name:'agent.log',kind:'live',bytes:1000}]}]}
   if(cmd==='local_log_tail') {
    if(test.slow && args.source==='desktop') await new Promise(resolve=>setTimeout(resolve,400))
    return {source:args.source,name:args.name,path:`/test/${args.name}`,min_level:args.minLevel,lines_read:2,lines_shown:2,lines:[{level:'info',text:`${args.name} latest`},{level:null,continues:true,text:'unclassified continuation'}]}
   }
   if(cmd==='local_save_dir_migration_plan')return {to:settings.save_dir,free_bytes:100000,needed_bytes:3,movable:[{name:'material.mp4',directory:'/test/old',bytes:3}],already_there:[],missing:[],unreadable:[]}
   if(cmd==='local_move_saved_files')return {directory:settings.save_dir,moved:[],deleted:[],missing:[],kept:[],failures:[{name:'material.mp4',reason:'目标已存在'}]}
   if(cmd==='local_agent_token_facts')return {configured:true}
   if(cmd==='local_diagnostic_export')return {path:'/test/diagnostic.tar.gz',bytes:10,entries:[],omitted:[{name:'agent.log',reason:'unavailable'}],agent_state:'unreachable',sha256:'abc'}
   return null
  },transformCallback:()=>1}
 },{base})
 await page.goto(`${base}/personal-info`)
 await page.getByRole('heading',{name:'账号资料',exact:true}).waitFor()
 assert.equal(await page.getByRole('textbox',{name:'昵称'}).count(),0)
 const headers=await page.locator('.resource-section--detail h3').evaluateAll(nodes=>nodes.map(n=>{const s=getComputedStyle(n),m=getComputedStyle(n,'::before');return [s.fontSize,s.fontWeight,s.lineHeight,m.width]}))
 assert.deepEqual(headers[0],headers[1])
 await page.getByRole('button',{name:'编辑',exact:true}).click()
 await page.getByRole('textbox',{name:'昵称'}).fill('new name')
 await page.getByRole('button',{name:'取消',exact:true}).click()
 assert.equal(user.nickname,'pika')
 await page.getByRole('button',{name:'编辑',exact:true}).click()
 await page.getByRole('textbox',{name:'昵称'}).fill('saved name')
 profileFail=true
 await page.getByRole('button',{name:'保存',exact:true}).click()
 await page.getByText('保存失败测试',{exact:true}).waitFor()
 assert.equal(await page.getByRole('textbox',{name:'昵称'}).inputValue(),'saved name')
 profileFail=false
 await page.getByRole('button',{name:'保存',exact:true}).click()
 await page.locator('.profile-name').getByText('saved name',{exact:true}).waitFor()
 await page.getByRole('button',{name:'编辑',exact:true}).click()
 await page.getByRole('textbox',{name:'昵称'}).fill('unsaved')
 await page.getByRole('button',{name:'桌面设置',exact:true}).click()
 await page.getByText('放弃未保存的修改？',{exact:true}).waitFor()
 await page.getByRole('button',{name:'取消',exact:true}).last().click()
 assert.match(page.url(),/personal-info/)
 await page.getByText('放弃未保存的修改？',{exact:true}).waitFor({state:'hidden'})
 await page.getByRole('region',{name:'账号资料'}).getByRole('button',{name:'取消',exact:true}).click()
 await page.screenshot({path:path.join(output,'account.png'),fullPage:true})
 await page.getByRole('button',{name:'桌面设置',exact:true}).click()
 await page.getByRole('heading',{name:'下载目录',exact:true}).waitFor()
 await page.getByText('/test/current',{exact:true}).waitFor()
 await page.getByRole('button',{name:'更改目录',exact:true}).click()
 assert.equal(await page.getByRole('dialog').count(),0)
 assert.equal(await page.evaluate(()=>window.__test.settings.save_dir),'/test/current')
 assert.equal(await page.evaluate(()=>window.__test.calls.filter(x=>x.cmd==='local_push_save_directory').length),0)
 await page.evaluate(()=>{window.__test.pick='/test/new';window.__test.pushFail=true})
 await page.getByRole('button',{name:'更改目录',exact:true}).click()
 await page.getByText(/目录已保存，本机执行服务尚未应用/).waitFor()
 assert.equal(await page.getByRole('dialog').count(),0)
 await page.evaluate(()=>{window.__test.pushFail=false})
 await page.getByRole('button',{name:'重试应用',exact:true}).click()
 await page.getByText('下载目录已保存并应用',{exact:true}).waitFor()
 const old=page.locator('.history-row').filter({hasText:'/test/old'})
 await old.getByRole('button',{name:'打开目录',exact:true}).click()
 assert.equal(await page.evaluate(()=>window.__test.calls.filter(x=>x.cmd==='local_open_save_directory').at(-1).args.directory),'/test/old')
 await old.getByRole('button',{name:'移除',exact:true}).click()
 await page.locator('.resource-feedback-dialog:visible').getByRole('button',{name:'取消',exact:true}).click()
 await page.getByText('移除历史查找目录？',{exact:true}).waitFor({state:'hidden'})
 assert.equal(await old.count(),1)
 await old.getByRole('button',{name:'移除',exact:true}).click()
 await page.locator('.resource-feedback-dialog:visible').getByRole('button',{name:'移除',exact:true}).click()
 await old.waitFor({state:'detached'})
 await page.getByText('移除历史查找目录？',{exact:true}).waitFor({state:'hidden'})
 assert.equal(await page.evaluate(()=>window.__test.calls.some(x=>x.cmd==='local_delete_saved_files')),false)
 await page.getByRole('button',{name:'整理文件',exact:true}).click()
 await page.getByText('material.mp4',{exact:true}).waitFor()
 assert.equal(await page.getByRole('button',{name:'移动所选（0）',exact:true}).isDisabled(),true)
 await page.getByRole('button',{name:'搬运',exact:true}).click()
 await page.getByRole('button',{name:'移动所选（1）',exact:true}).click()
 await page.getByText(/目标已存在/).waitFor()
 assert.equal(await page.getByRole('dialog').count(),1)
 assert.equal(await page.getByRole('button',{name:'移动所选（1）',exact:true}).isEnabled(),true)
 await page.getByRole('button',{name:'关闭',exact:true}).click()
 await page.getByRole('dialog',{name:'整理已下载文件',exact:true}).waitFor({state:'hidden'})
 for (const close of await page.locator('.t-message__close').all()) await close.click().catch(()=>{})
 for(const width of [1024,1280,1440,1600]){
  await page.setViewportSize({width,height:900})
  assert.equal(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth),true,`storage overflow ${width}`)
  const box=await page.getByRole('button',{name:'桌面设置',exact:true}).boundingBox()
  assert.ok(box.y+box.height<=900 && box.y>800)
 }
 await page.screenshot({path:path.join(output,'settings.png'),fullPage:true})
 await page.locator('.t-tabs__nav-item').filter({hasText:'本地日志'}).click()
 await page.getByText('desktop.log latest',{exact:true}).waitFor()
 await page.evaluate(()=>{window.__test.slow=true})
 await page.getByRole('button',{name:'刷新',exact:true}).click()
 await page.locator('.log-file').filter({hasText:'agent.log'}).click()
 await page.getByText('agent.log latest',{exact:true}).waitFor()
 await page.waitForTimeout(550)
 assert.equal(await page.getByText('desktop.log latest',{exact:true}).count(),0)
 assert.equal(await page.getByText('unclassified continuation',{exact:true}).count(),1)
 await page.screenshot({path:path.join(output,'logs.png'),fullPage:true})
 await page.goto(`${base}/logs`)
 await page.getByText('desktop.log latest',{exact:true}).waitFor()
 assert.match(page.url(),/settings\?tab=log/)
 assert.deepEqual(errors,[])
 console.log('PASS: profile read/edit/cancel/error/save/topbar/leave guard; native directory cancel/save/apply/open/remove; file partial retry; fixed footer and 4 widths; log stale response and legacy redirect. Screenshots:',output)
} finally {await browser?.close();await server.close()}
