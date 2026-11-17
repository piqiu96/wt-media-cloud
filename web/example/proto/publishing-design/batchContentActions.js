import { validate } from './model.js';
import { coverSignature, coverTextFieldsForRow, coverReadinessIssues } from './covers.js';
import { editField, adopt } from './content.js';
export const approvalIssue = '请审核当前版本的内容与已合成封面';
export function contentIssues(row, db) { return validate(row, db, row.id); }
export function contentState(row, db) {
  const errors=contentIssues(row,db);
  return errors.some(e=>e!==approvalIssue) ? '待完善' : errors.length ? '待确认' : '已就绪';
}
export function applyPlatformFields(rows, ids, changes) {
  if (Object.keys(changes).some(key=>!['declaration','visibility'].includes(key))) throw Error('请选择可批量编辑的平台字段');
  const selected=new Set(ids);let count=0;
  for(const row of rows) if(selected.has(row.candidateId)) {
    for(const [key,value] of Object.entries(changes)) if(row[key]!==value) {
      editField(row,key,value);row.overrides=[...new Set([...(row.overrides||[]),key])];
    }
    count++;
  }
  return count;
}
export function suggestionFields(row) {
  const fields=row.platform==='百家号'?['description','tags']:row.platform==='B站'?['title','description','tags']:['title','description','topics'];
  if(['模板合成','规则生成封面'].includes(row.coverMode)) fields.push(...coverTextFieldsForRow(row).map(field=>field.key));
  return fields;
}
export function fillSuggestedBlanks(rows, fields) {
  let count=0;
  for(const row of rows) for(const field of fields) if(suggestionFields(row).includes(field) && !String(row[field]||'').trim() && adopt(row,field)) {
    row.contentVersion=(row.contentVersion||1)+1;count++;
  }
  return count;
}
export function confirmContent(row, db) {
  const coverErrors=coverReadinessIssues(row);
  if(coverErrors.length) return {ok:false,errors:coverErrors};
  const errors=validate({...row,contentApproved:true,coverApproved:true},db,row.id);
  if(errors.length) return {ok:false,errors};
  row.contentApproved=true;row.coverApproved=true;
  return {ok:true,errors:[]};
}
