import {test} from 'node:test';
import assert from 'node:assert/strict';
import {mkdtemp,mkdir,writeFile,readFile,rm} from 'node:fs/promises';
import {tmpdir} from 'node:os';
import {join,dirname} from 'node:path';
import {createHash} from 'node:crypto';
import {execFileSync} from 'node:child_process';
import {originalSources,replayDisplay,displayFiles,verifyProviderLabels} from './production-provenance.mjs';
import {installDisplayRuntime} from './display-runtime.mjs';
const hash=b=>createHash('sha256').update(b).digest('hex');
test('display replay binds pinned originals and rejects altered or incomplete patches',async()=>{
 const root=await mkdtemp(join(tmpdir(),'display-proof-test-'));
 try{
 const texts=[
 '  return backgroundJob<DisplayMeshRequest, DisplayMesh[]>(PreviewWorker, request, signal, undefined, 20000);',
 'function getGeometryWorker(): Worker | undefined {',
 'self.onmessage = (event: MessageEvent<WorkerRequest>) => {\n  const request = event.data;',
 '    const frames = new DemandRenderer(() => {\n      if (!host.clientWidth || !host.clientHeight || renderer.getContext().isContextLost()) {\n        publishRenderStatus("");\n        return false;\n      }\n      const changed = controls.update();\n      return changed || (moving && !interacting);'];
 const r={source:root,sourceHashes:{},sourceIdentity:{cleanCommittedArchive:true}};
 for(let i=0;i<displayFiles.length;i++){const p=join(root,displayFiles[i]);await mkdir(dirname(p),{recursive:true});await writeFile(p,texts[i]);r.sourceHashes[displayFiles[i]]=hash(texts[i]);}
 const git=(...args)=>execFileSync('git',args,{cwd:root,encoding:'utf8',stdio:['ignore','pipe','ignore']});
 git('init');git('add','.');git('-c','user.name=Test','-c','user.email=test@example.invalid','commit','-m','pinned');r.sourceIdentity.head=git('rev-parse','HEAD').trim();
 const {originals,missing}=originalSources(r,hash);assert.equal(originals.size,displayFiles.length);assert.deepEqual(missing,[]);
 delete r.sourceHashes[displayFiles[1]];
 assert.throws(()=>originalSources(r,hash),/Unrecorded pinned source/);
 r.sourceHashes[displayFiles[1]]=hash(texts[1]);
 r.runtimeAdaptations=await installDisplayRuntime(root,hash);
 const replay=await replayDisplay(r,originals,hash);assert.equal(replay.size,displayFiles.length);
 for(const f of displayFiles)assert.equal(replay.get(f),await readFile(join(root,f),'utf8'));
 const good=structuredClone(r.runtimeAdaptations);
 r.runtimeAdaptations[1].afterSha256=hash(replay.get(displayFiles[1]).replace('20000','200000'));
 await assert.rejects(replayDisplay(r,originals,hash),/Invalid display runtime replay/);
 r.runtimeAdaptations=good.slice(1);await assert.rejects(replayDisplay(r,originals,hash),/Incomplete/);
 r.runtimeAdaptations=[good[0],good[0],...good.slice(2)];await assert.rejects(replayDisplay(r,originals,hash),/Incomplete/);
 r.runtimeAdaptations=good;await assert.rejects(replayDisplay(r,new Map(),hash),/Missing pinned/);
 r.sourceHashes[displayFiles[0]]=hash('tampered original');assert.throws(()=>originalSources(r,hash),/Original source hash mismatch/);
 }finally{await rm(root,{recursive:true,force:true});}
});

test('provider substitution cannot authorize modified assertions',()=>{
 const file='e2e/example.spec.ts',before='expect(provider).toBe("opencascade");',after=before.replace('opencascade','plainsolid');
 const label={file,count:1,beforeSha256:hash(before),afterSha256:hash(after)};
 const r={sourceHashes:{[file]:hash(before)},providerLabelAdaptations:[label]},originals=new Map([[file,before]]);
 assert.equal(verifyProviderLabels(r,originals,hash).size,1);
 label.afterSha256=hash(after+' // changed assertion');
 assert.throws(()=>verifyProviderLabels(r,originals,hash),/Invalid provider adaptation/);
 label.afterSha256=hash(after);r.providerLabelAdaptations.push(label);
 assert.throws(()=>verifyProviderLabels(r,originals,hash),/Invalid provider adaptation/);
});
