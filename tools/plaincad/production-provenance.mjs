import {execFileSync} from 'node:child_process';
import {mkdtemp,mkdir,writeFile,readFile,rm} from 'node:fs/promises';
import {tmpdir} from 'node:os';
import {join,dirname} from 'node:path';
import {installDisplayRuntime} from './display-runtime.mjs';

export const displayFiles=['src/cad/worker/displayMeshClient.ts','src/state/useCadStore.ts','src/cad/worker/geometryWorker.ts','src/viewer/CadViewer.tsx'];
const sourceRoots=['src','e2e','production-e2e','cross-browser-e2e'];
// A single batch retains exact bytes and does not execute app source.
export function originalSources(r,hash){
 const files=Object.keys(r.sourceHashes??{});
 if(files.some(p=>p.includes('\n')||p.includes('\r')||p.startsWith('/')||p.split('/').includes('..')))throw Error('Unsafe original source path');
 const head=r.sourceIdentity?.head;
 if(!/^[a-f0-9]{40}$/.test(head??''))throw Error('Invalid original revision');
 const cwd=r.sourceIdentity.sourceRepository??r.source;
 if(r.sourceIdentity.cleanCommittedArchive||r.sourceIdentity.status===''){
  const names=execFileSync('git',['ls-tree','-r','--name-only','-z',head,'--',...sourceRoots],{cwd,encoding:'utf8',maxBuffer:16*1024*1024});
  for(const file of names.split('\0'))if(/\.[cm]?[jt]sx?$/.test(file)&&!Object.hasOwn(r.sourceHashes??{},file))throw Error(`Unrecorded pinned source: ${file}`);
 }
 const bytes=execFileSync('git',['cat-file','--batch'],{cwd,input:files.map(p=>`${head}:${p}\n`).join(''),maxBuffer:128*1024*1024});
 let offset=0;const originals=new Map(),missing=[];
 for(const file of files){
  const end=bytes.indexOf(10,offset);if(end<0)throw Error('Malformed original source batch');
  const header=bytes.subarray(offset,end).toString();offset=end+1;
  if(header.endsWith(' missing')){missing.push(file);continue;}
  const match=/^[a-f0-9]+ blob (\d+)$/.exec(header);if(!match)throw Error('Original source is not a blob');
  const size=Number(match[1]),body=bytes.subarray(offset,offset+size);offset+=size+1;
  if(hash(body)!==r.sourceHashes[file])throw Error(`Original source hash mismatch: ${file}`);
  originals.set(file,body.toString('utf8'));
 }
 if((r.sourceIdentity.cleanCommittedArchive||r.sourceIdentity.status==='')&&missing.length)throw Error('Committed source files missing');
 return {originals,missing};
}
export function verifyProviderLabels(r,originals,hash){
 const adapted=new Map();
 for(const a of r.providerLabelAdaptations??[]){
  if(adapted.has(a.file)||!Object.hasOwn(r.sourceHashes??{},a.file)||r.sourceHashes[a.file]!==a.beforeSha256)throw Error(`Invalid provider adaptation: ${a.file}`);
  const before=originals.get(a.file);
  if(before===undefined)throw Error(`Missing pinned provider source: ${a.file}`);
  const pattern=/(["'])opencascade\1/g,count=[...before.matchAll(pattern)].length;
  const after=before.replace(pattern,'$1plainsolid$1');
  if(!count||count!==a.count||hash(after)!==a.afterSha256)throw Error(`Invalid provider adaptation: ${a.file}`);
  adapted.set(a.file,a);
 }
 return adapted;
}
export async function replayDisplay(r,originals,hash){
 const selected=(r.runtimeAdaptations??[]).filter(a=>displayFiles.includes(a.file));
 if(!selected.length)return new Map();
 if(selected.length!==displayFiles.length||new Set(selected.map(a=>a.file)).size!==displayFiles.length)throw Error('Incomplete display runtime adaptation');
 const directory=await mkdtemp(join(tmpdir(),'plainsolid-display-replay-'));
 try{
  for(const file of displayFiles){
   let before=originals.get(file);if(before===undefined)throw Error(`Missing pinned display source: ${file}`);
   const labels=(r.providerLabelAdaptations??[]).filter(a=>a.file===file);
   if(labels.length>1)throw Error('Duplicate provider adaptation');
   if(labels.length){const a=labels[0],matches=[...before.matchAll(/(["'])opencascade\1/g)];const after=before.replace(/(["'])opencascade\1/g,'$1plainsolid$1');
    if(a.beforeSha256!==hash(before)||a.afterSha256!==hash(after)||a.count!==matches.length)throw Error('Invalid display provider adaptation');before=after;
   }
   await mkdir(dirname(join(directory,file)),{recursive:true});await writeFile(join(directory,file),before);
  }
  const generated=await installDisplayRuntime(directory,hash),out=new Map();
  if(generated.length!==selected.length)throw Error('Incomplete display runtime replay');
  for(const actual of generated){const claimed=selected.find(a=>a.file===actual.file);
   if(!claimed||claimed.beforeSha256!==actual.beforeSha256||claimed.afterSha256!==actual.afterSha256)throw Error('Invalid display runtime replay');
   out.set(actual.file,await readFile(join(directory,actual.file),'utf8'));
  }
  return out;
 }finally{await rm(directory,{recursive:true,force:true});}
}
