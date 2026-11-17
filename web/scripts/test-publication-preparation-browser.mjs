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
 const requested=page.waitForRequest(request=>request.url().includes('/api/v1/publications?')&&new URL(request.url()).searchParams.get('status')==='uncertain')
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
 await page.getByRole('button',{name:'选择成片',exact:true}).click()
 const outputs=page.getByRole('dialog',{name:'选择成片',exact:true})
 await outputs.waitFor()
 if(process.env.WT_MEDIA_BROWSER_INCOMPATIBLE_OUTPUT_ID){
  const row=outputs.locator('tbody tr').filter({has:page.locator('td').filter({hasText:new RegExp(`^${process.env.WT_MEDIA_BROWSER_INCOMPATIBLE_OUTPUT_ID}$`)})})
  await row.getByText('成片类目不匹配',{exact:true}).waitFor()
  assert.equal(await row.getByRole('button',{name:'选择',exact:true}).count(),0,'incompatible output must have no selectable action')
 }
 await page.screenshot({path:path.join(evidence,'02-real-resource-eligibility.png'),fullPage:true})
 await outputs.getByRole('button',{name:'关闭选择成片',exact:true}).click()
 await outputs.waitFor({state:'hidden'})
 await page.screenshot({path:path.join(evidence,'03-quick-publish.png'),fullPage:true})
 await page.getByRole('button',{name:'返回',exact:true}).click()
 const confirm=page.locator('.resource-feedback-dialog').filter({hasText:'当前修改尚未保存'})
 await confirm.waitFor()
 await confirm.getByRole('button',{name:'离开',exact:true}).click()
 await page.getByRole('heading',{name:'发布任务',exact:true}).waitFor()
 assert.deepEqual(errors,[],'unexpected page exceptions')
 console.log(JSON.stringify({base,checks:['real server list and summary','metric filter request','empty form rejection','real account selection','incompatible output rejected','resource picker closure','unsaved input confirmation'],pageErrors:errors,screenshots:3},null,2))
}catch(error){await page.screenshot({path:path.join(evidence,'failure.png'),fullPage:true});throw error}finally{await browser.close()}
