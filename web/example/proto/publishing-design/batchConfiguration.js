import { clone } from './relationships.js';
import { editField, demoValues } from './content.js';
import { coverSpecs, coverTextFields, coverTextFieldsForRow, coverTextErrors, ensureCoverResults, inspectCoverImage, isTemplateCover } from './covers.js';

export const copyFields = ['title', 'shortTitle', 'description', 'topics', 'tags'];
const publicationFields = row => row.platform === '百家号' ? ['description','tags'] : row.platform === 'B站' ? ['title','description','tags'] : ['title','description','topics'];
export const applicableCopyFields = row => [...publicationFields(row), ...(isTemplateCover(row) ? coverTextFieldsForRow(row).map(field=>field.key) : [])];
// Use the effective cover configuration, not stale template data on an image row.
export function configurationTextFields(row,config,db) {
  if(!config.sections?.includes('cover'))return isTemplateCover(row) ? coverTextFieldsForRow(row).map(field=>field.key) : [];
  if(config.cover?.mode!=='template')return [];
  const options=config.cover,specs=coverSpecs(row.platform);
  const ids=specs.length ? specs.map(spec=>options.templateIds?.[spec.key] || options.templateId) : [options.templateId];
  return [...new Set(ids.filter(Boolean).flatMap(id=>coverTextFields(db.templates.find(template=>template.id===id && template.enabled)?.document)).map(field=>field.key))];
}
export const configurationCopyFields = (row,config,db) => [...publicationFields(row),...configurationTextFields(row,config,db)];
const snapshot = row => JSON.stringify(row);
function change(row,key,value) {
  if (JSON.stringify(row[key]) === JSON.stringify(value)) return;
  editField(row,key,clone(value));
}
function templateCover(row,config,db,values,generated) {
  const specs=coverSpecs(row.platform), ids=config.templateIds || {};
  const template=db.templates.find(t=>t.id===(ids[specs[0]?.key] || config.templateId) && t.enabled);
  if(!template) throw Error('请选择可用的封面模板');
  const documents={},snapshots={};
  for(const spec of specs) {
    const chosen=ids[spec.key] || config.templateId;
    const match=db.templates.find(t=>t.id===chosen && t.enabled);
    if(!match) throw Error(`请选择 ${spec.key} 封面模板`);
    if(Math.abs(match.document.width/match.document.height-spec.width/spec.height)>=0.01) throw Error(`${spec.key} 模板比例不匹配，请重新选择`);
    documents[spec.key]=clone(match.document);snapshots[spec.key]={id:match.id,version:match.version || 1};
  }
  change(row,'coverMode','模板合成');change(row,'templateId',template.id);
  change(row,'templateVersion',template.version || 1);change(row,'templateDocument',template.document);
  change(row,'templateDocuments',documents);change(row,'templateSnapshots',snapshots);
  const image=config.imageId ? db.images.find(i=>i.id===config.imageId && i.enabled && i.uses?.includes('底图')) : null;
  if(config.imageId && !image) throw Error('封面底图已不可用');
  const source=image?.src || db.assets.find(a=>a.id===row.assetId)?.cover;
  if(!source && template.document.backgroundMode!=='solid') throw Error('素材没有可用的封面底图');
  change(row,'coverSource',source || '');change(row,'imageId',image?.id || null);change(row,'imageVersion',image?.version || null);
  for(const field of coverTextFieldsForRow(row)) {
    if(!generated.has(field.key) && config.textPolicy==='ai' && (config.overwriteText || !String(row[field.key] || '').trim())) {change(row,field.key,values[field.key]);generated.add(field.key);}
  }
  const errors=coverTextErrors(row);if(errors.length)throw Error(errors.join('；'));
}

