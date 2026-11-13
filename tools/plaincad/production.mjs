#!/usr/bin/env node
// Optional second stage for run.mjs. Original tests/configuration stay intact.
import {readFile,writeFile,readdir,open} from 'node:fs/promises';
import {resolve,join,relative} from 'node:path';
import {createHash} from 'node:crypto';
import {execFileSync,spawn} from 'node:child_process';
import {originalSources,replayDisplay,displayFiles} from './production-provenance.mjs';

const usage='Usage: node tools/plaincad/production.mjs --work /staged/workspace [--port 5280] [--test production-e2e/file.spec.ts] [--grep expression] [--check | --run]\nDefault --check verifies inputs without building or running tests. --run builds and runs the selected originals (all production tests by default).';
const args=process.argv.slice(2), options={};
if(args.includes('--help')){console.log(usage);process.exit(0)}
for(let i=0;i<args.length;i++){
 const k=args[i];if(!['--work','--port','--test','--grep','--check','--run'].includes(k))throw Error(`Unknown option ${k}\n${usage}`);
 if(k in options)throw Error(`Duplicate ${k}`);
 if(k==='--check'||k==='--run')options[k]=true;
 else {if(!args[i+1]||args[i+1].startsWith('--'))throw Error(`Missing ${k} value`);options[k]=args[++i]}
}
if(!options['--work']||options['--check']&&options['--run'])throw Error(usage);
const work=resolve(options['--work']),port=options['--port']??'5280';
if(!/^\d+$/.test(port)||+port<1024||+port>65535)throw Error('Invalid port');
const hash=b=>createHash('sha256').update(b).digest('hex');
const readinessPath=join(work,'readiness.json'),readinessBytes=await readFile(readinessPath),r=JSON.parse(readinessBytes);
if(r.backend!=='plainsolid-go-wasm'||r.typecheck!=='passed'||!r.backendVerified||!r.tests?.some(t=>t.title?.startsWith('PlainSolid backend:')&&t.status==='expected'&&t.results?.length&&t.results.every(x=>x.status==='passed')))throw Error('Require passing staged typecheck and real backend probe');
if(resolve(r.workspace)!==work)throw Error('Readiness belongs to a different workspace');
const adaptations=new Map((r.providerLabelAdaptations??[]).map(x=>[x.file,x]));
const {originals,missing}=originalSources(r,hash);
const displayReplay=await replayDisplay(r,originals,hash);
const runtime=new Map();
for(const a of r.runtimeAdaptations??[]){
 if((a.file!=='src/cad/features/rebuildGraph.ts'&&!displayFiles.includes(a.file))||runtime.has(a.file))throw Error('Unsupported runtime adaptation');
 runtime.set(a.file,a);
}
let checked=0,production=0;
for(const [file,before] of Object.entries(r.sourceHashes??{})){
 if(file==='src/cad/kernel/OpenCascadeKernel.ts')continue;
 if(relative(work,resolve(work,file)).startsWith('..'))throw Error('Unsafe source path');
 const a=adaptations.get(file);if(a&&a.beforeSha256!==before)throw Error(`Inconsistent adaptation ${file}`);
 const content=await readFile(join(work,file),'utf8'),rt=runtime.get(file);
 const prior=a?.afterSha256??before;
 if(rt&&displayReplay.has(file)){
  if(content!==displayReplay.get(file))throw Error('Staged display replay mismatch');
  runtime.delete(file);
 }else if(rt){
  const boundary='        operationCount += 1;', insertion='\n        options.onNativeProgress?.("native_feature");';
  if(rt.beforeSha256!==prior||rt.afterSha256!==hash(content)||content.split(boundary+insertion).length!==2||!content.includes('onNativeProgress?:')||hash(content.replace(boundary+insertion,boundary))!==prior)throw Error('Invalid heartbeat adaptation');
  runtime.delete(file);
 }else if(hash(content)!==prior)throw Error(`Staged source changed: ${file}`);
 if(port!=='5280'&&file.startsWith('production-e2e/')&&/https?:\/\/(?:127\.0\.0\.1|localhost):5280\b/.test(content)&&(!options['--test']||new RegExp(options['--test']).test(file)))throw Error(`Fixed-origin fixture requires original port 5280: ${file}`);
 checked++;if(file.startsWith('production-e2e/'))production++;
}
if(runtime.size)throw Error('Unbound runtime adaptation');
if(!checked||!production)throw Error('Missing original production fixture hashes');
for(const entry of await readdir(join(work,'production-e2e'),{recursive:true,withFileTypes:true})){if(entry.isFile()){const file=relative(work,join(entry.parentPath,entry.name));if(!(file in r.sourceHashes))throw Error(`Unrecorded production input: ${file}`)}}
const wasmPath=join(work,'public/assets/plainsolid-kernel.wasm');
if(hash(await readFile(wasmPath))!==r.wasmSha256)throw Error('Staged WASM changed');
const adapter=await readFile(join(work,'src/cad/kernel/OpenCascadeKernel.ts'));
if(hash(adapter)!==r.plainSolidSourceHashes?.['tools/plaincad/PlainSolidKernel.ts'])throw Error('Staged adapter changed');
const configFile='playwright.production.config.ts',config=await readFile(join(work,configFile),'utf8');
const original=execFileSync('git',['show',`${r.sourceIdentity.head}:${configFile}`],{cwd:r.sourceIdentity.sourceRepository??r.source,encoding:'utf8'});
if(config!==original)throw Error('Original production configuration changed');
if(!/workers:\s*1\b/.test(config)||!config.includes('5280'))throw Error('Expected original single-worker production config');
const derived=config.replaceAll('5280',port),configName='playwright.plainsolid-production.config.ts';
const report={schema:1,backend:r.backend,sourceIdentity:r.sourceIdentity,plainSolidIdentity:r.plainSolidIdentity,plainSolidSourceHashes:r.plainSolidSourceHashes,wasmSha256:r.wasmSha256,readinessSha256:hash(readinessBytes),sourceHashes:r.sourceHashes,providerLabelAdaptations:r.providerLabelAdaptations,runtimeAdaptations:r.runtimeAdaptations,adapterSha256:hash(adapter),originalConfigSha256:hash(config),derivedConfigSha256:hash(derived),port,selection:{test:options['--test']??'all production originals',grep:options['--grep']??null},checkedSourceFiles:checked,originalSourceVerification:{verified:originals.size,missing},startedAt:new Date().toISOString(),status:'verified',notes:['Original assertions and timeouts preserved; derived config changes only port5280.','Single worker; run with other expensive qualification jobs paused.','Selected passes do not qualify unselected tests.']};
if(!options['--run']){console.log(JSON.stringify(report,null,2));process.exit(0)}
await writeFile(join(work,configName),derived);
async function command(cmd,argv,log,env={}){
 const file=await open(join(work,log),'w');try{return await new Promise((yes,no)=>{const child=spawn(cmd,argv,{cwd:work,env:{...process.env,...env},stdio:['ignore',file.fd,file.fd]});child.on('error',no);child.on('exit',code=>yes(code))})}finally{await file.close()}
}
try{
 report.buildExitCode=await command('npm',['run','build'],'production-build.log');
 if(report.buildExitCode!==0)throw Error('Production build failed');
 if(hash(await readFile(join(work,'dist/assets/plainsolid-kernel.wasm')))!==r.wasmSha256)throw Error('Built WASM differs');
 const resultPath=join(work,'production-results.json'),argv=['node_modules/@playwright/test/cli.js','test','--config',configName];
 if(options['--test'])argv.push(options['--test']);if(options['--grep'])argv.push('--grep',options['--grep']);argv.push('--reporter=list,json');
 report.command=['node',...argv];report.exitCode=await command('node',argv,'production-tests.log',{PLAYWRIGHT_JSON_OUTPUT_FILE:resultPath});
 const bytes=await readFile(resultPath),result=JSON.parse(bytes);report.resultSha256=hash(bytes);report.stats=result.stats;report.tests=[];
 const visit=s=>{for(const spec of s.specs??[])for(const t of spec.tests??[])report.tests.push({title:spec.title,file:spec.file,status:t.status,results:t.results.map(x=>({status:x.status,durationMs:x.duration,errors:x.errors}))});for(const child of s.suites??[])visit(child)};
 for(const s of result.suites??[])visit(s);
 report.status=report.exitCode===0&&report.tests.length>0&&report.tests.every(t=>t.status==='expected'&&t.results.length&&t.results.every(x=>x.status==='passed'))?'passed':'failed';
}catch(e){report.status='failed';report.error=String(e)}
report.finishedAt=new Date().toISOString();await writeFile(join(work,'production-readiness.json'),JSON.stringify(report,null,2));console.log(`Production ${report.status}: ${join(work,'production-readiness.json')}`);if(report.status!=='passed')process.exitCode=1;
