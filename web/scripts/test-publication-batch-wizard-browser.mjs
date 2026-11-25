// Production Vue routes; synthetic Cloud and native draft boundaries, no platform posting.
import assert from 'node:assert/strict';
import { chromium } from 'playwright';

const base = process.env.WT_MEDIA_BROWSER_BASE_URL || 'http://127.0.0.1:5173';
const browser = await chromium.launch({ channel: process.env.WT_MEDIA_BROWSER_CHANNEL || 'chrome', headless: true });
const actor = { id:'71', role:'operator', team_id:'3', category_ids:['12'], status:'enabled' };
const category = { id:'12', name:'走查类目', enabled:true };
const account = { id:'81', name:'本人账号', platform:'baidu', category_ids:['12'], eligible:true };
const output = { id:'91', composite_output_id:'91', name:'本人可用成片', category_id:'12', team_id:'3', occupied:false, platforms:['baidu'], video:{status:'ready'} };
const form = () => ({name:'步骤异常走查', platform:'baidu', recommendation_quantity:3,
  targets:[{category_id:'12', target_count:2}],
  config:{accounts:[{media_account_id:'81',limit:2}],output_ids:['91'],manual_pairs:[],candidates:[{
    row_key:'row-one', input_revision:'1', field_revisions:{}, order:0, category_id:'12',media_account_id:'81',composite_output_id:'91',content:{description:'原有人工文案'},
  }]},execution_setting:{concurrency_limit:5,group_interval_seconds:0,between_group_policy:'pause_on_exception',exception_task_limit:3}});

async function scene({draft=null,native=true,inventoryFailure=false,saveFailure=false,conflict=false}={}) {
  const context = await browser.newContext({viewport:{width:1440,height:1000}});
  const page = await context.newPage(); page.setDefaultTimeout(6000);
  const errors=[], calls=[]; let saved, validationGate;
  page.on('pageerror',error=>errors.push(error.message));
  if(native) await context.addInitScript(({draft})=>{
    const key='wizard-test-draft';
    if(!sessionStorage.getItem(key)) sessionStorage.setItem(key,JSON.stringify({payload:draft,revision:'0',saved_at:null,schema_version:1}));
    window.__TAURI_INTERNALS__={
      invoke:async(command,{args}={})=>{
        const current=JSON.parse(sessionStorage.getItem(key));
        if(command==='local_publication_draft_get')return current;
        if(command==='local_publication_draft_save'||command==='local_publication_draft_clear'){
          if(sessionStorage.getItem('wizard-write-failure'))throw Error('本机草稿写入失败');
          assertRevision(args.expected_revision,current.revision);
          const next={...current,payload:command.endsWith('_clear')?null:args.payload,revision:String(Number(current.revision)+1)};
          sessionStorage.setItem(key,JSON.stringify(next));return next;
        }
        return null;
      },transformCallback:()=>1,unregisterCallback:()=>{},
      metadata:{currentWindow:{label:'main'},currentWebview:{label:'main'}},
    };
    function assertRevision(expected,actual){if(expected!==actual)throw Error('草稿修订冲突');}
  },{draft});
  const source=()=>({id:'5',revision:'4',...form(),operator_id:'71',team_id:'3',allowed_actions:['edit'],execution_status:'not_started',configuration_status:'incomplete',readiness:{ready:false,issues:[],shortfalls:[]}});
  await page.route('**/api/v1/**',async route=>{
    const request=route.request(),url=new URL(request.url()),path=url.pathname,input=request.postDataJSON();
    calls.push({path,method:request.method(),input});
    const send=data=>route.fulfill({json:{errcode:0,message:'success',data,logid:'wizard-test'}});
    const reject=message=>route.fulfill({status:409,json:{errcode:18001,message,data:null,error:{code:'publication_conflict'}}});
    if(path==='/api/v1/auth/me')return send(actor);
    if(path==='/api/v1/categories')return send([category]);
    if(path==='/api/v1/publication/account')return send({items:[account],total:1});
    if(path==='/api/v1/publication/output')return send({items:[output],total:1});
    if(path==='/api/v1/publication/resource')return inventoryFailure?route.abort('failed'):send({availability:'available',available_output_count:1,eligible_account_count:1,as_of:'2026-11-25T10:00:00Z'});
    if(path==='/api/v1/publication-batch/allocate')return send({config:{candidates:form().config.candidates},shortfalls:[]});
    if(path==='/api/v1/publication-batch/validate'){
      const gate=validationGate; if(gate)await gate;
      return send({ready:false,issues:[{row_key:'row-one',message:'迟到的校验结果'}],shortfalls:[]});
    }
    if(path==='/api/v1/publication-batch/5/configuration')return conflict?reject('批次已被修改，请重新读取'):send({...source(),...input,revision:'5'});
    if(path==='/api/v1/publication-batch/5')return send(saved||source());
    if(path.endsWith('/record'))return send({items:[],total:0});
    if(path==='/api/v1/publication-batch'&&request.method()==='POST'){
      if(saveFailure)return reject('保存被拒绝，原配置保留');
      saved={...source(),...input};return send(saved);
    }
    if(path==='/api/v1/publication-batch')return send({items:saved?[saved]:[],total:saved?1:0});
    return send([]);
  });
  const button=name=>page.getByRole('button',{name,exact:true});
  const field=name=>page.locator(`[aria-label="${name}"] input,input[aria-label="${name}"]`).first();
  const content=()=>page.getByRole('region',{name:'内容配置',exact:true});
  async function confirm(){await button('确认资源').click();await page.getByRole('dialog',{name:'确认资源',exact:true}).waitFor();await button('进入内容配置').click();await content().waitFor();}
  async function verifyLocked(){await content().waitFor();assert.equal(await button('确认资源').count(),0);assert.equal(await button('上一步').count(),0);assert.equal(await button('返回内容配置').count(),0);assert.equal(await content().locator('.batch-content-more').count(),0);}
  async function close(){assert.deepEqual(errors,[]);assert.equal(calls.some(c=>c.path==='/api/v1/publication'||c.path.endsWith('/start')),false,'wizard must never create tasks or start publishing');await context.close();}
  return {page,button,field,content,confirm,verifyLocked,close,calls,source,
    validationDelay:()=>{let resolve;validationGate=new Promise(r=>resolve=r);return resolve;},saved:()=>saved};
}

