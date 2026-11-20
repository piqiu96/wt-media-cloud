// Uses a running local stack and its existing resources. Never authorizes or submits a post.
import assert from 'node:assert/strict'
import fs from 'node:fs/promises'
import path from 'node:path'
import { chromium } from 'playwright'

const base=process.env.WT_MEDIA_BROWSER_BASE_URL
const statePath=process.env.WT_MEDIA_BROWSER_STATE
const evidence=process.env.WT_MEDIA_BROWSER_EVIDENCE
assert(base && statePath && evidence,'Set WT_MEDIA_BROWSER_BASE_URL, WT_MEDIA_BROWSER_STATE and WT_MEDIA_BROWSER_EVIDENCE from the running environment')
const state=JSON.parse(await fs.readFile(statePath,'utf8'))
state.cookies=state.cookies.map(cookie=>({...cookie,expires:cookie.expires??-1,httpOnly:cookie.httpOnly??false,secure:cookie.secure??false,sameSite:cookie.sameSite??'Lax'}))
await fs.mkdir(evidence,{recursive:true})
const browser=await chromium.launch({channel:process.env.WT_MEDIA_BROWSER_CHANNEL||'chrome',headless:true})
const context=await browser.newContext({storageState:state,viewport:{width:1440,height:1100}})
const page=await context.newPage(),errors=[]
page.on('pageerror',error=>errors.push(error.message))
try{
 await page.goto(new URL('/publish',base).href)
 await page.getByRole('heading',{name:'发布任务',exact:true}).waitFor()
 await page.getByText('全部任务',{exact:true}).waitFor()
 await page.getByRole('button',{name:'快速发布',exact:true}).waitFor()
 await page.screenshot({path:path.join(evidence,'01-task-list.png'),fullPage:true})
 const existing=process.env.WT_MEDIA_BROWSER_EXISTING_TASK_ID
 if(existing){
  await page.locator('[aria-label="任务搜索"] input').fill(existing)
  await page.getByRole('button',{name:'查询',exact:true}).click()
  const row=page.locator('tbody tr').filter({has:page.locator('td').filter({hasText:new RegExp(`^${existing}$`)})})
  await row.getByRole('button',{name:'详情',exact:true}).click()
  await page.getByRole('heading',{name:'发布任务详情',exact:true}).waitFor()
  await page.getByText('任务尚未执行',{exact:true}).waitFor()
  await page.screenshot({path:path.join(evidence,'02-shared-task-readback.png'),fullPage:true})
  await page.getByRole('button',{name:'返回',exact:true}).click()
  await row.getByRole('button',{name:'编辑',exact:true}).click()
  await page.locator('[aria-label="话题"] input').fill('#三角洲行动 #两端内容核对')
  await page.getByText('已核对当前文案、封面和声明，确认内容可用',{exact:true}).click()
  assert.equal(await page.getByRole('checkbox',{name:'已核对当前文案、封面和声明，确认内容可用',exact:true}).isChecked(),true)
  const save=page.waitForResponse(r=>r.request().method()==='PUT'&&new URL(r.url()).pathname===`/api/v1/publication/${existing}/content`)
  await page.getByRole('button',{name:'保存',exact:true}).click()
  const response=await save,task=(await response.json()).data
  assert.equal(response.status(),200);assert.equal(task.status,'pending');assert.equal(task.attempt_count,0);assert.equal(task.content.topics,'#三角洲行动 #两端内容核对')
  const read=(await (await context.request.get(new URL(`/api/v1/publication/${existing}`,base).href)).json()).data
  assert.equal(read.revision,task.revision);assert.equal(read.content.topics,task.content.topics)
  await page.getByRole('heading',{name:'发布任务详情',exact:true}).waitFor()
  await page.screenshot({path:path.join(evidence,'03-shared-task-edited.png'),fullPage:true})
  await fs.writeFile(path.join(evidence,'edited-task.json'),JSON.stringify({task_id:task.id,revision:task.revision,status:task.status,attempt_count:task.attempt_count,checks:['real shared list','existing task detail','actual content edit','server read-back']},null,2))
 }else{
 const requested=page.waitForRequest(request=>request.url().includes('/api/v1/publication?')&&new URL(request.url()).searchParams.get('status')==='uncertain')
 await page.getByRole('button',{name:/结果待确认/}).click()
 await requested
 await page.getByRole('button',{name:'重置',exact:true}).click()
 await page.getByRole('button',{name:'快速发布',exact:true}).click()
 await page.getByRole('button',{name:'创建任务',exact:true}).click()
 await page.getByText('请选择发布类目、账号和成片',{exact:true}).waitFor()
 await page.locator('[aria-label="发布类目"]').click()
 await page.locator('.t-select-option').filter({hasText:/^三角洲$/}).click()
 await page.getByRole('button',{name:'选择',exact:true}).click()
 const accounts=page.getByRole('dialog',{name:'选择发布账号',exact:true})
 await accounts.waitFor()
 const accountId=process.env.WT_MEDIA_BROWSER_ACCOUNT_ID
 assert(accountId,'Set WT_MEDIA_BROWSER_ACCOUNT_ID to an existing authorized account')
 const row=accounts.locator('tbody tr').filter({has:page.locator('td').filter({hasText:new RegExp(`^${accountId}$`)})})
 await row.getByRole('button',{name:'选择',exact:true}).click()
 await accounts.waitFor({state:'hidden'})
 assert.equal(await page.getByText('请选择发布类目、账号和成片',{exact:true}).count(),0,'changing resources clears stale validation feedback')
 await page.getByRole('button',{name:'选择成片',exact:true}).click()
 const outputs=page.getByRole('dialog',{name:'选择成片',exact:true})
 await outputs.waitFor()
 if(process.env.WT_MEDIA_BROWSER_INCOMPATIBLE_OUTPUT_ID){
  const row=outputs.locator('tbody tr').filter({has:page.locator('td').filter({hasText:new RegExp(`^${process.env.WT_MEDIA_BROWSER_INCOMPATIBLE_OUTPUT_ID}$`)})})
  await row.getByText('成片类目不匹配',{exact:true}).waitFor()
  assert.equal(await row.getByRole('button',{name:'选择',exact:true}).count(),0,'incompatible output must have no selectable action')
 }
 await page.screenshot({path:path.join(evidence,'02-real-resource-eligibility.png'),fullPage:true})
 const outputId=process.env.WT_MEDIA_BROWSER_OUTPUT_ID
 if(outputId){
  const outputRow=outputs.locator('tbody tr').filter({has:page.locator('td').filter({hasText:new RegExp(`^${outputId}$`)})})
  await outputRow.getByRole('button',{name:'选择',exact:true}).click()
  await outputs.waitFor({state:'hidden'})
  await page.locator('[aria-label="发布标题"] textarea').fill('学会架点了吧宝宝～超简单的！三角洲新手教学')
  await page.getByRole('button',{name:'选择图片',exact:true}).click()
  const images=page.getByRole('dialog',{name:'选择发布封面',exact:true})
  const coverId=process.env.WT_MEDIA_BROWSER_COVER_ID
  assert(coverId,'Set WT_MEDIA_BROWSER_COVER_ID to an existing authorized image')
  const imageRow=images.locator('tbody tr').filter({has:page.locator('td').filter({hasText:new RegExp(`^${coverId}$`)})})
  await imageRow.getByRole('button',{name:'选择',exact:true}).click()
  await images.waitFor({state:'hidden'})
  await page.locator('[aria-label="话题"] input').fill('#三角洲行动 #新手教学')
  await page.locator('[aria-label="创作声明"]').click()
  await page.locator('.t-select-option').filter({hasText:/^无需声明$/}).click()
  await page.getByText('已核对当前文案、封面和声明，确认内容可用',{exact:true}).click()
  assert.equal(await page.getByRole('checkbox',{name:'已核对当前文案、封面和声明，确认内容可用',exact:true}).isChecked(),true)
  await page.screenshot({path:path.join(evidence,'03-ready-content.png'),fullPage:true})
  const creating=page.waitForResponse(r=>r.request().method()==='POST'&&new URL(r.url()).pathname==='/api/v1/publication')
  await page.getByRole('button',{name:'创建任务',exact:true}).click()
  const response=await creating,packet=await response.json()
  await fs.writeFile(path.join(evidence,'create-request.json'),JSON.stringify(response.request().postDataJSON(),null,2))
  assert.equal(response.status(),201,JSON.stringify({errcode:packet.errcode,message:packet.message}))
  const task=packet.data;assert(task.id);assert.equal(task.status,'pending');assert.equal(task.attempt_count,0);assert.equal(task.authorized_at,null)
  const input=response.request().postDataJSON()
  await page.getByRole('heading',{name:'发布任务详情',exact:true}).waitFor()
  await page.getByText('任务尚未执行',{exact:true}).waitFor()
  await page.screenshot({path:path.join(evidence,'04-created-task.png'),fullPage:true})
  await page.getByRole('button',{name:'返回',exact:true}).click()
  await page.locator('[aria-label="任务搜索"] input').fill(task.id)
  await page.getByRole('button',{name:'查询',exact:true}).click()
  await page.getByRole('button',{name:'编辑',exact:true}).click()
  const changed=task.content.description+' · 已核对'
  await page.locator('[aria-label="发布标题"] textarea').fill(changed)
  await page.getByText('已核对当前文案、封面和声明，确认内容可用',{exact:true}).click()
  assert.equal(await page.getByRole('checkbox',{name:'已核对当前文案、封面和声明，确认内容可用',exact:true}).isChecked(),true)
  const saving=page.waitForResponse(r=>r.request().method()==='PUT'&&new URL(r.url()).pathname===`/api/v1/publication/${task.id}/content`)
  await page.getByRole('button',{name:'保存',exact:true}).click()
  const editResponse=await saving,edited=(await editResponse.json()).data
  assert.equal(editResponse.status(),200);assert.equal(edited.revision,String(BigInt(task.revision)+1n));assert.equal(edited.content.description,changed)
  const read=await (await context.request.get(new URL(`/api/v1/publication/${task.id}`,base).href)).json()
  assert.equal(read.data.content.description,changed);assert.equal(read.data.status,'pending');assert.equal(read.data.attempt_count,0)
  const replay=await context.request.post(new URL('/api/v1/publication',base).href,{data:input})
  assert.equal((await replay.json()).data.id,task.id)
  const duplicate=await context.request.post(new URL('/api/v1/publication',base).href,{data:{...input,request_key:crypto.randomUUID()}})
  assert.equal(duplicate.status(),409);assert.equal((await duplicate.json()).errcode,18004)
  const stale=await context.request.put(new URL(`/api/v1/publication/${task.id}/content`,base).href,{data:{expected_revision:task.revision,content:task.content,not_before:null}})
  assert.equal(stale.status(),409);assert.equal((await stale.json()).error.details.current_revision,edited.revision)
  const deletion=await context.request.delete(new URL(`/api/v1/files/${coverId}`,base).href)
  assert.equal(deletion.status(),409);assert.equal((await deletion.json()).errcode,17006)
  const final=await (await context.request.get(new URL(`/api/v1/publication/${task.id}`,base).href)).json()
  assert.equal(final.data.revision,edited.revision);assert.equal(final.data.content.description,changed)
  await fs.writeFile(path.join(evidence,'created-task.json'),JSON.stringify({task_id:task.id,account_id:accountId,output_id:outputId,cover_file_id:coverId,status:final.data.status,revision:final.data.revision,attempt_count:final.data.attempt_count,checks:['actual form create','read-back','content edit','same key replay','duplicate output rejected','stale edit rejected','referenced image deletion rejected']},null,2))
  await page.getByRole('heading',{name:'发布任务详情',exact:true}).waitFor()
  await page.screenshot({path:path.join(evidence,'05-edited-task.png'),fullPage:true})
  console.log('Real prepared task verified',task.id)
 }else{
  await outputs.getByRole('button',{name:'关闭选择成片',exact:true}).click()
  await outputs.waitFor({state:'hidden'})
  await page.screenshot({path:path.join(evidence,'03-quick-publish.png'),fullPage:true})
  await page.getByRole('button',{name:'返回',exact:true}).click()
  const confirm=page.locator('.resource-feedback-dialog').filter({hasText:'当前修改尚未保存'})
  await confirm.waitFor()
  await confirm.getByRole('button',{name:'离开',exact:true}).click()
  await page.getByRole('heading',{name:'发布任务',exact:true}).waitFor()
 }
 }
 assert.deepEqual(errors,[],'unexpected page exceptions')
 console.log(JSON.stringify({base,mode:existing?'read and edit existing task':process.env.WT_MEDIA_BROWSER_OUTPUT_ID?'create, edit and reject unsafe mutations':'selection and unsaved input',pageErrors:errors,screenshots:process.env.WT_MEDIA_BROWSER_OUTPUT_ID?5:3},null,2))
}catch(error){await page.screenshot({path:path.join(evidence,'failure.png'),fullPage:true});throw error}finally{await browser.close()}