const ignoredFields = new Set(['version','fieldVersions','contentApproved','coverApproved','overrides','contentVersion']);
const labels={title:'发布标题',shortTitle:'封面短标题',subTitle:'封面副标题',description:'简介 / 正文',topics:'话题',tags:'平台标签'};
const sectionKey=(config,key)=>JSON.stringify(config.sections?.includes(key) ? config[key] : null);
const coverOutputFields=['cover','coverVariants','coverSignature','coverRender','coverError','coverApproved'];
function refreshChanges(item) {
  if(!item.before)return;
  const original=JSON.parse(item.before),row=item.row;
  item.changedFields=Object.keys(row).filter(key=>!ignoredFields.has(key) && JSON.stringify(row[key])!==JSON.stringify(original[key]));
  if(item.changedFields.length) {
    row.contentApproved=false;row.contentVersion=(original.contentVersion || 1)+1;
    row.overrides=[...new Set([...(original.overrides || []),...item.changedFields])];
  } else item.row=clone(original);
}
export function configurationFieldError(item,key) {
  if(!['title','shortTitle','subTitle'].includes(key))return '';
  const required=key==='title' || coverTextFieldsForRow(item.row).some(field=>field.key===key);
  return required && !String(item.row[key] || '').trim() ? `请填写${labels[key]}` : '';
}
export function configurationIssues(item) {
  const issues=item.error?[item.error]:[];
  for(const key of item.editableFields || []) {
    const message=configurationFieldError(item,key);if(message)issues.push(message);
  }
  if(item.coverNeedsUpdate)issues.push('封面文字已修改，请更新封面');
  return [...new Set(issues)];
}
export function editConfigurationField(item,key,value) {
  if(item.applied || !(item.editableFields || []).includes(key))return false;
  if(item.row[key]===value)return true;
  const affectsCover=isTemplateCover(item.row) && coverTextFieldsForRow(item.row).some(field=>field.key===key);
  if(affectsCover) item.previewCovers=clone(Object.keys(item.row.coverVariants || {}).length?item.row.coverVariants:item.previewCovers || {source:item.row.cover});
  change(item.row,key,value);
  item.manualFields=[...new Set([...(item.manualFields || []),key])];
  if(affectsCover) item.coverNeedsUpdate=true;
  refreshChanges(item);return true;
}
export function regenerateConfigurationField(item,key,{replaceManual=false}={}) {
  if(item.applied || !(item.editableFields || []).includes(key))return false;
  if(item.manualFields?.includes(key) && !replaceManual)return false;
  const values={...demoValues(item.row,item.config.copy?.style || item.config.cover?.style),subTitle:'实战记录 · 进阶思路'};
  editConfigurationField(item,key,values[key]);
  item.manualFields=(item.manualFields || []).filter(field=>field!==key);
  return true;
}
export async function updateConfigurationCover(item,{render=ensureCoverResults}={}) {
  if(item.applied || !isTemplateCover(item.row))return false;
  if(coverTextErrors(item.row).length)return false;
  // Rendering a clone prevents an obsolete async response from replacing edited text.
  const row=clone(item.row),signature=JSON.stringify(row);
  try {
    await render(row);
    if(JSON.stringify(item.row)!==signature)return false;
    Object.assign(item.row,Object.fromEntries(coverOutputFields.map(key=>[key,row[key]])));
    item.coverNeedsUpdate=false;
    if(item.errorStage==='cover'){item.error='';item.errorStage='';}
    item.previewCovers=clone(row.coverVariants || {});refreshChanges(item);return true;
  } catch(e) {
    if(JSON.stringify(item.row)===signature){item.error=e.message || '封面更新失败，请重试';item.errorStage='cover';item.coverNeedsUpdate=true;}
    return false;
  }
}

