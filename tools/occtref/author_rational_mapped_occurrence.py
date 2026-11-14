# Author one exact noncardinal mapped occurrence independently of Go. Retain the
# original OCCT box/model/pcurve graph and all other instances unchanged.
import re,sys
from pathlib import Path
s=Path(sys.argv[1]).read_text()
records={int(i):v.strip() for i,v in re.findall(r'#(\d+)\s*=\s*([^;]+);',s)}
item=next(v for v in records.values() if v.startswith("MAPPED_ITEM('first translated'"))
target=int(re.findall(r'#(\d+)',item)[-1]);n=max(records)
extra=f"\n#{n+1}=DIRECTION('',(3.,4.,0.));\n#{n+2}=DIRECTION('',(-4.,3.,0.));\n"
old=records[target];refs=list(map(int,re.findall(r'#(\d+)',old)));assert len(refs)==4
new=f"CARTESIAN_TRANSFORMATION_OPERATOR_3D('','','',#{n+1},#{n+2},#{refs[2]},1.,#{refs[3]})"
s=re.sub(r'#'+str(target)+r'\s*=\s*[^;]+;',f'#{target}={new};',s)
# Old axes remain shared by other mapped operators, so no geometry is orphaned.
pos=s.rfind('ENDSEC;');s=s[:pos]+extra+s[pos:]
Path(sys.argv[2]).write_text(s)
