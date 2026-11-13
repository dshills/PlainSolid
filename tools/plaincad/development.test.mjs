import {test} from 'node:test';
import assert from 'node:assert/strict';
import {mkdtempSync,mkdirSync,writeFileSync,rmSync,readFileSync} from 'node:fs';
import {tmpdir} from 'node:os';
import {join,resolve} from 'node:path';
import {createHash} from 'node:crypto';
import {execFileSync,spawnSync} from 'node:child_process';
const script=resolve('tools/plaincad/development.mjs'),hash=x=>createHash('sha256').update(x).digest('hex');
test('usage and unknown flags reject without launching',()=>{assert.equal(spawnSync(process.execPath,[script,'--help']).status,0);assert.notEqual(spawnSync(process.execPath,[script,'--wat']).status,0)});
test('check validates staged provenance and rejects changed/unrecorded tests',()=>{
 const root=mkdtempSync(join(tmpdir(),'development-harness-'));try{
 const source=join(root,'source'),work=join(root,'work');mkdirSync(source);mkdirSync(work);
 const config='export default {workers: 1, timeout: 90000, baseURL: "http://127.0.0.1:5279"};\n';writeFileSync(join(source,'playwright.config.ts'),config);
 const git=(...a)=>execFileSync('git',a,{cwd:source,encoding:'utf8',stdio:['ignore','pipe','ignore']});git('init');git('add','.');git('-c','user.name=Test','-c','user.email=test@example.invalid','commit','-m','fixture');
 for(const d of ['e2e','src/cad/kernel','public/assets'])mkdirSync(join(work,d),{recursive:true});
 const fixture='original assertions',adapter='adapter',wasm='wasm';writeFileSync(join(work,'e2e/a.spec.ts'),fixture);writeFileSync(join(work,'src/cad/kernel/OpenCascadeKernel.ts'),adapter);writeFileSync(join(work,'public/assets/plainsolid-kernel.wasm'),wasm);writeFileSync(join(work,'playwright.config.ts'),config);
 const r={backend:'plainsolid-go-wasm',workspace:work,source,sourceIdentity:{head:git('rev-parse','HEAD').trim()},typecheck:'passed',backendVerified:true,tests:[{title:'PlainSolid backend: probe',status:'expected',results:[{status:'passed'}]}],sourceHashes:{'e2e/a.spec.ts':hash(fixture)},plainSolidSourceHashes:{'tools/plaincad/PlainSolidKernel.ts':hash(adapter)},wasmSha256:hash(wasm)};writeFileSync(join(work,'readiness.json'),JSON.stringify(r));
 const check=()=>spawnSync(process.execPath,[script,'--work',work,'--check'],{encoding:'utf8'});
 let result=check();assert.equal(result.status,0,result.stderr);assert.equal(JSON.parse(result.stdout).status,'verified');
 writeFileSync(join(work,'playwright.config.ts'),config.replaceAll('5279','5499'));assert.equal(check().status,0,check().stderr);
 writeFileSync(join(work,'playwright.config.ts'),config.replace('90000','900000'));assert.match(check().stderr,/configuration changed beyond port/);writeFileSync(join(work,'playwright.config.ts'),config);
 writeFileSync(join(work,'e2e/a.spec.ts'),'changed assertion');assert.match(check().stderr,/Staged source changed/);writeFileSync(join(work,'e2e/a.spec.ts'),fixture);

 assert.equal(JSON.parse(check().stdout).port,'5279');
 const fixed='focusedWorkspaceStorageState("http://127.0.0.1:5279")';
 writeFileSync(join(work,'e2e/a.spec.ts'),fixed);r.sourceHashes['e2e/a.spec.ts']=hash(fixed);writeFileSync(join(work,'readiness.json'),JSON.stringify(r));
 assert.equal(check().status,0);
 const alternate=(...extra)=>spawnSync(process.execPath,[script,'--work',work,'--port','5415','--check',...extra],{encoding:'utf8'});
 assert.match(alternate().stderr,/Fixed-origin fixture/);
 assert.match(alternate('--test','e2e/a.spec.ts').stderr,/Fixed-origin fixture/);
 assert.equal(alternate('--test','e2e/other.spec.ts').status,0);
 mkdirSync(join(work,'src/cad/features'),{recursive:true});
 const prior='onNativeProgress?: callback\n        operationCount += 1;',after=prior+'\n        options.onNativeProgress?.("native_feature");';
 const path='src/cad/features/rebuildGraph.ts';r.sourceHashes[path]=hash(prior);r.runtimeAdaptations=[{file:path,beforeSha256:hash(prior),afterSha256:hash(after)}];writeFileSync(join(work,path),after);writeFileSync(join(work,'readiness.json'),JSON.stringify(r));
 assert.equal(check().status,0,check().stderr);
 r.runtimeAdaptations[0].afterSha256=hash(after+'changed');writeFileSync(join(work,path),after+'changed');writeFileSync(join(work,'readiness.json'),JSON.stringify(r));assert.match(check().stderr,/Invalid heartbeat/);
 writeFileSync(join(work,path),after);r.runtimeAdaptations[0].afterSha256=hash(after);writeFileSync(join(work,'readiness.json'),JSON.stringify(r));

 // Exercise runner/reporting without a browser: only a synthetic CLI writes a
 // standard Playwright result. It verifies the generated preflight is excluded.
 mkdirSync(join(work,'node_modules/@playwright/test'),{recursive:true});
 writeFileSync(join(work,'node_modules/@playwright/test/package.json'),'{}');
 writeFileSync(join(work,'node_modules/@playwright/test/cli.js'),`const fs=require('fs');const a=process.argv.slice(2);if(!a.some(x=>x.endsWith('$')&&x.includes('spec'))||a.some(x=>x.includes('backend')))process.exit(3);fs.writeFileSync(process.env.PLAYWRIGHT_JSON_OUTPUT_FILE,JSON.stringify({stats:{expected:1},suites:[{specs:[{title:'original',file:'e2e/a.spec.ts',tests:[{status:'expected',results:[{status:'passed',duration:1,errors:[]}]}]}]}]}));`);
 writeFileSync(join(work,'e2e/plainsolid-backend.spec.ts'),'generated probe');
 const run=spawnSync(process.execPath,[script,'--work',work,'--run'],{encoding:'utf8'});assert.equal(run.status,0,run.stderr+run.stdout+String(readFileSync(join(work,'development-readiness.json'))));
 const report=JSON.parse(readFileSync(join(work,'development-readiness.json')));assert.equal(report.status,'passed');assert.deepEqual(report.selectedOriginalSpecs,['e2e/a.spec.ts']);assert.equal(report.tests.length,1);assert.ok(report.runnerSha256);assert.ok(report.resultSha256);
 writeFileSync(join(work,'e2e/extra.spec.ts'),'extra');assert.match(check().stderr,/Unrecorded development input/);
 }finally{rmSync(root,{recursive:true,force:true})}
});
