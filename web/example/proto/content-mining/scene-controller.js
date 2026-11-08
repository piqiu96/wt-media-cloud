/* 独立场景装配：示例状态仅留在内存，切换场景恢复初始样本。 */
contents.push(...contents.slice(0,2).map((c,i)=>({...c,id:8301+i,team:'游戏运营二组',status:'pending',material:null})));
const miningInitial=JSON.parse(JSON.stringify({contents,strategies,tasks}));
let miningSceneId='P01';
function clearLayers(){document.querySelectorAll('dialog').forEach(d=>{d.close();d.remove()});window.WTControls?.dismiss()}
function resetMining(){clearLayers();contents=JSON.parse(JSON.stringify(miningInitial.contents));strategies=JSON.parse(JSON.stringify(miningInitial.strategies));tasks=JSON.parse(JSON.stringify(miningInitial.tasks));Object.keys(demoFlags).forEach(k=>delete demoFlags[k]);Object.keys(viewStates).forEach(k=>delete viewStates[k]);selection.clear();sortState={key:'day',direction:'desc'};filters={};reviewSession=null;displayFields=new Set(['category','metrics','source','status','published']);role='senior';scene='normal';routeStrategy=null;pageSize=20;currentPage=1;importedCandidates.clear();contents[4].noCover=true;}
function setReviewPage(target){page=target;history.replaceState(null,'',`?scene=${miningSceneId}#${target}`);render();$('.ex-sidebar-foot').innerHTML=role==='admin'?'跨团队管理<br><small>管理员工作空间</small>':'<span class="team-dot"></span>游戏运营一组<br><small>内容运营工作空间</small>';}
function sceneSearch(mode='keyword',query='王者荣耀 上分'){discover(mode);const d=[...document.querySelectorAll('dialog[open]')].at(-1);$('#discover-query',d).value=query;$('#import-category',d).value='王者荣耀';$('#import-category',d).dispatchEvent(new Event('change',{bubbles:true}));return d}
function selectScene(id){if(!window.MINING_SCENARIOS.some(s=>s.id===id))return;miningSceneId=id;resetMining();const target=id.startsWith('S')?'strategies':id.startsWith('T')?'tasks':'pool';setReviewPage(target);
 switch(id){
 case 'P02':filters={category:['王者荣耀'],source:'strategy'};render();$('#filters .ex-filter-advanced').open=true;break;
 case 'P03':selection=new Set([8201,8203,8205]);drawTable();break;
 case 'P04':discover('id');break;
 case 'P05':discover('keyword');break;
 case 'P06':{const d=sceneSearch();searchDiscovery(d);break}
 case 'P07':{demoFlags.queryPartial=true;const d=sceneSearch('id','7695748247591095603\nhttps://v.douyin.com/demo/');searchDiscovery(d);break}
 case 'P08':importResult([{id:0,result:'新增',reason:'已加入游戏运营一组，尚未生成素材'},{id:1,result:'重复',reason:'内容池已存在，不重复创建'},{id:2,result:'新增',reason:'已加入游戏运营一组，尚未生成素材'}]);break;
 case 'P09':contentDetail(8203);break;
 case 'P10':reviewSession=null;contentDetail(8201,true);break;
 case 'P11':reviewSession={ids:[8201,8202,8204,8206],processed:[8201,8202],skipped:[8204,8206]};reviewSession.processed.forEach(id=>applyContent(contents.find(c=>c.id===id),'material'));reviewSummary();break;
 case 'P12':demoFlags.actionFailure=true;contents[0].lastError='素材创建失败，请稍后重试';contentDetail(8201,true);break;
 case 'P13':sourceDetail(8203);break;
 case 'P14':batch('material',[8201,8202,8203]);controlledBatch().finish(true);break;
 case 'P15':contents[1].lastError='素材创建失败，请稍后重试';render();break;
 case 'P16':scene='empty';render();break;
 case 'S02':filters={category:['原神']};render();break;
 case 'S03':editor();break;
 case 'S04':editor(3101);break;
 case 'S05':editor(3108);break;
 case 'S06':strategies[0].rules=[{action:'filter',logic:'all',conditions:[{field:'published_at',op:'lt',value:'2026-11-01T00:00'}]},{action:'match',logic:'any',conditions:[{field:'like_count',op:'gte',value:5000},{field:'favorite_count',op:'gte',value:500}]}];editor(3101);break;
 case 'S07':{demoFlags.saveFailure=true;editor(3101);const d=[...document.querySelectorAll('dialog[open]')].at(-1);$('#strategy-name',d).value+=' · 调整';d.dataset.dirty='true';$('#strategy-error',d).textContent='策略尚未保存，请稍后重试。填写的资料与规则已保留。';break}
 case 'S08':strategyDetail(3102);break;
 case 'S09':strategyDetail(3101);break;
 case 'S10':document.querySelector('[data-menu-trigger]')?.click();break;
 case 'T02':taskDetail(9202);break;
 case 'T03':Object.assign(tasks[5],{found:20,added:0,auto:0,pending:0,failed:0});taskDetail(9206);break;
 case 'T04':taskDetail(9203);break;
 case 'T05':taskDetail(9204);break;
 case 'T06':strategies[0].keywords='王者荣耀 新版本 热点';strategies[0].schedule='每 120 分钟';taskDetail(9201,'snapshot');break;
 case 'T07':demoFlags.snapshotMissing=true;taskDetail(9201,'snapshot');break;
 case 'T08':role='operator';render();break;
 case 'G02':role='admin';filters={team:'游戏运营二组'};render();break;
 case 'G03':contents[0].title='王者荣耀新赛季上分完整复盘：从开局资源判断到逆风团战选择，三个关键细节帮助你稳定建立优势';contents[0].noCover=true;render();break;
 case 'G04':render();$('#notice').innerHTML='<div class="ex-notice warning">自动更新暂未完成，当前仍显示上次读取的数据。已保留筛选与页码。</div>';break;
 }
 if(['S04','S05','S06'].includes(id)){const d=[...document.querySelectorAll('dialog[open]')].at(-1),body=$('.ex-overlay-body',d),target=d.querySelectorAll('.ex-section')[id==='S04'?1:2];if(target)body.scrollTop+=target.getBoundingClientRect().top-body.getBoundingClientRect().top-52;}
 const roleSelect=$('#role');roleSelect.value=role;$('#scene').value=scene;
 // Keep page height separate from overlay scroll height.
 requestAnimationFrame(()=>parent.postMessage({kind:'mining-height',height:document.querySelector('.ex-shell').scrollHeight},'*'));
}
function sceneControl(action){
 if(action==='action-success'){demoFlags.actionFailure=false;message('演示控制：下一次转素材将返回成功结果');return}
 if(action==='save-failure'||action==='save-success'){demoFlags.saveFailure=action==='save-failure';if(!demoFlags.saveFailure){const error=$('#strategy-error');if(error)error.textContent=''}return}
 if(action==='retry-failure'||action==='retry-success'){demoFlags.retryFailure=action==='retry-failure';return}
 if(action==='snapshot-retry'){$('[data-action="snapshot-retry"]')?.click();return}
 if(action==='query-retry'){$('[data-action="query-retry"]')?.click();return}
 if(action==='review-again'){$('[data-action="review-again"]')?.click();return}
 if(action.startsWith('schedule-')){const d=[...document.querySelectorAll('dialog[open]')].at(-1),mode=action.slice(9);$('#schedule-mode',d).value=mode;$('#schedule-mode',d).dispatchEvent(new Event('change',{bubbles:true}));return}
 if(action.startsWith('role-')){clearLayers();role=action.slice(5);filters={};selection.clear();render();return}
 if(action==='batch-processing'){clearLayers();contents[0].status='pending';contents[1].status='pending';batch('material',[8201,8202,8203]);beginBatch(controlledBatch());return}
 if(action==='batch-success'||action==='batch-partial'){const d=controlledBatch();if(d)d.finish(action==='batch-partial');return}
 if(action==='empty'||action==='no-match'||action==='load-error'){clearLayers();scene=action==='empty'?'empty':action==='load-error'?'error':'normal';filters=action==='no-match'?{search:'不存在的内容'}:{};render();return}
 if(action==='task-pending'||action==='task-running'){clearLayers();const t=tasks[1];t.status=action==='task-pending'?'pending':'running';taskDetail(t.id);return}
 if(action==='task-failed'||action==='task-partial'){clearLayers();taskDetail(action==='task-failed'?9204:9203);return}
 if(action==='import-failure'){clearLayers();importResult([{id:0,result:'新增',reason:'已加入内容池'},{id:1,result:'失败',reason:'请求未完成，请重新提交'}]);return}
 if(action==='refresh-success'){$('#notice').innerHTML='';message('列表已恢复读取，筛选与页码保持不变');return}
}
window.addEventListener('message',e=>{if(e.source!==parent||e.data?.kind!=='mining-review')return;if(e.data.action==='scene')selectScene(e.data.id);if(e.data.action==='control')sceneControl(e.data.control)});
selectScene(new URLSearchParams(location.search).get('scene')||'P01');parent.postMessage({kind:'mining-ready'},'*');
