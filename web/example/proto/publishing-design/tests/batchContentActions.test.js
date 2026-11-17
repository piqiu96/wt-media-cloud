import { it, expect } from 'vitest';
import { seed, defaults, declarationOptions } from '../model.js';
import { coverSignature, defaultDocument } from '../covers.js';
import { initCandidate, requestSuggestion, receiveSuggestion, editField } from '../content.js';
import { applyPlatformFields, fillSuggestedBlanks, confirmContent, suggestionFields } from '../batchContentActions.js';
function fixture(){ const db=seed();db.tasks=[]; const rows=db.assets.slice(0,2).map(v=>initCandidate({...defaults('B站',v),platform:'B站',assetId:v.id,category:v.category,accountId:db.accounts.find(a=>a.platform==='B站'&&a.categories.includes(v.category)).id,declaration:declarationOptions('B站')[0].value}));for(const r of rows){r.coverSource=r.cover;r.coverSignature=coverSignature(r);r.coverVariants={'4:3':r.cover,'16:9':r.cover};}return {db,rows}; }
it('bulk edits only selected rows and explicitly selected fields, invalidating their approval',()=>{const {rows}=fixture();rows.forEach(r=>r.contentApproved=true);const original=JSON.stringify(rows[1]);applyPlatformFields(rows,[rows[0].candidateId],{visibility:'仅自己可见'});expect(rows[0].visibility).toBe('仅自己可见');expect(rows[0].contentApproved).toBe(false);expect(JSON.stringify(rows[1])).toBe(original);expect(rows[0].declaration).toBe('内容无需标注');});
it('does not allow bulk editing identity or ordinary text fields',()=>{const {rows}=fixture();const original=JSON.stringify(rows);expect(()=>applyPlatformFields(rows,[rows[0].candidateId],{accountId:999,title:'覆盖'})).toThrow();expect(JSON.stringify(rows)).toBe(original);});
it('filling suggestions preserves nonempty content and rejects stale suggestions',()=>{const {rows}=fixture();const r=rows[0];r.description='';const req=requestSuggestion(r,['title','description']);receiveSuggestion(r,req,{title:'AI标题',description:'AI简介'});expect(fillSuggestedBlanks([r],['title','description'])).toBe(1);expect(r.title).toBe('三角洲实战精彩片段');expect(r.description).toBe('AI简介');const next=requestSuggestion(r,['description']);receiveSuggestion(r,next,{description:'过期'});editField(r,'description','');expect(fillSuggestedBlanks([r],['description'])).toBe(0);});
it('confirmation does not generate or alter a stale cover',()=>{const {db,rows}=fixture();const r=rows[0];r.coverMode="模板合成";r.templateDocument=defaultDocument();r.shortTitle="模板标题";r.coverSignature=null;r.coverVariants={};const before=JSON.stringify(r);expect(confirmContent(r,db).ok).toBe(false);expect(JSON.stringify(r)).toBe(before);});
it('confirmation blocks missing required fields and accepts the actual current rendered cover',()=>{const {db,rows}=fixture();rows[0].declaration='';expect(confirmContent(rows[0],db).ok).toBe(false);expect(rows[0].contentApproved).not.toBe(true);expect(confirmContent(rows[1],db)).toEqual({ok:true,errors:[]});expect(rows[1].contentApproved).toBe(true);expect(rows[1].coverApproved).toBe(true);});

it('fills only applicable platform fields and preserves an existing-image cover',()=>{const {db,rows}=fixture();const r=rows[0];const originalCover=r.coverSignature;const req=requestSuggestion(r,['description','shortTitle','subTitle','topics']);receiveSuggestion(r,req,{description:'建议简介',shortTitle:'模板标题',subTitle:'模板副标题',topics:'不适用'});expect(fillSuggestedBlanks([r],['description','shortTitle','subTitle','topics'])).toBe(1);expect(r.coverSignature).toBe(originalCover);expect(confirmContent(r,db).ok).toBe(true);expect(r.topics).toBe('');});

it('AI recommendations include only text used in any active composition output',()=>{
 const {rows}=fixture();const row=rows[0],main=defaultDocument(),sub=defaultDocument(1600,900);sub.layers=[{...sub.layers[0],id:'subtitle',slot:'sub_title'}];
 Object.assign(row,{coverMode:'模板合成',templateDocument:main,templateDocuments:{'16:9':sub}});
 expect(suggestionFields(row)).toEqual(['title','description','tags','shortTitle','subTitle']);
 row.coverMode='现成封面';expect(suggestionFields(row)).toEqual(['title','description','tags']);
});
