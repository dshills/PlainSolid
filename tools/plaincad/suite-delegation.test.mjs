import {test} from 'node:test';
import assert from 'node:assert/strict';
import {suitePorts,passedProbe,verifyDelegatedReport} from './suite-delegation.mjs';
test('suite defaults and independent preflight ports',()=>{
 assert.deepEqual(suitePorts('development'),{acceptance:'5279',preflight:'5279'});
 assert.deepEqual(suitePorts('production'),{acceptance:'5280',preflight:'5279'});
 assert.deepEqual(suitePorts('cross-browser'),{acceptance:'5281',preflight:'5279'});
 assert.deepEqual(suitePorts('cross-browser','5427','5428'),{acceptance:'5427',preflight:'5428'});
 for(const v of ['','NaN','80','65536'])assert.throws(()=>suitePorts('production',v));
});
test('probe evidence must pass and delegation cannot hide skipped or absent browsers',()=>{
 const test={projectName:'chromium',status:'expected',results:[{status:'passed',duration:1,errors:[]}]};
 const probe={suites:[{specs:[{file:'backend.spec.ts',title:'PlainSolid backend: native',tests:[test]}]}]};
 assert.equal(passedProbe(probe).length,1);
 assert.throws(()=>passedProbe({suites:[]}));
 const report={status:'passed',port:'5427',tests:['chromium','firefox','webkit'].map(projectName=>({...test,projectName}))};
 verifyDelegatedReport(report,'cross-browser','5427');
 assert.throws(()=>verifyDelegatedReport({...report,status:'failed'},'cross-browser','5427'));
 assert.throws(()=>verifyDelegatedReport({...report,tests:report.tests.slice(1)},'cross-browser','5427'));
 assert.throws(()=>verifyDelegatedReport(report,'cross-browser','5281'));
 assert.throws(()=>verifyDelegatedReport({...report,tests:[{...test,results:[{status:'skipped'}]}]},'production','5427'));
 assert.throws(()=>verifyDelegatedReport({...report,tests:[]},'production','5427'));
});
