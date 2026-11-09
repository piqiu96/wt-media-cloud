/* 已审批的窗口状态交互：进入读取一次；手动刷新；失败仅Toast。不调用业务服务。 */
(() => {
  const states = [
    ['w-runtime-reading', '窗口状态 · 正在读取', 'runtime-reading', 'R1'],
    ['w-runtime-error', '窗口状态 · 读取失败Toast', 'runtime-error', 'R2'],
  ].map(([id, name, state, originalIndex]) => ({id, name, state, originalIndex, module:'window', view:'list', group:'窗口状态 · 自动与手动读取', role:'operator'}));
  SCENES.push(...states);
  const priorRender = v3Render;
  let toastDismissed = false;
  let previousScene = '';
  v3Render = function() {
    priorRender();
    const stage = document.getElementById('stage');
    if (previousScene !== v3Current.id) toastDismissed = false;
    previousScene = v3Current.id;
    if (!v3Local()) return;
    const table = stage.querySelector('.window-v3');
    const headings = [...(table?.querySelectorAll('thead th') || [])];
    const index = headings.findIndex(th => th.textContent.trim() === '运行状态');
    if (index < 0) return;
    const reading = v3Current.id === 'w-runtime-reading';
    const failed = v3Current.id === 'w-runtime-error';
    const actions = stage.querySelector('.page-header .actions');
    actions?.insertAdjacentHTML('afterbegin', `<button class="btn runtime-page-refresh" data-action="runtime-refresh" title="只读取比特窗口的已打开或已关闭状态，不同步窗口资料。" ${reading ? 'disabled aria-busy="true"' : ''}>${reading ? '<span class="spin"></span> 读取中' : '刷新运行状态'}</button>`);
    if (reading || failed) {
      table.querySelectorAll('tbody tr[data-v3-row]').forEach(row => {
        row.children[index].textContent = '未知';
        const action = row.querySelector('[data-action="v3-window"]');
        if (action) { action.disabled = true; action.textContent = '打开'; action.title = '运行状态未知，请点击右上角「刷新运行状态」'; }
      });
    }
    if (failed && !toastDismissed) {
      const toast = document.createElement('div');
      toast.className = 'toast runtime-failure-toast';
      toast.setAttribute('role', 'alert');
      toast.innerHTML = `${icon('ErrorCircle')}<span>读取运行状态失败，请确认比特已启动并登录，稍后再试。</span><button class="btn text" data-action="runtime-dismiss-toast" aria-label="关闭提示">×</button>`;
      stage.append(toast);
    }
    if (v3Current.view === 'list') {
      const review = document.createElement('div');
      review.className = 'sync-scene-navigation runtime-preview-navigation';
      review.innerHTML = `<span>评审预览 · 非产品控件</span>${btn('读取中', 'w-runtime-reading')}${btn('读取失败', 'w-runtime-error')}${btn('读取成功', 'w-list')}`;
      document.getElementById('book-header').append(review);
    }
  };
  const priorAction = v3Action;
  v3Action = function(action, target) {
    if (action === 'runtime-refresh') { v3Go('w-runtime-reading'); return; }
    if (action === 'runtime-dismiss-toast') { toastDismissed = true; target.closest('.toast')?.remove(); return; }
    priorAction(action, target);
  };
  const catalog = document.getElementById('catalog');
  catalog.insertAdjacentHTML('beforeend', `<div class="catalog-group">窗口状态 · 自动与手动读取</div>${states.map(s=>`<button class="catalog-item" data-go="${s.id}"><span class="catalog-num">${s.originalIndex}</span>${s.name}</button>`).join('')}`);
  catalog.querySelector('.book-count').innerHTML = `${SCENES.length} 个场景 · 同步窗口完整状态<br>纯本地评审，不连接真实服务`;
  window.DESIGN_SCENES = SCENES;
  v3Go(new URLSearchParams(location.search).get('scene') || 'w-list', false);
})();
