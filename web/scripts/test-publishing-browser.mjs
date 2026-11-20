import { createServer } from 'vite';
import { spawn } from 'node:child_process';
import { readdir, mkdtemp, mkdir } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const root=fileURLToPath(new URL('../',import.meta.url));
const tests=path.join(root,'example/proto/publishing-design/tests');
const requested=new Set(process.argv.slice(2));
const files=(await readdir(tests)).filter(name=>name.endsWith('.browser.cjs')&&(!requested.size||requested.has(name))).sort().map(name=>path.join(tests,name));
if(!files.length||requested.size&&files.length!==requested.size)throw Error('Unknown browser test file');
const output=process.env.SCREENSHOT_DIR||await mkdtemp(path.join(tmpdir(),'wt-publishing-browser-'));
await mkdir(output,{recursive:true});
const server=await createServer({cacheDir:path.join(output,'.vite-example'),configFile:path.join(root,'vite.config.example.js'),server:{host:'127.0.0.1',port:0,strictPort:false,open:false}});
let child;
try {
  await server.listen();
  const base=server.resolvedUrls.local[0].replace(/\/$/,'');
  console.log(`Browser checks: ${files.length} files; temporary output: ${output}`);
  child=spawn(process.execPath,['--test','--test-concurrency=1','--test-reporter=tap',...files],{cwd:root,stdio:'inherit',env:{...process.env,EXAMPLE_URL:base,SCREENSHOT_DIR:output}});
  process.exitCode=await new Promise((resolve,reject)=>{child.on('error',reject);child.on('exit',(code,signal)=>resolve(signal?1:code??1));});
} finally {
  if(child?.exitCode===null)child.kill();
  await server.close();
}
