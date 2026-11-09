/* 同一评审入口：当前逐页粗评；历史锚点仍可访问综合依据。 */
(() => {
  const sections = [...document.querySelectorAll('main > section[id]')];
  const navigate = () => {
    let key;
    try { key = decodeURIComponent(location.hash.slice(1)); } catch { key = ''; }
    const target = key && document.getElementById(key);
    const section = target?.closest('main > section[data-layer]');
    const mode = section?.dataset.layer === 'reference' ? 'reference' : 'pages';
    const active = section || document.getElementById('page-review');
    document.body.dataset.reviewMode = mode;
    sections.forEach(s => { s.hidden = mode === 'reference' ? s.dataset.layer === 'pages' : s !== active; });
    document.querySelector('[data-page-toc]').hidden = mode !== 'pages';
    document.querySelector('[data-reference-toc]').hidden = mode !== 'reference';
    document.querySelector('[data-reference-banner]').hidden = mode !== 'reference';
    document.querySelectorAll('[data-review-mode-link]').forEach(a => {
      if (a.dataset.reviewModeLink === mode) a.setAttribute('aria-current', 'page');
      else a.removeAttribute('aria-current');
    });
    document.querySelectorAll('.review-toc a').forEach(a => {
      if (a.pathname === location.pathname && a.search === location.search && a.hash === '#' + active.id) a.setAttribute('aria-current', 'page');
      else a.removeAttribute('aria-current');
    });
    document.title = `${active.querySelector('h1,h2')?.textContent || '逐页粗评'} · 内容运营评审`;
    requestAnimationFrame(() => (target || active).scrollIntoView({block:'start'}));
  };
  addEventListener('hashchange', navigate);
  navigate();
})();
