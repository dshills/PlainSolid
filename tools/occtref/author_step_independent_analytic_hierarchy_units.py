"""Independent AP214 mixed-context derivative; no Go writer or float scaling.
Root and analytic group use metres, carrier/box/cylinder mm; torus metres.
Source values scaled by exact decimal arithmetic, leaving angles/directions alone.
"""
from decimal import Decimal
import re
from pathlib import Path
p=Path('exchange/step/testdata/independent-analytic-hierarchy.step').read_text()
r={int(i):v for i,v in re.findall(r'#(\d+)=(.*?);',p,re.S)}
reps=[i for i,v in r.items() if v.startswith('SHAPE_REPRESENTATION(')]
ctx=int(re.findall(r'#(\d+)',r[reps[0]])[-1]); unit=int(re.findall(r'#(\d+)',r[ctx])[0]); n=max(r)+1
r[n]=r[unit].replace('.MILLI.','$');r[n+1]=r[ctx].replace('#'+str(unit)+',','#'+str(n)+',')
for ri in [reps[0],reps[2]]:
 r[ri]=r[ri].replace('#'+str(ctx)+')','#'+str(n+1)+')')
 for fr in map(int,re.findall(r'#(\d+)',r[ri])[:-1]):
  pt=int(re.findall(r'#(\d+)',r[fr])[0]);v=r[pt]
  nums=v[v.index('(' ,v.index('(')+1)+1:v.index('))')].split(',')
  r[pt]="CARTESIAN_POINT('',("+','.join(str(Decimal(a)/1000) for a in nums)+"))"
# The torus local representation and all dimensional model records use metres.
ri=next(i for i,v in r.items() if v.startswith('ADVANCED_BREP_SHAPE_REPRESENTATION') and any(r[int(k)].startswith('MANIFOLD_SOLID_BREP') and 'torus' in r[int(k)].lower() for k in re.findall(r'#(\d+)',v)))
r[ri]=r[ri].replace('#'+str(ctx)+')','#'+str(n+1)+')')
body=next(int(k) for k in re.findall(r'#(\d+)',r[ri]) if r[int(k)].startswith('MANIFOLD_SOLID_BREP'))
seen=set()
def scale_model(i):
 if i in seen:return
 seen.add(i)
 for k in re.findall(r'#(\d+)',r[i]):scale_model(int(k))
 v=r[i]
 if v.startswith('CARTESIAN_POINT'):
  a,b=v.split(',(',1);nums=b[:-2].split(',');r[i]=a+',('+','.join(str(Decimal(x)/1000) for x in nums)+'))'
 elif v.startswith('CIRCLE') or v.startswith('TOROIDAL_SURFACE'):
  parts=v[:-1].split(',');count=2 if v.startswith('TOROIDAL') else 1
  parts[-count:]=[str(Decimal(x)/1000) for x in parts[-count:]];r[i]=','.join(parts)+')'
scale_model(body)
out=p[:p.index('DATA;')+5]+'\n'+'\n'.join(f'#{i}={v};' for i,v in r.items())+'\nENDSEC;END-ISO-10303-21;\n'
Path('exchange/step/testdata/independent-analytic-hierarchy-units.step').write_text(out)

# Nonzero child mapping origin is one millimetre. The parent targets are
# independently authored in metres and compensate it under both rotations.
new=max(r)+1
r[new]="CARTESIAN_POINT('',(1.,0.,0.))"
r[new+1]="DIRECTION('',(0.,0.,1.))";r[new+2]="DIRECTION('',(1.,0.,0.))"
r[new+3]=f"AXIS2_PLACEMENT_3D('',#{new},#{new+1},#{new+2})"
carrier=reps[1]
v=r[carrier];marker='),#';pos=v.rindex(marker);r[carrier]=v[:pos]+f',#{new+3}'+v[pos:]
relations=[(i,v) for i,v in r.items() if v.startswith('(REPRESENTATION_RELATIONSHIP') and re.search(rf",#{carrier},#{reps[0]}\)",v)]
for i,v in relations:
 tid=int(re.search(r'REPRESENTATION_RELATIONSHIP_WITH_TRANSFORMATION\(#(\d+)\)',v)[1])
 refs=list(map(int,re.findall(r'#(\d+)',r[tid])));target=refs[1]
 r[tid]=f"ITEM_DEFINED_TRANSFORMATION('','',#{new+3},#{target})"
 pt=int(re.findall(r'#(\d+)',r[target])[0]);coords=r[pt].split(',(',1)[1][:-2].split(',')
 axis=0 if Decimal(coords[0]) else 1
 coords[axis]=str(Decimal(coords[axis])+Decimal('.001'))
 r[pt]="CARTESIAN_POINT('',("+','.join(coords)+"))"
out=p[:p.index('DATA;')+5]+'\n'+'\n'.join(f'#{i}={v};' for i,v in r.items())+'\nENDSEC;END-ISO-10303-21;\n'
Path('exchange/step/testdata/independent-analytic-hierarchy-units-nonzero-origin.step').write_text(out)
