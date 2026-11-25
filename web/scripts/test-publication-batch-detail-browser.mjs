// Production batch detail, synthetic read-only Cloud facts; never posts or starts tasks.
import assert from 'node:assert/strict';
import { chromium } from 'playwright';
const base=process.env.WT_MEDIA_BROWSER_BASE_URL||'http://127.0.0.1:5173';
const browser=await chromium.launch({channel:process.env.WT_MEDIA_BROWSER_CHANNEL||'chrome',headless:true});
const longName='原型批次详情长成片名称，用于验证完整内容可读、两行收敛和容器不被撑开。'.repeat(5);
const complete={description:'已核对的发布文案',cover_file_id:'31',portrait_cover_file_id:'32',declaration:'original',confirmed:true};
const row=i=>({row_key:'row-'+i,category_id:'4',media_account_id:'11',composite_output_id:String(41+i),input_revision:'1',field_revisions:{},order:i,content:{...complete,description:complete.description+i}});
const scenes=[
  {key:'cancelled-empty',state:'cancelled',rows:[],target:60},
  {key:'not-started-empty',state:'not_started',rows:[],target:60},
  {key:'unknown-config',state:'not_started',rows:null,target:60},
  {key:'incomplete',state:'not_started',rows:[{...row(0),content:{description:'人工未完成文案'}}],target:2},
  {key:'populated',state:'not_started',rows:Array.from({length:13},(_,i)=>row(i)),target:20},
  {key:'multi-category',state:'not_started',rows:[row(0)],target:20,multi:true},
  {key:'occupied',state:'not_started',rows:[row(0)],target:2,occupied:true},
  {key:'catalog-failure',state:'not_started',rows:[row(0)],target:2,catalogFailure:true},
  ...['running','paused','stopping','ended'].map(state=>({key:state,state,rows:[row(0)],target:2,started:true})),
];
try {
  for(const scene of scenes){
    const context=await browser.newContext({viewport:{width:1440,height:1000}}),page=await context.newPage(),errors=[],calls=[];
    page.setDefaultTimeout(6000);page.on('pageerror',e=>errors.push(e.message));
    const batch={id:'1',revision:'3',name:'详情状态 '+scene.key,team_id:'7',operator_id:'9',created_by:'9',platform:'baidu',
      execution_status:scene.state,configuration_status:scene.key==='populated'?'ready':'incomplete',allowed_actions:[],
      targets:[{category_id:'4',target_count:scene.target},...(scene.multi?[{category_id:'5',target_count:3}]:[])],
      config:scene.rows===null?null:{accounts:[{media_account_id:'11',limit:20}],output_ids:scene.rows.map(r=>r.composite_output_id),manual_pairs:[],candidates:scene.rows},
      candidate_count:scene.rows?.length??3,
      readiness:{ready:scene.key==='populated',issues:[],shortfalls:[{category_id:'4',target_count:scene.target,candidate_count:scene.rows?.length||0,shortfall:scene.target-(scene.rows?.length||0)}]},
      execution_setting:{concurrency_limit:5,group_interval_seconds:0,between_group_policy:'auto_continue',exception_task_limit:5},
      execution:{phase:scene.state,first_started_at:scene.started?'2026-11-25T10:00:00Z':null,stopped_at:scene.state==='cancelled'?'2026-11-25T10:00:00Z':null,
        stop_reason:scene.state==='cancelled'?'验收记录取消':null,target_count:scene.target,created_count:scene.started?1:0,published_count:scene.state==='ended'?1:0,creation_shortfall:scene.started?1:null,target_shortfall:scene.target-1,task_counts:scene.started?{[scene.state==='ended'?'published':'running']:1}:{},verification_required_count:0}};
    await page.route('**/api/v1/**',async route=>{
      const req=route.request(),path=new URL(req.url()).pathname;calls.push({path,method:req.method()});
      assert.equal(req.method(),'GET','detail views may only read facts');
      const send=data=>route.fulfill({json:{errcode:0,data,message:'success',logid:'detail-test'}});
      if(path==='/api/v1/auth/me')return send({id:'9',role:'operator',team_id:'7',status:'enabled',category_ids:['4','5']});
      if(path==='/api/v1/categories')return send([{id:'4',name:'三角洲',enabled:true},{id:'5',name:'火影忍者',enabled:true}]);
      if(path==='/api/v1/publication-batch/1')return send(batch);
      if(path==='/api/v1/media-accounts'||path==='/api/v1/publication/account')return send({items:[{id:'11',name:'本人真实账号',platform:'baidu',enabled:true,eligible:true,category_ids:['4']}],total:1});
      if(path==='/api/v1/publication/output'){
        if(scene.catalogFailure)return route.abort('failed');
        const outputs=(scene.rows||[]).map(r=>({id:r.composite_output_id,composite_output_id:r.composite_output_id,name:longName,category_id:'4',team_id:'7',platforms:['baidu'],occupied:!!scene.occupied,video:{status:'ready'},media:{duration_ms:30000}}));
        return send({items:outputs,total:outputs.length});
      }
      if(path.endsWith('/record'))return send({items:[{id:'1',event:'publication_batch.group_assigned',actor_id:'9',created_at:'2026-11-25T10:00:00Z'},{id:'2',event:'publication_batch.ended',created_at:'2026-11-25T10:00:00Z'}],total:2});
      if(path.endsWith('/task'))return send({items:[],total:0});
      return send({items:[],total:0});
    });
    await page.goto(base+'/publish-batches?view=detail&id=1');
    await page.getByRole('heading',{name:batch.name,exact:true}).waitFor();
    const candidateSection=page.getByRole('region',{name:'候选配置',exact:true});
    if(scene.started){
      assert.equal(await candidateSection.count(),0,'started batches use formal tasks, never candidate previews');
      await page.getByRole('heading',{name:'执行情况',exact:true}).waitFor();
      assert.equal(await page.getByRole('heading',{name:'执行设置',exact:true}).count(),0,'execution settings must appear once');
      await page.getByRole('heading',{name:'批次任务',exact:true}).waitFor();
    }else{
      await candidateSection.waitFor();
      assert.equal(await page.getByRole('heading',{name:'执行情况',exact:true}).count(),0,'a never-started cancelled batch must not masquerade as an execution');
      assert.equal(await page.getByRole('heading',{name:'执行设置',exact:true}).count(),1);
      assert.equal(await page.getByRole('heading',{name:'目标与缺口',exact:true}).count(),0,'single-category target facts must not be duplicated in a second section');
      if(scene.rows===null){
        await candidateSection.getByText('内容配置暂时无法读取',{exact:true}).waitFor();
        assert.equal(await candidateSection.locator('.resource-data-table table').count(),0);
        await candidateSection.getByText('待刷新',{exact:true}).first().waitFor();
      }else if(!scene.rows.length){
        await candidateSection.getByRole('heading',{name:'尚未配置发布内容',exact:true}).waitFor();
        assert.equal(await candidateSection.locator('.resource-data-table table').count(),0,'empty content must explain itself, not leave a bare table header');
        if(scene.state==='cancelled')await page.getByText('此批次在开始发布前已取消。',{exact:true}).waitFor();
      }else{
        const content=page.getByRole('region',{name:'批次候选配置，可横向滚动'});
        assert.deepEqual(await content.locator('thead th').allTextContents(),['发布内容','发布账号','发布成片','配置状态']);
        for(const name of ['编辑','调整配对','移除','执行'])assert.equal(await content.getByRole('button',{name,exact:true}).count(),0);
        await content.getByText(scene.rows[0].content.description,{exact:true}).waitFor();
        await content.getByText('本人真实账号',{exact:true}).first().waitFor();
        if(!scene.catalogFailure){
          const title=content.locator('.batch-readonly-output').first();assert.equal(await title.getAttribute('title'),longName);
          const geometry=await title.evaluate(e=>({height:e.getBoundingClientRect().height,line:parseFloat(getComputedStyle(e).lineHeight)}));
          assert.ok(geometry.height<=2*geometry.line+1);
        }
        if(scene.key==='populated'){
          assert.equal(await content.locator('tbody tr').count(),10);await content.getByText('2',{exact:true}).click();
          await content.getByText(scene.rows[10].content.description,{exact:true}).waitFor();assert.equal(await content.locator('tbody tr').count(),3);
        }
        if(scene.occupied)await content.getByText('成片已被当前平台任务占用',{exact:true}).waitFor();
        if(scene.catalogFailure)await content.getByText('成片信息暂不可用',{exact:true}).waitFor();
        if(scene.multi)await candidateSection.getByRole('columnheader',{name:'内容类目',exact:true}).waitFor();
      }
    }
    await page.getByText('安排本组任务',{exact:true}).waitFor();
    await page.getByText('批次执行结束',{exact:true}).waitFor();
    await page.setViewportSize({width:390,height:844});
    assert.equal(await page.evaluate(()=>document.documentElement.scrollWidth>innerWidth),false);
    assert.deepEqual(errors,[]);assert.equal(calls.some(c=>c.method!=='GET'),false);
    await context.close();console.log('PASS batch detail '+scene.key);
  }
}finally{await browser.close();}
