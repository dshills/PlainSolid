import {test} from 'node:test';
import assert from 'node:assert/strict';
import {mkdtempSync,mkdirSync,writeFileSync,rmSync} from 'node:fs';
import {tmpdir} from 'node:os';
import {join,resolve} from 'node:path';
import {createHash} from 'node:crypto';
import {execFileSync,spawnSync} from 'node:child_process';
const script=resolve('tools/plaincad/cross-browser.mjs'),hash=x=>createHash('sha256').update(x).digest('hex');
test('usage and unknown flags reject without launching',()=>{assert.equal(spawnSync(process.execPath,[script,'--help']).status,0);assert.notEqual(spawnSync(process.execPath,[script,'--wat']).status,0)});
test('check validates staged provenance and rejects changed/unrecorded tests',()=>{
 const root=mkdtempSync(join(tmpdir(),'production-harness-'));try{
 const source=join(root,'source'),work=join(root,'work');mkdirSync(source);mkdirSync(work);
 const config='export default {workers: 1, projects:[{name:"chromium"},{name:"firefox"},{name:"webkit"}], timeout: 90000, port: 5281};\n';writeFileSync(join(source,'playwright.cross-browser.config.ts'),config);
 const git=(...a)=>execFileSync('git',a,{cwd:source,encoding:'utf8',stdio:['ignore','pipe','ignore']});git('init');git('add','.');git('-c','user.name=Test','-c','user.email=test@example.invalid','commit','-m','fixture');
 for(const d of ['cross-browser-e2e','src/cad/kernel','public/assets'])mkdirSync(join(work,d),{recursive:true});
 const fixture='original assertions',adapter='adapter',wasm='wasm';writeFileSync(join(work,'cross-browser-e2e/a.spec.ts'),fixture);writeFileSync(join(work,'src/cad/kernel/OpenCascadeKernel.ts'),adapter);writeFileSync(join(work,'public/assets/plainsolid-kernel.wasm'),wasm);writeFileSync(join(work,'playwright.cross-browser.config.ts'),config);
 const r={backend:'plainsolid-go-wasm',workspace:work,source,sourceIdentity:{head:git('rev-parse','HEAD').trim()},typecheck:'passed',backendVerified:true,tests:[{title:'PlainSolid backend: probe',status:'expected',results:[{status:'passed'}]}],sourceHashes:{'cross-browser-e2e/a.spec.ts':hash(fixture)},plainSolidSourceHashes:{'tools/plaincad/PlainSolidKernel.ts':hash(adapter)},wasmSha256:hash(wasm)};writeFileSync(join(work,'readiness.json'),JSON.stringify(r));
 const check=()=>spawnSync(process.execPath,[script,'--work',work,'--check'],{encoding:'utf8'});
 let result=check();assert.equal(result.status,0,result.stderr);assert.equal(JSON.parse(result.stdout).status,'verified');
 writeFileSync(join(work,'cross-browser-e2e/a.spec.ts'),'changed assertion');assert.match(check().stderr,/Staged source changed/);writeFileSync(join(work,'cross-browser-e2e/a.spec.ts'),fixture);

 assert.equal(JSON.parse(check().stdout).port,'5281');
 const fixed='focusedWorkspaceStorageState("http://127.0.0.1:5281")';
 writeFileSync(join(work,'cross-browser-e2e/a.spec.ts'),fixed);r.sourceHashes['cross-browser-e2e/a.spec.ts']=hash(fixed);writeFileSync(join(work,'readiness.json'),JSON.stringify(r));
 assert.equal(check().status,0);
 const alternate=(...extra)=>spawnSync(process.execPath,[script,'--work',work,'--port','5415','--check',...extra],{encoding:'utf8'});
 assert.match(alternate().stderr,/Fixed-origin fixture/);
 assert.match(alternate('--test','cross-browser-e2e/a.spec.ts').stderr,/Fixed-origin fixture/);
 assert.equal(alternate('--test','cross-browser-e2e/other.spec.ts').status,0);
 mkdirSync(join(work,'src/cad/features'),{recursive:true});
 const prior='onNativeProgress?: callback\n        operationCount += 1;',after=prior+'\n        options.onNativeProgress?.("native_feature");';
 const path='src/cad/features/rebuildGraph.ts';r.sourceHashes[path]=hash(prior);r.runtimeAdaptations=[{file:path,beforeSha256:hash(prior),afterSha256:hash(after)}];writeFileSync(join(work,path),after);writeFileSync(join(work,'readiness.json'),JSON.stringify(r));
 assert.equal(check().status,0,check().stderr);
 r.runtimeAdaptations[0].afterSha256=hash(after+'changed');writeFileSync(join(work,path),after+'changed');writeFileSync(join(work,'readiness.json'),JSON.stringify(r));assert.match(check().stderr,/Invalid heartbeat/);
 writeFileSync(join(work,path),after);r.runtimeAdaptations[0].afterSha256=hash(after);writeFileSync(join(work,'readiness.json'),JSON.stringify(r));
 writeFileSync(join(work,'cross-browser-e2e/extra.spec.ts'),'extra');assert.match(check().stderr,/Unrecorded production input/);
 }finally{rmSync(root,{recursive:true,force:true})}
});

import {crossBrowserResults} from './cross-browser-results.mjs';
test('project qualification preserves skips, failures and missing engines',()=>{
 const names=['chromium','firefox','webkit'];
 const results=(statuses)=>({suites:[{specs:[{title:'native workflow',file:'cross-browser-e2e/native.spec.ts',tests:statuses.map((status,i)=>({projectName:names[i],projectId:names[i],expectedStatus:'passed',status:status==='passed'?'expected':'unexpected',results:[{status,duration:1,errors:[]}]}))}]}]});
 let r=crossBrowserResults(results(['passed','passed','passed']),names,0);assert.equal(r.status,'passed');assert.equal(r.projects.length,3);
 r=crossBrowserResults(results(['passed','skipped','failed']),names,1);assert.equal(r.status,'failed');assert.equal(r.projects[1].skipped,1);assert.equal(r.tests[2].results[0].status,'failed');
 assert.equal(crossBrowserResults(results(['passed']),names,0).status,'failed');
 const retry=results(['passed','passed','passed']);retry.suites[0].specs[0].tests[0].results.unshift({status:'failed',duration:2,errors:[{message:'retry'}]});assert.equal(crossBrowserResults(retry,names,0).status,'failed');
 assert.equal(crossBrowserResults(results(['passed']),['chromium'],0).status,'passed');
});
