const assert=require('node:assert/strict'),fs=require('fs'),vm=require('vm');
const context={window:{},URL,Date};vm.runInNewContext(fs.readFileSync('example/proto/content-operations/production/mine-model.js','utf8'),context);const M=context.window.MineVideo;
const ready={cloudFileExists:true,cloudUrl:'https://example.test/a.mp4',cloudUrlVerified:true,sourceState:'expired'};
const u={status:'active',files:{a:{exists:true,verified:true}}};
assert.equal(M.state(ready,u,'a').key,'available');assert.equal(M.state(ready,u,'a').download,false);
assert.equal(M.state(ready,u,'b').download,true);
assert.equal(M.state({cloudState:'notSaved'},u,'b').key,'pending');
assert.equal(M.state({cloudState:'failed'},u,'b').key,'failed');
assert.equal(M.state({cloudState:'ready'},{status:'active',download:'downloaded'},'a').key,'unknown');
assert.equal(M.state(ready,{...u,transfers:{a:{phase:'failed',reason:'空间不足'}}},'a').key,'available');
assert.equal(M.state(ready,{status:'active',transfers:{a:{phase:'failed',reason:'空间不足'}}},'a').canCompose,true);
assert.equal(M.state(ready,{status:'removed'},'a').download,false);
assert.equal(M.state(ready,{status:'active',transfers:{a:{phase:'downloading',progress:45}}},'a').key,'downloading');
assert.equal(M.state({...ready,cloudUrlExpiresAt:'2020-01-01'}, {status:'active'},'a').key,'unknown');
console.log('PASS: 10 video state / device / relationship checks');
// Exercise the real page/controller and shared library with an offline DOM adapter.
const callbacks=[],listeners={},doc={addEventListener:(e,f)=>(listeners[e]||=[]).push(f),querySelectorAll:()=>[],querySelector:()=>null};
let currentDialog;
const C={db:{materials:[{id:1,title:'A',platform:'百度',category:'剪辑教程',cloudState:'notSaved',published:'2026-10-01',created:'2026-11-01'},{id:2,title:'B',platform:'B站',category:'剪辑教程',cloudState:'notSaved',published:'2026-10-02',created:'2026-11-01'}],outputs:[{id:10,materialId:1,owner:'陈小雨'},{id:11,materialId:1,owner:'林予安'}],usages:{陈小雨:[{materialId:1,status:'active',added:'11-01 09:00'},{materialId:2,status:'active',added:'11-02 09:00'}],林予安:[{materialId:1,status:'active',added:'11-01 09:00'}]}},ui:{filters:{}},prod:{table:(heads,body)=>'<table><thead>'+heads.map(x=>'<th>'+x+'</th>').join('')+'</thead><tbody>'+body+'</tbody></table>'},e:v=>String(v??''),save:()=>{},render:()=>{},dialogs:[],toast:()=>{},head:(t,d)=>t+d,field:()=>'',select:()=>'',tag:(s)=>s,btn:(label,fn,cls='',disabled=false,attrs='')=>{callbacks.push({label,fn});return `<button class="${cls}" ${disabled?'disabled':''} ${attrs}>${label}</button>`;},row:(label,fn)=>C.btn(label,fn),link:(label,fn)=>C.btn(label,fn),materialPage:()=>'<old-page>',materialDetail:()=>'<old-detail>',facts:rows=>rows.map(x=>x.join(':')).join(' '),section:(t,b)=>t+b,open:opts=>{const d={opts,paint:()=>{d.html=opts.body();},querySelector:()=>({}),minePending:null};d.paint();currentDialog=d;C.dialogs.push(d);return d;},close:d=>{C.dialogs=C.dialogs.filter(x=>x!==d);}};
const O={actor:'陈小雨',categories:['剪辑教程'],platforms:['百度','B站'],sourcePlatforms:['抖音'],people:[{name:'陈小雨',role:'高级运营'},{name:'林予安',role:'运营'}],render:()=>{},role:()=>O.actor==='陈小雨'?'高级运营':'运营',peopleOptions:()=>[],reference:(s)=>s};
C.coverPlaceholder=()=>'<span class="cp-cover cp-cover-default" role="img" aria-label="默认视频封面"></span>';
C.get=(key,id)=>C.db[key].find(x=>x.id===+id);C.usages=(owner=O.actor)=>C.db.usages[owner]||=[];C.usage=(id,owner=O.actor)=>C.usages(owner).find(x=>x.materialId===id);C.canManage=()=>O.role()!=='运营';
const ctx={window:{},CP:C,OPS:O,document:doc,CPSeed:()=>({materials:[]}),URL,Date,navigator:{},addEventListener:()=>{},setTimeout};vm.createContext(ctx);
for(const file of ['mine-model.js','library.js','mine.js']){if(file==='mine.js')ctx.MineVideo=ctx.window.MineVideo;vm.runInContext(fs.readFileSync('example/proto/content-operations/production/'+file,'utf8'),ctx);}
const before=JSON.stringify(C.db.materials);C.route='mine';
assert.equal(O.mine.state().usage,'unused');assert.equal(O.mine.state().sort,'published');assert.equal(O.mine.state().desc,true);
assert.deepEqual(Array.from(O.mine.rows(),m=>m.id),[2]);
O.mine.state().usage='';assert.deepEqual(Array.from(O.mine.rows(),m=>m.id),[2,1]);
C.usage(2).status='removed';O.mine.state().relation='all';assert.deepEqual(Array.from(O.mine.rows(),m=>m.id),[1]);C.usage(2).status='active';delete O.mine.state().relation;
const html=C.materialPage(true);assert.equal((html.match(/<th>/g)||[]).length,15);assert.match(html,/平台作品 ID/);assert.match(html,/团队使用/);
let batch=O.mine.download([1,2]);assert.equal(batch.entries.filter(x=>x.result==='running').length,2);
O.mine.advance(batch,'progress');assert.equal(O.mine.video(C.db.materials[0]).canCompose,true);
O.mine.advance(batch,'partial');assert.equal(batch.entries[0].result,'failed');assert.equal(batch.entries[1].result,'success');assert.equal(O.mine.video(C.db.materials[0]).canCompose,true);
let retry=O.mine.download([1]);assert.equal(C.usage(1).transfers.current.phase,'downloading');O.mine.advance(retry,'success');assert.equal(O.mine.video(C.db.materials[0]).download,false);
O.mineDevice='other';assert.equal(O.mine.video(C.db.materials[0]).download,true);assert.equal(O.mine.video(C.db.materials[0]).key,'available');
O.actor='林予安';assert.equal(O.mine.video(C.db.materials[0]).key,'pending');O.library.usageDetail(1);assert.doesNotMatch(currentDialog.html,/陈小雨|林予安|<table>/);
O.actor='陈小雨';O.library.usageDetail(1);assert.match(currentDialog.html,/陈小雨/);assert.match(currentDialog.html,/林予安/);
C.usage(1).status='removed';O.mine.relation(1,true);assert.equal(C.usage(1).status,'active');assert.equal(C.usage(1).added,new Date().toLocaleString('sv-SE',{timeZone:'Asia/Shanghai'}).slice(0,16));assert.equal(JSON.stringify(C.db.materials),before);
C.route='tasks';assert.equal(C.materialDetail(1),'<old-detail>');
C.route='mine';O.mineDevice='current';delete C.usage(1).files.current;O.mine.download([1]);vm.runInContext(fs.readFileSync('example/proto/content-operations/production/mine.js','utf8'),ctx);assert.equal(O.mine.video(C.db.materials[0]).key,'failed');assert.equal(O.mine.video(C.db.materials[0]).download,true);
console.log('PASS: 14 columns, ordering, partial retry, prepared-resource retention, role/device isolation, restore time, source preservation, selector fallback, reload recovery');

