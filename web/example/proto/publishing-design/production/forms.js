/* 合成表单共享配置：按模板字段展示，同一设置可以生成多份文件。 */
(() => {
  const C=CP,O=OPS;
  const table=(heads,rows)=>`<div class="ex-table-scroll"><table class="ex-table ops-table ${heads.at(-1)==='操作'?'production-action-table':'production-data-table'}"><thead><tr>${heads.map(h=>`<th>${h}</th>`).join('')}</tr></thead><tbody>${rows||`<tr><td colspan="${heads.length}">${C.empty('暂无匹配内容')}</td></tr>`}</tbody></table></div>`;
  const copy=v=>JSON.parse(JSON.stringify(v));
  const newVersion=base=>copy({...O.version(),guideUrl:'',insertSeconds:0,guideDuration:0,maxDuration:0,...base});
  const schemas={
    '视频截取':[['start','截取起点','text'],['end','截取终点','text']],
    '引导视频插入':[['guideUrl','引导视频地址','url'],['insertSeconds','插入位置（秒）','number'],['guideDuration','引导视频时长（秒）','number'],['maxDuration','成片时长上限（秒）','number']],
    '引导画中画':[['guideUrl','引导视频地址','url'],['insertSeconds','开始位置（秒）','number'],['guideDuration','引导视频时长（秒）','number'],['maxDuration','成片时长上限（秒）','number']]
  };
  const parameterHint=(key)=>({insertSeconds:'0 表示片头，其他数值表示从主体第几秒开始。',guideDuration:'0 表示使用完整引导视频。',maxDuration:'0 表示不限制成片时长。'})[key]||'';
  const parameterValue=(key,value)=>value==null?'—':Number(value)===0?({insertSeconds:'片头（0 秒）',guideDuration:'完整视频',maxDuration:'不限'})[key]??String(value):['insertSeconds','guideDuration','maxDuration'].includes(key)?value+' 秒':String(value);
  const versionForm=(vs,template='引导视频插入')=>vs.map((v,i)=>`<article class="ops-version" data-version-index="${i}"><div class="ops-version-head"><strong>成片设置 ${i+1}</strong>${C.prod.orderActions(vs,i,{copy:true})}</div>
    <div class="ex-form-grid">${C.field('成片设置名称','v.'+i+'.name',v.name,{required:true})}<div class="ex-field required"><div class="production-field-label"><label id="version-platform-${i}">适用平台</label>${C.help('适用平台','至少选择一个平台。平台标签不增加成片份数。')}</div><div class="ops-platforms" role="group" aria-labelledby="version-platform-${i}" aria-describedby="version-platform-help-${i}">${O.platforms.map(p=>`<label><input type="checkbox" data-platform-version="${i}" value="${p}" ${v.platforms.includes(p)?'checked':''}>${p}</label>`).join('')}</div><span id="version-platform-help-${i}" class="ex-help" data-platform-help hidden></span></div>
    ${C.field('每条素材生成份数','v.'+i+'.qty',v.qty,{type:'number',required:true,hint:'每条素材按本组配置生成 1–20 份成片。'})}</div><div class="production-template-parameters"><h4>模板参数</h4><div class="ex-form-grid">${(schemas[template]||[]).map(([key,label,type])=>C.field(label,'v.'+i+'.'+key,v[key]??(type==='number'?0:''),{type,required:true,full:type==='url',placeholder:type==='url'?'https://...':'',hint:parameterHint(key)})).join('')}</div></div></article>`).join('');
  const wireVersions=(d,vs,onChange=()=>{})=>{
    const updatePlatforms=()=>d.querySelectorAll('[data-version-index]').forEach(row=>{const empty=!vs[+row.dataset.versionIndex].platforms.length,group=row.querySelector('[role="group"]'),help=row.querySelector('[data-platform-help]');group.setAttribute('aria-invalid',String(empty));help.classList.toggle('ops-error',empty);help.hidden=!empty;help.textContent=empty?'请至少选择一个平台。':'';});
    d.querySelectorAll('[data-platform-version]').forEach(el=>el.addEventListener('change',()=>{const v=vs[+el.dataset.platformVersion];v.platforms=el.checked?[...new Set([...v.platforms,el.value])]:v.platforms.filter(p=>p!==el.value);d.dirty=true;onChange();updatePlatforms();updateCount(d,vs);}));
    updatePlatforms();
    d.querySelectorAll('[data-field$=".qty"]').forEach(el=>{el.min='1';el.max='20';el.step='1';});
    C.prod.wireOrder(d,vs,'[data-version-index]','versionIndex');
  };
  const updateCount=(d,vs,n)=>{const el=d.querySelector('[data-count]');if(el){const count=n??Number(el.dataset.count);el.textContent=`${count} 条素材，共生成 ${count*vs.reduce((s,v)=>s+Number(v.qty||0),0)} 条合成任务`;}};
  const changeVersion=(vs,k,value)=>{const m=k.match(/^v\.(\d+)\.(.+)$/);if(!m)return false;vs[+m[1]][m[2]]=['qty','insertSeconds','guideDuration','maxDuration'].includes(m[2])?Number(value):value;return true;};
  const seconds=t=>/^\d{1,3}:[0-5]\d$/.test(t)?Number(t.split(':')[0])*60+Number(t.split(':')[1]):NaN;
  const valid=(vs,template)=>vs.length&&vs.every(v=>v.name.trim()&&Number.isInteger(v.qty)&&v.qty>=1&&v.qty<=20&&v.platforms.length&&v.platforms.every(p=>O.platforms.includes(p))&&(template&&!['视频截取'].includes(template)?/^https?:\/\//.test(v.guideUrl||''):true)&&Number.isFinite(seconds(v.start))&&Number.isFinite(seconds(v.end))&&seconds(v.end)>seconds(v.start)&&['insertSeconds','guideDuration','maxDuration'].every(k=>Number.isInteger(v[k]??0)&&(v[k]??0)>=0));
  C.prod={parameterHint,parameterValue,table,versionForm,wireVersions,updateCount,changeVersion,valid,copy,schemas,newVersion};
})();
