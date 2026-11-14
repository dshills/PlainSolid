# Independently combine two native OCCT box representation closures, retaining
# each original model/pcurve token. Both native exports use the same MM context.
import re,sys
from pathlib import Path
def parse(p):
 s=Path(p).read_text();return s,{int(i):v.strip() for i,v in re.findall(r'#(\d+)\s*=\s*([^;]+);',s)}
s,records=parse(sys.argv[1]);_,second=parse(sys.argv[2]);offset=max(records)
def adv(rs):
 entries=[(i,v) for i,v in rs.items() if v.startswith('ADVANCED_BREP_SHAPE_REPRESENTATION')];assert len(entries)==1
 id,v=entries[0];refs=list(map(int,re.findall(r'#(\d+)',v)));return id,refs[0],refs[-1]
r1,o1,c1=adv(records);r2,o2,c2=adv(second);used=set()
def walk(i):
 if i==c2 or i in used:return
 used.add(i)
 for j in map(int,re.findall(r'#(\d+)',second[i])):walk(j)
walk(r2)
extra=[]
for i in sorted(used):
 v=re.sub(r'#(\d+)',lambda m:'#'+str(c1 if int(m[1])==c2 else offset+int(m[1])),second[i]);extra.append(f'#{offset+i}={v};')
nextid=offset+max(used)
def add(v):
 global nextid
 nextid+=1;extra.append(f'#{nextid}={v};');return nextid
maps=[add(f'REPRESENTATION_MAP(#{o1},#{r1})'),add(f'REPRESENTATION_MAP(#{offset+o2},#{offset+r2})')]
x=add("DIRECTION('',(1.,0.,0.))");y=add("DIRECTION('',(0.,1.,0.))");z=add("DIRECTION('',(0.,0.,1.))")
items=[]
for label,which,xyz,axes in [('first translated',0,(10,0,0),(x,y,z)),('first rotated',0,(0,20,0),(y,z,x)),('second translated',1,(-10,0,0),(x,y,z))]:
 p=add("CARTESIAN_POINT('',("+','.join(str(v)+'.' for v in xyz)+"))")
 op=add(f"CARTESIAN_TRANSFORMATION_OPERATOR_3D('','','',#{axes[0]},#{axes[1]},#{p},1.,#{axes[2]})")
 items.append(add(f"MAPPED_ITEM('{label}',#{maps[which]},#{op})"))
root=add("SHAPE_REPRESENTATION('distinct mapped boxes',("+','.join('#'+str(v) for v in items)+f'),#{c1})')
s=re.sub(r'(SHAPE_DEFINITION_REPRESENTATION\(#[0-9]+,\s*)#'+str(r1)+r'(\))',lambda m:m[1]+'#'+str(root)+m[2],s)
pos=s.rfind('ENDSEC;');s=s[:pos]+'\n'.join(extra)+'\n'+s[pos:];Path(sys.argv[3]).write_text('\n'.join(x.rstrip() for x in s.splitlines())+'\n')
