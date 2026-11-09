/* 仅用于规范HTML的本地演示。无网络调用、无自动成功计时器，不供业务Runtime引用。 */
(() => {
  'use strict';
  const $ = (s, root = document) => root.querySelector(s);
  const $$ = (s, root = document) => [...root.querySelectorAll(s)];
  const openDialogs = [];
  const esc = text => String(text).replace(/[&<>"']/g, c => ({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
  const overlayKinds = {
    create: ['新增窗口', 'window', ''], edit: ['编辑窗口', 'window', ''],
    paired: ['编辑资源资料', 'paired', 'medium'],
    'detail-complex': ['配置与关联详情', 'detail', 'ex-drawer complex'],
    proxy: ['新增代理', 'proxy', 'ex-drawer'], 'proxy-edit': ['编辑代理', 'proxy', 'ex-drawer'],
    'proxy-detail': ['代理详情', 'proxy-detail', 'ex-drawer'],
    detail: ['窗口详情', 'detail', 'ex-drawer'], summary: ['代理摘要', 'summary', ''],
    picker: ['选择托管代理', 'picker', 'wide'], 'picker-empty': ['选择托管代理', 'picker-empty', ''],
    confirm: ['停用窗口', 'confirm', ''], batch: ['批量检测代理', 'batch', 'wide'],
    code: ['参考结构', 'code', 'wide']
  };
  function notify(text) {
    const active = openDialogs.at(-1);
    if (active) {
      let msg = $('[data-demo-message]', active);
      if (!msg) {
        msg = document.createElement('p'); msg.dataset.demoMessage = ''; msg.className = 'ex-help';
        msg.setAttribute('role', 'status'); msg.style.marginTop = '16px'; $('.ex-overlay-body', active).append(msg);
      }
      msg.textContent = text;
    } else {
      const toast = $('#example-toast');
      if (toast) { toast.textContent = text; toast.title = '点击收起'; }
    }
  }
  function refreshLayers() {
    openDialogs.forEach((d, i) => d.classList.toggle('ex-underlay', i < openDialogs.length - 1));
    document.body.style.overflow = openDialogs.length ? 'hidden' : '';
  }
  function closeDialog(dialog) {
    if (!dialog || dialog.dataset.busy === 'true') return;
    window.WTControls?.dismiss(); dialog.close();
  }
  function openExample(kind, record = 'B站主号 · 上海01', trigger = null) {
    const selectedRows = trigger?.closest('[data-selection]') ? $$('[data-row-check]:checked').map(e => e.closest('[data-resource-row]')) : [];
    if (selectedRows.length) record = '已选 ' + selectedRows.length + ' 个窗口';
    const config = overlayKinds[kind], template = config && $('#tpl-' + config[1]);
    if (!template) { notify('请从公共参考入口打开此示例。'); return null; }
    const dialog = document.createElement('dialog');
    dialog.className = 'ex-overlay ' + config[2]; dialog.dataset.kind = kind;
    const titleId = 'example-dialog-title-' + (openDialogs.length + 1);
    dialog.setAttribute('aria-labelledby', titleId);
    const header = document.createElement('header'); header.className = 'ex-overlay-header';
    const h2 = document.createElement('h2'); h2.id = titleId;
    h2.textContent = document.title.includes('起始页') && kind === 'create' ? '新建资源' : config[0];
    const close = document.createElement('button'); close.type = 'button'; close.className = 'ex-close';
    close.dataset.close = ''; close.setAttribute('aria-label', '关闭' + h2.textContent); close.textContent = '×';
    header.append(h2, close); dialog.append(header, template.content.cloneNode(true));
    $$('[data-record-name],[data-detail-name]', dialog).forEach(e => { e.textContent = record; });
    if (kind === 'edit') {
      $('[name="name"]', dialog).value = record; $('[name="group"]', dialog).value = '主力账号组';
      $('[name="note"]', dialog).value = '用于日常内容运营。';
    }
    if (kind === 'proxy-edit') {
      $('[name="name"]', dialog).value = '沪线 · 静态01'; $('#proxy-host', dialog).value = '203.0.113.18';
    }
    dialog.addEventListener('cancel', event => { event.preventDefault(); closeDialog(dialog); });
    // 不注册背景点击关闭；避免选择/输入过程中误关。原生dialog管理焦点与下层inert。
    dialog.addEventListener('close', () => {
      openDialogs.splice(openDialogs.indexOf(dialog), 1); dialog.remove(); refreshLayers();
      if (trigger?.isConnected) trigger.focus();
    });
    document.body.append(dialog); openDialogs.push(dialog); refreshLayers(); dialog.showModal(); window.WTControls?.enhance(dialog);
    if (kind === 'batch') {
      dialog._batchNames = selectedRows.length ? selectedRows.map(e => e.dataset.name) : null;
      dialog._batchItems = dialog._batchNames ? dialog._batchNames.map((name,i)=>({name,status:i===0?'processing':'pending'})) : itemsFor('processing');
      paintBatchDialog(dialog);
    }
    return dialog;
  }
  function showScene() {
    const key = location.hash.slice(1) || 'list';
    if (!$$('[data-scene]').length) return;
    const known = $('[data-scene="' + CSS.escape(key) + '"]') ? key : 'list';
    $$('[data-scene]').forEach(e => { e.hidden = e.dataset.scene !== known; });
    $$('[data-scene-link]').forEach(e => {
      if (e.dataset.sceneLink === known) e.setAttribute('aria-current', 'page'); else e.removeAttribute('aria-current');
    });
    $('#example-toast').textContent = '';
    if (known === 'details') requestAnimationFrame(updateDetailCaption);
  }
  window.addEventListener('hashchange', showScene);
  showScene();

  function updateSelection() {
    const panel = $('[data-list]'); if (!panel) return;
    const selected = $$('[data-row-check]:checked', panel).length;
    $('[data-selection]', panel).hidden = !selected;
    $('[data-selection-count]', panel).textContent = selected;
    const visible = $$('[data-resource-row]', panel).filter(e => !e.hidden);
    const checked = visible.filter(e => $('[data-row-check]', e).checked).length;
    const all = $('[data-check-all]', panel);
    all.checked = visible.length > 0 && checked === visible.length;
    all.indeterminate = checked > 0 && checked < visible.length;
  }
  function filterList() {
    const form = $('[data-filter-form]'); if (!form) return;
    const q = form.elements.q.value.trim().toLowerCase();
    const countLabel = $('[data-advanced-count]', form);
    if(countLabel){const applied=Number(Boolean(form.elements.group.value));countLabel.textContent=applied?'（已生效 '+applied+' 项）':'';countLabel.hidden=!applied;}
    let count = 0;
    $$('[data-resource-row]').forEach(row => {
      const ok = row.textContent.toLowerCase().includes(q) && (!form.elements.mode.value || row.dataset.mode === form.elements.mode.value) &&
        (!form.elements.status.value || row.dataset.status === form.elements.status.value) &&
        (!form.elements.group.value || row.textContent.includes(form.elements.group.value));
      row.hidden = !ok; if (ok) count++;
      if (!ok) $('[data-row-check]', row).checked = false;
    });
    $('[data-list-empty]').hidden = !!count;
    $('[data-list] .ex-table-scroll').hidden = !count;
    $('[data-list-count]').textContent = '共 ' + count + ' 条'; updateSelection();
  }
  const filterForm = $('[data-filter-form]');
  filterForm?.addEventListener('submit', e => { e.preventDefault(); filterList(); });
  filterForm?.addEventListener('reset', () => {
    // Native reset restores values after its event dispatch; refresh in the next task.
    setTimeout(() => { const advanced=$('#list-advanced');if(advanced)advanced.open=false; $$('[data-row-check]').forEach(e => { e.checked = false; }); filterList(); },0);
  });
  function applyDetail() {
    const panel = $('#detail-preview'); if (!panel) return;
    const density = $('#detail-density').value;
    panel.style.width = $('#detail-width').value + 'px';
    $('.ex-description-list', panel).dataset.density = density;
    const fields = $$('.ex-desc-item', panel);
    fields.forEach((e, i) => { e.hidden = density === 'sparse' && [2,3,4,5].includes(i); });
    $$('[data-dense-only]', panel).forEach(e => { e.hidden = density === 'sparse'; });
    requestAnimationFrame(updateDetailCaption);
  }
  function updateDetailCaption() {
    const list = $('#detail-preview .ex-description-list'); if (!list || list.closest('[hidden]')) return;
    const n = getComputedStyle(list).gridTemplateColumns.split(' ').length;
    $('[data-detail-layout]').textContent = '当前：' + (n === 2 ? '双列' : '单列') + ' · 长字段满行';
  }
  if ($('#detail-preview')) new ResizeObserver(updateDetailCaption).observe($('#detail-preview'));

  const stateLabels = {success:'成功',failed:'失败',skipped:'跳过',processing:'处理中',pending:'未处理',unconfirmed:'待确认'};
  function itemsFor(kind) {
    const states = {
      processing:['success','success','failed','processing','pending','pending','unconfirmed','skipped'],
      partial:['success','success','success','failed','failed','skipped','unconfirmed','unconfirmed'],
      success:Array(8).fill('success'), failed:Array(8).fill('failed'),
      uncertain:['success','success','success','success','unconfirmed','unconfirmed','unconfirmed','unconfirmed'],
      interrupted:['success','success','failed','pending','pending','pending','pending','pending']
    }[kind];
    return states.map((status, i) => ({name:'运营代理 · ' + String(i+1).padStart(2,'0'), status}));
  }
  function batchMarkup(items) {
    const counts = Object.fromEntries(Object.keys(stateLabels).map(k => [k, items.filter(x => x.status === k).length]));
    const done = counts.success + counts.failed + counts.skipped;
    const busy = counts.processing > 0;
    const title = busy ? '正在检测代理' : counts.success === items.length ? '全部处理完成' : counts.failed === items.length ? '检测失败' : counts.pending ? '处理已中断' : counts.unconfirmed && !counts.failed ? '部分结果待确认' : '处理部分完成';
    const tone = busy ? '' : counts.success === items.length ? 'success' : 'warning';
    const notes = {success:'连接正常',failed:'连接超时，可重试此项',skipped:'资源不可用，已跳过',processing:'正在检测连接',pending:'尚未开始',unconfirmed:'未收到明确结果，请先核实'};
    const metrics = [['总数',items.length],...Object.entries(stateLabels).map(([k,v])=>[v,counts[k]])];
    return '<div class="ex-result-summary"><div class="ex-result-icon '+tone+'" aria-hidden="true">'+(busy?'…':tone==='success'?'✓':'!')+'</div><h3>'+title+'</h3><p>'+ (busy ? '请保留当前处理页面，逐项结果会在这里更新。' : '结果来自手动选择的演示状态，请查看逐项原因。')+'</p></div><div class="ex-metrics" aria-label="处理计数">'+metrics.map(([label,value])=>'<div data-metric="'+label+'">'+label+'<strong>'+value+'</strong></div>').join('')+'</div>'+ (busy ? '<div class="ex-progress"><div class="ex-progress-label"><span>已明确处理 '+done+' / '+items.length+'</span><span>待确认项单独核实</span></div><progress max="'+items.length+'" value="'+done+'" aria-label="已明确处理的项目"></progress></div>':'')+'<div class="ex-table-scroll"><table class="ex-table"><thead><tr><th>项目</th><th>结果</th><th>原因与下一步</th></tr></thead><tbody>'+items.map(x=>'<tr><td>'+x.name+'</td><td><span class="ex-tag '+({success:'success',failed:'danger',processing:'info',unconfirmed:'warning'}[x.status]||'')+'">'+stateLabels[x.status]+'</span></td><td>'+notes[x.status]+'</td></tr>').join('')+'</tbody></table></div>';
  }
  function paintBatchDialog(dialog) {
    const items = dialog._batchItems;
    $('[data-batch-content]',dialog).innerHTML = batchMarkup(items);
    const busy = items.some(x => x.status === 'processing'); dialog.dataset.busy = String(busy);
    $('[data-close]',dialog).disabled = busy;
    $('[data-next-batch]',dialog).hidden = !busy;
    const hasFailure = items.some(x => x.status === 'failed'), uncertain = items.some(x => x.status === 'unconfirmed');
    $('[data-batch-footer]',dialog).innerHTML = busy ? '<span class="ex-help">处理中不可关闭</span><button class="ex-button" data-close disabled>关闭</button>' :
      '<button class="ex-button" data-close>完成</button>'+ (hasFailure?'<button class="ex-button primary" data-retry-failed>演示重试失败项</button>':'') + (uncertain?'<button class="ex-button '+(hasFailure?'':'primary')+'" data-verify-items>演示核实待确认项</button>':'');
  }
  function renderPreviewBatch() {
    const target = $('[data-batch-preview]'); if (target) target.innerHTML = batchMarkup(itemsFor($('#batch-case').value));
  }
  function renderFeedback() {
    const target = $('#feedback-state'); if (!target) return;
    const mode = $('#empty-case').value;
    if (mode === 'loading') { target.innerHTML = '<div class="ex-stack" role="status" aria-label="资源加载中"><p class="ex-help">正在读取资源…</p><div class="ex-skeleton"></div><div class="ex-skeleton" style="width:90%"></div><div class="ex-skeleton" style="width:55%"></div></div>'; return; }
    if (mode === 'unavailable') { target.innerHTML = '<div class="ex-notice warning"><span class="ex-notice-icon" aria-hidden="true">!</span><div>当前资源均不可用，请查看原因或补充资源。</div></div><div class="ex-table-scroll" style="margin-top:16px"><table class="ex-table"><thead><tr><th>资源</th><th>不可用原因</th></tr></thead><tbody><tr><td>沪线 · 静态02</td><td><span class="ex-tag warning">配额已满</span></td></tr><tr><td>粤线 · 静态01</td><td><span class="ex-tag danger">已到期</span></td></tr></tbody></table></div>'; return; }
    const config = {
      empty:['＋','当前没有代理资源','补充资源后，即可为窗口选择托管代理。','新增代理','data-open="proxy"'],
      filtered:['⌕','没有匹配的资源','试试其他关键词，或清除筛选条件。','清除筛选','data-reset-empty'],
      error:['!','读取资源失败','可以重新读取，当前已填写内容会保留。','重新读取','data-local-note="演示重新读取入口；不会请求真实服务。"'],
      denied:['—','暂无查看权限','请联系管理员确认资源访问权限。',null,null]
    }[mode];
    target.innerHTML = '<div class="ex-empty"><div class="ex-empty-symbol" aria-hidden="true">'+config[0]+'</div><h3>'+config[1]+'</h3><p>'+config[2]+'</p>'+(config[3]?'<button class="ex-button" '+config[4]+'>'+config[3]+'</button>':'')+'</div>';
  }
  document.addEventListener('click', async event => {
    const button = event.target.closest('button,[data-open]'); if (!button || button.disabled) return;
    const dialog = button.closest('dialog');
    if (button.dataset.open) { openExample(button.dataset.open,button.dataset.record,button); return; }
    if (button.hasAttribute('data-close')) { closeDialog(dialog); return; }
    if (button.dataset.proxyPeek) {
      const preview = $('[data-proxy-preview]',dialog), kind=button.dataset.proxyPeek;
      const configs = {
        available:['沪线 · 静态01',[['供应商','示例线路商'],['认证方式','用户名与密码'],['到期时间','2026-12-31'],['最近检测','2026-10-28 11:30']]],
        quota:['沪线 · 静态02',[['不可用原因','配额已满'],['建议','补充配额或选择其他可用代理']]],
        expired:['粤线 · 静态01',[['不可用原因','已到期'],['建议','续费后重新核实，或选择其他代理']]]
      };
      const [title,fields]=configs[kind];
      const same=preview.dataset.peek===kind && !preview.hidden;
      preview.hidden=same;preview.dataset.peek=kind;
      if(!same)preview.innerHTML='<h3>'+title+'<button class="ex-text-button" data-close-peek>收起</button></h3><div class="ex-description-container"><dl class="ex-description-list" data-density="dense">'+fields.map(([k,v])=>'<div class="ex-desc-item"><dt>'+k+'</dt><dd>'+v+'</dd></div>').join('')+'</dl></div>'+(kind==='available'?'<button class="ex-text-button" data-open="proxy-detail" style="margin-top:12px">查看完整代理详情</button>':'');
      return;
    }
    if (button.hasAttribute('data-close-peek')) { $('[data-proxy-preview]',dialog).hidden=true;return; }

    if (button.dataset.localNote) { notify(button.dataset.localNote); return; }
    if (button.hasAttribute('data-toggle')) {
      const target = document.getElementById(button.dataset.toggle);
      target.hidden = !target.hidden; button.setAttribute('aria-expanded', String(!target.hidden)); return;
    }
    if (button.hasAttribute('data-clear-selection')) { $$('[data-row-check]').forEach(e => { e.checked = false; }); updateSelection(); }
    if (button.hasAttribute('data-reset-list')) filterForm.reset();
    if (button.hasAttribute('data-demo-confirm')) { closeDialog(dialog); notify('已完成确认交互演示，未停用真实窗口。'); }
    if (button.hasAttribute('data-clear-picker')) { $('[data-picker-search]', dialog).reset(); filterPicker(dialog); }
    if (button.hasAttribute('data-apply-picker')) {
      const selected = $('input[name="proxy-choice"]:checked',dialog); if (!selected) return;
      const name = selected.value; closeDialog(dialog); notify('演示已选择“'+name+'”，未更改真实代理。');
    }
    if (button.hasAttribute('data-reset-empty')) { $('#empty-case').value = 'empty'; renderFeedback(); }
    if (button.hasAttribute('data-next-batch')) { dialog._batchItems = dialog._batchNames ? dialog._batchNames.map((name,i)=>({name,status:itemsFor('partial')[i%8].status})) : itemsFor('partial'); paintBatchDialog(dialog); }
    if (button.hasAttribute('data-retry-failed') || button.hasAttribute('data-verify-items')) {
      const from = button.hasAttribute('data-retry-failed') ? 'failed' : 'unconfirmed';
      dialog._batchItems = dialog._batchItems.map(x => x.status === from ? {...x,status:'success'} : x);
      paintBatchDialog(dialog); notify(from === 'failed' ? '仅失败项已切换为演示成功；待确认项保持不变。' : '待确认项已切换为演示核实结果，未重复写入。');
    }
    if (button.hasAttribute('data-show-code')) {
      const section = $('[data-scene="'+button.dataset.showCode+'"]');
      const clone = section.cloneNode(true); clone.removeAttribute('hidden');
      $$('.ex-reference,.ex-demo-controls',clone).forEach(e=>e.remove());
      const d = openExample('code',undefined,button); $('#example-code',d).value = clone.outerHTML;
    }
    if (button.hasAttribute('data-copy-code')) {
      const area = $('#example-code',dialog); area.focus(); area.select();
      let copied = false;
      try { await navigator.clipboard.writeText(area.value); copied = true; } catch { copied = document.execCommand('copy'); }
      $('[data-copy-status]',dialog).textContent = copied ? '结构已复制；配套样式与行为见使用说明。' : '已选中结构，请使用系统复制快捷键。';
    }
  });
  document.addEventListener('change', event => {
    const input = event.target;
    if (input.matches('[data-check-all]')) { $$('[data-resource-row]').filter(e=>!e.hidden).forEach(e=>{ $('[data-row-check]',e).checked=input.checked; }); updateSelection(); }
    if (input.matches('[data-row-check]')) updateSelection();
    if (input.id === 'detail-density' || input.id === 'detail-width') applyDetail();
    if (input.id === 'batch-case') renderPreviewBatch();
    if (input.id === 'empty-case') renderFeedback();
    if (input.name === 'proxy-choice') {
      const dialog = input.closest('dialog'); $('[data-apply-picker]',dialog).disabled = false;
      $('[data-picker-selected]',dialog).textContent = '已选：' + input.value;
    }
    if (input.id === 'picker-region') filterPicker(input.closest('dialog'));
    const receipt = input.closest('[data-local-form]')?.querySelector('[data-form-receipt]');
    if (receipt) receipt.hidden = true;
  });
  function filterPicker(dialog) {
    const q = $('#picker-q',dialog).value.toLowerCase(), region = $('#picker-region',dialog).value;
    let count = 0;
    $$('[data-proxy-row]',dialog).forEach(row=>{
      row.hidden = !(row.textContent.toLowerCase().includes(q) && (!region || row.dataset.region === region));
      if (!row.hidden) count++;
    });
    $('[data-picker-results]',dialog).hidden = count === 0; $('[data-picker-empty]',dialog).hidden = count > 0;
  }
  document.addEventListener('input', event => {
    const anyReceipt = event.target.closest('[data-local-form]')?.querySelector('[data-form-receipt]');
    if (anyReceipt) anyReceipt.hidden = true;
    if (event.target.id === 'picker-q') filterPicker(event.target.closest('dialog'));
    if (event.target.name === 'name') {
      const field = event.target.closest('.ex-field'); const error = $('.ex-error',field);
      event.target.removeAttribute('aria-invalid'); if (error) error.hidden = true;
      const receipt = $('[data-form-receipt]',event.target.closest('form')); if (receipt) receipt.hidden = true;
    }
  });
  document.addEventListener('submit', event => {
    if (event.target.matches('[data-picker-search]')) { event.preventDefault(); return; }
    if (!event.target.matches('[data-local-form]')) return;
    event.preventDefault(); const form = event.target, name = $('[name="name"]',form);
    if (!name.value.trim()) {
      name.setAttribute('aria-invalid','true'); $('.ex-error',name.closest('.ex-field')).hidden = false; name.focus(); return;
    }
    if (!form.checkValidity()) { form.reportValidity(); return; }
    $('[data-form-receipt]',form).hidden = false;
  });
  $('#example-toast')?.addEventListener('click', e => { e.currentTarget.textContent = ''; });
  const swatches = $('[data-swatches]');
  if (swatches) {
    const colors = [['primary','品牌蓝'],['bg-page','页面背景'],['bg-subtle','浅层背景'],['text-primary','主要文字'],['text-secondary','次级文字'],['success-text','成功文字'],['warning-text','警示文字'],['danger-text','错误文字']];
    swatches.innerHTML = colors.map(([name,label])=>'<div class="ex-swatch"><i aria-hidden="true" style="--swatch:var(--wt-color-'+name+')"></i><div>'+label+'<small>'+esc(getComputedStyle(document.documentElement).getPropertyValue('--wt-color-'+name).trim())+' · '+name+'</small></div></div>').join('');
  }
  renderFeedback(); renderPreviewBatch(); applyDetail();
  // 原型可显式使用此演示入口；它不是业务组件API。
  window.WTExamples = {open:openExample, close:()=>closeDialog(openDialogs.at(-1))};
})();
