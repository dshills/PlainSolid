"""Exact radius-only authored variant of retained model-only cylinder hierarchies.
The cylinder's local origin/axis stay unchanged. Only its owned 3D Cartesian
radial coordinates and analytic radii scale by 1/2; all product/pose records and
other local materials retain their original source declarations.
"""
from pathlib import Path
from fractions import Fraction
import re
root=Path('exchange/step/testdata')
for base in ['rational-spherical-hierarchy','occt-rational-spherical-hierarchy']:
 text=(root/(base+'.step')).read_text()
 records={int(k):v.strip() for k,v in re.findall(r'#(\d+)\s*=\s*(.*?);',text,re.S)}
 def closure(seed):
  seen=set();todo=[seed]
  while todo:
   k=todo.pop()
   if k in seen:continue
   seen.add(k);todo.extend(int(v) for v in re.findall(r'#(\d+)',records[k]))
  return seen
 bodies=[k for k,v in records.items() if v.startswith('MANIFOLD_SOLID_BREP')]
 targets=[closure(k) for k in bodies if any(records[q].startswith('CYLINDRICAL_SURFACE') for q in closure(k))]
 assert len(targets)==1
 for k in targets[0]:
  v=records[k]
  if v.startswith('CARTESIAN_POINT'):
   match=re.fullmatch(r"CARTESIAN_POINT\('([^']*)',\(([^()]*)\)\)",v)
   assert match
   coords=[Fraction(x.strip()) for x in match[2].split(',')]
   assert len(coords)==3 and coords[0] in [-2,0,2] and coords[1] in [-2,0,2]
   coords[0]/=2;coords[1]/=2
   assert all(x.denominator==1 for x in coords)
   records[k]="CARTESIAN_POINT('%s',(%s))"%(match[1],','.join(str(x.numerator)+'.' for x in coords))
  elif v.startswith(('CIRCLE(','CYLINDRICAL_SURFACE(')):
   match=re.fullmatch(r'(.*),([^,()]*)\)',v,re.S);assert match and Fraction(match[2])==2
   records[k]=match[1]+',1.)'
 out=text[:text.index('DATA;')+5]+'\n'+'\n'.join(f'#{k}={v};' for k,v in records.items())+'\nENDSEC;END-ISO-10303-21;\n'
 (root/(base+'-thin-cylinder.step')).write_text(out)
