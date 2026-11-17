import { it, expect } from 'vitest';
import { seed, defaults } from '../model.js';
import { initCandidate, editField } from '../content.js';
import { coverReadinessIssues, coverSignature, defaultDocument } from '../covers.js';
import { contentState, confirmContent } from '../batchContentActions.js';
import { prepareBatchConfiguration, applyBatchConfiguration, applicableCopyFields, configurationCopyFields, editConfigurationField, configurationIssues, updateConfigurationCover, regenerateConfigurationField } from '../batchConfiguration.js';

function fixture() {
  const db=seed(); db.tasks=[];
  const rows=db.assets.slice(0,2).map((v,i)=>initCandidate({...defaults('B站',v), candidateId:`bulk-${i}`,platform:'B站',assetId:v.id,category:v.category,accountId:db.accounts.find(a=>a.platform==='B站'&&a.categories.includes(v.category)).id,declaration:'内容无需标注',contentApproved:true,coverApproved:true}));
  return {db,rows};
}
const inspect=async()=>({width:1200,height:900});
it('direct source images can be confirmed without rendering records',()=>{
  const {db,rows}=fixture(); const row=rows[0]; row.contentApproved=false;row.coverApproved=false;
  expect(contentState(row,db)).toBe('待确认');
  expect(confirmContent(row,db)).toEqual({ok:true,errors:[]});
  expect(row.coverVariants).toBeUndefined();expect(contentState(row,db)).toBe('已就绪');
});
it('stale template outputs are incomplete, and confirmation never renders them',()=>{
  const {db,rows}=fixture(); const row=rows[0];Object.assign(row,{coverMode:'模板合成',shortTitle:'封面',templateDocument:defaultDocument(),coverSignature:null,coverVariants:{}});
  const before=JSON.stringify(row);expect(contentState(row,db)).toBe('待完善');expect(confirmContent(row,db).ok).toBe(false);expect(JSON.stringify(row)).toBe(before);
});
it('previewing cover-only changes preserves source rows and published copy',async()=>{
  const {db,rows}=fixture();const before=JSON.stringify(rows);const image=db.images.find(i=>i.enabled&&i.uses.includes('现成封面'));
  const results=await prepareBatchConfiguration(rows,[rows[0].candidateId],{sections:['cover'],cover:{mode:'image',imageId:image.id}},db,{inspect});
  expect(JSON.stringify(rows)).toBe(before);expect(results[0].error).toBe('');expect(results[0].row.title).toBe(rows[0].title);
  expect(applyBatchConfiguration(rows,results).changed).toBe(1);
  expect(rows[0].cover).toBe(image.src);expect(rows[0].title).toBe(JSON.parse(before)[0].title);expect(rows[0].description).toBe(JSON.parse(before)[0].description);
  expect(rows[0].contentApproved).toBe(false);expect(rows[0].coverApproved).toBe(false);expect(JSON.stringify(rows[1])).toBe(JSON.stringify(JSON.parse(before)[1]));
  expect(contentState(rows[0],db)).toBe('待确认');
});
it('AI copy fills blanks by default and ignores unselected common and cover settings',async()=>{
  const {db,rows}=fixture();const title=rows[0].title,cover=rows[0].cover;
  const results=await prepareBatchConfiguration(rows,[rows[0].candidateId],{sections:['copy'],common:{visibility:'仅自己可见'},copy:{fields:['title','description'],style:'简洁',overwrite:false},cover:{mode:'image',imageId:999}},db,{inspect});
  expect(results[0].error).toBe('');applyBatchConfiguration(rows,results);
  expect(rows[0].title).toBe(title);expect(rows[0].description).toBeTruthy();expect(rows[0].cover).toBe(cover);expect(rows[0].visibility).toBe('公开');
});
it('no-op edits preserve approval and applying twice never repeats a change',async()=>{
  const {db,rows}=fixture();const before=JSON.stringify(rows[0]);
  const results=await prepareBatchConfiguration(rows,[rows[0].candidateId],{sections:['common'],common:{visibility:'公开'}},db,{inspect});
  expect(applyBatchConfiguration(rows,results).changed).toBe(0);expect(JSON.stringify(rows[0])).toBe(before);
  const next=await prepareBatchConfiguration(rows,[rows[0].candidateId],{sections:['common'],common:{visibility:'仅自己可见'}},db,{inspect});
  expect(applyBatchConfiguration(rows,next).changed).toBe(1);const saved=JSON.stringify(rows[0]);expect(applyBatchConfiguration(rows,next).changed).toBe(0);expect(JSON.stringify(rows[0])).toBe(saved);
});
it('failed previews leave originals intact and stale or removed rows cannot be overwritten',async()=>{
  const {db,rows}=fixture();const original=JSON.stringify(rows[0]);
  const bad=await prepareBatchConfiguration(rows,[rows[0].candidateId],{sections:['cover'],cover:{mode:'image',imageId:999}},db,{inspect});
  expect(bad[0].error).toBeTruthy();expect(applyBatchConfiguration(rows,bad).changed).toBe(0);expect(JSON.stringify(rows[0])).toBe(original);
  const previews=await prepareBatchConfiguration(rows,rows.map(r=>r.candidateId),{sections:['common'],common:{visibility:'仅自己可见'}},db,{inspect});
  editField(rows[0],'title','预览之后的人工修改');rows.pop();const result=applyBatchConfiguration(rows,previews);
  expect(result.changed).toBe(0);expect(previews.every(r=>r.error)).toBe(true);expect(rows[0].title).toBe('预览之后的人工修改');expect(rows[0].visibility).toBe('公开');
});
it('template generation failures are per-item and never apply an old cover as a new result',async()=>{
  const {db,rows}=fixture();const before=JSON.stringify(rows);db.templates=[{id:9001,enabled:true,version:1,document:defaultDocument()},{id:9002,enabled:true,version:1,document:defaultDocument(1600,900)}];
  const results=await prepareBatchConfiguration(rows,rows.map(r=>r.candidateId),{sections:['cover'],cover:{mode:'template',templateIds:{'4:3':9001,'16:9':9002},textPolicy:'ai'}},db,{inspect,render:async(row)=>{if(row.candidateId==='bulk-1')throw Error('图片读取失败');row.cover='new-cover';row.coverVariants={'4:3':'new-cover'};row.coverSignature=coverSignature(row);}});
  expect(results[0].error).toBe('');expect(results[1].error).toBe('图片读取失败');expect(JSON.stringify(rows)).toBe(before);
  expect(applyBatchConfiguration(rows,results).changed).toBe(1);expect(rows[1].cover).toBe(JSON.parse(before)[1].cover);
});
it('never substitutes an unrelated template when selected template has the wrong platform ratio',async()=>{
  const {db,rows}=fixture();const row=rows[0];row.platform='小红书';db.templates=[{id:9001,enabled:true,document:defaultDocument()},{id:9002,enabled:true,document:defaultDocument(1080,1440)}];
  const results=await prepareBatchConfiguration(rows,[row.candidateId],{sections:['cover'],cover:{mode:'template',templateId:9001,textPolicy:'ai'}},db,{inspect,render:async()=>{}});
  expect(results[0].error).toBeTruthy();expect(results[0].row.templateSnapshots?.['3:4']?.id).not.toBe(9002);
});
it('requires every selected output template text slot and fills cover text without changing publication copy',async()=>{
  const {db,rows}=fixture();const wide=defaultDocument(1600,900);wide.layers=[{...wide.layers[0],id:'subtitle',slot:'sub_title',name:'副标题'}];
  db.templates=[{id:9001,enabled:true,version:1,document:defaultDocument()},{id:9002,enabled:true,version:1,document:wide}];
  rows[0].shortTitle='已有封面主标题';const config={sections:['cover'],cover:{mode:'template',templateIds:{'4:3':9001,'16:9':9002},textPolicy:'keep'}};
  const before=JSON.stringify(rows[0]);const missing=await prepareBatchConfiguration(rows,[rows[0].candidateId],config,db,{inspect,render:async()=>{}});expect(missing[0].error).toContain('副标题');
  config.cover.textPolicy='ai';const generated=await prepareBatchConfiguration(rows,[rows[0].candidateId],config,db,{inspect,render:async row=>{row.cover='rendered';row.coverSignature=coverSignature(row);row.coverVariants={'4:3':'rendered','16:9':'rendered'};}});
  expect(generated[0].error).toBe('');expect(generated[0].row.subTitle).toBeTruthy();expect(generated[0].row.shortTitle).toBe('已有封面主标题');expect(generated[0].row.title).toBe(rows[0].title);expect(JSON.stringify(rows[0])).toBe(before);
});

