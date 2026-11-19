// Uses a real authorized task. It authorizes execution only when explicitly
// selected, and never uploads to or clicks a platform publishing form.
import assert from 'node:assert/strict'
import fs from 'node:fs/promises'
import path from 'node:path'
import {chromium} from 'playwright'
const base=process.env.WT_MEDIA_BROWSER_BASE_URL, stateFile=process.env.WT_MEDIA_BROWSER_STATE,
 evidence=process.env.WT_MEDIA_BROWSER_EVIDENCE,id=process.env.WT_MEDIA_BROWSER_TASK_ID
assert(base&&stateFile&&evidence&&id,'Provide current service address, login state, evidence directory and authorized task ID')
const state=JSON.parse(await fs.readFile(stateFile,'utf8'))
state.cookies=state.cookies.map(c=>({...c,expires:c.expires??-1,httpOnly:c.httpOnly??false,secure:c.secure??false,sameSite:c.sameSite??'Lax'}))
await fs.mkdir(evidence,{recursive:true})
const browser=await chromium.launch({channel:process.env.WT_MEDIA_BROWSER_CHANNEL||'chrome',headless:true})
const context=await browser.newContext({storageState:state,viewport:{width:Number(process.env.WT_MEDIA_BROWSER_WIDTH||1440),height:1100}})
const page=await context.newPage(),errors=[];page.on('pageerror',e=>errors.push(e.message))
async function read(){const response=await context.request.get(new URL(`/api/v1/publication/${id}`,base).href);assert.equal(response.status(),200);const body=await response.json();assert.equal(body.errcode,0);return body.data}
try{
 const before=await read()
 await page.goto(new URL('/publish',base).href)
 await page.getByRole('heading',{name:'发布任务',exact:true}).waitFor()
 await page.locator('[aria-label="任务搜索"] input').fill(id)
 await page.getByRole('button',{name:'查询',exact:true}).click()
 const row=page.locator('tbody tr').filter({has:page.locator('td').filter({hasText:new RegExp(`^${id}$`)})})
 await row.getByRole('button',{name:'详情',exact:true}).click()
 await page.getByRole('heading',{name:'发布任务详情',exact:true}).waitFor()
 await page.getByRole('heading',{name:'执行历史',exact:true}).waitFor()
 if(['1','cancel'].includes(process.env.WT_MEDIA_BROWSER_AUTHORIZE_TASK)){
  assert.equal(before.status,'pending');assert.equal(before.authorized_at,null);assert.equal(before.attempt_count,0)
  await page.getByRole('button',{name:'返回',exact:true}).click()
  await row.getByRole('button',{name:'执行',exact:true}).click()
  const dialog=page.locator('.resource-feedback-dialog');await dialog.waitFor()
  await dialog.getByRole('button',{name:'取消',exact:true}).click();await dialog.waitFor({state:'hidden'})
  assert.deepEqual(await read(),before,'cancelling confirmation must not mutate the task')
  if(process.env.WT_MEDIA_BROWSER_AUTHORIZE_TASK==='1'){
  await row.getByRole('button',{name:'执行',exact:true}).click()
  await dialog.waitFor();await page.screenshot({path:path.join(evidence,'01-confirm-execution.png'),fullPage:true})
  const saved=page.waitForResponse(r=>r.request().method()==='POST'&&new URL(r.url()).pathname===`/api/v1/publication/${id}/start`)
  await dialog.getByRole('button',{name:'确认执行',exact:true}).click()
  const response=await saved;assert.equal(response.status(),200);await dialog.waitFor({state:'hidden'})
  const after=await read();assert.equal(after.status,'queued');assert(after.authorized_at);assert.equal(after.attempt_count,0)
  assert.equal(await page.getByRole('button',{name:'编辑',exact:true}).count(),0)
  assert.equal(await page.getByRole('button',{name:'执行',exact:true}).count(),0)
  const repeated=await context.request.post(new URL(`/api/v1/publication/${id}/start`,base).href,{data:{expected_revision:before.revision}})
  assert.equal(repeated.status(),409)
  }
 }
 if(await page.getByRole('heading',{name:'发布任务详情',exact:true}).isVisible()) await page.getByRole('button',{name:'返回',exact:true}).click()
 await page.getByRole('button',{name:'查询',exact:true}).click()
 await row.getByRole('button',{name:'详情',exact:true}).click()
 await page.getByRole('heading',{name:'发布任务详情',exact:true}).waitFor()
 await page.waitForLoadState('networkidle')
 await page.screenshot({path:path.join(evidence,'02-execution-history.png'),fullPage:true})
 const after=await read()
 assert.equal(await page.evaluate(()=>document.documentElement.scrollWidth>window.innerWidth),false,'detail must fit the current viewport')
 if(after.attempt_count>0){
  await page.getByRole('heading',{name:/尝试 1/}).waitFor()
  assert.equal(await page.getByText('任务尚未执行',{exact:true}).count(),0)
 }
 await fs.writeFile(path.join(evidence,'readback.json'),JSON.stringify({task_id:after.id,status:after.status,revision:after.revision,attempt_count:after.attempt_count,authorized:!!after.authorized_at,public_success:after.status==='published'},null,2))
 assert.deepEqual(errors,[])
 console.log(JSON.stringify({task_id:id,status:after.status,attempt_count:after.attempt_count,page_errors:errors,platform_submit:false}))
}catch(error){await page.screenshot({path:path.join(evidence,'failure.png'),fullPage:true}).catch(()=>{});throw error}
finally{await browser.close()}
