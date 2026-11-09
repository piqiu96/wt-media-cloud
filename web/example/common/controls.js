/* HTML参考专用选择器/菜单；真实产品使用TDesign Select/Dropdown。不含业务与联网。 */
(() => {
  'use strict';
  let sequence = 0, active = null;
  const all = (selector, root = document) => [...root.querySelectorAll(selector)];
  const supported = typeof HTMLElement.prototype.showPopover === 'function';
  function dismiss(restore = false) {
    if (!active) return;
    const previous = active; active = null;
    if (previous.popup.matches(':popover-open')) previous.popup.hidePopover();
    previous.trigger.setAttribute('aria-expanded', 'false');
    previous.trigger.removeAttribute('aria-activedescendant');
    if (restore && previous.trigger.isConnected) previous.trigger.focus();
  }
  function position(popup, trigger, minWidth = 0) {
    const r = trigger.getBoundingClientRect();
    const width = Math.min(Math.max(r.width, minWidth), innerWidth - 24);
    const below = innerHeight - r.bottom - 16, above = r.top - 16;
    const upward = below < 180 && above > below;
    popup.style.width = width + 'px';
    popup.style.maxHeight = Math.max(64, Math.min(256, upward ? above : below)) + 'px';
    popup.style.left = Math.max(12, Math.min(r.left, innerWidth - width - 12)) + 'px';
    popup.style.top = upward ? 'auto' : r.bottom + 6 + 'px';
    popup.style.bottom = upward ? innerHeight - r.top + 6 + 'px' : 'auto';
  }
  function reveal(popup, trigger, minWidth = 0) {
    dismiss(); position(popup, trigger, minWidth);
    popup.showPopover(); trigger.setAttribute('aria-expanded','true');
    active = {popup,trigger};
  }
  function enhanceSelect(select) {
    if (select.dataset.enhanced || select.multiple || select.size > 1) return;
    select.dataset.enhanced = 'true';
    const id = select.id || 'example-select-' + ++sequence;
    select.id = id;
    const wrapper = document.createElement('div'); wrapper.className = 'ex-select';
    select.before(wrapper); wrapper.append(select);
    const trigger = document.createElement('button'); trigger.type = 'button';
    trigger.className = 'ex-select-trigger'; trigger.id = id + '-trigger';
    trigger.setAttribute('role','combobox'); trigger.setAttribute('aria-haspopup','listbox');
    trigger.setAttribute('aria-expanded','false'); trigger.setAttribute('aria-controls',id + '-options');
    const text = document.createElement('span'); text.className = 'ex-select-value';
    const arrow = document.createElement('span'); arrow.className = 'ex-chevron'; arrow.setAttribute('aria-hidden','true');
    trigger.append(text,arrow);
    const label = all('label[for]').find(e => e.htmlFor === id);
    if (label) {
      label.id ||= id + '-label'; label.htmlFor = trigger.id;
      trigger.setAttribute('aria-labelledby',label.id);
    } else trigger.setAttribute('aria-label',select.getAttribute('aria-label') || '选择选项');
    if (select.getAttribute('aria-describedby')) trigger.setAttribute('aria-describedby',select.getAttribute('aria-describedby'));
    const popup = document.createElement('div'); popup.className = 'ex-select-popup';
    popup.id = id + '-options'; popup.setAttribute('role','listbox'); popup.setAttribute('popover','manual');
    popup.setAttribute('aria-label',label?.textContent.trim() || '选项');
    wrapper.append(trigger,popup); select.hidden = true;
    let current = -1, buffer = '', lastTyped = 0;
    const enabled = () => [...select.options].map((option,i)=>!option.disabled ? i : -1).filter(i=>i>=0);
    function sync() {
      text.textContent = select.selectedOptions[0]?.textContent || select.getAttribute('placeholder') || '请选择';
      trigger.disabled = select.disabled; trigger.setAttribute('aria-required',String(select.required));
      if (select.hasAttribute('aria-invalid')) trigger.setAttribute('aria-invalid',select.getAttribute('aria-invalid'));
      else trigger.removeAttribute('aria-invalid');
      if (active?.trigger === trigger && select.disabled) dismiss();
      all('[role="option"]',popup).forEach((node,i)=>node.setAttribute('aria-selected',String(i===select.selectedIndex)));
    }
    function render() {
      popup.replaceChildren();
      [...select.options].forEach((option,i)=>{
        const row = document.createElement('div'); row.className = 'ex-select-option';
        row.id = id + '-option-' + i; row.dataset.index = i;
        row.setAttribute('role','option'); row.setAttribute('aria-selected',String(option.selected));
        row.setAttribute('aria-disabled',String(option.disabled));
        const value = document.createElement('span'); value.textContent = option.textContent;
        const mark = document.createElement('span'); mark.className='ex-option-mark'; mark.textContent='✓'; mark.setAttribute('aria-hidden','true');
        row.append(value,mark); popup.append(row);
      }); sync();
    }
    function highlight(index) {
      if (index < 0) return;
      current = index;
      all('[role="option"]',popup).forEach((row,i)=>row.classList.toggle('is-active',i===index));
      trigger.setAttribute('aria-activedescendant',id + '-option-' + index);
      popup.children[index]?.scrollIntoView({block:'nearest'});
    }
    function open(index = select.selectedIndex) {
      render(); reveal(popup,trigger); highlight(enabled().includes(index)?index:enabled()[0]);
    }
    function commit(index) {
      if (!enabled().includes(index)) return;
      select.selectedIndex = index; select.dispatchEvent(new Event('input',{bubbles:true}));
      select.dispatchEvent(new Event('change',{bubbles:true})); sync(); dismiss(true);
    }
    trigger.addEventListener('click',()=>active?.trigger === trigger ? dismiss() : open());
    trigger.addEventListener('keydown',event=>{
      const list = enabled(), isOpen = active?.trigger === trigger;
      if (event.key === 'Escape' && isOpen) { event.preventDefault();event.stopPropagation();dismiss(true);return; }
      if (event.key === 'Tab') { if (isOpen) dismiss(); return; }
      if (['ArrowDown','ArrowUp','Home','End','Enter',' '].includes(event.key)) {
        event.preventDefault(); event.stopPropagation();
        if (!isOpen) { open(event.key==='ArrowUp'?list.at(-1):select.selectedIndex); return; }
        if (event.key==='Enter' || event.key===' ') { commit(current);return; }
        if (event.key==='Home') highlight(list[0]);
        else if (event.key==='End') highlight(list.at(-1));
        else { const i=list.indexOf(current), step=event.key==='ArrowDown'?1:-1;highlight(list[(i+step+list.length)%list.length]); }
      } else if (event.key.length===1 && !event.ctrlKey && !event.metaKey && !event.altKey) {
        event.preventDefault();
        buffer = Date.now()-lastTyped>1000 ? event.key : buffer+event.key; lastTyped=Date.now();
        const index=list.find(i=>select.options[i].textContent.toLowerCase().startsWith(buffer.toLowerCase()));
        if (index!==undefined) {if(!isOpen)open(index);else highlight(index);}
      }
    });
    popup.addEventListener('pointerdown',event=>{if(event.pointerType==='mouse' && event.target.closest('[role="option"]'))event.preventDefault();});
    popup.addEventListener('click',event=>{const row=event.target.closest('[role="option"]');if(row)commit(Number(row.dataset.index));});
    select.addEventListener('change',sync);
    select.addEventListener('invalid',event=>{event.preventDefault();trigger.setAttribute('aria-invalid','true');trigger.focus();});
    new MutationObserver(render).observe(select,{attributes:true,childList:true,subtree:true,attributeFilter:['disabled','required','aria-invalid','selected','label']});
    render();
  }
  function enhanceMenu(root) {
    if (root.dataset.enhanced) return;
    root.dataset.enhanced='true';
    const trigger=root.querySelector('[data-menu-trigger]'),popup=root.querySelector('[data-menu-popup]');
    popup.id ||= 'example-menu-'+ ++sequence; popup.setAttribute('popover','manual');popup.setAttribute('role','menu');
    trigger.setAttribute('aria-haspopup','menu');trigger.setAttribute('aria-expanded','false');trigger.setAttribute('aria-controls',popup.id);
    all('button',popup).forEach(e=>e.setAttribute('role','menuitem'));
    const items=()=>all('[role="menuitem"]:not(:disabled)',popup);
    function open(last=false){reveal(popup,trigger,160);(last?items().at(-1):items()[0])?.focus();}
    trigger.addEventListener('click',()=>active?.trigger===trigger?dismiss(true):open());
    trigger.addEventListener('keydown',event=>{
      if(event.key==='ArrowDown'||event.key==='ArrowUp'){event.preventDefault();open(event.key==='ArrowUp');}
    });
    popup.addEventListener('keydown',event=>{
      const list=items(), i=list.indexOf(document.activeElement);
      if(event.key==='Escape'){event.preventDefault();event.stopPropagation();dismiss(true);}
      else if(event.key==='Tab')dismiss();
      else if(['ArrowDown','ArrowUp','Home','End'].includes(event.key)){
        event.preventDefault();list[event.key==='Home'?0:event.key==='End'?list.length-1:(i+(event.key==='ArrowDown'?1:-1)+list.length)%list.length]?.focus();
      }
    });
    popup.addEventListener('click',event=>{if(event.target.closest('[role="menuitem"]'))dismiss();});
  }
  document.addEventListener('pointerdown',event=>{if(active && !active.popup.contains(event.target) && !active.trigger.contains(event.target))dismiss();},true);
  document.addEventListener('scroll',event=>{if(active && !active.popup.contains(event.target))dismiss();},true);
  window.addEventListener('resize',()=>dismiss());
  window.addEventListener('hashchange',()=>dismiss());
  // Observe native reset's restored values, not the values during reset dispatch.
  document.addEventListener('reset',event=>setTimeout(()=>{if(!event.defaultPrevented)all('select',event.target).forEach(e=>e.dispatchEvent(new Event('change',{bubbles:true})));},0));
  document.addEventListener('close',()=>{if(active && !active.trigger.isConnected)dismiss();},true);
  function enhance(root=document){if(!supported)return;all('select',root).forEach(enhanceSelect);all('[data-action-menu]',root).forEach(enhanceMenu);}
  window.WTControls={enhance,dismiss}; enhance();
})();
