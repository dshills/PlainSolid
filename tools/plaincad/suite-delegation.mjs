export function suitePorts(suite, port, preflightPort) {
 if(!['development','production','cross-browser','unit'].includes(suite))throw Error('Invalid suite');
 const acceptance=port??({development:'5279',production:'5280','cross-browser':'5281',unit:'5279'}[suite]);
 const preflight=preflightPort??(suite==='development'?acceptance:'5279');
 for(const value of [acceptance,preflight])if(!/^\d+$/.test(value)||+value<1024||+value>65535)throw Error('Invalid suite port');
 if(suite==='development'&&acceptance!==preflight)throw Error('Development preflight and acceptance share one port');
 return {acceptance,preflight};
}
export function reportCases(result) {
 const tests=[];
 const visit=s=>{for(const spec of s.specs??[])for(const test of spec.tests??[])tests.push({file:spec.file,line:spec.line,title:spec.title,project:test.projectName,status:test.status,results:test.results.map(r=>({status:r.status,durationMs:r.duration,errors:r.errors}))});for(const child of s.suites??[])visit(child)};
 for(const s of result.suites??[])visit(s);return tests;
}
export function passedProbe(result) {
 const tests=reportCases(result);
 if(!tests.length||!tests.every(t=>t.title.startsWith('PlainSolid backend:')&&t.status==='expected'&&t.results.length>0&&t.results.every(r=>r.status==='passed')))throw Error('Backend probe did not pass');
 return tests;
}
export function verifyDelegatedReport(report,suite,port){
 if(report.status!=='passed'||report.port!==port||!report.tests?.length||report.tests.some(t=>t.status!=='expected'||!t.results?.length||t.results.some(r=>r.status!=='passed')))throw Error('Delegated acceptance failed');
 if(suite==='cross-browser'&&['chromium','firefox','webkit'].some(name=>!report.tests.some(t=>t.projectName===name)))throw Error('Delegated browser project missing');
}