it('reports selected rows removed before preview instead of silently narrowing the scope',async()=>{
 const {db,rows}=fixture();const results=await prepareBatchConfiguration(rows,['removed',rows[0].candidateId],{sections:['common'],common:{visibility:'仅自己可见'}},db,{inspect});
 expect(results).toHaveLength(2);expect(results[0].error).toContain('已移出');expect(applyBatchConfiguration(rows,results)).toEqual({changed:1,unchanged:0,failed:1});
});

it('template confirmation requires every platform output, while native images need no rendered variants',()=>{
 const {db,rows}=fixture();const row=rows[0];Object.assign(row,{coverMode:'模板合成',shortTitle:'完整封面',templateDocument:defaultDocument(),coverVariants:{'4:3':row.cover}});row.coverSignature=coverSignature(row);
 expect(coverReadinessIssues(row)).not.toEqual([]);expect(contentState(row,db)).toBe('待完善');expect(confirmContent(row,db).ok).toBe(false);
 row.coverVariants['16:9']=row.cover;expect(coverReadinessIssues(row)).toEqual([]);expect(confirmContent(row,db).ok).toBe(true);
});

it('retains edited publication drafts when revisiting or changing unrelated configuration',async()=>{
 const {db,rows}=fixture();
 expect(applicableCopyFields(rows[0])).not.toContain('shortTitle');
 const config={sections:['copy'],copy:{fields:['title'],style:'简洁',overwrite:true}};
 const first=await prepareBatchConfiguration(rows,[rows[0].candidateId],config,db);
 expect(editConfigurationField(first[0],'title','人工审阅后的标题')).toBe(true);
 const same=await prepareBatchConfiguration(rows,[rows[0].candidateId],config,db,{previous:first});
 expect(same[0].row.title).toBe('人工审阅后的标题');
 const next=await prepareBatchConfiguration(rows,[rows[0].candidateId],{...config,sections:['common','copy'],common:{visibility:'仅自己可见'}},db,{previous:same});
 expect(next[0].row.title).toBe('人工审阅后的标题');expect(next[0].row.visibility).toBe('仅自己可见');
 expect(applyBatchConfiguration(rows,next).changed).toBe(1);expect(rows[0].title).toBe('人工审阅后的标题');
});
it('edits only selected generated fields and blocks empty titles without losing input',async()=>{
 const {db,rows}=fixture();const config={sections:['copy'],copy:{fields:['title'],overwrite:true}};
 const [item]=await prepareBatchConfiguration(rows,[rows[0].candidateId],config,db);
 expect(editConfigurationField(item,'accountId',999)).toBe(false);
 expect(editConfigurationField(item,'title','')).toBe(true);expect(item.row.title).toBe('');
 expect(configurationIssues(item).join()).toContain('发布标题');expect(applyBatchConfiguration(rows,[item]).changed).toBe(0);
 editConfigurationField(item,'title','修复标题');expect(configurationIssues(item)).toEqual([]);expect(applyBatchConfiguration(rows,[item]).changed).toBe(1);
});
it('text edits invalidate template previews, update only that cover, and render failures preserve manual copy',async()=>{
 const {db,rows}=fixture();db.templates=[{id:9001,enabled:true,document:defaultDocument()},{id:9002,enabled:true,document:defaultDocument(1600,900)}];
 const render=async row=>{row.cover='rendered:'+row.shortTitle;row.coverVariants={'4:3':row.cover,'16:9':row.cover};row.coverSignature=coverSignature(row);row.coverError='';};
 const config={sections:['copy','cover'],copy:{fields:['shortTitle'],overwrite:true},cover:{mode:'template',templateIds:{'4:3':9001,'16:9':9002},textPolicy:'keep'}};
 const results=await prepareBatchConfiguration(rows,rows.map(r=>r.candidateId),config,db,{render});const original=JSON.stringify(rows),second=JSON.stringify(results[1]);
 editConfigurationField(results[0],'shortTitle','人工封面标题');
 expect(configurationIssues(results[0]).join()).toContain('更新封面');expect(results[0].previewCovers['4:3']).toBeTruthy();
 expect(await updateConfigurationCover(results[0],{render:async()=>{throw Error('合成失败')}})).toBe(false);expect(results[0].row.shortTitle).toBe('人工封面标题');
 expect(await updateConfigurationCover(results[0],{render})).toBe(true);expect(configurationIssues(results[0])).toEqual([]);
 expect(results[0].row.cover).toBe('rendered:人工封面标题');expect(JSON.stringify(results[1])).toBe(second);expect(JSON.stringify(rows)).toBe(original);
 expect(applyBatchConfiguration(rows,results).changed).toBe(2);
});
it('regenerates a single field only after explicit permission to replace manual text',async()=>{
 const {db,rows}=fixture();const [item]=await prepareBatchConfiguration(rows,[rows[0].candidateId],{sections:['copy'],copy:{fields:['title','description'],overwrite:true}},db);
 editConfigurationField(item,'title','手改');const description=item.row.description;
 expect(regenerateConfigurationField(item,'title')).toBe(false);expect(item.row.title).toBe('手改');
 expect(regenerateConfigurationField(item,'title',{replaceManual:true})).toBe(true);expect(item.row.title).not.toBe('手改');expect(item.row.description).toBe(description);
});
it('changing templates preserves manual text and removing a section discards only that section preview',async()=>{
 const {db,rows}=fixture();db.templates=[{id:9001,enabled:true,document:defaultDocument()},{id:9002,enabled:true,document:defaultDocument(1600,900)}];
 Object.assign(rows[0],{coverMode:'模板合成',templateDocument:defaultDocument()});
 const firstConfig={sections:['common','copy'],common:{visibility:'仅自己可见'},copy:{fields:['title','shortTitle'],overwrite:true}};
 const first=await prepareBatchConfiguration(rows,[rows[0].candidateId],firstConfig,db);editConfigurationField(first[0],'shortTitle','手改封面');
 const next=await prepareBatchConfiguration(rows,[rows[0].candidateId],{...firstConfig,sections:['copy','cover'],cover:{mode:'template',templateIds:{'4:3':9001,'16:9':9002},textPolicy:'keep'}},db,{previous:first,render:async row=>{row.coverVariants={'4:3':'x','16:9':'x'};row.coverSignature=coverSignature(row);}});
 expect(next[0].row.visibility).toBe(rows[0].visibility);expect(next[0].row.shortTitle).toBe('手改封面');expect(configurationIssues(next[0])).toEqual([]);
});

