# Independent mixed-group DAG over retained OCCT local models. Group A reuses
# the prior nested group; group B shares two original leaf use records; group C
# directly reuses the original inner group through a different root branch.
import re,sys
from pathlib import Path
s=Path(sys.argv[1]).read_text();rs={int(i):v.strip() for i,v in re.findall(r'#(\d+)\s*=\s*([^;]+);',s)}
sdr=next(v for v in rs.values() if v.startswith('SHAPE_DEFINITION_REPRESENTATION'));oldroot=int(re.findall(r'#(\d+)',sdr)[-1]);ctx=int(re.findall(r'#(\d+)',rs[oldroot])[-1]);maps=[(i,v) for i,v in rs.items() if v.startswith('REPRESENTATION_MAP')];origin=int(re.findall(r'#(\d+)',maps[0][1])[0]);groupmap=next(i for i,v in maps if rs[int(re.findall(r'#(\d+)',v)[1])].startswith('SHAPE_REPRESENTATION'))
leaves=[i for i,v in rs.items() if v.startswith('MAPPED_ITEM') and int(re.findall(r'#(\d+)',v)[0])!=groupmap];assert len(leaves)==3
n=max(rs);extra=[]
def add(v):
 global n
 n+=1;extra.append(f'#{n}={v};');return n
def coord(k,v):return add(k+"('',("+','.join(str(x)+'.' for x in v)+"))")
mapa=add(f'REPRESENTATION_MAP(#{origin},#{oldroot})');groupb=add(f"SHAPE_REPRESENTATION('selected leaves',(#{leaves[0]},#{leaves[2]}),#{ctx})");mapb=add(f'REPRESENTATION_MAP(#{origin},#{groupb})');items=[]
for name,mid,x,y,z,p in [('nested branch',mapa,(1,0,0),(0,1,0),(0,0,1),(0,0,0)),('selected branch',mapb,(0,1,0),(0,0,1),(1,0,0),(200,0,0)),('shared inner branch',groupmap,(1,0,0),(0,1,0),(0,0,1),(0,0,50))]:
 xi=coord('DIRECTION',x);yi=coord('DIRECTION',y);zi=coord('DIRECTION',z);pi=coord('CARTESIAN_POINT',p);op=add(f"CARTESIAN_TRANSFORMATION_OPERATOR_3D('','','',#{xi},#{yi},#{pi},1.,#{zi})");items.append(add(f"MAPPED_ITEM('{name}',#{mid},#{op})"))
root=add("SHAPE_REPRESENTATION('mixed shared DAG',("+','.join('#'+str(i) for i in items)+f'),#{ctx})')
s=re.sub(r'(SHAPE_DEFINITION_REPRESENTATION\(#[0-9]+,\s*)#'+str(oldroot)+r'(\))',lambda m:m[1]+'#'+str(root)+m[2],s);pos=s.rfind('ENDSEC;');s=s[:pos]+'\n'.join(extra)+'\n'+s[pos:];Path(sys.argv[2]).write_text(s)
