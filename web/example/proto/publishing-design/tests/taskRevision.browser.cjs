const {test,before,after}=require('node:test');
const assert=require('node:assert/strict');
const {chromium}=require('playwright');
const {attachStore}=require('./previewFixture.cjs');
const {pathToFileURL}=require('node:url');
const path=require('node:path');
const root=path.resolve(__dirname,'..');
const base=process.env.EXAMPLE_URL || 'http://127.0.0.1:8768';
async function screenshot(page,name){if(process.env.SCREENSHOT_DIR) await page.screenshot({path:path.join(process.env.SCREENSHOT_DIR,name),fullPage:true});}
let browser,seed,defaults,declarationOptions,freezeBatch;
before(async()=>{({seed,defaults,declarationOptions}=await import(pathToFileURL(root+'/model.js')));({freezeBatch}=await import(pathToFileURL(root+'/batchLifecycle.js')));browser=await chromium.launch({...(process.env.CHROMIUM_EXECUTABLE ? {executablePath:process.env.CHROMIUM_EXECUTABLE} : {}),headless:true});});
after(async()=>browser?.close());
async function setup(t,mutate=()=>{},path='publishing/tasks',viewport={width:1440,height:1000}){
 const context=await browser.newContext({viewport});t.after(()=>context.close());const page=await context.newPage();const errors=[];page.on('pageerror',e=>errors.push(e.message));t.after(()=>assert.deepEqual(errors,[]));const data=seed();mutate(data);const writes=[];
 await attachStore(page,data,saved=>writes.push(saved));
 await page.goto(base+'/example/operations/'+path);await page.locator('.publish-shell').waitFor();return{page,writes,data};
}
function draft(data){const task={...data.tasks[1],status:'待执行',platform:'B站',batchId:null,claimed:false,batchStopped:false,assetId:129,category:'火影忍者',accountId:12,...defaults('B站',data.assets[1],data)};task.declaration=declarationOptions('B站')[0].value;data.tasks=[];data.batches=[];data.quickDraft={active:true,platform:'B站',category:task.category,accountId:task.accountId,row:task,creationKey:'task-revision-browser',savedAt:'2026-11-16T00:00:00Z'};}
async function openDraft(page){await page.getByRole('button',{name:'快速发布',exact:true}).first().click();await page.getByRole('button',{name:'继续编辑',exact:true}).click();await page.getByRole('heading',{name:'快速发布',exact:true}).waitFor();await page.locator('.t-dialog:visible').waitFor({state:'hidden'});}
test('account and asset pickers keep inherited platform/category locked; cover uses source cards',async t=>{
 const{page}=await setup(t,draft);await openDraft(page);
 const selectedInput=page.locator('.quick-account-input input');assert.equal(await selectedInput.inputValue(),'实战研究所');assert.equal(await selectedInput.getAttribute('readonly'),'');assert.equal(await page.locator('.quick-account-name').count(),0);assert.equal(await page.getByRole('button',{name:'查看发布账号 实战研究所',exact:true}).count(),0);
 assert.equal(await page.locator('.publishing-execution-settings input:disabled').inputValue(),'浏览器辅助发布（自动提交）');
 await page.getByRole('radio',{name:/使用现成封面/}).waitFor();
 await page.getByRole('radio',{name:/模板合成/}).check();await page.locator('.quick-cover-controls').getByText('封面模板',{exact:true}).waitFor();await page.getByRole('radio',{name:/使用现成封面/}).check();
 await page.getByRole('button',{name:'更换账号',exact:true}).click();
 let scope=page.locator('.publishing-scope-filters:visible');assert.equal(await scope.locator('input:disabled').count(),2);
 const picker=page.locator('.account-list:visible');assert.ok((await picker.innerText()).includes('实战研究所'));assert.ok(!(await picker.innerText()).includes('游戏魔王嘟嘟'));
 await page.keyboard.press('Escape');await picker.waitFor({state:'hidden'});
 await selectedInput.click();await picker.waitFor();assert.equal(await page.getByText('发布账号详情',{exact:true}).count(),0);await page.keyboard.press('Escape');await picker.waitFor({state:'hidden'});
 await page.getByRole('button',{name:'更换成片',exact:true}).click();scope=page.locator('.publishing-scope-filters:visible');assert.equal(await scope.locator('input:disabled').count(),2);
 assert.ok((await page.locator('.quick-picker-grid:visible').innerText()).includes('火影'));assert.ok(!(await page.locator('.quick-picker-grid:visible').innerText()).includes('三角洲'));
 await page.keyboard.press('Escape');await page.locator('.quick-picker-grid:visible').waitFor({state:'hidden'});await page.waitForTimeout(450);await screenshot(page,'task-cover.png');
});
test('content thumbnail opens complete content and account drawer preserves the list',async t=>{
 const{page}=await setup(t,data=>{data.tasks[0].batchId=null;data.tasks[0].batchConfigLocked=false;});
 const row=page.getByRole('row').filter({hasText:'1001'});
 await row.getByRole('button',{name:/查看发布账号/}).click();await page.getByText('发布账号详情',{exact:true}).waitFor();await page.getByText('绑定窗口',{exact:true}).waitFor();await page.keyboard.press('Escape');
 await row.getByRole('button',{name:'查看完整发布内容 1001',exact:true}).click();await page.getByRole('heading',{name:'发布任务详情',exact:true}).waitFor();await page.getByText('发布方式',{exact:true}).waitFor();
 await page.getByRole('button',{name:/查看发布账号/}).click();await page.getByText('发布账号详情',{exact:true}).waitFor();await page.keyboard.press('Escape');
 await page.getByRole('button',{name:'返回',exact:true}).click();await row.waitFor();
});
test('unclaimed pending content can be edited in a started batch without changing its frozen snapshot',async t=>{
 const{page,writes,data}=await setup(t,data=>{const task=data.tasks[0];Object.assign(task,{claimed:false,status:'待执行',batchId:201,declaration:declarationOptions(task.platform)[0].value,version:1});data.tasks=[task];data.batches=[{id:201,name:'已启动批次',platform:task.platform,targets:[{category:task.category,count:1}],phase:'内容预检通过',control:'执行中'}];freezeBatch(data,data.batches[0]);});
 const row=page.getByRole('row').filter({hasText:'1001'});await row.getByRole('button',{name:'编辑',exact:true}).click();await page.locator('[data-field="title"] input').fill('待执行内容修订');await page.getByRole('button',{name:'保存',exact:true}).click();await page.getByRole('heading',{name:'发布任务',exact:true}).waitFor();
 assert.equal(writes.at(-1).tasks[0].title,'待执行内容修订');assert.deepEqual(writes.at(-1).batches[0].frozenSnapshot,data.batches[0].frozenSnapshot);
});
test('platform file defaults are selected automatically and there is no configuration page',async t=>{
 const{page,writes}=await setup(t,data=>{data.publishingConfig={'B站':{version:99,tags:'旧页面配置',partition:'旧分区',visibility:'仅自己可见'}};});
 assert.equal(await page.getByRole('link',{name:'平台发布配置',exact:true}).count(),0);
 assert.equal(await page.getByText('系统配置',{exact:true}).count(),0);
 await page.getByRole('button',{name:'快速发布',exact:true}).first().click();
 const resource=page.locator('.quick-resource-grid').first();
 await resource.locator('label').filter({hasText:'发布平台'}).locator('.t-select').click();await page.locator('.t-select-option').filter({hasText:'B站'}).click();
 await resource.locator('label').filter({hasText:'发布类目'}).locator('.t-select').click();await page.locator('.t-select-option').filter({hasText:'火影忍者'}).click();
 assert.equal(await page.locator('[data-field="platform"] input').inputValue(),'火影忍者');
 assert.equal(await page.locator('.quick-copy-fields > label').filter({hasText:'可见范围'}).locator('input').inputValue(),'公开');
 assert.equal(await page.getByText(/配置版本/).count(),0);
 await page.locator('[data-field="declaration"] .t-select').click();await page.locator('.t-select-option').filter({hasText:'内容无需标注'}).click();
 await page.getByRole('button',{name:'创建任务',exact:true}).click();await page.getByRole('heading',{name:'发布任务详情',exact:true}).waitFor();
 const task=writes.at(-1).tasks.at(-1);assert.equal(task.tags,'火影忍者');assert.equal(task.partition,'游戏');assert.equal(task.visibility,'公开');assert.equal(task.publishMethod,'local_assisted');assert.equal(task.publishingConfigVersion,1);assert.equal('publishingConfig' in writes.at(-1),false);
 await page.goto(base+'/example/operations/settings/publishing');await page.getByRole('heading',{name:'素材库',exact:true}).waitFor();assert.equal(await page.getByRole('button',{name:'保存配置',exact:true}).count(),0);
});
test('the final publication section uses a minute date-time picker and persists the chosen local time',async t=>{
 const{page,writes}=await setup(t,draft);await openDraft(page);
 const sections=page.locator('.quick-publish > section');assert.equal(await sections.count(),3);assert.equal(await sections.last().getAttribute('aria-label'),'发布设置');
 const settings=page.getByRole('region',{name:'发布设置',exact:true});await settings.scrollIntoViewIfNeeded();
 assert.equal(await settings.getByText(/前置动作|后置动作/).count(),0);assert.equal(await settings.locator('input:disabled').inputValue(),'浏览器辅助发布（自动提交）');assert.equal(await settings.getByText('留空表示启动后尽快发布。',{exact:true}).count(),0);
 assert.equal(await page.locator('.quick-publish input[type="datetime-local"]').count(),0);
 const input=settings.locator('.resource-date-time input');await input.click();await page.locator('.t-date-picker__panel-content:visible').waitFor();await input.fill('2030-11-16 12:34');await input.press('Enter');
 const confirm=page.locator('.t-date-picker__footer-confirm:visible');if(await confirm.count())await confirm.click();await input.blur();await page.locator('.t-date-picker__panel-content:visible').waitFor({state:'hidden'});
 assert.equal(await input.inputValue(),'2030-11-16 12:34');
 if(process.env.SCREENSHOT_DIR)await settings.screenshot({path:path.join(process.env.SCREENSHOT_DIR,'publishing-execution-settings.png')});
 await page.getByRole('button',{name:'创建任务',exact:true}).click();await page.getByRole('heading',{name:'发布任务详情',exact:true}).waitFor();assert.equal(writes.at(-1).tasks.at(-1).schedule,'2030-11-16T12:34');
});
for(const platform of ['百家号','小红书']) test(platform+' topic edits replace inherited defaults in the saved task and detail',async t=>{
 const{page,writes}=await setup(t,data=>{const account=data.accounts.find(a=>a.platform===platform&&a.categories.includes('火影忍者'));const task={platform,assetId:129,accountId:account.id,category:'火影忍者',...defaults(platform,data.assets[1],data),declaration:declarationOptions(platform)[0].value};task[platform==='百家号'?'tags':'topics']='已保存话题';data.tasks=[];data.batches=[];data.quickDraft={active:true,platform,category:task.category,accountId:account.id,row:task,creationKey:'topics-'+platform};});
 await openDraft(page);const topics=page.locator('.quick-copy-fields > label').filter({hasText:'话题'}).locator('input');assert.equal(await topics.inputValue(),'已保存话题');await topics.fill('本次发布话题');await page.getByRole('button',{name:'创建任务',exact:true}).click();await page.getByRole('heading',{name:'发布任务详情',exact:true}).waitFor();
 const saved=writes.at(-1).tasks.at(-1);assert.equal(saved[platform==='百家号'?'tags':'topics'],'本次发布话题');assert.match(page.url(),new RegExp('/publishing/tasks/'+saved.id+'$'));await page.getByText('本次发布话题',{exact:true}).waitFor();assert.equal(await page.getByText('已保存话题',{exact:true}).count(),0);
});
test('visible publication titles are searchable and counted in task metrics',async t=>{
 const{page}=await setup(t,data=>{const task=data.tasks[2];Object.assign(task,{status:'待执行',batchId:null,platform:'百家号',accountId:21,title:'成片原名',description:'列表实际发布描述'});data.tasks=[task];data.batches=[];});
 await page.getByRole('button',{name:'列表实际发布描述',exact:true}).waitFor();await page.getByPlaceholder('搜索任务名称或 ID').fill('列表实际发布描述');await page.getByRole('button',{name:'查询',exact:true}).click();await page.waitForTimeout(150);assert.equal(await page.getByRole('row').filter({hasText:'1003'}).count(),1);
});