// Review regression: no detail download, and a shared in-progress guard for batch/row actions.
C.dialogs=[];O.mineDevice='current';
for(const id of [1,2]){const u=C.usage(id);delete u.files;delete u.preparedDemo;delete u.transfers;delete u.videoDemo;}
C.db.materials[0].cloudState='preparing';C.db.materials[1].cloudState='notSaved';
O.mine.state().chosen=[1];
assert.match(C.materialPage(true),/<button[^>]*disabled[^>]*>下载<\/button>/);
const waiting=O.mine.download([1]);assert.equal(waiting.entries[0].result,'skipped');assert.equal(C.usage(1).transfers,undefined);
const running=O.mine.download([2]);O.mine.advance(running,'progress');
const previousTransfer=JSON.stringify(C.usage(2).transfers);
const duplicate=O.mine.download([2]);assert.equal(duplicate.entries[0].result,'skipped');assert.equal(JSON.stringify(C.usage(2).transfers),previousTransfer);
O.mine.state().chosen=[2];assert.match(C.materialPage(true),/<button[^>]*disabled[^>]*>下载<\/button>/);
for(const id of [1,2]){C.materialDetail(id);assert.doesNotMatch(currentDialog.html,/>下载<\/button>/);}
O.mine.advance(running,'success');C.materialDetail(2);assert.doesNotMatch(currentDialog.html,/>下载<\/button>/);
O.mine.setDemo(2,'pending');C.materialDetail(2);assert.doesNotMatch(currentDialog.html,/>下载<\/button>/);
assert.equal(M.state({cloudState:'ready'},{status:'active'},'a').download,false);
console.log('PASS: read-only download presentation; preparing/downloading batch guards; no transfer overwrite; unknown state action consistency');

