/* 人工评审：沿用既有存储，逐页结论必须由评审人明确保存。 */
(() => {
  'use strict';
  const KEY = 'wt-content-operations-review-v3', REVISION = '2026-11-03-v8-compose-integrated';
  const sections = [...document.querySelectorAll('main > section[id]')];
  const feedbackSections = sections.filter(s => !s.hasAttribute('data-no-feedback'));
  const titles = Object.fromEntries(sections.map(s => [s.id, s.querySelector('h1,h2')?.textContent || s.id]));
  const pages = window.OPSReviewPages;
  const pageMap = Object.fromEntries(pages.map(p => [p.id, p]));
  const productionReview = window.OPSProductionReview;
  const progressPages = productionReview ? pages.filter(p=>productionReview.ids.includes(p.id)) : pages;
  const escape = v => String(v ?? '').replace(/[&<>"']/g, c => ({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
  const statusLabels = {open:'待讨论', accepted:'已采纳', resolved:'已处理'};
  const decisionLabels = {pending:'未评审', adjust:'需要调整', approved:'粗评通过'};
  const roles = ['运营','产品','设计','其他'];
  const priorities = [['P0','P0 · 影响核心流程'],['P1','P1 · 影响日常体验'],['P2','P2 · 后续细节'],['INFO','补充 / 整体结论']];
  let db = {schema:1, revision:REVISION, author:'', comments:[], drafts:{}, pageDecisions:{}, decisionDrafts:{}}, timer;
  try {
    const saved = JSON.parse(localStorage.getItem(KEY));
    if (saved?.schema === 1 && Array.isArray(saved.comments)) db = {...db, ...saved};
  } catch {}
  db.revision = REVISION;
  db.drafts ||= {}; db.pageDecisions ||= {}; db.decisionDrafts ||= {};
  const toast = document.createElement('div');
  toast.id = 'review-toast'; toast.setAttribute('role','status'); toast.setAttribute('aria-live','polite'); document.body.append(toast);
  const notify = text => { toast.textContent = text; clearTimeout(timer); timer = setTimeout(() => toast.textContent = '', 4500); };
  const persist = () => { try { localStorage.setItem(KEY, JSON.stringify(db)); return true; } catch { notify('浏览器未能保存，请立即导出 JSON 备份意见。'); return false; } };
  const id = () => globalThis.crypto?.randomUUID?.() || Date.now() + '-' + Math.random().toString(36).slice(2);
  const format = iso => new Date(iso).toLocaleString('zh-CN', {hour12:false});
  const options = (values, selected) => values.map(([v,label]) => `<option value="${escape(v)}" ${v===selected?'selected':''}>${escape(label)}</option>`).join('');
  const authorChanged = value => {
    db.author = value;
    document.getElementById('review-author').value = value;
    document.querySelectorAll('.review-author-local').forEach(el => { if (el !== document.activeElement) el.value = value; });
  };
  document.getElementById('review-toolbar').innerHTML = `<label for="review-author">评审人 <input id="review-author" placeholder="填写姓名或称呼" value="${escape(db.author)}" maxlength="80"></label><button class="ex-button" id="export-json">导出评审 JSON</button><button class="ex-button" id="export-md">导出评审 Markdown</button><button class="ex-button" id="import-button">导入并合并评审</button><input id="import-json" type="file" accept=".json,application/json" hidden><a href="#global-review">补充整体结论 →</a><span id="review-count" class="review-status" role="status"></span><small>意见、逐页结论和草稿保存在本浏览器；通过导出文件交接，不会自动同步。</small>`;
  const count = () => {
    document.getElementById('review-count').textContent = `已记录 ${db.comments.length} 条意见 · 待讨论 ${db.comments.filter(c=>c.status==='open').length} 条`;
    const counts = {pending:0, adjust:0, approved:0};
    progressPages.forEach(p => {
      const status = db.pageDecisions[p.id]?.status || 'pending';
      counts[status]++;
      const badge = document.querySelector(`[data-page-status="${p.id}"]`);
      if(badge){badge.textContent = decisionLabels[status]; badge.className = 'page-status ' + status;}
    });
    document.getElementById('page-progress').textContent = productionReview
      ? `历史粗评记录：${counts.approved} / ${progressPages.length} 通过 · ${counts.adjust} 需调整 · ${counts.pending} 未评审。本轮内容生产方案仍待人工评审，历史结论不自动沿用为本轮通过。`
      : `人工粗评：${counts.approved} / ${pages.length} 通过 · ${counts.adjust} 需调整 · ${counts.pending} 未评审${counts.approved===pages.length?'；仍需整体确认，细评未开始。':''}`;
  };
  const renderDecision = p => {
    const box = document.querySelector(`[data-page-decision="${p.id}"]`);
    const saved = db.pageDecisions[p.id], draft = db.decisionDrafts[p.id];
    const value = draft || saved || {status:'pending', note:''};
    box.className = 'page-decision';
    box.innerHTML = `<h3>人工结论 · ${escape(p.name)}</h3><p class="review-small">意见标记“已处理”不等于本页粗评通过。这里的结论只由评审人明确保存；下一阶段不会自动开启。</p>${saved?`<p class="page-decision-saved"><span class="page-status ${saved.status}">${decisionLabels[saved.status]}</span> ${escape(saved.author)} · ${format(saved.updated)}</p>`:''}<form><div class="page-decision-fields"><div><label for="decision-${p.id}">本页粗评结论</label><select name="status" id="decision-${p.id}">${options(Object.entries(decisionLabels),value.status)}</select></div><div><label for="decision-author-${p.id}">评审人</label><input class="review-author-local" id="decision-author-${p.id}" name="author" maxlength="80" value="${escape(draft?.author || db.author)}" placeholder="姓名或称呼"></div></div><label for="decision-note-${p.id}">结论说明／必须先改的项</label><textarea id="decision-note-${p.id}" name="note" maxlength="20000" placeholder="需要调整时，写明问题编号或 Tab、要调整什么；通过时可注明已复核范围。">${escape(value.note)}</textarea><div class="feedback-footer"><small data-decision-save>${draft?'结论草稿已恢复；点击保存后才更新目录':'填写自动存草稿；点击保存后才更新目录'}</small><button type="submit" class="ex-button">保存本页结论</button></div></form>`;
    if (p.detailProposal) {
      box.querySelector('h3').textContent = '历史粗评结论 · 不代表本轮细评提案已确认';
      box.querySelector('.review-small').textContent = '原有结论与草稿完整保留。本轮方案请在补充意见中说明或在对话中确认；体验确认安排在方案接入工作台之后。';
    }
    const form = box.querySelector('form');
    const saveDraft = () => {
      const f = new FormData(form);
      authorChanged(String(f.get('author') || ''));
      db.decisionDrafts[p.id] = {status:f.get('status'), author:db.author, note:f.get('note')};
      box.querySelector('[data-decision-save]').textContent = persist() ? '结论草稿已保存，尚未更新目录' : '未能保存，请导出备份';
    };
    form.addEventListener('input',saveDraft); form.addEventListener('change',saveDraft);
    form.addEventListener('submit',e => {
      e.preventDefault(); saveDraft();
      const d = db.decisionDrafts[p.id];
      if (!d.author.trim()) { notify('请填写评审人，结论草稿已保留'); form.elements.author.focus(); return; }
      if (d.status === 'adjust' && !d.note.trim()) { notify('请写明需要调整的问题或区域'); form.elements.note.focus(); return; }
      db.pageDecisions[p.id] = {...d, author:d.author.trim(), note:d.note.trim(), updated:new Date().toISOString()};
      delete db.decisionDrafts[p.id];
      const ok = persist(); renderDecision(p); count(); if (ok) notify('本页人工结论已保存到本机');
    });
  };
  const render = s => {
    const key = s.id, page = pageMap[key], draft = db.drafts[key] || {role:'运营', priority:'P1', text:''};
    let box = s.querySelector('.review-feedback');
    if (!box) { box = document.createElement('div'); box.className = 'review-feedback'; s.append(box); }
    if(page?.productionReport) s.querySelector('[id="feedback-'+key+'"]')?.append(box);
    const focusOptions = page ? ['整页／跨区域', ...page.units.map(u=>u.area)] : [];
    if(page&&draft.focus&&!focusOptions.includes(draft.focus))focusOptions.push(draft.focus);
    box.innerHTML = `<h3>${key==='global-review'?'补充整体粗评意见':page?'补充本页粗评':'补充本节历史评审'}</h3><form data-comment-section="${key}">${page?`<div class="review-focus"><label for="focus-${key}">针对哪个 Tab／区域</label><select name="focus" id="focus-${key}">${options(focusOptions.map(v=>[v,v]),draft.focus||focusOptions[0])}</select></div>`:''}<div class="review-comment-fields"><div><label for="role-${key}">评审视角</label><select name="role" id="role-${key}">${options(roles.map(v=>[v,v]),draft.role)}</select></div><div><label for="priority-${key}">建议优先级</label><select name="priority" id="priority-${key}">${options(priorities,draft.priority)}</select></div><div><label for="author-${key}">评审人</label><input id="author-${key}" class="review-author-local" name="author" maxlength="80" value="${escape(db.author)}" placeholder="姓名或称呼"></div></div><label class="sr-only" for="text-${key}">对${escape(titles[key])}的补充意见</label><textarea name="text" id="text-${key}" placeholder="${page?'写明问题编号或 Tab、功能问题和调整建议；细节留到下一阶段。':'发生在哪个场景？哪里不顺？建议如何调整？'}" maxlength="20000">${escape(draft.text)}</textarea><div class="feedback-footer"><small data-save-status>${draft.text?'草稿已恢复':'填写时自动保存草稿'}</small><div>${draft.editId?'<button type="button" class="ex-button" data-cancel-edit>取消修改</button>':''}<button class="ex-button primary" type="submit">${draft.editId?'保存修改':'添加评审意见'}</button></div></div></form><div class="review-comments">${db.comments.filter(c=>c.section===key).sort((a,b)=>a.created.localeCompare(b.created)).map(c=>`<article class="review-comment ${c.status==='resolved'?'done':''}" data-comment="${escape(c.id)}"><header><strong>${escape(c.author)} · ${escape(c.role)} · ${escape(c.priority)}</strong><time>${format(c.updated)}</time></header>${c.focus?`<div class="review-comment-focus">${escape(c.focus)}</div>`:''}<p>${escape(c.text)}</p><footer><label for="status-${escape(c.id)}">意见状态</label><select id="status-${escape(c.id)}" data-status>${options(Object.entries(statusLabels),c.status)}</select><button type="button" class="ex-text-button" data-edit>修改意见</button></footer></article>`).join('')}</div>`;
    if (page?.detailProposal) {
      box.querySelector('h3').textContent = '补充本轮细评意见';
      box.querySelector('textarea').placeholder = '请注明 D1—D3 或具体区域，描述预期操作和需要调整的地方。';
    }
    if(page?.productionReport){
      box.querySelector('h3').textContent='补充本页评审意见';
      box.querySelector('textarea').placeholder='可写本页场景、实际业务补充与希望的行为；已确认的规则无需重复选择。';
    }
    if(productionReview&&key==='global-review'){
      box.querySelector('h3').textContent='集中补充本轮内容生产意见';
      box.querySelector('textarea').placeholder='按页码／场景 → 业务补充 → 期望行为，一次写下需要修订的内容。';
    }
    const form = box.querySelector('form');
    const saveDraft = () => {
      const f = new FormData(form); authorChanged(String(f.get('author') || ''));
      db.drafts[key] = {role:f.get('role'), priority:f.get('priority'), text:f.get('text'), focus:f.get('focus')||'', editId:db.drafts[key]?.editId||null};
      box.querySelector('[data-save-status]').textContent = persist()?'草稿已保存到本机':'未能保存，请导出备份';
    };
    form.addEventListener('input',saveDraft); form.addEventListener('change',saveDraft);
    form.addEventListener('submit',e => {
      e.preventDefault(); saveDraft(); const d = db.drafts[key];
      if (!d.text.trim()) { notify('请填写评审意见'); form.elements.text.focus(); return; }
      if (!db.author.trim()) { notify('请填写评审人，草稿已保留'); form.elements.author.focus(); return; }
      const now = new Date().toISOString(), old = d.editId && db.comments.find(c=>c.id===d.editId);
      const fields = {role:d.role, priority:d.priority, text:d.text.trim(), focus:d.focus, updated:now};
      if (old) Object.assign(old, fields);
      else db.comments.push({id:id(), section:key, author:db.author.trim(), ...fields, status:'open', created:now});
      delete db.drafts[key]; const ok = persist(); render(s); count(); if (ok) notify(old?'评审意见已修改并保存':'评审意见已保存到本机');
    });
    box.querySelector('[data-cancel-edit]')?.addEventListener('click',()=>{delete db.drafts[key];persist();render(s);});
    box.querySelectorAll('[data-comment]').forEach(article => {
      const c = db.comments.find(c=>c.id===article.dataset.comment);
      article.querySelector('[data-status]').addEventListener('change',e=>{c.status=e.target.value;c.updated=new Date().toISOString();persist();render(s);count();});
      article.querySelector('[data-edit]').addEventListener('click',()=>{
        if (db.drafts[key]?.text && !confirm('本页已有未提交草稿，替换为这条意见进行修改？')) return;
        db.drafts[key] = {role:c.role,priority:c.priority,text:c.text,focus:c.focus||'',editId:c.id};persist();render(s);s.querySelector('.review-feedback textarea').focus();
      });
    });
  };
  feedbackSections.forEach(render); pages.forEach(renderDecision); count();
  document.getElementById('review-author').addEventListener('input',e=>{authorChanged(e.target.value);persist();});
  const download = (text,type,ext) => {
    const url=URL.createObjectURL(new Blob([text],{type})),a=document.createElement('a');
    a.href=url;a.download='内容运营逐页粗评-'+new Date().toISOString().slice(0,10)+'.'+ext;document.body.append(a);a.click();a.remove();setTimeout(()=>URL.revokeObjectURL(url),1000);
  };
  document.getElementById('export-json').onclick=()=>{
    download(JSON.stringify({...db,phase:productionReview?'production-product-interaction-review':'page-coarse-review',...(productionReview?{proposalRevision:productionReview.revision}:{}),exportedAt:new Date().toISOString()},null,2),'application/json;charset=utf-8','json');notify('已生成 JSON 备份，包含意见、逐页结论和未提交草稿');
  };
  document.getElementById('export-md').onclick=()=>{
    const lines=['# 内容运营逐页评审 · 人工补充',`评审稿版本：${productionReview?.revision||REVISION}`,productionReview?'当前阶段：内容生产七个单元的产品与交互评审；历史粗评记录保留，不代表本轮方案或原型已通过。':'当前阶段：第 06 页新增合成任务及素材选择器细评提案；已有页面结论仍为历史粗评结论，不代表当前提案确认。',`导出时间：${new Date().toLocaleString('zh-CN')}`,''];
    feedbackSections.forEach(s=>{
      const comments=db.comments.filter(c=>c.section===s.id),draft=db.drafts[s.id],decision=db.pageDecisions[s.id],decisionDraft=db.decisionDrafts[s.id];
      if(!comments.length&&!draft?.text&&!decision&&!decisionDraft)return;
      lines.push('## '+titles[s.id],'');
      if(decision)lines.push(`人工结论：${decisionLabels[decision.status]} · ${decision.author} · ${format(decision.updated)}`,'',decision.note,'');
      if(decisionDraft)lines.push(`结论草稿（未提交）：${decisionLabels[decisionDraft.status]}`,'',decisionDraft.note,'');
      comments.forEach(c=>lines.push(`### ${c.priority} · ${c.author} · ${c.role} · ${statusLabels[c.status]}`,`更新：${format(c.updated)}`,c.focus?'对应区域：'+c.focus:'','',c.text,'','---',''));
      if(draft?.text)lines.push('### 未提交意见草稿',draft.focus?'对应区域：'+draft.focus:'','',draft.text,'');
    });
    download(lines.join('\n'),'text/markdown;charset=utf-8','md');
  };
  const input=document.getElementById('import-json');document.getElementById('import-button').onclick=()=>input.click();
  input.addEventListener('change',async()=>{
    const file=input.files[0];if(!file)return;
    try {
      if(file.size>3*1024*1024)throw Error('文件超过 3 MB');
      const data=JSON.parse(await file.text());
      if(data.schema!==1||!['2026-10-31-v3','2026-11-01-v4','2026-11-01-v5','2026-11-03-v6','2026-11-03-v8-compose-design2',REVISION].includes(data.revision)||!Array.isArray(data.comments)||data.comments.length>2000)throw Error('不是兼容的评审导出文件');
      const string=(s,max)=>typeof s==='string'&&s.length<=max;
      const focusValid=d=>d.focus===undefined||string(d.focus,160);
      const valid=c=>c&&string(c.id,100)&&Object.hasOwn(titles,c.section)&&string(c.author,80)&&roles.includes(c.role)&&priorities.some(([v])=>v===c.priority)&&string(c.text,20000)&&focusValid(c)&&Object.hasOwn(statusLabels,c.status)&&Number.isFinite(Date.parse(c.created))&&Number.isFinite(Date.parse(c.updated));
      if(!data.comments.every(valid))throw Error('意见字段不完整或格式无效');
      const isRecord=value=>value&&typeof value==='object'&&!Array.isArray(value);
      for(const field of ['drafts','pageDecisions','decisionDrafts'])if(data[field]!==undefined&&!isRecord(data[field]))throw Error('评审数据格式无效');
      const validDecision=d=>d&&Object.hasOwn(decisionLabels,d.status)&&string(d.author,80)&&string(d.note,20000);
      for(const [key,d]of Object.entries(data.pageDecisions||{}))if(!Object.hasOwn(pageMap,key)||!validDecision(d)||!Number.isFinite(Date.parse(d.updated)))throw Error('逐页结论字段无效');
      for(const [key,d]of Object.entries(data.decisionDrafts||{}))if(!Object.hasOwn(pageMap,key)||!validDecision(d))throw Error('结论草稿字段无效');
      // 所有有效性检查完成后再合并，失败时不改变现有记录。
      const merged=new Map(db.comments.map(c=>[c.id,c]));let added=0,updated=0,decisions=0;
      data.comments.forEach(c=>{const old=merged.get(c.id);if(!old){merged.set(c.id,c);added++;}else if(Date.parse(c.updated)>Date.parse(old.updated)){merged.set(c.id,c);updated++;}});
      db.comments=[...merged.values()];
      for(const [key,d]of Object.entries(data.drafts||{})){
        if(!Object.hasOwn(titles,key)||db.drafts[key]?.text||!d||!string(d.text,20000)||!focusValid(d))continue;
        db.drafts[key]={text:d.text,role:roles.includes(d.role)?d.role:'运营',priority:priorities.some(([v])=>v===d.priority)?d.priority:'P1',focus:d.focus||'',editId:typeof d.editId==='string'&&merged.get(d.editId)?.section===key?d.editId:null};
      }
      for(const [key,d]of Object.entries(data.pageDecisions||{}))if(!db.pageDecisions[key]||Date.parse(d.updated)>Date.parse(db.pageDecisions[key].updated)){db.pageDecisions[key]=d;decisions++;}
      for(const [key,d]of Object.entries(data.decisionDrafts||{}))if(!db.decisionDrafts[key])db.decisionDrafts[key]=d;
      const ok=persist();feedbackSections.forEach(render);pages.forEach(renderDecision);count();
      if(ok)notify(`合并完成：新增 ${added} 条意见，更新 ${updated} 条意见，更新 ${decisions} 个页面结论`);
    } catch(e) {notify('导入失败：'+e.message+'；现有意见保留。');}
    finally {input.value='';}
  });
})();
