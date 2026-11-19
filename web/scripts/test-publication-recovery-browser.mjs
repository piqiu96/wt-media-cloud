// Runs against an explicitly selected real task. Default is read-only; recording
// a verification requires an explicit conclusion and actual operator evidence.
import assert from 'node:assert/strict'
import fs from 'node:fs/promises'
import path from 'node:path'
import {chromium} from 'playwright'
const base=process.env.WT_MEDIA_BROWSER_BASE_URL,stateFile=process.env.WT_MEDIA_BROWSER_STATE,
 evidence=process.env.WT_MEDIA_BROWSER_EVIDENCE,id=process.env.WT_MEDIA_BROWSER_TASK_ID,
 conclusion=process.env.WT_MEDIA_BROWSER_RESOLVE,basis=process.env.WT_MEDIA_BROWSER_BASIS,
 retry=process.env.WT_MEDIA_BROWSER_RETRY==='1'
assert(base&&stateFile&&evidence&&id,'Current service address, session, evidence directory and task ID are required')
assert(!conclusion||['uncertain','failed'].includes(conclusion),'Unsupported verification conclusion')
assert(!conclusion||(basis&&basis.trim().length>=10),'Actual verification evidence is required before a write')
assert(!retry||!conclusion,'Resolve and retry require separate runs and read-back')
const state=JSON.parse(await fs.readFile(stateFile,'utf8'))
state.cookies=state.cookies.map(c=>({...c,expires:c.expires??-1,httpOnly:c.httpOnly??false,secure:c.secure??false,sameSite:c.sameSite??'Lax'}))
await fs.mkdir(evidence,{recursive:true})
const browser=await chromium.launch({channel:'chrome',headless:true})
const context=await browser.newContext({storageState:state,viewport:{width:1440,height:1100}})
const page=await context.newPage(),errors=[];page.on('pageerror',e=>errors.push(e.message))
const route=`/api/v1/publication/${id}`
async function get(suffix=''){const r=await context.request.get(new URL(route+suffix,base).href);assert.equal(r.status(),200);const v=await r.json();assert.equal(v.errcode,0);return v.data}
try{
 const before=await get(),original=await get('/attempt');assert(['uncertain','failed'].includes(before.status))
 await page.goto(new URL('/publish',base).href)
 await page.getByRole('heading',{name:'发布任务',exact:true}).waitFor()
 await page.locator('[aria-label="任务搜索"] input').fill(id)
 await page.getByRole('button',{name:'查询',exact:true}).click()
 const row=page.locator('tbody tr').filter({has:page.locator('td').filter({hasText:new RegExp(`^${id}$`)})})
 if(before.status==='failed'){
  await row.getByRole('button',{name:'重试',exact:true}).click()
  const confirmation=page.locator('.resource-feedback-dialog');await confirmation.waitFor()
  await confirmation.getByRole('button',{name:'取消',exact:true}).click();await confirmation.waitFor({state:'hidden'})
  assert.deepEqual(await get(),before,'Cancel retry must preserve authorization, revision and occupancy')
  assert.deepEqual(await get('/attempt'),original,'Cancel retry must preserve original attempts')
  if(retry){
   await row.getByRole('button',{name:'重试',exact:true}).click();await confirmation.waitFor()
   const response=page.waitForResponse(r=>r.request().method()==='POST'&&new URL(r.url()).pathname===route+'/retry')
   await confirmation.getByRole('button',{name:'确认重试',exact:true}).click()
   assert.equal((await response).status(),200);await confirmation.waitFor({state:'hidden'})
   const after=await get();assert.equal(after.status,'queued');assert.equal(after.id,before.id)
   assert.equal(after.current_attempt_id,null);assert.equal(after.attempt_count,before.attempt_count)
   assert.equal(BigInt(after.revision),BigInt(before.revision)+1n)
   assert.deepEqual(after.content,before.content);assert.deepEqual(after.resources,before.resources)
   assert.deepEqual(await get('/attempt'),original,'Retry must preserve the original attempts')
  }
 }else{
 await row.getByRole('button',{name:'核实结果',exact:true}).click()
 const dialog=page.getByRole('dialog',{name:'核实发布结果',exact:true})
 await dialog.waitFor();await dialog.getByRole('button',{name:'取消',exact:true}).click();await dialog.waitFor({state:'hidden'})
 assert.deepEqual(await get(),before,'Cancel must not modify the task')
 await row.getByRole('button',{name:'核实结果',exact:true}).click()
 await dialog.getByRole('button',{name:'保存核实结果',exact:true}).click()
 await dialog.getByText('请选择核实结论，填写原因及实际核实依据（至少 10 字）。',{exact:true}).waitFor()
 assert.deepEqual(await get(),before,'Incomplete evidence must not modify the task')
 await page.screenshot({path:path.join(evidence,'01-verification-validation.png'),fullPage:true})
 if(conclusion){
  await dialog.locator('[aria-label="核实结论"]').click()
  await page.getByText(conclusion==='failed'?'确认未发布':'仍无法判断',{exact:true}).last().click()
  await dialog.locator('[aria-label="核实原因"] input').fill(conclusion==='failed'?'已核对原账号与原执行现场':'平台安全验证阻塞，尚不能核对原账号稿件')
  await dialog.locator('[aria-label="核实依据"] textarea').fill(basis)
  const response=page.waitForResponse(r=>r.request().method()==='POST'&&new URL(r.url()).pathname===route+'/resolve')
  await dialog.getByRole('button',{name:'保存核实结果',exact:true}).click()
  assert.equal((await response).status(),200);await dialog.waitFor({state:'hidden'})
  const after=await get(),attempts=await get('/attempt')
  assert.equal(after.status,conclusion);assert.equal(BigInt(after.revision),BigInt(before.revision)+1n)
  assert.equal(after.current_attempt_id,before.current_attempt_id);assert.equal(after.attempt_count,before.attempt_count)
  const oldRows=Array.isArray(original)?original:original.items,newRows=Array.isArray(attempts)?attempts:attempts.items
  assert.deepEqual(newRows.map(a=>[a.id,a.result_hash,a.result_status,a.evidence]),oldRows.map(a=>[a.id,a.result_hash,a.result_status,a.evidence]),'Verification must preserve the original result')
  const stale=await context.request.post(new URL(route+'/resolve',base).href,{data:{attempt_id:before.current_attempt_id,expected_revision:before.revision,conclusion:'uncertain',evidence:{platform_uid:before.platform_account_uid,reason:'旧版本核实不能覆盖新版本',basis,scene_closed:false,no_publishable_work:false}}})
  assert.equal(stale.status(),409)
  assert.deepEqual(await get(),after)
 }
 }
 if(await page.getByRole('dialog',{name:'核实发布结果',exact:true}).isVisible()) await page.getByRole('button',{name:'取消',exact:true}).click()
 await row.getByRole('button',{name:'详情',exact:true}).click()
 await page.getByRole('heading',{name:'发布任务详情',exact:true}).waitFor()
 await page.screenshot({path:path.join(evidence,'02-original-attempt-preserved.png'),fullPage:true})
 assert.deepEqual(errors,[]);assert.equal(await page.evaluate(()=>document.documentElement.scrollWidth>innerWidth),false)
 const current=await get();await fs.writeFile(path.join(evidence,'readback.json'),JSON.stringify({task_id:id,status:current.status,revision:current.revision,attempt_count:current.attempt_count,conclusion:conclusion||'read_only',page_errors:errors,platform_submit:false},null,2))
 console.log(JSON.stringify({status:current.status,attempt_count:current.attempt_count,cancel_unchanged:true,invalid_evidence_rejected:before.status==='uncertain',retry_cancelled:before.status==='failed',retry_confirmed:retry,original_preserved:true,page_errors:errors,platform_submit:false}))
}catch(e){await page.screenshot({path:path.join(evidence,'failure.png'),fullPage:true}).catch(()=>{});throw e}finally{await browser.close()}