O.actor='陈小雨';
assert.equal(O.library.advancedCount({from:'2026-11-01',to:'2026-11-04',enteredFrom:'2026-11-01',enteredTo:'2026-11-04',creator:'陈小雨'}),3);
assert.equal(O.library.advancedCount({q:'107',platforms:['百度']}),0);
assert.match(O.library.advancedFilters('mine',{from:'2026-11-01',to:'2026-11-04',advanced:false},''),/已生效 1 项/);
assert.doesNotMatch(O.library.advancedFilters('mine',{from:'2026-11-01',advanced:false},''),/ open/);
O.actor='林予安';assert.equal(O.library.advancedCount({creator:'陈小雨'}),0);
O.actor='陈小雨';C.route='materials';const libhtml=C.materialPage(false);
assert.equal((libhtml.match(/<th>/g)||[]).length,14);assert.doesNotMatch(libhtml,/匹配 \d+ 条 · 按/);assert.match(libhtml,/ex-pagination-compact/);
console.log('PASS: applied advanced count, range grouping, collapsed context, permission-aware count, library fields and pagination');

// Video access uses verified resources for this actor/device, independently of platform status.
const videoFacts=M.resources(ready,{status:'active',files:{a:{exists:true,verified:true,path:'/downloads/a.mp4'}}},'a');
assert.equal(videoFacts.remote,true);assert.equal(videoFacts.cloudUrl,ready.cloudUrl);assert.equal(videoFacts.file.path,'/downloads/a.mp4');
assert.equal(M.resources(ready,{files:{a:{exists:true,verified:true,path:'/downloads/a.mp4'}}},'b').local,false);
assert.equal(M.resources({...ready,cloudUrlExpiresAt:'2020-01-01'},{},'a').remote,false);
O.actor='陈小雨';O.mineDevice='current';C.route='mine';O.mine.setDemo(1,'downloaded');
for(const route of ['mine','materials']){C.route=route;C.materialDetail(1);assert.match(currentDialog.html,/打开云端视频/);assert.match(currentDialog.html,/打开所在文件夹/);assert.doesNotMatch(currentDialog.html,/href="https:\/\/media.example.test/);assert.doesNotMatch(currentDialog.html,/>下载<\/button>/);}
O.mineDevice='other';C.materialDetail(1);assert.doesNotMatch(currentDialog.html,/打开所在文件夹/);
O.mineDevice='current';O.actor='林予安';C.materialDetail(1);assert.doesNotMatch(currentDialog.html,/打开所在文件夹/);
O.actor='陈小雨';O.mine.setDemo(1,'missing');C.materialDetail(1);assert.match(currentDialog.html,/打开云端视频/);assert.doesNotMatch(currentDialog.html,/打开所在文件夹/);
C.usage(1).files.current={exists:true,verified:true};C.materialDetail(1);assert.match(currentDialog.html,/文件位置未记录/);assert.doesNotMatch(currentDialog.html,/打开所在文件夹/);
console.log('PASS: cloud/local access, expired URL, source independence, demo boundaries, device/user isolation, missing path/file, read-only details');
