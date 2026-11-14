# Independently authored IGES topological tetrahedron, explicit polynomial charts.
import itertools
records=[]
def add(kind,form,v):
 i=2*len(records)+1;records.append((kind,form,','.join(map(str,v))+';'));return i
def curve(a,b):return add(126,0,[126,1,1,0,0,1,0,0,0,1,1,1,1,*a,*b,0,1,0,0,0])
verts=[(0,0,0),(2,0,0),(0,2,0),(0,0,2)]
vid=add(502,1,[502,4,*sum((list(v) for v in verts),[])])
edges=list(itertools.combinations(range(4),2)); mids=[curve(verts[a],verts[b]) for a,b in edges]
ev=[504,6]
for m,(a,b) in zip(mids,edges):ev += [m,vid,a+1,vid,b+1]
eid=add(504,1,ev); faces=[]
for tri in [(0,2,1),(0,1,3),(0,3,2),(1,2,3)]:
 p,a,b=[verts[i] for i in tri];q=tuple(a[j]+b[j]-p[j] for j in range(3))
 sid=add(128,0,[128,1,1,1,1,0,0,1,0,0,0,0,1,1,0,0,1,1,1,1,1,1,*p,*a,*b,*q,0,1,0,1])
 uv={tri[0]:(0,0,0),tri[1]:(1,0,0),tri[2]:(0,1,0)}
 lv=[508,3]
 for aa,bb in zip(tri,tri[1:]+tri[:1]):
  edge=tuple(sorted((aa,bb)));i=edges.index(edge);pc=curve(uv[edge[0]],uv[edge[1]])
  lv += [0,eid,i+1,int(aa<bb),1,0,pc]
 loop=add(508,1,lv);faces.append(add(510,1,[510,sid,1,1,loop]))
shell=add(514,1,[514,4,*sum(([x,1] for x in faces),[])])
add(186,0,[186,shell,1,0])
global_lines=[x[:72] for x in open('exchange/iges/testdata/cylinder-solid.igs').read().splitlines() if x[72]=='G']
out=[]
def line(s,k,n):out.append(f'{s:<72}{k}{n:7d}')
line('Independent exact tetrahedron topology','S',1)
for i,g in enumerate(global_lines):line(g,'G',i+1)
params=[]
for i,(kind,form,s) in enumerate(records):
 chunks=[s[j:j+64] for j in range(0,len(s),64)];start=len(params)+1
 params += [(v,2*i+1) for v in chunks]
 status=0 if kind==186 else (10001 if kind==504 else 10000)
 line(''.join(f'{x:8d}' for x in [kind,start,0,0,0,0,0,0])+f'{status:08d}','D',2*i+1)
 line(''.join(f'{x:8d}' for x in [kind,0,0,len(chunks),form,0,0])+' '*8+f'{0:8d}','D',2*i+2)
for i,(v,owner) in enumerate(params):line(f'{v:<64}{owner:8d}','P',i+1)
line(f'S{1:7d}G{len(global_lines):7d}D{2*len(records):7d}P{len(params):7d}','T',1)
open('exchange/iges/testdata/tetrahedron-solid.igs','w').write('\n'.join(out)+'\n')