// Previews work on isolated copies. Only an explicit apply writes to selected rows.
export async function prepareBatchConfiguration(rows,ids,config,db,{render=ensureCoverResults,inspect=inspectCoverImage,previous=[],generate=demoValues,rebase=false}={}) {
  const selected=new Set(ids), sections=new Set(config.sections || []), results=[];
  for(const id of selected) {
    const saved=previous.find(item=>item.id===id);
    if(saved?.applied){results.push(clone(saved));continue;}
    const current=rows.find(row=>row.candidateId===id);
    if(!current){results.push({id,before:'',row:{candidateId:id},error:'该条内容已移出，请重新选择',changedFields:[],applied:false,editableFields:[]});continue;}
    const conflict=saved && saved.before!==snapshot(current);
    const mappingChanged=saved && ['assetId','accountId','platform'].some(key=>saved.row[key]!==current[key]);
    if(conflict && (!rebase || mappingChanged)) {
      results.push({...clone(saved),error:mappingChanged?'账号或成片已改变，请保留人工稿并重新选择内容':'内容已发生变化，请先核对最新内容再重新审阅',errorStage:'conflict'});continue;
    }
    const prior=saved;
    if(prior && !conflict && !prior.error && JSON.stringify(prior.config)===JSON.stringify(config)){results.push(clone(prior));continue;}
    const row=clone(current), item={id:row.candidateId,before:snapshot(current),row,error:'',errorStage:'config',changedFields:[],applied:false,config:clone(config),editableFields:[],manualFields:[],coverNeedsUpdate:false};
    const generated=new Set(),values={...generate(row,config.copy?.style || config.cover?.style),subTitle:'实战记录 · 进阶思路'};
    try {
      if(!sections.size) throw Error('请选择本次修改项');
      if(sections.has('common')) {
        const changes=config.common || {};
        if(Object.keys(changes).some(key=>!['declaration','visibility'].includes(key))) throw Error('请选择可批量编辑的公共字段');
        for(const [key,value] of Object.entries(changes)) change(row,key,value);
      }
      if(sections.has('copy')) {
        const options=config.copy || {};
        // Cover text is editable only when the effective composition uses it.
        const allowed=configurationCopyFields(row,config,db);
        for(const key of (options.fields || []).filter(key=>allowed.includes(key))) {
          item.editableFields.push(key);
          if(prior?.editableFields?.includes(key) && (prior.manualFields?.includes(key) || (prior.config.copy?.style===options.style && prior.config.copy?.overwrite===options.overwrite))) {
            change(row,key,prior.row[key]);
            if(prior.manualFields?.includes(key))item.manualFields.push(key);
          } else if(options.overwrite || !String(row[key] || '').trim()) change(row,key,values[key]);
          generated.add(key);
        }
      }
      if(sections.has('cover')) {
        const options=config.cover || {};
        if(options.mode==='template') {
          templateCover(row,options,db,values,generated);
          if(options.textPolicy==='ai')for(const field of coverTextFieldsForRow(row)) {
            if(!item.editableFields.includes(field.key))item.editableFields.push(field.key);
            if(prior?.manualFields?.includes(field.key)){change(row,field.key,prior.row[field.key]);item.manualFields.push(field.key);}
          }
          item.errorStage='cover';
          if(prior && sectionKey(prior.config,'cover')===sectionKey(config,'cover') && prior.row.shortTitle===row.shortTitle && prior.row.subTitle===row.subTitle && !prior.coverNeedsUpdate && !prior.error) {
            for(const key of coverOutputFields)if(key in prior.row)row[key]=clone(prior.row[key]);
          } else await render(row);
        } else {
          const image=options.mode==='image' ? db.images.find(i=>i.id===options.imageId && i.enabled && i.uses?.includes('现成封面')) : null;
          if(options.mode==='image' && !image) throw Error('请选择可用的现成封面');
          const src=image?.src || db.assets.find(a=>a.id===row.assetId)?.cover;
          if(!src) throw Error('素材没有可用的自带图片');
          await inspect(src);
          const changed=row.coverMode!=='现成封面' || row.cover!==src || (row.imageId || null)!==(image?.id || null);
          if(changed) {
            change(row,'coverMode','现成封面');change(row,'coverSource',src);change(row,'cover',src);
            change(row,'imageId',image?.id || null);change(row,'imageVersion',image?.version || null);
            row.coverVariants={};row.coverSignature=null;row.coverApproved=false;row.coverRender='已选用';row.coverError='';
          }
        }
      } else if(isTemplateCover(row) && coverTextFieldsForRow(row).some(field=>generated.has(field.key) && row[field.key]!==current[field.key])) {
        item.coverNeedsUpdate=true;
      }
      item.errorStage='';
    } catch(e) {item.error=e.message || '配置失败，请重试';if(item.errorStage==='cover')item.coverNeedsUpdate=true;}
    item.previewCovers=clone(Object.keys(row.coverVariants || {}).length?row.coverVariants:current.coverVariants || {source:current.cover});
    refreshChanges(item);results.push(item);
  }
  return results;
}

export function applyBatchConfiguration(rows,results) {
  let changed=0,unchanged=0;
  for(const item of results) {
    if(item.applied || configurationIssues(item).length) continue;
    const current=rows.find(r=>r.candidateId===item.id);
    if(!current) {item.error='该条内容已移出，请重新选择';item.errorStage='conflict';continue;}
    if(snapshot(current)!==item.before) {item.error='内容已发生变化，请重新预览';item.errorStage='conflict';continue;}
    if(item.changedFields.length) {Object.assign(current,clone(item.row));changed++;}
    else unchanged++;
    item.applied=true;
  }
  return {changed,unchanged,failed:results.filter(item=>!item.applied && configurationIssues(item).length).length};
}
