# Attach independently authored standard mapped items to native OCCT box geometry.
# All original geometry records and numeric tokens are preserved.
import re,sys
from pathlib import Path
s=Path(sys.argv[1]).read_text();records={int(i):v.strip() for i,v in re.findall(r'#(\d+)\s*=\s*([^;]+);',s)}
reps=[(i,v) for i,v in records.items() if v.startswith('ADVANCED_BREP_SHAPE_REPRESENTATION')];assert len(reps)==1
rep,text=reps[0];refs=list(map(int,re.findall(r'#(\d+)',text)));origin=refs[0];ctx=refs[-1]
nextid=max(records);extra=[]
def add(v):
 global nextid
 nextid+=1;extra.append(f'#{nextid}={v};');return nextid
mapping=add(f'REPRESENTATION_MAP(#{origin},#{rep})')
x=add("DIRECTION('',(1.,0.,0.))");y=add("DIRECTION('',(0.,1.,0.))");z=add("DIRECTION('',(0.,0.,1.))")
p=add("CARTESIAN_POINT('',(10.,0.,0.))");a=add(f"CARTESIAN_TRANSFORMATION_OPERATOR_3D('','','',#{x},#{y},#{p},1.,#{z})");first=add(f"MAPPED_ITEM('translated box',#{mapping},#{a})")
p=add("CARTESIAN_POINT('',(0.,20.,0.))");a=add(f"CARTESIAN_TRANSFORMATION_OPERATOR_3D('','','',#{y},#{z},#{p},1.,#{x})");second=add(f"MAPPED_ITEM('rotated box',#{mapping},#{a})")
root=add(f"SHAPE_REPRESENTATION('mapped boxes',(#{first},#{second}),#{ctx})")
s=re.sub(r'(SHAPE_DEFINITION_REPRESENTATION\(#[0-9]+,\s*)#'+str(rep)+r'(\))',lambda m:m[1]+'#'+str(root)+m[2],s)
pos=s.rfind('ENDSEC;');s=s[:pos]+'\n'.join(extra)+'\n'+s[pos:];Path(sys.argv[2]).write_text('\n'.join(x.rstrip() for x in s.splitlines())+'\n')
