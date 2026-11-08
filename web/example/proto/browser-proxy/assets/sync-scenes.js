/* 当前同步窗口评审稿。仅演示视图，不调用服务、不以计时器冒充业务完成。 */
(() => {
  const group = '同步窗口 · 完整状态';
  const definitions = [
    ['w-sync-reading', '01 · 正在读取本机窗口', 'reading'],
    ['w-sync-comparing', '02 · 正在比对窗口信息', 'comparing'],
    ['w-sync-preview', '03 · 新增与变更确认', 'preview'],
    ['w-sync-changes', '04 · 仅有字段变化', 'changes'],
    ['w-sync-ownership', '05 · 新窗口归属冲突', 'ownership'],
    ['w-sync-applying', '06 · 正在保存同步结果', 'applying'],
    ['w-sync', '07 · 同步完成', 'success'],
    ['w-sync-issues', '08 · 完成后有异常待处理', 'issues'],
    ['w-sync-nochange', '09 · 没有变化', 'nochange'],
    ['w-sync-empty', '10 · 比特中没有窗口', 'empty'],
    ['w-sync-offline', '11 · 本机服务或比特未连接', 'offline'],
    ['w-sync-identity', '12 · 比特主账号不一致', 'identity'],
    ['w-sync-submit-error', '13 · 比对请求失败', 'submit-error'],
    ['w-sync-expired', '14 · 扫描结果已过期', 'expired'],
    ['w-sync-forbidden', '15 · 操作权限已变化', 'forbidden'],
    ['w-sync-apply-error', '16 · 明确保存失败', 'apply-error'],
    ['w-sync-uncertain', '17 · 保存响应丢失，结果待核实', 'uncertain'],
    ['w-sync-verifying', '18 · 正在核实已保存结果', 'verifying'],
    ['w-sync-reload-error', '19 · 已保存，列表刷新失败', 'reload-error'],
  ];
  const ids = new Set(definitions.map(([id]) => id));
  // 保留原23/24编号与深链接；新状态聚在一处，避免分散于通用批量示例。
  const oldSync = SCENES.find(s => s.id === 'w-sync');
  const position = SCENES.indexOf(oldSync);
  const retained = SCENES.filter(s => !ids.has(s.id));
  const scenes = definitions.map(([id, name, state], i) => ({
    id, name, state, group, module: 'window', view: 'syncReview', role: 'operator',
    originalIndex: id === 'w-sync' ? 23 : id === 'w-sync-issues' ? 24 : 'S' + String(i + 1).padStart(2, '0'),
  }));
  retained.splice(position, 0, ...scenes);
  SCENES.splice(0, SCENES.length, ...retained);

  let selected = new Set();
  let activeTab = 'new';
  let issueTab = 'pending';
  let previewOrigin = 'preview';
  let returnScene = '';
  let resultModel = { added: 1, updated: 2, missing: 1, skipped: 2 };
  const added = [
    { key: 'bit-new-1', name: 'B站新号 · 上海05', bit: 'b71a…0e32', group: '主力账号组' },
    { key: 'bit-new-2', name: '百度 · 杭州03', bit: 'c82b…1f43', group: '备用账号组' },
    { key: 'bit-new-3', name: 'B站备用 · 北京01', bit: 'd93c…2a54', group: '华北运营组', conflict: true },
  ];
  const changes = [
    ['328', 'B站主号 · 上海01', '窗口名称', 'B站主号', 'B站主号 · 上海01'],
    ['329', '百度 · 上海02', '比特分组、备注', '备用账号组 · 备用窗口', '主力账号组 · 日常发布'],
  ];
  const pending = [
    ['332', 'B站备用 · 华东03', '代理配置不一致', 'w-mismatch'],
    ['335', 'B站主号 · 广州01', '代理已到期', 'w-replace'],
    ['341', 'B站备用 · 南京02', '本机窗口缺失', 'w-missing'],
    ['348', 'B站备用 · 南京03', '历史未找到', 'w-missing', 'history'],
  ];
  const button = (label, action, style = '', disabled = false) => btn(label, '!sync-review-' + action, style, disabled);
  const head = (title, text, tone = 'green') => resultHead(title, text, tone);
  const table = (headers, rows) => `<div class="sync-table-scroll"><table class="dense sync-table"><thead><tr>${headers.map(t => `<th>${t}</th>`).join('')}</tr></thead><tbody>${rows.map(cells => `<tr>${cells.map(c => `<td>${c}</td>`).join('')}</tr>`).join('')}</tbody></table></div>`;
  const metric = items => `<div class="sync-metrics">${items.map(([value, label]) => `<div><strong>${value}</strong><span>${label}</span></div>`).join('')}</div>`;
  const note = text => `<p class="sync-note">${text}</p>`;
  function shell(title, body, footer, compact = false, busy = false) {
    return v3Dialog(title, body, footer, compact ? 'compact sync-review' : 'wide sync-review')
      .replace('aria-modal="true"', 'aria-modal="true" aria-describedby="sync-review-description"')
      .replace('class="dialog-body"', 'class="dialog-body" id="sync-review-description"')
      .replace('class="close"', busy ? 'class="close" disabled title="请等待当前步骤结束"' : 'class="close"');
  }
  function tabs(items, active, prefix = 'tab') {
    return `<div class="sync-tabs" role="tablist" aria-label="同步内容">${items.map(([key, label, count]) => `<button class="sync-tab ${active === key ? 'active' : ''}" role="tab" aria-selected="${active === key}" data-action="sync-review-${prefix}-${key}">${label}<span>${count}</span></button>`).join('')}</div>`;
  }
  function review(state) {
    const onlyChanges = state === 'changes', onlyConflict = state === 'ownership';
    const active = onlyChanges ? 'changed' : onlyConflict ? 'new' : activeTab;
    const fresh = onlyConflict ? added.filter(r => r.conflict) : added;
    let body = `<div class="sync-intro"><h2>${onlyConflict ? '有 1 个窗口已归属其他运营' : '发现需要同步的窗口变化'}</h2><p>新增窗口由你选择关联，已有窗口信息一并更新。</p></div>`;
    body += metric(onlyChanges ? [[8, '比特窗口'], [2, '信息变化']] : onlyConflict ? [[8, '比特窗口'], [1, '归属冲突']] : [[8, '比特窗口'], [3, '新发现'], [2, '信息变化'], [1, '本机缺失']]);
    body += tabs(onlyChanges ? [['changed', '信息变化', 2]] : onlyConflict ? [['new', '新发现', 1]] : [['new', '新发现', 3], ['changed', '信息变化', 2], ['missing', '本机缺失', 1]], active);
    if (active === 'new') {
      body += `<div class="sync-list-caption"><span>${onlyConflict ? '归属冲突的窗口不可选择' : `已选择 <strong>${selected.size}</strong> / 2 个可关联窗口`}</span><span>未勾选的窗口不会关联</span></div>`;
      body += table(['选择', '窗口名称 / 比特窗口 ID', '比特分组', '关联状态'], fresh.map(r => [
        `<input type="checkbox" class="check" data-sync-select="${r.key}" aria-label="关联 ${r.name}" ${selected.has(r.key) && !r.conflict ? 'checked' : ''} ${r.conflict ? 'disabled' : ''}>`,
        `<strong>${r.name}</strong><small class="sync-secondary mono">${r.bit}</small>`, r.group,
        r.conflict ? badge('已归属其他运营', 'orange') : badge('待选择'),
      ]));
    } else if (active === 'changed') {
      body += table(['ID', '窗口', '变化内容'], changes.map(r => [r[0], r[1], `<span>${r[2]}</span><small class="sync-secondary">${r[3]} → ${r[4]}</small>`]));
    } else {
      body += table(['ID', '窗口', '同步后的状态'], [['341', 'B站备用 · 南京02', badge('本机缺失', 'orange')]]);
      body += note('仅标记缺失，保留窗口记录与关联账号。请先确认比特账号及窗口访问权限。');
    }
    body += note(onlyConflict ? '不会自动转移归属。如需使用，请联系管理员核实。' : '同步不修改运营备注、账号关联和业务启停，也不自动写入代理。');
    const footer = onlyConflict ? button('完成', 'close', 'primary') : button('取消', 'close') + button(onlyChanges ? '更新 2 个窗口' : `确认同步 · 关联 ${selected.size} 个新窗口`, 'confirm', 'primary');
    return shell('同步窗口', body, footer);
  }
  function completed(issues) {
    const model = resultModel;
    const updates = model.updated + model.added;
    const freshPending = pending.filter(row => row[4] !== 'history').length;
    let body = head(issues ? `同步完成，本次 ${freshPending} 个窗口待处理` : '同步完成', issues ? '本次异常请分别处理；另有 1 个历史未处理的缺失窗口。' : '窗口名称、分组和比特备注已更新。', issues ? 'orange' : 'green');
    body += metric([[8, '比特窗口'], [updates, '已同步'], [issues ? freshPending : 0, '本次待处理'], [1, '历史待处理']]);
    body += `<p class="sync-meta">新关联 ${model.added} · 信息更新 ${model.updated} · 标记缺失 ${model.missing} · 未关联 ${model.skipped}</p>`;
    if (issues) body += tabs([['pending', '待处理', pending.length], ['updated', '已同步', updates]], issueTab, 'result');
    if (issues && issueTab === 'pending') {
      body += table(['ID', '窗口', '来源', '异常', '操作'], pending.map(r => [r[0], r[1], r[4] === 'history' ? '历史待处理' : '本次', badge(r[2], 'orange'), r[4] === 'history' ? `<button class="btn text" data-action="sync-review-handle" data-target="${r[3]}" data-window-id="${r[0]}">核实</button><button class="btn text danger-text" data-action="cleanup-one-1">删除</button>` : `<button class="btn text" data-action="sync-review-handle" data-target="${r[3]}" data-window-id="${r[0]}">处理</button>`]));
    } else {
      const rows = changes.map(r => [r[0], r[1], r[2] + '已更新', badge('已同步', 'green')]);
      if (model.added) rows.unshift(['346', 'B站新号 · 上海05', '已建立关联', badge('已同步', 'green')]);
      if (model.added > 1) rows.splice(1, 0, ['347', '百度 · 杭州03', '已建立关联', badge('已同步', 'green')]);
      body += table(['ID', '窗口', '更新内容', '结果'], rows);
    }
    return shell('同步结果', body, button('完成', 'close', 'primary'));
  }
  function view(state) {
    if (['preview', 'changes', 'ownership'].includes(state)) return review(state);
    if (['success', 'issues'].includes(state)) return completed(state === 'issues');
    if (state === 'verifying') return shell('核实同步结果', `<div class="sync-wait" role="status"><span class="sync-spinner" aria-hidden="true"></span><h2>正在核实同步结果</h2><p>读取系统记录，确认已保存内容；本次不重复提交同步。</p></div>` + note('核实已保存 → 显示结果；明确未保存 → 返回确认；仍无法判断 → 保持待核实。'), '<span class="sync-footer-text">正在读取系统记录</span>' + btn('核实中', '', 'primary', true), true, true);
    if (['reading', 'comparing', 'applying'].includes(state)) {
      const applying = state === 'applying';
      const title = applying ? '正在保存同步结果' : state === 'reading' ? '正在读取本机窗口' : '正在比对窗口信息';
      const body = `<div class="sync-wait" role="status"><span class="sync-spinner" aria-hidden="true"></span><h2>${title}</h2><p>${applying ? '请等待保存结果，避免重复提交。' : '核对比特主账号后，读取窗口并与系统记录比对。'}</p></div><ol class="sync-steps"><li class="${state === 'reading' ? 'active' : 'done'}"><b>${state === 'reading' ? '1' : '✓'}</b>读取比特窗口</li><li class="${state === 'comparing' ? 'active' : applying ? 'done' : ''}"><b>${applying ? '✓' : '2'}</b>比对窗口变化</li><li class="${applying ? 'active' : ''}"><b>3</b>确认并保存</li></ol>`;
      return shell('同步窗口', body, '<span class="sync-footer-text">请等待当前步骤完成</span>' + btn('<span class="spin"></span> 处理中', '', 'primary', true), true, true);
    }
    if (state === 'nochange') return shell('同步窗口', head('窗口信息已是最新', '已读取 8 个比特窗口，本次没有新增变化或异常。') + note('另有 1 个历史未处理的缺失窗口；可核实后决定是否删除。') + table(['ID', '窗口', '来源', '操作'], [['348', 'B站备用 · 南京03', '历史待处理', '<button class="btn text" data-action="sync-review-handle" data-target="w-missing" data-window-id="348">核实</button><button class="btn text danger-text" data-action="cleanup-one-1">删除</button>']]), button('完成', 'close', 'primary'), true);
    if (state === 'empty') return shell('同步窗口', head('无法核实当前比特主账号', '比特未返回任何窗口，当前主账号身份无法从窗口记录中确认。', 'orange') + note('本次不会把系统中的窗口标记为缺失。请确认比特已登录正确主账号，或在比特中恢复窗口后重新同步。'), button('关闭', 'close') + button('重新读取', 'restart', 'primary'), true);
    const errors = {
      offline: ['无法读取本机窗口', '本机服务或比特未连接，请确认已启动并登录。', '尚未提交窗口快照，系统记录未修改。', '重新读取', 'restart', 'red'],
      identity: ['比特主账号不一致', '当前登录的主账号与系统绑定不一致，本次同步已停止。', '请切回已绑定的比特主账号；换绑请到个人信息处理。', '重新检测', 'restart', 'orange'],
      'submit-error': ['窗口比对失败', '已读取本机窗口，未能获取系统比对结果。', '请检查网络后重新同步；本次尚未保存窗口变化。', '重新同步', 'restart', 'red'],
      expired: ['扫描结果已过期', '窗口信息可能已变化，请重新扫描后再确认。', '重新扫描后需再次选择新增窗口，不沿用旧结果提交。', '重新扫描', 'restart', 'orange'],
      forbidden: ['当前无法继续同步', '操作权限或资源归属已发生变化。', '请重新加载页面确认当前权限，或联系管理员。', '返回列表', 'close', 'orange'],
      'apply-error': ['同步未保存', '系统明确返回保存失败，本次变化未提交成功。', '保留了本次选择。扫描仍有效时可重新确认，过期后需重新扫描。', '返回确认', 'back', 'red'],
      uncertain: ['同步结果待核实', '保存请求已发送，但没有收到明确结果。', '先核实系统已保存的窗口记录，避免重复认领或重复提交。', '核实结果', 'verify', 'orange'],
      'reload-error': ['同步已保存，列表未能刷新', '系统已明确确认保存成功，列表暂时未加载完成。', '只需重新加载列表，无需再次同步或提交。', '重新加载列表', 'reload', 'orange'],
    };
    const [title, description, hint, label, action, tone] = errors[state];
    let body = head(title, description, tone);
    if (state === 'identity') body += `<div class="sync-identity"><div><span>系统已绑定</span><strong>18••••••26</strong></div><div><span>当前检测</span><strong>19••••••37</strong></div></div>`;
    body += note(hint);
    return shell('同步窗口', body, button('关闭', 'close') + button(label, action, 'primary'), true);
  }

  const priorView = v3View;
  v3View = s => ids.has(s.id) ? view(s.state) : priorView(s);
  const priorGo = v3Go;
  v3Go = function(id, push = true, options = {}) {
    if (ids.has(id) && (options.catalog || !push)) {
      selected = new Set(); activeTab = 'new'; issueTab = 'pending';
      previewOrigin = id === 'w-sync-changes' ? 'changes' : 'preview';
      resultModel = { added: 1, updated: 2, missing: id === 'w-sync' ? 0 : 1, skipped: 2 };
    }
    priorGo(id, push, options);
  };
  window.designGo = v3Go;
  const priorAction = v3Action;
  v3Action = function(action, target) {
    if (action === 'v3-sync') { v3Go('w-sync-reading'); return; }
    if (!action.startsWith('sync-review-')) {
      if (action === 'v3-close' && ids.has(v3Current.id) && ['reading', 'comparing', 'applying', 'verifying'].includes(v3Current.state)) return;
      priorAction(action, target); return;
    }
    const command = action.slice('sync-review-'.length);
    if (command.startsWith('tab-')) { activeTab = command.slice(4); v3Render(); return; }
    if (command.startsWith('result-')) { issueTab = command.slice(7); v3Render(); return; }
    if (command === 'close') { v3Go(v3Base('window')); return; }
    if (command === 'restart') { selected = new Set(); v3Go('w-sync-reading'); return; }
    if (command === 'back') { v3Go(previewOrigin === 'changes' ? 'w-sync-changes' : 'w-sync-preview'); return; }
    if (command === 'confirm') {
      previewOrigin = v3Current.state;
      resultModel = { added: previewOrigin === 'changes' ? 0 : selected.size, updated: 2, missing: previewOrigin === 'changes' ? 0 : 1, skipped: previewOrigin === 'changes' ? 0 : 3 - selected.size };
      v3Go('w-sync-applying'); return;
    }
    // 核实不伪造成功；由画布外的状态导航选择明确的评审结果。
    if (command === 'verify') { v3Go('w-sync-verifying'); return; }
    if (command === 'reload') { v3Go(v3Base('window')); return; }
    if (command === 'next') {
      const next = {reading:'w-sync-comparing',comparing:'w-sync-preview',applying:previewOrigin === 'changes' ? 'w-sync' : 'w-sync-issues'};
      if (next[v3Current.state]) v3Go(next[v3Current.state]);
      return;
    }
    if (command === 'handle') {
      returnScene = v3Current.id;
      const index = V3_WINDOWS.findIndex(r => String(r.id) === target.dataset.windowId);
      v3Go(target.dataset.target);
      v3Focus = Math.max(0, index); v3Render(); return;
    }
    if (command === 'return') { v3Go(returnScene || 'w-sync-issues'); return; }
  };
  // 比旧change监听先捕获，避免旧事件拦截；选择只影响新增认领数量。
  document.addEventListener('input', event => {
    const checkbox = event.target.closest('[data-sync-select]');
    if (!checkbox || checkbox.disabled) return;
    if (checkbox.checked) selected.add(checkbox.dataset.syncSelect);
    else selected.delete(checkbox.dataset.syncSelect);
    v3Render();
  }, true);
  const priorRender = v3Render;
  v3Render = function() {
    priorRender();
    const stage = document.getElementById('stage');
    stage.classList.toggle('sync-stage', ids.has(v3Current.id));
    if (ids.has(v3Current.id)) {
      const navigation = document.createElement('div');
      navigation.className = 'sync-scene-navigation';
      navigation.innerHTML = `<label for="sync-review-state">同步状态预览</label><select id="sync-review-state" class="input" aria-label="同步状态预览">${definitions.map(([id, name]) => `<option value="${id}" ${id === v3Current.id ? 'selected' : ''}>${name}</option>`).join('')}</select>${['reading','comparing','applying'].includes(v3Current.state) ? button('预览下一步', 'next') : ''}<span>评审导航 · 非产品控件</span>`;
      document.getElementById('book-header').append(navigation);
      navigation.querySelector('select').addEventListener('change', event => v3Go(event.target.value));
    } else if (returnScene && ['w-mismatch', 'w-replace', 'w-missing'].includes(v3Current.id)) {
      stage.querySelector('.dialog-footer,.drawer-footer')?.insertAdjacentHTML('afterbegin', button('返回同步结果', 'return'));
    }
  };
  const groups = [...new Set(SCENES.map(s => s.group))];
  document.getElementById('catalog').innerHTML = `<div class="book-brand">运营资源 · 视觉全案</div><div class="book-eyebrow">PRODUCT EXPERIENCE</div><div class="book-count">${SCENES.length} 个场景 · 同步窗口完整状态<br>纯本地评审，不连接真实服务</div>${groups.map(g => `<div class="catalog-group">${g}</div>${SCENES.filter(s => s.group === g).map(s => `<button class="catalog-item" data-go="${s.id}"><span class="catalog-num">${s.originalIndex}</span>${s.name}</button>`).join('')}`).join('')}`;
  window.DESIGN_SCENES = SCENES;
  v3Go(new URLSearchParams(location.search).get('scene') || 'w-list', false);
})();
