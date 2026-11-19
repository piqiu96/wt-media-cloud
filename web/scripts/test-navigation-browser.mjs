// Read-only check against an explicitly selected Cloud or Desktop dev entry.
// Browser storage and menu expansion are isolated from the operator's browser.
import assert from 'node:assert/strict'
import fs from 'node:fs/promises'
import path from 'node:path'
import {chromium} from 'playwright'

const base=process.env.WT_MEDIA_BROWSER_BASE_URL
const stateFile=process.env.WT_MEDIA_BROWSER_STATE
const evidence=process.env.WT_MEDIA_BROWSER_EVIDENCE
assert(base&&stateFile&&evidence,'Provide current service address, login state and evidence directory')
const state=JSON.parse(await fs.readFile(stateFile,'utf8'))
state.cookies=state.cookies.map(c=>({...c,expires:c.expires??-1,httpOnly:c.httpOnly??false,secure:c.secure??false,sameSite:c.sameSite??'Lax'}))
await fs.mkdir(evidence,{recursive:true})
const browser=await chromium.launch({channel:process.env.WT_MEDIA_BROWSER_CHANNEL||'chrome',headless:true})
const context=await browser.newContext({storageState:state,viewport:{width:1280,height:800},deviceScaleFactor:2})
const page=await context.newPage(),errors=[]
page.on('pageerror',error=>errors.push(error.message))
try{
 await page.goto(new URL('/publish',base).href)
 await page.getByRole('heading',{name:'发布任务',exact:true}).waitFor()
 await page.evaluate(()=>localStorage.setItem('wt-media:sidebar-expanded-groups','[]'))
 await page.reload()
 await page.getByRole('heading',{name:'发布任务',exact:true}).waitFor()
 await page.evaluate(()=>document.fonts.ready)
 const aside=page.locator('.app-aside')
 for(const title of ['数据分析','数据统计','评论模板'])assert.equal(await aside.getByText(title,{exact:true}).count(),0)
 const task=aside.getByText('发布任务',{exact:true})
 await task.waitFor()
 const hierarchy=await task.evaluate(el=>{
  const parent=el.closest('.t-submenu'),outer=parent.parentElement.closest('.t-submenu')
  assertOpen(parent);assertOpen(outer)
  function assertOpen(node){if(!node.classList.contains('t-is-opened'))throw Error('Active ancestor is collapsed')}
  return [outer.querySelector(':scope > .t-menu__item').innerText.trim(),parent.querySelector(':scope > .t-menu__item').innerText.trim(),el.innerText.trim()]
 })
 assert.deepEqual(hierarchy,['运营执行','发布管理','发布任务'])
 for(const title of ['内容挖掘','内容生产'])await aside.getByText(title,{exact:true}).click()
 await aside.getByText('成片管理',{exact:true}).waitFor()
 // Verify the chosen visual hierarchy against rendered components, not source text.
 const visual=await aside.evaluate(el=>{
  const rows=[...el.querySelectorAll('.t-menu__item')].filter(row=>row.getClientRects().length)
  const childRows=rows.filter(row=>row.closest('.t-menu__sub'))
  const active=childRows.find(row=>row.classList.contains('t-is-active')&&!row.parentElement.classList.contains('t-submenu'))
  const style=getComputedStyle(active)
  return {
   childIcons:childRows.reduce((sum,row)=>sum+row.querySelectorAll('.t-icon').length,0),
   childFonts:[...new Set(childRows.map(row=>getComputedStyle(row).fontSize))],
   rowHeights:[...new Set(rows.map(row=>getComputedStyle(row).height))],
   primaryWeights:[...new Set(rows.filter(row=>!row.closest('.t-menu__sub')).map(row=>getComputedStyle(row).fontWeight))],
   childWeights:[...new Set(childRows.filter(row=>row!==active).map(row=>getComputedStyle(row).fontWeight))],
   selectedWeight:style.fontWeight,
   primaryFonts:[...new Set(rows.filter(row=>!row.closest('.t-menu__sub')).map(row=>getComputedStyle(row).fontSize))],
   border:style.borderLeftWidth,borderColor:style.borderLeftColor,radius:style.borderTopLeftRadius,
   fill:style.backgroundColor,asideWidth:parseFloat(getComputedStyle(el).width),
  }
 })
 assert.equal(visual.childIcons,0,'Only top-level categories have leading icons')
 assert.deepEqual(visual.childFonts,['15px'])
 assert.deepEqual(visual.rowHeights,['42px'])
 assert.deepEqual(visual.primaryWeights,['500'])
 assert.deepEqual(visual.childWeights,['400'])
 assert.equal(visual.selectedWeight,'500')
 assert.deepEqual(visual.primaryFonts,['15px'])
 assert.equal(visual.border,'3px')
 assert.equal(visual.borderColor,'rgb(37, 99, 235)')
 assert.equal(visual.radius,'7px')
 assert.equal(visual.fill,'rgb(237, 241, 248)')
 assert.equal(visual.asideWidth,248)
 const breadcrumb=await page.getByLabel('当前页面位置').innerText()
 assert(breadcrumb.indexOf(hierarchy[0])<breadcrumb.indexOf(hierarchy[1])&&breadcrumb.indexOf(hierarchy[1])<breadcrumb.indexOf(hierarchy[2]))
 for(const fontSize of [15,16]){
  const style=await page.addStyleTag({content:`.app-aside .t-menu__content {font-size:${fontSize}px !important}`})
  const clipped=await aside.locator('.t-menu__content').filter({visible:true}).evaluateAll(es=>es.filter(e=>e.scrollWidth>e.clientWidth+1).map(e=>({text:e.textContent.trim(),available:e.clientWidth,required:e.scrollWidth})))
  await aside.screenshot({path:path.join(evidence,`sidebar-${fontSize}px.png`)})
  assert.deepEqual(clipped,[],`Menu labels must be readable at ${fontSize}px`)
  await style.evaluate(e=>e.remove())
 }
 await aside.getByText('运营资源',{exact:true}).click()
 await aside.getByText('代理管理',{exact:true}).click()
 await page.waitForURL('**/proxies')
 const proxyRow=aside.getByText('代理管理',{exact:true}).locator('..')
 assert(await proxyRow.evaluate(el=>el.classList.contains('t-is-active')&&getComputedStyle(el).borderLeftWidth==='3px'))
 await page.mouse.move(700,100)
 await aside.screenshot({path:path.join(evidence,'secondary-selected.png')})
 await page.goto(new URL('/publish',base).href)
 await task.waitFor()
 await page.mouse.move(700,100)
 await aside.screenshot({path:path.join(evidence,'final-sidebar.png')})
 await aside.getByText('发布管理',{exact:true}).click()
 await task.waitFor({state:'hidden'})
 await page.reload()
 await task.waitFor()
 await page.goto(new URL('/interact',base).href)
 await aside.getByText('互动管理',{exact:true}).waitFor()
 await page.locator('.topbar').getByRole('button').first().click()
 await page.waitForFunction(()=>document.querySelector('.app-aside').style.width==='64px')
 const operations=aside.locator('.t-submenu').filter({has:page.getByText('运营执行',{exact:true})})
 await operations.locator(':scope > .t-menu__item').hover()
 await page.getByText('发布管理',{exact:true}).filter({visible:true}).hover()
 await page.getByText('发布任务',{exact:true}).filter({visible:true}).click()
 await page.getByRole('heading',{name:'发布任务',exact:true}).waitFor()
 assert.equal(new URL(page.url()).pathname,'/publish')
 assert.deepEqual(errors,[])
 assert.equal(await page.evaluate(()=>document.documentElement.scrollWidth>innerWidth),false)
 console.log(JSON.stringify({hierarchy,visual,secondary_selection:true,font_sizes:[15,16],labels_unclipped:true,reload:true,collapsed_navigation:true,page_errors:errors}))
}catch(error){
 await page.screenshot({path:path.join(evidence,'failure.png'),fullPage:true}).catch(()=>{})
 throw error
}finally{await context.close();await browser.close()}
