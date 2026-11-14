"""Independent exact 3:4:5 AP214 target frames; original local bodies unchanged."""
from pathlib import Path
import re
root=Path('exchange/step/testdata');s=(root/'independent-spherical-hierarchy.step').read_text()
r={int(i):v for i,v in re.findall(r'#(\d+)=(.*?);',s,re.S)}
def target_at(position,x,z):
 for i,v in list(r.items()):
  if not v.startswith('AXIS2_PLACEMENT_3D'):continue
  p,za,xa=map(int,re.findall(r'#(\d+)',v))
  if r[p]!="CARTESIAN_POINT('',("+position+"))":continue
  r[xa]="DIRECTION('',("+x+"))";r[za]="DIRECTION('',("+z+"))";return
 raise ValueError(position)
target_at('20.,0.,0.','3.,0.,-4.','4.,0.,3.')
target_at('-20.,10.,0.','3.,4.,0.','0.,0.,1.')
(root/'rational-spherical-hierarchy.step').write_text(s[:s.index('DATA;')+5]+'\n'+'\n'.join(f'#{i}={v};' for i,v in r.items())+'\nENDSEC;END-ISO-10303-21;\n')
