"""Independent signed from-frame variants of the retained mixed AP214 graph.
No Go emitter or normalization; source units and original geometry retained.
"""
from pathlib import Path
import re
root=Path('exchange/step/testdata')
s=(root/'independent-analytic-hierarchy-units-nonzero-origin.step').read_text()
r={int(i):v for i,v in re.findall(r'#(\d+)=(.*?);',s,re.S)}
reps=[i for i,v in r.items() if v.startswith('SHAPE_REPRESENTATION(')]
rootrep,carrier,group=reps
new=max(r)
def emit(v):
 global new
 new+=1;r[new]=v;return new
# Each local original frame is authored independently, including opposite
# source/target unit directions at adjacent hierarchy levels.
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
add_from(carrier,rootrep,('1.','-2.','3.'),('0.','1.','0.'),('1.','0.','0.'))
add_from(group,carrier,('0.001','0.002','-0.001'),('-1.','0.','0.'),('0.','0.','1.'))
# Retain now-unreferenced prior origin frame in representation items: it is
# actual original metadata, not geometry to be silently dropped.
out=s[:s.index('DATA;')+5]+'\n'+'\n'.join(f'#{i}={v};' for i,v in r.items())+'\nENDSEC;END-ISO-10303-21;\n'
(root/'independent-analytic-hierarchy-occt-frames.step').write_text(out)
