/* 演示适配层；继承V3场景与交互，不导入实际产品。 */
(() => {
 let selectId=0;
 const size=()=>{const stage=document.getElementById('stage'),wrap=document.getElementById('stage-wrap');const h=Math.max(900,innerHeight-150);stage.style.height=h+'px';stage.style.transform='none';wrap.style.height=h+'px';};
 v3Scale=size;
 function adapt(){
  const stage=document.getElementById('stage');
  stage.querySelectorAll('.field').forEach(field=>{const label=field.querySelector('label');if(label?.textContent.trim()==='比特标签'){field.remove();return;}const select=field.querySelector('select');if(select){select.id||='proto-select-'+ ++selectId;if(label)label.htmlFor=select.id;}if(label&&/名称|地址|备注|链接/.test(label.textContent))field.classList.add('full');});
  stage.querySelectorAll('input[placeholder]').forEach(input=>{input.placeholder=input.placeholder.replace('比特标签 / ','');});
  stage.querySelectorAll('.window-v3 thead th').forEach((th,index)=>{if(th.textContent.includes('比特标签')){th.textContent='窗口名称';stage.querySelectorAll('.window-v3 tbody tr').forEach(row=>{row.children[index]?.querySelector('strong + div')?.remove();});}});
  stage.querySelectorAll('.facts dt').forEach(dt=>{if(dt.textContent.trim()==='比特标签'){dt.nextElementSibling?.remove();dt.remove();}});
  stage.querySelectorAll('.row-actions').forEach(row=>{row.classList.add('ex-row-actions');const actions=[...row.querySelectorAll('.btn')];actions.forEach(b=>{b.classList.add('ex-text-button');if(b.classList.contains('danger-text'))b.classList.add('danger');});const main=actions.find(b=>/^(打开|关闭|检测)$/.test(b.textContent.trim()));if(main){main.classList.add('row-primary','ex-row-primary');main.querySelectorAll('.icon').forEach(e=>e.remove());row.prepend(main);}});
  stage.querySelectorAll('.facts').forEach(facts=>{if(facts.classList.contains('adaptive-facts'))return;const nodes=[...facts.children], list=document.createElement('dl');list.className='ex-description-list';list.dataset.density=nodes.filter((n,i)=>i%2===0&&!/名称|地址|备注|原因|说明|链接|密码|当前代理|托管代理/.test(n.textContent)).length>=4?'dense':'sparse';for(let i=0;i<nodes.length;i+=2){if(!nodes[i+1])continue;const item=document.createElement('div');item.className='ex-desc-item';if(/名称|地址|备注|原因|说明|链接|密码/.test(nodes[i].textContent))item.classList.add('full');if(nodes[i].textContent.trim()==='窗口信息')nodes[i+1].textContent=nodes[i+1].textContent.replace('、比特标签','');item.append(nodes[i],nodes[i+1]);list.append(item);}facts.classList.add('adaptive-facts','ex-description-container');facts.append(list);});
  if(['w-create','w-create-error','w-edit','w-note','p-quota','p-quota-error'].includes(v3Current.id))stage.querySelector('.drawer')?.classList.add('short-task');
  if(v3Current.id==='w-note')stage.querySelector('.dialog,.drawer')?.classList.add('note-editor');
  stage.querySelectorAll('.logo-crop img').forEach(e=>{if(e.getAttribute('src')==='assets/logo.png')e.src='assets/logo.png';});
  window.WTControls?.enhance(stage);size();
  document.getElementById('book-footer').innerHTML='<span>最新frontend规范 · 纯本地业务原型，不连接真实服务</span><a href="../../common/index.html">公共视觉交互参考</a>';
 }
 const previous=v3Render;
 v3Render=function(){window.WTControls?.dismiss();previous();adapt();};
 window.addEventListener('resize',size);v3Render();
})();