it('generates copy and template short title once, keeps unselected manual copy, and does not regenerate unchanged review results',async()=>{
 const {db,rows}=fixture();db.templates=[{id:9001,enabled:true,document:defaultDocument()},{id:9002,enabled:true,document:defaultDocument(1600,900)}];
 rows[0].description='保留人工正文';let calls=0,renders=0;
 const generate=()=>{calls++;return {shortTitle:'一次生成的短标题'};};
 const render=async row=>{renders++;row.coverVariants={'4:3':'new','16:9':'new'};row.coverSignature=coverSignature(row);};
 const config={sections:['copy','cover'],copy:{fields:['shortTitle'],overwrite:true},cover:{mode:'template',templateIds:{'4:3':9001,'16:9':9002},textPolicy:'ai',overwriteText:true}};
 const first=await prepareBatchConfiguration(rows,[rows[0].candidateId],config,db,{generate,render});
 expect(first[0].row.shortTitle).toBe('一次生成的短标题');expect(calls).toBe(1);expect(first[0].row.description).toBe('保留人工正文');
 const same=await prepareBatchConfiguration(rows,[rows[0].candidateId],config,db,{generate,render,previous:first});expect(calls).toBe(1);expect(renders).toBe(1);expect(same[0].row.shortTitle).toBe(first[0].row.shortTitle);
});
it('does not write an obsolete cover response when the text changes during rendering',async()=>{
 const {db,rows}=fixture();db.templates=[{id:9001,enabled:true,document:defaultDocument()},{id:9002,enabled:true,document:defaultDocument(1600,900)}];
 const [item]=await prepareBatchConfiguration(rows,[rows[0].candidateId],{sections:['copy','cover'],copy:{fields:['shortTitle'],overwrite:true},cover:{mode:'template',templateIds:{'4:3':9001,'16:9':9002}}},db,{render:async row=>{row.coverVariants={'4:3':'initial','16:9':'initial'};row.coverSignature=coverSignature(row);}});
 editConfigurationField(item,'shortTitle','第一次修改');let release;const gate=new Promise(resolve=>release=resolve);
 const pending=updateConfigurationCover(item,{render:async row=>{await gate;row.cover='obsolete';row.coverVariants={'4:3':'obsolete','16:9':'obsolete'};row.coverSignature=coverSignature(row);}});
 editConfigurationField(item,'shortTitle','第二次修改');release();expect(await pending).toBe(false);expect(item.row.shortTitle).toBe('第二次修改');expect(item.row.cover).not.toBe('obsolete');expect(item.coverNeedsUpdate).toBe(true);
});
it('can repair a failed template render without losing another item or applying stale output',async()=>{
 const {db,rows}=fixture();db.templates=[{id:9001,enabled:true,document:defaultDocument()},{id:9002,enabled:true,document:defaultDocument(1600,900)}];
 const results=await prepareBatchConfiguration(rows,rows.map(row=>row.candidateId),{sections:['copy','cover'],copy:{fields:['shortTitle'],overwrite:true},cover:{mode:'template',templateIds:{'4:3':9001,'16:9':9002}}},db,{render:async row=>{if(row.candidateId===rows[0].candidateId)throw Error('图片读取失败');row.coverVariants={'4:3':'ok','16:9':'ok'};row.coverSignature=coverSignature(row);}});
 expect(results[0].errorStage).toBe('cover');editConfigurationField(results[0],'shortTitle','失败后的人工修订');expect(applyBatchConfiguration(rows,results)).toEqual({changed:1,unchanged:0,failed:1});
 expect(await updateConfigurationCover(results[0],{render:async row=>{row.cover='repaired';row.coverVariants={'4:3':'repaired','16:9':'repaired'};row.coverSignature=coverSignature(row);row.coverError='';}})).toBe(true);
 expect(applyBatchConfiguration(rows,results)).toEqual({changed:1,unchanged:0,failed:0});expect(rows[0].shortTitle).toBe('失败后的人工修订');
});

