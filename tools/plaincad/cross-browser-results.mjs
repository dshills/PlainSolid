// Preserve project identities and individual attempts; skips and expected
// failures are evidence, never successful cross-engine qualification.
export function crossBrowserResults(result, expectedProjects, exitCode) {
 const tests=[];
 const visit=s=>{for(const spec of s.specs??[])for(const t of spec.tests??[])tests.push({title:spec.title,file:spec.file,projectName:t.projectName,projectId:t.projectId,expectedStatus:t.expectedStatus,annotations:t.annotations,status:t.status,results:t.results.map(x=>({status:x.status,durationMs:x.duration,errors:x.errors}))});for(const child of s.suites??[])visit(child)};
 for(const s of result.suites??[])visit(s);
 const passed=t=>t.status==='expected'&&t.expectedStatus==='passed'&&t.results.length>0&&t.results.every(r=>r.status==='passed');
 const projects=expectedProjects.map(name=>({name,cases:tests.filter(t=>t.projectName===name).length,passed:tests.filter(t=>t.projectName===name&&passed(t)).length,skipped:tests.filter(t=>t.projectName===name&&t.results.some(r=>r.status==='skipped')).length}));
 const status=exitCode===0&&projects.length>0&&projects.every(p=>p.cases>0)&&tests.length>0&&tests.every(t=>expectedProjects.includes(t.projectName)&&passed(t))?'passed':'failed';
 return {tests,projects,status};
}