try {
  // New input: invalid scope stays editable; confirmation can cancel; target mismatch is advisory.
  {
    const s=await scene();const {page,button,field}=s;
    await page.goto(base+'/publish-batches?view=create');
    await button('确认资源').click();await page.getByText('请填写批次名称',{exact:true}).waitFor();
    await field('批次名称').fill('新建确认边界');
    assert.equal(await page.getByText('请填写批次名称',{exact:true}).count(),0,'corrected name must clear its obsolete validation error');
    await button('确认资源').click();
    await page.getByText('请至少添加一个目标类目',{exact:true}).waitFor();
    await page.locator('.batch-category-chip').filter({hasText:category.name}).click();
    await field(category.name+'发布目标').fill('');await field(category.name+'发布目标').press('Tab');
    await button('确认资源').click();await page.getByText('发布目标须为非负整数',{exact:true}).waitFor();
    await field(category.name+'发布目标').fill('2');await field('目标数量').fill('3');
    await button('重新自动匹配').click();await page.getByText('已配对 1 条候选',{exact:true}).waitFor();
    await button('确认资源').click();const modal=page.getByRole('dialog',{name:'确认资源',exact:true});
    await modal.getByText('目标缺口 1 条；可先保存待完善配置。',{exact:true}).waitFor();
    assert.equal(s.calls.some(c=>c.path==='/api/v1/publication-batch'&&c.method==='POST'),false);
    await button('继续调整').click();assert.equal(await field('批次名称').inputValue(),'新建确认边界');
    await s.confirm();await s.verifyLocked();
    await button('保存草稿').click();await page.getByText('已保存草稿，可在本机继续编辑',{exact:true}).waitFor();
    await page.reload();await s.verifyLocked();
    assert.equal(await s.content().getByText('原有人工文案',{exact:true}).count(),1);
    await button('下一步').click();await field('异常任务上限').fill('3');
    await button('返回内容配置').click();await s.verifyLocked();
    await page.setViewportSize({width:390,height:844});
    assert.equal(await page.evaluate(()=>document.documentElement.scrollWidth>innerWidth),false);
    await s.close();console.log('PASS new scope/confirmation cancel/shortfall/native draft recovery/locked content/narrow viewport');
  }
  for(const step of [1,2]){
    const s=await scene({draft:{form:form(),step}});await s.page.goto(base+'/publish-batches?view=create');
    if(step===2)await s.button('返回内容配置').click();await s.verifyLocked();await s.close();
    console.log('PASS legacy draft step '+step+' inherits confirmed resources');
  }
  {
    const f=form();f.config.candidates=[];f.execution_setting.exception_task_limit=null;
    const s=await scene({draft:{form:f,step:0},inventoryFailure:true});await s.page.goto(base+'/publish-batches?view=create');
    await s.button('采用系统推荐').click();await s.page.getByText('资源盘点尚未核实完整，请重新盘点后再推荐。原目标和候选保持不变。',{exact:true}).waitFor();
    assert.equal(await s.field(category.name+'发布目标').inputValue(),'2');
    await s.confirm();await s.page.getByText('暂无候选，可先保存批次，再从继续配置补充资源',{exact:true}).waitFor();
    await s.button('下一步').click();
    await s.page.getByRole('region',{name:'发布内容',exact:true}).getByText('暂无候选，可先保存批次，再从继续配置补充资源',{exact:true}).waitFor();
    await s.page.getByText('草稿仅保存在本机，保存批次后进入批次详情。',{exact:true}).waitFor();
    await s.page.getByText('（必填）',{exact:true}).waitFor();
    await s.button('保存批次').click();
    await s.page.getByText('请填写异常任务上限（正整数）',{exact:true}).first().waitFor();
    assert.equal(s.calls.some(c=>c.path==='/api/v1/publication-batch'&&c.method==='POST'),false);
    await s.field('异常任务上限').fill('5');
    await s.button('保存批次').click();
    await s.page.getByRole('heading',{name:f.name,exact:true}).waitFor();
    assert.equal(s.saved().targets[0].target_count,2);assert.equal(s.saved().config.candidates.length,0);
    await s.close();console.log('PASS inventory failure/zero candidates/incomplete save preserves original target and no task side effects');
  }
  {
    const s=await scene({draft:{form:form(),step:2},saveFailure:true});await s.page.goto(base+'/publish-batches?view=create');
    await s.button('保存批次').click();await s.page.getByText('保存被拒绝，原配置保留',{exact:true}).waitFor();
    await s.button('返回内容配置').click();await s.verifyLocked();
    await s.page.evaluate(()=>sessionStorage.setItem('wizard-write-failure','1'));
    await s.button('保存草稿').click();await s.page.getByText('本机草稿写入失败',{exact:true}).waitFor();
    await s.content().getByText('原有人工文案',{exact:true}).waitFor();
    await s.close();console.log('PASS rejected save/native draft write failure preserve locked stage and input');
  }
  {
    const s=await scene({conflict:true});await s.page.goto(base+'/publish-batches?view=edit&id=5');
    await s.field('批次名称').fill('冲突前的人工修改');await s.confirm();
    await s.button('下一步').click();await s.button('保存修改').click();
    await s.page.getByText('批次已被修改，请重新读取',{exact:true}).waitFor();
    assert.equal(await s.field('复查批次名称').inputValue(),'冲突前的人工修改');
    await s.button('返回内容配置').click();await s.verifyLocked();await s.close();
    console.log('PASS saved batch opens editable resources; revision conflict retains current name/content');
  }
  {
    const s=await scene({draft:{form:form(),step:1}});await s.page.goto(base+'/publish-batches?view=create');await s.verifyLocked();
    const release=s.validationDelay(), requested=s.page.waitForRequest(r=>r.url().endsWith('/publication-batch/validate'));
    await s.button('下一步').click();await requested;
    await s.field('并发数').fill('4');await s.field('并发数').press('Tab');release();
    await s.page.waitForLoadState('networkidle');
    assert.equal(await s.page.getByText('迟到的校验结果',{exact:true}).count(),0,'changed input must discard obsolete review');
    await s.close();console.log('PASS late review cannot overwrite changed execution input');
  }
  {
    const s=await scene({native:false});await s.page.goto(base+'/publish-batches?view=create');await s.field('批次名称').fill('Web刷新前输入');
    await s.page.reload();assert.equal(await s.field('批次名称').inputValue(),'');await s.close();
    console.log('PASS Web refresh does not masquerade as persistent Desktop draft');
  }
} finally { await browser.close(); }
