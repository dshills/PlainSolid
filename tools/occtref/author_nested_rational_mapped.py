# One independently authored mapping level over the retained OCCT two-box graph.
import re,sys
from pathlib import Path
s=Path(sys.argv[1]).read_text();rs={int(i):v.strip() for i,v in re.findall(r'#(\d+)\s*=\s*([^;]+);',s)}
root=next(i for i,v in rs.items() if v.startswith('SHAPE_REPRESENTATION('));ctx=int(re.findall(r'#(\d+)',rs[root])[-1]);mapping=next(v for v in rs.values() if v.startswith('REPRESENTATION_MAP('));origin=int(re.findall(r'#(\d+)',mapping)[0]);n=max(rs);extra=[]
def add(v):
 global n
 n+=1;extra.append(f'#{n}={v};');return n
mid=add(f'REPRESENTATION_MAP(#{origin},#{root})');z=add("DIRECTION('',(0.,0.,1.))")
uses=[]
for name,x,y,p in [('group original',(1,0,0),(0,1,0),(0,0,0)),('group rational',(3,4,0),(-4,3,0),(100,0,0))]:
 def coord(kind,v):return add(kind+"('',("+','.join(str(k)+'.' for k in v)+"))")
 xi=coord('DIRECTION',x);yi=coord('DIRECTION',y);pi=coord('CARTESIAN_POINT',p)
 op=add(f"CARTESIAN_TRANSFORMATION_OPERATOR_3D('','','',#{xi},#{yi},#{pi},1.,#{z})");uses.append(add(f"MAPPED_ITEM('{name}',#{mid},#{op})"))
new=add("SHAPE_REPRESENTATION('nested rational products',("+','.join('#'+str(i) for i in uses)+f'),#{ctx})')
s=re.sub(r'(SHAPE_DEFINITION_REPRESENTATION\(#[0-9]+,\s*)#'+str(root)+r'(\))',lambda m:m[1]+'#'+str(new)+m[2],s)
pos=s.rfind('ENDSEC;');s=s[:pos]+'\n'.join(extra)+'\n'+s[pos:];Path(sys.argv[2]).write_text(s)