it('retains manual review text across a snapshot conflict and rebases only with explicit permission',async()=>{
 const {db,rows}=fixture();const config={sections:['copy'],copy:{fields:['title'],overwrite:true}};
 const first=await prepareBatchConfiguration(rows,[rows[0].candidateId],config,db);editConfigurationField(first[0],'title','冲突前的人工标题');
 editField(rows[0],'visibility','仅自己可见');expect(applyBatchConfiguration(rows,first).failed).toBe(1);
 const retry=await prepareBatchConfiguration(rows,[rows[0].candidateId],config,db,{previous:first});
 expect(retry[0].row.title).toBe('冲突前的人工标题');expect(retry[0].errorStage).toBe('conflict');expect(applyBatchConfiguration(rows,retry).changed).toBe(0);
 const rebased=await prepareBatchConfiguration(rows,[rows[0].candidateId],config,db,{previous:retry,rebase:true});
 expect(rebased[0].row.title).toBe('冲突前的人工标题');expect(rebased[0].manualFields).toContain('title');expect(rebased[0].row.visibility).toBe('仅自己可见');expect(applyBatchConfiguration(rows,rebased).changed).toBe(1);
});
it('adjusts remaining failed entries after partial application and keeps applied entries permanently unchanged',async()=>{
 const {db,rows}=fixture();const config={sections:['copy'],copy:{fields:['title'],overwrite:true}};
 const first=await prepareBatchConfiguration(rows,rows.map(row=>row.candidateId),config,db);editConfigurationField(first[1],'title','');
 expect(applyBatchConfiguration(rows,first)).toEqual({changed:1,unchanged:0,failed:1});const saved=JSON.stringify(rows[0]);
 const next=await prepareBatchConfiguration(rows,rows.map(row=>row.candidateId),{...config,sections:['common','copy'],common:{visibility:'仅自己可见'}},db,{previous:first});
 expect(next[0].applied).toBe(true);expect(next[0]).toEqual(first[0]);editConfigurationField(next[1],'title','修复剩余项');expect(applyBatchConfiguration(rows,next)).toEqual({changed:1,unchanged:0,failed:0});expect(JSON.stringify(rows[0])).toBe(saved);
});

