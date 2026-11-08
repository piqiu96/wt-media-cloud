/* CHG099 最终评审决策：缺失窗口单独清理，日常列表不加记录筛选。纯原型。 */
(() => {
  function keepSupplierInDetails(html) {
    return html
      .replace('<th>地区 / 供应商</th>', '<th>地区</th>')
      .replace(/<td>([^<]*)<div class="tiny muted">[^<]*<\/div><\/td>/g, '<td>$1</td>');
  }

  const originalProxyTable = proxyTable;
  proxyTable = function() {
    return keepSupplierInDetails(originalProxyTable());
  };

  const originalWindowFilters = v3WindowFilters;
  v3WindowFilters = function() {
    let html = originalWindowFilters();
    html = html.replace(
      /<select class="input" data-v3-team>[\s\S]*?<\/select>/,
      '<input class="input" data-v3-team-search placeholder="输入团队名称或 ID">',
    );
    html = html.replace('<div class="field narrow v3-runtime-filter">', '<div class="field medium window-state-filter"><label>窗口状态</label><div class="window-state-control"><select class="input" data-v3-window-state aria-label="窗口状态"><option value="" hidden>全部窗口</option><option value="found">已找到</option><option value="missing">未找到</option></select><button type="button" class="window-state-clear" data-action="clear-window-state" aria-label="清除窗口状态筛选" hidden>×</button></div></div><div class="field narrow v3-runtime-filter">');
    return html;
  };

  const originalProxyPage = v3ProxyPage;
  v3ProxyPage = function() {
    let html = keepSupplierInDetails(originalProxyPage());
    html = html.replace(
      /<select class="input" data-v3-team>[\s\S]*?<\/select>/,
      '<input class="input" data-v3-team-search placeholder="输入团队名称或 ID">',
    );
    html = html.replace(
      '<div class="field medium"><label>运营团队</label><input class="input" readonly value="星河运营组"></div>',
      '',
    );
    return html;
  };

  const originalFilter = v3Filter;
  v3Filter = function() {
    originalFilter();
    const stage = document.getElementById('stage');
    const team = stage.querySelector('[data-v3-team-search]')?.value.trim().toLowerCase() || '';
    const windowState = stage.querySelector('[data-v3-window-state]')?.value || '';
    stage.querySelectorAll('tr[data-v3-row], tr[data-v3-proxy-row]').forEach(row => {
      if (row.hidden) return;
      const windowIndex = row.dataset.v3Row;
      const item = windowIndex === undefined ? V3_PROXIES[Number(row.dataset.v3ProxyRow)] : V3_WINDOWS[Number(windowIndex)];
      if (team && !String(item.team || '').toLowerCase().includes(team)) row.hidden = true;
      if (windowIndex !== undefined && cleanedIds.has(item.id)) row.hidden = true;
      if (windowIndex !== undefined && windowState && (item.issue === 'missing' ? 'missing' : 'found') !== windowState) row.hidden = true;
    });
    v3TableMeta();
    const visibleWindows = [...stage.querySelectorAll('tr[data-v3-row]')].filter(row => !row.hidden).map(row => V3_WINDOWS[Number(row.dataset.v3Row)]);
    v3Eligible = [...stage.querySelectorAll('tr[data-v3-row]')].filter(row => !row.hidden).map(row => Number(row.dataset.v3Row));
    const clear = stage.querySelector('[data-action="clear-window-state"]');
    if (clear) clear.hidden = !windowState;
    const normal = stage.querySelector('.v3-stats [data-value="normal"] strong');
    const pending = stage.querySelector('.v3-stats [data-value="pending"] strong');
    if (normal && pending) {
      normal.textContent = visibleWindows.filter(item => item.issue === 'normal').length;
      pending.textContent = visibleWindows.filter(item => item.issue !== 'normal').length;
    }
  };

  SCENES.push({ id: 'w-cleanup', originalIndex: 'C1', name: '清理比特已删除的窗口', group: '浏览器窗口', module: 'window', view: 'cleanup', baseRole: 'operator' });
  SCENES.push({ id: 'w-cleanup-confirm', originalIndex: 'C2', name: '删除前确认影响', group: '浏览器窗口', module: 'window', view: 'cleanupConfirm', baseRole: 'operator' });
  SCENES.push({ id: 'w-cleanup-result', originalIndex: 'C3', name: '批量清理结果', group: '浏览器窗口', module: 'window', view: 'cleanupResult', baseRole: 'operator' });
  SCENES.push({ id: 'w-proxy-info-managed', originalIndex: 'C4', name: '窗口托管代理详情', group: '浏览器窗口', module: 'window', view: 'managedProxyInfo', baseRole: 'operator' });
  SCENES.push({ id: 'w-proxy-info-self', originalIndex: 'C5', name: '比特自管代理信息', group: '浏览器窗口', module: 'window', view: 'selfProxyInfo', baseRole: 'operator' });
  V3_WINDOWS.push({ ...V3_WINDOWS.find(item => item.issue === 'missing'), id: 348, name: 'B站备用 · 南京03', issue: 'missing', note: '历史未处理窗口' });
  const missingSamples = V3_WINDOWS.filter(item => item.issue === 'missing');
  const cleanedIds = new Set();
  v3Stats = function() {
    const rows = V3_WINDOWS.filter(item => !cleanedIds.has(item.id) && (v3Team === 'all' || item.team === v3Team));
    const pending = rows.filter(item => item.issue !== 'normal').length;
    return `<div class="stats v3-stats"><button class="stat clickable" data-v3="stat" data-value="normal"><span><label>正常</label><strong>${rows.length - pending}</strong></span>${icon('CheckCircle')}</button><button class="stat clickable" data-v3="stat" data-value="pending"><span><label>待处理</label><strong class="orange">${pending}</strong></span>${icon('ErrorCircle')}</button></div>`;
  };
  let cleanupSelection = [];
  const originalView = v3View;
  v3View = function(scene) {
    if (scene.view === 'managedProxyInfo') {
      const windowItem = V3_WINDOWS[v3Focus];
      const proxy = V3_PROXIES.find(item => item.name === windowItem.proxyName);
      return v3Drawer('托管代理详情', (windowItem.issue === 'missing' ? notice('该窗口最近同步未找到', '以下窗口配置来自上次同步，不能视为当前配置。', 'orange') : '') + section('代理资源', facts([['代理名称', proxy?.name || windowItem.proxyName], ['代理 ID', proxy?.id || '—'], ['协议', proxy?.protocol || '—'], ['资源地址', proxy?.address || '—'], ['地区', proxy?.region || '—'], ['供应商', proxy?.supplier || '—'], ['有效期至', proxy?.expiry || '—'], ['最近检测', proxy?.check === 'ok' ? '连接正常' : proxy?.check === 'expired' ? '已到期' : proxy?.check === 'fail' ? '连接失败' : '未检测'], ['已用 / 最大窗口数', proxy ? `${proxy.used} / ${proxy.max}` : '—']])) + section('当前窗口', facts([['窗口', windowItem.name], ['上次同步的代理地址', windowItem.address], ['配置结果', windowItem.issue === 'missing' ? '待窗口恢复后核实' : windowItem.issue === 'mismatch' ? '配置不一致' : '见窗口详情']])), '', true);
    }
    if (scene.view === 'selfProxyInfo') {
      const windowItem = V3_WINDOWS[v3Focus === 0 ? 1 : v3Focus];
      return v3Dialog('比特自管代理信息', (windowItem.issue === 'missing' ? notice('该窗口最近同步未找到', '以下代理信息来自上次同步，不能视为当前配置。', 'orange') : '') + facts([['窗口', windowItem.name], ['管理方式', '比特自管'], ['最近同步的代理地址', windowItem.address], ['最近同步', '2026-10-28 10:32']]), btn('关闭', v3Base('window')));
    }
    if (!scene.view.startsWith('cleanup')) return originalView(scene);
    if (scene.view === 'cleanupResult') return v3Dialog('清理结果', resultHead('窗口记录已删除', '账号保留并解除关联，托管代理名额已释放。') + facts([['已删除窗口', cleanupSelection.length + ' 个'], ['关联账号', '保留，待重新绑定'], ['技术追踪', '已记录操作审计']]), btn('完成', 'w-list', 'primary'));
    if (scene.view === 'cleanupConfirm') return v3Dialog('确认删除窗口记录', notice('请先在比特确认所选窗口确实已删除', '若只是切换主账号或权限变化，先恢复访问再同步。', 'orange') + facts([['窗口记录', cleanupSelection.length + ' 条'], ['关联账号', '保留账号，解除窗口关联'], ['托管代理', '释放占用名额']]) + '<label class="cleanup-ack"><input type="checkbox" data-cleanup-ack> 我已确认所选窗口在比特中删除</label>', btn('返回', 'w-cleanup') + btn('删除', '!cleanup-confirm', 'danger-solid', true));
    return v3Dialog('清理已删除窗口', notice('最近一次成功同步中未找到以下窗口。这里只清理系统记录，不操作比特。', '请先确认比特登录账号及窗口权限。') + '<div class="table-wrap"><table><thead><tr><th>选择</th><th>ID</th><th>窗口名称</th><th>关联账号</th><th>操作</th></tr></thead><tbody>' + missingSamples.filter(item => !cleanedIds.has(item.id)).map(item => { const index = missingSamples.indexOf(item); return `<tr><td><input type="checkbox" data-cleanup-select="${index}" ${cleanupSelection.includes(index) ? 'checked' : ''}></td><td>${item.id}</td><td>${item.name}</td><td>${v3Accounts(V3_WINDOWS.indexOf(item)).length} 个</td><td>${btn('删除', '!cleanup-one-' + index, 'text danger-text')}</td></tr>`; }).join('') + '</tbody></table></div>', btn('关闭', 'w-list') + btn('批量删除', '!cleanup-batch', 'danger-outline', !cleanupSelection.length));
  };
  const originalRender = v3Render;
  v3Render = function() {
    originalRender();
    const stage = document.getElementById('stage');
    if (v3Current.module !== 'window') return;
    const table = stage.querySelector('table.window-v3');
    if (table) {
      const headings = [...table.querySelectorAll('thead th')];
      const stateBeforeIndex = headings.findIndex(item => item.textContent === '运行状态' || item.textContent === '最近同步');
      const proxyIndex = headings.findIndex(item => item.textContent === '代理信息');
      if (stateBeforeIndex >= 0) {
        headings[stateBeforeIndex].insertAdjacentHTML('beforebegin', '<th>窗口状态</th>');
        table.querySelector('colgroup')?.children[stateBeforeIndex]?.insertAdjacentHTML('beforebegin', '<col style="width:112px">');
      }
      table.querySelectorAll('tbody tr[data-v3-row]').forEach(row => {
        const windowItem = V3_WINDOWS[Number(row.dataset.v3Row)];
        if (stateBeforeIndex >= 0) row.children[stateBeforeIndex]?.insertAdjacentHTML('beforebegin', `<td>${badge(windowItem.issue === 'missing' ? '未找到' : '已找到', windowItem.issue === 'missing' ? 'orange' : 'green')}</td>`);
        if (windowItem.issue === 'missing') {
          const actions = row.querySelector('.row-actions');
          actions?.querySelectorAll('.btn').forEach(button => {
            if (['打开', '关闭', '编辑', '处理异常', '管理代理'].includes(button.textContent.trim())) button.remove();
          });
          if (v3Local()) actions?.insertAdjacentHTML('beforeend', btn('核实缺失', 'w-missing', 'text'));
          if (v3Local() || v3Role() === 'admin') actions?.insertAdjacentHTML('beforeend', btn('删除', '!cleanup-one-' + missingSamples.indexOf(windowItem), 'text danger-text'));
        }
        const proxyCell = row.children[proxyIndex];
        if (!proxyCell || (windowItem.mode === 'self' && windowItem.address === '—')) return;
        const lines = [...proxyCell.children].filter(item => item.tagName === 'DIV').slice(0, 2);
        const link = document.createElement('button');
        link.type = 'button';
        link.className = 'window-proxy-link';
        link.dataset.action = 'view-proxy-info';
        link.setAttribute('aria-label', `查看${windowItem.name}的代理信息`);
        proxyCell.prepend(link);
        lines.forEach(item => link.appendChild(item));
      });
      v3TableMeta();
    }
    const remainingMissing = missingSamples.filter(item => !cleanedIds.has(item.id)).length;
    if (remainingMissing && v3Current.view === 'list') {
      stage.querySelector('.page-header')?.insertAdjacentHTML('afterend', `<div class="cleanup-entry">最近一次同步有 ${remainingMissing} 个窗口未在比特中找到。${btn('清理已删除窗口', 'w-cleanup', 'text')}</div>`);
    }
  };
  const originalAction = v3Action;
  v3Action = function(action, target) {
    if (action === 'view-proxy-info') { const item = V3_WINDOWS[v3Focus]; if (item.mode === 'self' && item.address === '—') return; v3Go(item.mode === 'system' ? 'w-proxy-info-managed' : 'w-proxy-info-self'); return; }
    if (action === 'clear-window-state') { const field = document.querySelector('[data-v3-window-state]'); if (field) field.value = ''; v3Filter(); return; }
    if (action === 'v3-reset') { const field = document.querySelector('[data-v3-window-state]'); if (field) field.value = ''; }
    if (action.startsWith('cleanup-one-')) { cleanupSelection = [Number(action.split('-').pop())]; v3Go('w-cleanup-confirm'); return; }
    if (action === 'cleanup-batch') { if (cleanupSelection.length) v3Go('w-cleanup-confirm'); return; }
    if (action === 'cleanup-confirm') { cleanupSelection.forEach(index => cleanedIds.add(missingSamples[index].id)); v3Go('w-cleanup-result'); return; }
    originalAction(action, target);
  };
  document.addEventListener('click', event => {
    if (event.target.matches('[data-cleanup-select]')) {
      const index = Number(event.target.dataset.cleanupSelect);
      cleanupSelection = event.target.checked ? [...new Set([...cleanupSelection, index])] : cleanupSelection.filter(value => value !== index);
      v3Render();
    }
    if (event.target.matches('[data-cleanup-ack]')) document.querySelector('[data-action="cleanup-confirm"]').disabled = !event.target.checked;
  }, true);
  window.addEventListener('change', event => {
    if (event.target.matches('[data-v3-window-state]')) {
      const clear = document.querySelector('[data-action="clear-window-state"]');
      if (clear) clear.hidden = !event.target.value;
    }
  }, true);
  const catalog = document.getElementById('catalog');
  catalog.insertAdjacentHTML('beforeend', '<div class="catalog-group">窗口状态与清理</div>' + SCENES.slice(-5).map(scene => `<button class="catalog-item" data-go="${scene.id}"><span class="catalog-num">${scene.originalIndex}</span>${scene.name}</button>`).join(''));
  window.DESIGN_SCENES = SCENES;
  v3Go(new URLSearchParams(location.search).get('scene') || 'w-list', false);
})();
