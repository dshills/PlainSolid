# Independently wrap the retained one-level mapped OCCT graph with another
# shared representation map. The new outer poses are identity and cyclic axes
# (x,y,z)->(z+200,x,y), so all three source mapping steps visibly affect geometry.
import re,sys
from pathlib import Path
s=Path(sys.argv[1]).read_text();rs={int(i):v.strip() for i,v in re.findall(r'#(\d+)\s*=\s*([^;]+);',s)}
sdr=next(v for v in rs.values() if v.startswith('SHAPE_DEFINITION_REPRESENTATION('));root=int(re.findall(r'#(\d+)',sdr)[-1]);ctx=int(re.findall(r'#(\d+)',rs[root])[-1]);mapping=next(v for v in rs.values() if v.startswith('REPRESENTATION_MAP('));origin=int(re.findall(r'#(\d+)',mapping)[0]);n=max(rs);extra=[]
def add(v):
 global n
 n+=1;extra.append(f'#{n}={v};');return n
def coord(kind,v):return add(kind+"('',("+','.join(str(k)+'.' for k in v)+"))")
mid=add(f'REPRESENTATION_MAP(#{origin},#{root})');uses=[]
for name,x,y,z,p in [('outer identity',(1,0,0),(0,1,0),(0,0,1),(0,0,0)),('outer cyclic',(0,1,0),(0,0,1),(1,0,0),(200,0,0))]:
 xi=coord('DIRECTION',x);yi=coord('DIRECTION',y);zi=coord('DIRECTION',z);pi=coord('CARTESIAN_POINT',p)
 op=add(f"CARTESIAN_TRANSFORMATION_OPERATOR_3D('','','',#{xi},#{yi},#{pi},1.,#{zi})");uses.append(add(f"MAPPED_ITEM('{name}',#{mid},#{op})"))
new=add("SHAPE_REPRESENTATION('two-level rational products',("+','.join('#'+str(i) for i in uses)+f'),#{ctx})')
s=re.sub(r'(SHAPE_DEFINITION_REPRESENTATION\(#[0-9]+,\s*)#'+str(root)+r'(\))',lambda m:m[1]+'#'+str(new)+m[2],s)
pos=s.rfind('ENDSEC;');s=s[:pos]+'\n'.join(extra)+'\n'+s[pos:];Path(sys.argv[2]).write_text(s)
