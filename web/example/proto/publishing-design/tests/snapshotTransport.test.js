import {it,expect} from 'vitest';
import http from 'node:http';
import {gzipSync} from 'node:zlib';
import {mkdtemp,rm} from 'node:fs/promises';
import os from 'node:os';
import path from 'node:path';
import {createHash,randomBytes} from 'node:crypto';
import {packSnapshot,unpackSnapshot} from '../snapshotTransport.js';
import {createPublishingMiddleware} from '../../../publishing-preview.mjs';
import {loadBatchSnapshot,saveBatchSnapshot} from '../batchApi.js';

it('real snapshot transport saves repeated original images and generated covers over 40MB without losing bytes on reload',async()=>{
 const root=await mkdtemp(path.join(os.tmpdir(),'wt-cover-transport-'));
 const middleware=createPublishingMiddleware({webRoot:root});const server=http.createServer((req,res)=>middleware(req,res,()=>{res.statusCode=404;res.end();}));
 await new Promise(resolve=>server.listen(0,'127.0.0.1',resolve));
 const base=`http://127.0.0.1:${server.address().port}`,fetchHTTP=globalThis.fetch;let encoding;
 globalThis.fetch=(url,options)=>{if(options?.method==='PUT')encoding=options.headers['Content-Encoding'];return fetchHTTP(new URL(url,base),options);};
 try{
  const db={tasks:[],batches:[],plans:[],images:[]};await loadBatchSnapshot(db);
  const source='data:image/png;base64,'+randomBytes(1536*1024).toString('base64');
  const output='data:image/png;base64,'+randomBytes(1536*1024).toString('base64');
  db.images=[{id:'original',url:source}];
  db.tasks=Array.from({length:6},(_,i)=>({id:i+1,status:'待执行',cover:source,coverVariants:{'4:3':output,'16:9':output},snapshot:{cover:source,coverVariants:{'4:3':output,'16:9':output}}}));
  expect(JSON.stringify(db).length).toBeGreaterThan(40*1024*1024);
  await saveBatchSnapshot(db);expect(encoding).toBe('gzip');
  const saved=await loadBatchSnapshot();expect(saved.tasks).toEqual(db.tasks);expect(saved.images).toEqual(db.images);
  // Older JSON clients, malformed gzip and write conflicts retain their original guarantees.
  const conflict=await fetchHTTP(base+'/__example/publishing/snapshot',{method:'PUT',headers:{'Content-Type':'application/json','Content-Encoding':'gzip'},body:gzipSync(JSON.stringify(packSnapshot({data:db,revision:1})))});expect(conflict.status).toBe(409);
  const broken=await fetchHTTP(base+'/__example/publishing/snapshot',{method:'PUT',headers:{'Content-Encoding':'gzip'},body:'invalid gzip'});expect(broken.status).toBe(422);
  expect((await loadBatchSnapshot()).tasks).toEqual(db.tasks);
 }finally{globalThis.fetch=fetchHTTP;await new Promise(resolve=>server.close(resolve));const hash=createHash('sha256').update(root+new URL(base).host).digest('hex').slice(0,16);await rm(path.join(os.tmpdir(),'wt-media-example',hash+'.json'),{force:true});await rm(root,{recursive:true,force:true});}
},30000);

it('packing shares embedded signature images, preserves arbitrary JSON values and rejects malformed references or expansion abuse',()=>{
 const image='data:image/png;base64,'+randomBytes(12000).toString('base64');
 const value={data:{image,snapshot:{image},signature:JSON.stringify([image,{image}])},reserved:{format:'wt-media-snapshot-strings-v1',value:[0,99]},array:[null,true,0,'短文案',{}]};
 const packed=packSnapshot(value);expect(packed.strings).toEqual([image]);expect(unpackSnapshot(JSON.parse(JSON.stringify(packed)))).toEqual(value);
 expect(()=>unpackSnapshot({...packed,value:[0,99]})).toThrow(/格式/);
 expect(()=>unpackSnapshot(packSnapshot(Array(100).fill(image)),{maxBytes:100000})).toThrow(/展开上限/);
});