it('only exposes text slots used by the effective template across output ratios',()=>{
 const {db,rows}=fixture();const row=rows[0];
 const main=defaultDocument(),sub=defaultDocument(1600,900);sub.layers=[{...sub.layers[0],id:'subtitle',slot:'sub_title'}];
 db.templates=[{id:9001,enabled:true,document:main},{id:9002,enabled:true,document:sub},{id:9003,enabled:true,document:{...main,layers:[]}}];
 const config={sections:['cover','copy'],cover:{mode:'template',templateIds:{'4:3':9001,'16:9':9002}}};
 expect(configurationCopyFields(row,config,db)).toEqual(['title','description','tags','shortTitle','subTitle']);
 config.cover.templateIds={'4:3':9003,'16:9':9002};
 expect(configurationCopyFields(row,config,db)).toEqual(['title','description','tags','subTitle']);
 config.cover.templateIds={'4:3':9003,'16:9':9003};
 expect(configurationCopyFields(row,config,db)).toEqual(['title','description','tags']);
 Object.assign(row,{coverMode:'模板合成',templateDocument:main,templateDocuments:{'16:9':sub}});
 expect(applicableCopyFields(row)).toContain('subTitle');
 for(const mode of ['source','image']) {
  config.cover.mode=mode;expect(configurationCopyFields(row,config,db)).toEqual(['title','description','tags']);
 }
 config.sections=['copy'];expect(configurationCopyFields(row,config,db)).toContain('shortTitle');
 row.coverMode='现成封面';expect(configurationCopyFields(row,config,db)).toEqual(['title','description','tags']);
});
it('ignores stale cover text selections on image covers without deleting stored text',async()=>{
 const {db,rows}=fixture();const row=rows[0];row.shortTitle='原有短标题';row.subTitle='原有副标题';row.templateDocument=defaultDocument();
 const image=db.images.find(image=>image.enabled&&image.uses.includes('现成封面'));
 for(const mode of ['source','image']) {
  const [item]=await prepareBatchConfiguration(rows,[row.candidateId],{sections:['copy','cover'],copy:{fields:['title','shortTitle','subTitle'],overwrite:true},cover:{mode,imageId:image.id}},db,{inspect});
  expect(item.error).toBe('');expect(item.editableFields).toEqual(['title']);expect(item.row.shortTitle).toBe('原有短标题');expect(item.row.subTitle).toBe('原有副标题');expect(item.coverNeedsUpdate).toBe(false);
  expect(editConfigurationField(item,'shortTitle','不能修改')).toBe(false);
 }
});
it('switching a composed preview back to a source image removes text editing and stale cover warnings',async()=>{
 const {db,rows}=fixture();const row=rows[0];row.shortTitle='原存文字';
 db.templates=[{id:9001,enabled:true,document:defaultDocument()},{id:9002,enabled:true,document:defaultDocument(1600,900)}];
 const config={sections:['copy','cover'],copy:{fields:['shortTitle'],overwrite:true},cover:{mode:'template',templateIds:{'4:3':9001,'16:9':9002}}};
 const first=await prepareBatchConfiguration(rows,[row.candidateId],config,db,{render:async()=>{}});editConfigurationField(first[0],'shortTitle','本轮人工文字');
 const [next]=await prepareBatchConfiguration(rows,[row.candidateId],{...config,cover:{mode:'source'}},db,{inspect,previous:first});
 expect(next.error).toBe('');expect(next.editableFields).toEqual([]);expect(next.row.shortTitle).toBe('原存文字');expect(next.coverNeedsUpdate).toBe(false);expect(configurationIssues(next)).toEqual([]);
});
it('generating a subtitle on an existing template requires the cover preview to update',async()=>{
 const {db,rows}=fixture();const row=rows[0],doc=defaultDocument();doc.layers=[{...doc.layers[0],id:'subtitle',slot:'sub_title'}];Object.assign(row,{coverMode:'模板合成',templateDocument:doc,subTitle:'原副标题'});
 const [item]=await prepareBatchConfiguration(rows,[row.candidateId],{sections:['copy'],copy:{fields:['subTitle'],overwrite:true}},db);
 expect(item.editableFields).toEqual(['subTitle']);expect(item.coverNeedsUpdate).toBe(true);expect(configurationIssues(item).join()).toContain('更新封面');
});
