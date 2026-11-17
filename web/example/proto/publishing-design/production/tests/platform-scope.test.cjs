const {test}=require('node:test'),assert=require('node:assert/strict'),fs=require('node:fs'),vm=require('node:vm'),path=require('node:path');
test('迁移保留素材来源与生产平台范围，无旧发布全局对象也可初始化',()=>{
 const context={window:{}};vm.createContext(context);
 vm.runInContext(fs.readFileSync(path.join(__dirname,'../runtime/data.js'),'utf8'),context);
 const seed=context.window.CPSeed();
 assert.deepEqual([...new Set(seed.materials.map(m=>m.platform))],['抖音']);
 assert.deepEqual([...seed.platforms],['B站','百度','小红书']);
 assert.equal(seed.accounts.some(a=>a.platform==='抖音'),false);
});
