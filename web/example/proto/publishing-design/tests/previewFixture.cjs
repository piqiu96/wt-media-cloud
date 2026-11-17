const {pathToFileURL}=require('node:url');
const path=require('node:path');
exports.attachStore=async function(page,data,onWrite=()=>{}){
  const {migrateRelationships}=await import(pathToFileURL(path.resolve(__dirname,'../relationships.js')));
  migrateRelationships(data);
  const {createBatchStore}=await import(pathToFileURL(path.resolve(__dirname,'../../../publishing-preview.mjs')));
  const store=createBatchStore();await store.bootstrap(data);
  await page.route('**/__example/publishing/**',async route=>{
    try {
      const request=route.request(),body=request.method()==='GET'?null:request.postDataJSON();
      const result=request.method()==='GET'?await store.get():request.url().endsWith('/command')?await store.command(body):request.url().endsWith('/bootstrap')?await store.bootstrap(body.data):await store.save(body.data,body.revision);
      if(request.method()!=='GET')onWrite(structuredClone(result.data));
      await route.fulfill({json:result});
    } catch(error){await route.fulfill({status:409,json:{error:error.message}});}
  });
  return store;
};
