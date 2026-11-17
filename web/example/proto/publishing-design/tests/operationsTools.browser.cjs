const {requestBody}=require('./previewFixture.cjs');
// Chrome checks with isolated prototype data; never writes the reviewer's records.
const {chromium}=require('playwright');
const assert=require('node:assert/strict');
const {pathToFileURL}=require('node:url');
const path=require('node:path'),fs=require('node:fs');
const root=process.env.PROTOTYPE_ROOT||path.resolve(__dirname,'..');
const base=process.env.EXAMPLE_URL||'http://127.0.0.1:8768';
const evidence=process.env.SCREENSHOT_DIR||'/tmp/wt-operations-tools';fs.mkdirSync(evidence,{recursive:true});
(async()=>{
 const {seed}=await import(pathToFileURL(path.join(root,'model.js')));
 const data=seed();data.images[1].uses=['现成封面'];data.images.push(...Array.from({length:24},(_,i)=>({...data.images[i%8],id:700+i,name:`走查图片 ${i+1}`})));
 const missing={...data.images[0],id:799,name:'元信息缺失样例'};delete missing.width;delete missing.height;delete missing.format;delete missing.source;data.images.push(missing);
 let snapshot={revision:1,data},delay=0;
 const browser=await chromium.launch({headless:true,...(process.env.CHROMIUM_EXECUTABLE?{executablePath:process.env.CHROMIUM_EXECUTABLE}:{})});
 const passed=[];const pass=name=>{passed.push(name);console.log('PASS',name)};
 try{
  const page=await browser.newPage({viewport:{width:1440,height:900}});page.setDefaultTimeout(6000);const errors=[];page.on('pageerror',e=>{errors.push(e.message);console.error('PAGEERROR',e.stack)});
  await page.route('**/__example/publishing/**',async r=>{if(r.request().method()!=='GET'){if(delay)await new Promise(resolve=>setTimeout(resolve,delay));snapshot={revision:snapshot.revision+1,data:requestBody(r.request()).data}}return r.fulfill({json:snapshot})});
  const basic=()=>page.locator('.resource-filter-bar:visible');
  const row=()=>page.locator('.t-table__body:visible tr').first();
  async function select(field,value,scope=basic()) {await scope.locator('.wt-filter-field').filter({has:page.locator('.wt-filter-field__label').filter({hasText:new RegExp(`^${field}$`)})}).locator('.t-select__wrap').click();await page.locator('.t-select-option:visible').filter({hasText:new RegExp(`^${value}$`)}).click();}
  const css=locator=>locator.evaluate(e=>{let s=getComputedStyle(e);return {background:s.backgroundColor,color:s.color}});
  async function shot(name){await page.evaluate(()=>document.querySelectorAll('.t-message__close').forEach(e=>e.click()));await page.waitForTimeout(350);await page.evaluate(()=>new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r))));await page.screenshot({path:path.join(evidence,name+'.png')});}
  await page.goto(base+'/example/operations/tools/images');await page.getByRole('heading',{name:'图片库',exact:true}).waitFor();
  assert.deepEqual(await basic().locator(':scope > .wt-filter-field > .wt-filter-field__label').allTextContents(),['名称','内容类目','用途','比例']);
  assert.deepEqual(await page.locator('table:visible thead th').allTextContents(),['','ID','图片','类目','用途','尺寸','启用','操作']);
  assert.deepEqual(await basic().locator('.t-select__wrap input').evaluateAll(es=>es.map(e=>e.value)),['全部','全部','全部']);
  await basic().getByRole('button',{name:/高级筛选/}).click();assert.deepEqual(await basic().locator('.wt-advanced-filters .wt-filter-field__label').allTextContents(),['来源']);
  await select('来源','人工上传');await basic().getByRole('button',{name:'查询',exact:true}).click();await page.getByText('没有符合条件的记录',{exact:true}).waitFor();await basic().getByRole('button',{name:'重置',exact:true}).click();await row().waitFor();
  pass('图片库精简筛选、全部默认项、来源查询及重置');
  await select('内容类目','三角洲行动');await basic().getByRole('button',{name:'查询',exact:true}).click();await page.locator('.t-pagination__btn-next:visible').click();
  const pageBefore=await page.locator('.t-pagination__number.t-is-current:visible').innerText();const nameBefore=await row().locator('.title-cell strong').innerText();
  await row().getByRole('button',{name:'详情',exact:true}).click();let detail=page.getByRole('dialog',{name:'图片详情',exact:true});await detail.waitFor();assert.equal(await detail.getByRole('switch').count(),0);await detail.getByText(nameBefore,{exact:true}).waitFor();await shot('image-detail');await detail.getByRole('button',{name:'关闭',exact:true}).click();
  assert.equal(await row().locator('.title-cell strong').innerText(),nameBefore);assert.equal(await page.locator('.t-pagination__number.t-is-current:visible').innerText(),pageBefore);
  await row().getByRole('button',{name:'编辑',exact:true}).click();let dialog=page.getByRole('dialog',{name:'编辑图片',exact:true});await dialog.waitFor();assert.equal(await dialog.getByRole('switch').count(),0);await dialog.getByRole('button',{name:'取消',exact:true}).click();
  pass('图片详情及编辑仅只读展示使用状态，关闭保留筛选与分页');
  await basic().getByRole('button',{name:'重置',exact:true}).click();await basic().getByPlaceholder('输入名称查询').fill('元信息缺失样例');await basic().getByRole('button',{name:'查询',exact:true}).click();await row().getByRole('button',{name:'详情',exact:true}).click();detail=page.getByRole('dialog',{name:'图片详情',exact:true});await detail.waitFor();assert.ok(await detail.locator('.t-descriptions__content').filter({hasText:/^—$/}).count()>=4);assert.equal(await detail.getByText('种子资源',{exact:true}).count(),0);await detail.getByRole('button',{name:'关闭',exact:true}).click();await basic().getByRole('button',{name:'重置',exact:true}).click();
  pass('详情缺失元信息显示空值，不编造格式、来源或版本');
  await shot('images-1440');await page.setViewportSize({width:1280,height:800});await shot('images-1280');
  await page.goto(base+'/example/operations/tools/templates');await page.getByRole('heading',{name:'封面模板',exact:true}).waitFor();await basic().getByRole('button',{name:/高级筛选/}).click();
  assert.deepEqual(await basic().locator('.t-select__wrap input').evaluateAll(es=>es.map(e=>e.value)),['全部','全部','全部','全部','全部']);
  const geometry=await basic().locator('.wt-filter-field').evaluateAll(es=>es.map(e=>({label:e.textContent.trim(),width:e.getBoundingClientRect().width})));
  assert.equal(geometry[0].width,240);assert.ok(geometry.slice(1).every(e=>e.width===160));assert.equal(await basic().locator('.wt-advanced-filters').evaluate(e=>getComputedStyle(e).borderTopWidth),'0px');
  await shot('templates-1280');await page.setViewportSize({width:1440,height:900});await shot('templates-1440');
  pass('模板筛选统一宽度与全部默认项，高级筛选无多余分隔线');
  await row().getByRole('button',{name:'编辑',exact:true}).click();await page.locator('.cover-editor-heading').waitFor();
  assert.equal(await page.locator('.editor-toolbar').getByText('安全区',{exact:true}).count(),0);assert.equal(await page.getByRole('button',{name:'模板设置',exact:true}).count(),0);assert.equal(await page.locator('.canvas-size-select').count(),0);
  assert.equal(await page.locator('.cover-save-status').count(),1);assert.equal(await page.getByRole('button',{name:'动态主标题',exact:true}).count(),0);
  assert.equal((await css(page.getByRole('button',{name:'上移动态主标题',exact:true}))).background,'rgba(0, 0, 0, 0)');const save=()=>page.getByRole('button',{name:'保存草稿',exact:true});assert.ok(await save().isDisabled());assert.deepEqual(await css(save()),{background:'rgb(246, 248, 251)',color:'rgb(140, 151, 165)'});
  const template=snapshot.data.templates[0];assert.ok(!decodeURIComponent(template.document.image).includes('<text'));
  await page.locator('.background-layer').click();await page.getByRole('heading',{name:'背景与画布',exact:true}).waitFor();await page.locator('.cover-canvas-settings summary').click();
  const safe=page.getByRole('checkbox',{name:'显示安全区辅助线',exact:true});await page.locator('.cover-canvas-settings .t-checkbox').click();assert.equal(await safe.isChecked(),false);assert.equal(await page.locator('svg>rect[data-editor]').count(),0);assert.ok(await save().isDisabled(),'隐藏辅助线不改设计草稿');
  await page.locator('.editor-layers .layer-name').filter({hasText:'动态主标题'}).click();const x=page.locator('.editor-properties-panel .editor-field').filter({hasText:'X 坐标'}).locator('input');await x.fill('0');await x.press('Tab');
  assert.equal(await page.locator('svg g[data-editor] rect').first().getAttribute('stroke'),'var(--wt-danger)');await page.getByRole('button',{name:'预览与校验',exact:true}).click();await page.getByText('文字超出画布或安全区',{exact:true}).waitFor();await page.locator('.cover-issue.error').getByRole('button',{name:'定位修改',exact:true}).first().click();await page.locator('.editor-properties-panel textarea').waitFor();
  assert.equal(await page.locator('.editor-layer.is-selected .layer-label').innerText(),'动态主标题\n动态字段');await page.locator('.image-editor').press('Meta+z');
  pass('安全区归入背景与画布；隐藏辅助线不绕过校验，问题可定位');
  const art=page.locator('.editor-canvas-viewport svg');await page.locator('.background-layer').click();await art.getByText('精彩实战指南',{exact:true}).click();assert.ok((await page.locator('.editor-layer.is-selected').innerText()).includes('动态主标题'));
  await page.getByRole('button',{name:'锁定',exact:true}).first().click();assert.ok(await page.locator('.editor-properties-panel textarea').isDisabled());await page.getByRole('button',{name:'解锁',exact:true}).click();
  await page.locator('.editor-properties-panel textarea').fill('新的标题样例');await page.waitForTimeout(250);await shot('editor-unsaved');assert.equal((await css(save())).background,'rgb(0, 82, 217)');assert.equal((await css(save())).color,'rgb(255, 255, 255)');await save().focus();await page.keyboard.press('Tab');await page.keyboard.press('Shift+Tab');assert.equal(await save().evaluate(e=>getComputedStyle(e).outlineStyle),'solid');await save().hover();await page.waitForTimeout(250);assert.equal((await css(save())).color,'rgb(255, 255, 255)');
  delay=650;await save().click();assert.ok(await page.getByRole('button',{name:'返回',exact:true}).isDisabled());await page.waitForTimeout(250);assert.equal((await css(save())).color,'rgb(255, 255, 255)');await page.locator('.cover-save-status').filter({hasText:/^已保存$/}).waitFor();delay=0;await page.waitForTimeout(250);assert.ok(await save().isDisabled());assert.equal((await css(save())).background,'rgb(246, 248, 251)');assert.equal(await page.locator('.cover-editor-heading').getByText('已发布',{exact:true}).count(),1);assert.equal(await page.locator('.cover-editor-heading').getByText('有草稿',{exact:true}).count(),1);
  pass('画布选中同步、锁定只读；Footer 正常／悬停／保存中／禁用态清晰');
  for(const size of [{width:1440,height:900},{width:1280,height:800}]){
   await page.setViewportSize(size);await page.waitForTimeout(150);const bounds=await page.locator('.editor-grid').boundingBox(),foot=await page.locator('.resource-detail-page__footer:visible').boundingBox();assert.ok(bounds.y+bounds.height<=foot.y-12);
   const svgBox=await art.boundingBox(),viewport=await page.locator('.editor-canvas-viewport').boundingBox();assert.ok(Math.abs(svgBox.x+svgBox.width/2-viewport.x-viewport.width/2)<2,'画布在中间区域水平居中');assert.ok(Math.abs(svgBox.width/svgBox.height-4/3)<.01);
   await shot('editor-'+size.width);
  }
  await page.locator('.canvas-zoom-select').click();await page.locator('.t-select-option:visible').filter({hasText:/^100%$/}).click();assert.equal(Math.round((await art.boundingBox()).width),1200);assert.equal(Math.round((await art.boundingBox()).height),900);assert.ok(await page.locator('.editor-canvas-viewport').evaluate(e=>e.scrollWidth>e.clientWidth&&e.scrollHeight>e.clientHeight));
  await page.locator('.canvas-zoom-select').click();await page.locator('.t-select-option:visible').filter({hasText:'适应画布'}).click();await page.getByRole('button',{name:'折叠图层',exact:true}).click();assert.equal(await page.locator('.editor-layers:visible').count(),0);await page.getByRole('button',{name:'展开图层',exact:true}).click();
  pass('两种桌面尺寸居中且 Footer 无遮挡，100% 双向滚动与图层折叠可用');
  await page.locator('.background-layer').click();
  await page.locator('.editor-properties-panel .cover-field').filter({hasText:'背景类型'}).locator('.t-select__wrap').click();await page.locator('.t-select-option:visible').filter({hasText:/^固定图片$/}).click();
  const png=await page.evaluate(()=>{const c=document.createElement('canvas');c.width=2;c.height=2;c.getContext('2d').fillRect(0,0,2,2);return c.toDataURL().split(',')[1]});
  await page.getByLabel('上传固定背景',{exact:true}).setInputFiles({name:'背景上传走查.png',mimeType:'image/png',buffer:Buffer.from(png,'base64')});
  const currentBackground=page.locator('.editor-properties-panel .cover-field').filter({has:page.getByText('固定背景图片',{exact:true})}).locator('.t-select__wrap input');
  await page.waitForFunction(()=>[...document.querySelectorAll('.editor-properties-panel .t-select__wrap input')].some(i=>i.value==='当前背景图片'));assert.equal(await currentBackground.inputValue(),'当前背景图片');
  await page.locator('.editor-properties-panel .cover-field').filter({hasText:'背景类型'}).locator('.t-select__wrap').click();await page.locator('.t-select-option:visible').filter({hasText:/^业务底图$/}).click();
  await page.locator('.editor-properties-panel .cover-field').filter({hasText:'底图预览样例'}).locator('.t-select__wrap').click();assert.equal(await page.locator('.t-select-option:visible').filter({hasText:/^火影忍者连招教学$/}).count(),0);await page.keyboard.press('Escape');
  await save().click();await page.locator('.cover-save-status').filter({hasText:/^已保存$/}).waitFor();pass('上传背景显示可读名称，业务底图样例遵守用途范围');
  await page.getByRole('button',{name:'返回',exact:true}).click();await page.getByRole('heading',{name:'封面模板',exact:true}).waitFor();
  await page.getByRole('button',{name:'新建模板',exact:true}).click();dialog=page.getByRole('dialog',{name:'新建模板',exact:true});await dialog.waitFor();assert.equal((await css(dialog.getByRole('button',{name:'创建并编辑',exact:true}))).color,'rgb(255, 255, 255)');await dialog.getByRole('button',{name:'取消',exact:true}).click();
  pass('新建 Dialog 与公共按钮的键盘焦点及主次样式正常');const failed=await browser.newPage();await failed.route('**/__example/publishing/**',r=>r.fulfill({status:500,json:{message:'走查模拟加载失败'}}));await failed.goto(base+'/example/operations/tools/images');await failed.getByRole('heading',{name:'图片库加载失败',exact:true}).waitFor();await failed.close();assert.deepEqual(errors,[]);fs.writeFileSync(path.join(evidence,'results.json'),JSON.stringify({passed,errors,viewports:['1440x900','1280x800'],isolation:'Route-backed snapshot; no user records changed'},null,2));console.log(`${passed.length} scenarios passed`);
 }finally{await browser.close()}
})().catch(e=>{console.error(e);process.exitCode=1});
