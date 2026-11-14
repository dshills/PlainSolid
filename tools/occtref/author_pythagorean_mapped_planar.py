# Independently transform the original OCCT box model and owner pcurve graph.
# Scale its 2x3x4 local body by5, rotate by exact (3,-4;4,3)/5, translate(5,-10,2).
# Mapped uses, other local product, source topology and all context records remain.
import re,sys
from pathlib import Path
from fractions import Fraction as Q
from math import lcm
s=Path(sys.argv[1]).read_text()
records={int(i):v for i,v in re.findall(r'#(\d+)\s*=\s*([^;]+);',s)}
body=next(i for i,v in records.items() if v.startswith('MANIFOLD_SOLID_BREP'))
used=set()
def walk(i):
 if i in used:return
 used.add(i)
 for j in map(int,re.findall(r'#(\d+)',records[i])):walk(j)
walk(body)
changed={}
for i in used:
 m=re.fullmatch(r"(CARTESIAN_POINT|DIRECTION)\('',\s*\(([^)]+)\)\)",records[i],re.S)
 if not m:continue
 v=[Q(x.strip()) for x in m[2].split(',')]
 if len(v)==3:
  a,b,c=v
  if m[1]=='CARTESIAN_POINT':v=[3*a-4*b+5,4*a+3*b-10,5*c+2]
  else:
   v=[(3*a-4*b)/5,(4*a+3*b)/5,c];d=lcm(*(x.denominator for x in v));v=[x*d for x in v]
 elif len(v)==2 and m[1]=='CARTESIAN_POINT':v=[x*5 for x in v]
 else:continue
 assert all(x.denominator==1 for x in v),v
 changed[i]=m[1]+"('',("+','.join(str(x.numerator)+'.' for x in v)+'))'
s=re.sub(r'#(\d+)\s*=\s*([^;]+);',lambda m:'#'+m[1]+' = '+changed.get(int(m[1]),m[2])+';',s)
Path(sys.argv[2]).write_text(s)
