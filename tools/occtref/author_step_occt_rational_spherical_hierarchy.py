"""Independent nonneutral rational from-frame variants of the AP214 graph.
No Go emitter or normalization; source units and original geometry retained.
"""
from pathlib import Path
import re
root=Path('exchange/step/testdata')
s=(root/'rational-spherical-hierarchy.step').read_text()
r={int(i):v for i,v in re.findall(r'#(\d+)=(.*?);',s,re.S)}
reps=[i for i,v in r.items() if v.startswith('SHAPE_REPRESENTATION(')]
rootrep,carrier,group=reps
new=max(r)
def emit(v):
 global new
 new+=1;r[new]=v;return new
# Each original child frame is authored independently with noncommuting
# Pythagorean axes and a nonbinary terminating-decimal origin. Common mm units.
def add_from(child,parent,xyz,x,z):
 p=emit("CARTESIAN_POINT('',("+','.join(xyz)+"))")
 a=emit("DIRECTION('',("+','.join(z)+"))")
 b=emit("DIRECTION('',("+','.join(x)+"))")
 fr=emit(f"AXIS2_PLACEMENT_3D('',#{p},#{a},#{b})")
 v=r[child];at=v.rindex('),#');r[child]=v[:at]+f',#{fr}'+v[at:]
 for i,v in list(r.items()):
  if v.startswith('(REPRESENTATION_RELATIONSHIP') and re.search(rf",#{child},#{parent}\)",v):
   tid=int(re.search(r'REPRESENTATION_RELATIONSHIP_WITH_TRANSFORMATION\(#(\d+)\)',v)[1])
   target=int(re.findall(r'#(\d+)',r[tid])[1])
   r[tid]=f"ITEM_DEFINED_TRANSFORMATION('','',#{fr},#{target})"
add_from(carrier,rootrep,('0.1','-2.','3.'),('3.','4.','0.'),('0.','0.','1.'))
add_from(group,carrier,('1.','2.','-1.'),('1.','0.','0.'),('0.','3.','4.'))
# Retain now-unreferenced prior origin frame in representation items: it is
# actual original metadata, not geometry to be silently dropped.
out=s[:s.index('DATA;')+5]+'\n'+'\n'.join(f'#{i}={v};' for i,v in r.items())+'\nENDSEC;END-ISO-10303-21;\n'
(root/'occt-rational-spherical-hierarchy.step').write_text(out)
