const {test}=require('node:test'),assert=require('node:assert/strict'),vm=require('node:vm'),fs=require('node:fs'),path=require('node:path');
const read=file=>fs.readFileSync(path.join(__dirname,'..',file),'utf8');
test('日期范围兼容明确的2026年旧演示日期，未知日期不命中单边范围',()=>{
 const C={db:{productionRevision:4,tasks:[],strategies:[],materials:[]},prod:{},read:()=>({from:'2026-10-01',to:'2026-10-31'})};
 C.get=(kind,id)=>C.db[kind].find(x=>x.id===id);
 const context={CP:C,OPS:{actor:'陈小雨'}};vm.runInNewContext(read('runtime/experience.js'),context);
 assert.equal(C.match({created:'10-30 10:12'},'tasks'),true);
 assert.equal(C.match({date:'10-30 10:12'},'outputs'),true);
 assert.equal(C.match({created:'2026-11-01 10:12'},'tasks'),false);
 C.read=()=>({to:'2026-10-31'});
 assert.equal(C.match({created:'未知'},'tasks'),false);
 assert.equal(C.match({created:'2026-10-30 10:12'},'tasks'),true);
});