test('publishing editor and inherited picker fit a narrow viewport',async t=>{
 const{page}=await setup(t,draft,'publishing/tasks',{width:390,height:844});await openDraft(page);assert.equal(await page.evaluate(()=>document.documentElement.scrollWidth>innerWidth),false);await page.getByRole('button',{name:'更换账号',exact:true}).click();const box=await page.locator('.publishing-scope-filters:visible').boundingBox();assert.ok(box.x>=0&&box.x+box.width<=390);await page.keyboard.press('Escape');await page.locator('.account-list:visible').waitFor({state:'hidden'});await page.waitForTimeout(450);await screenshot(page,'task-mobile.png');
});

test('date-time popup fits a narrow viewport and exposes only hour and minute columns',async t=>{
 const{page}=await setup(t,draft,'publishing/tasks',{width:390,height:844});await openDraft(page);
 const settings=page.getByRole('region',{name:'发布设置',exact:true});await settings.locator('.resource-date-time input').click();const panel=page.locator('.t-date-picker__panel-content:visible');await panel.waitFor();
 const box=await panel.boundingBox();assert.ok(box.x>=0&&box.x+box.width<=390,JSON.stringify(box));assert.equal(await panel.locator('.t-time-picker__panel-body-scroll').count(),2);
 await page.keyboard.press('Escape');
});

test('batch confirmation shows the same disabled publication method',async t=>{
 const{page}=await setup(t,data=>{data.tasks=[];data.batches=[];const asset=data.assets[0],platform='B站';const row={...defaults(platform,asset,data),platform,category:asset.category,assetId:asset.id,accountId:11,candidateId:'batch-method',contentApproved:true,coverApproved:true,declaration:declarationOptions(platform)[0].value};data.publishDraft={active:true,mode:'批量发布',step:3,name:'发布方式验证',platforms:[platform],categories:[asset.category],rows:[row],assets:[asset.id],targets:[{category:asset.category,count:1}],resourcesLocked:true,creationKey:'batch-method'};},'publishing/batches');
 await page.getByRole('button',{name:'批量发布',exact:true}).click();await page.getByRole('button',{name:'继续编辑',exact:true}).click();const settings=page.locator('.batch-review-card').filter({hasText:'发布设置'});const method=settings.locator('input:disabled');await method.waitFor();assert.equal(await method.inputValue(),'浏览器辅助发布（自动提交）');
});
