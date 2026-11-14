# Independently authored IGES topological parabolic-roof, explicit polynomial charts.
import itertools,sys
root_only="--root-only" in sys.argv
records=[]
def add(kind,form,v):
 i=2*len(records)+1;records.append((kind,form,','.join(map(str,v))+';'));return i
def curve(a,b):
 if a[0]==0 and b[0]==1 and a[1]==b[1] and a[2]==1 and b[2]==2:
  return add(126,0,[126,2,2,0,0,1,0,0,0,0,1,1,1,1,1,1,*a,.5,a[1],1,*b,0,1,0,0,0])
 return add(126,0,[126,1,1,0,0,1,0,0,0,1,1,1,1,*a,*b,0,1,0,0,0])
verts=[(i&1,(i>>1)&1,(i>>2)*(1+(i&1))) for i in range(8)]
vid=add(502,1,[502,8,*sum((list(v) for v in verts),[])])
edges=[(i,i^bit) for i in range(8) for bit in (1,2,4) if (i^bit)>i]; mids=[curve(verts[a],verts[b]) for a,b in edges]
ev=[504,12]
for m,(a,b) in zip(mids,edges):ev += [m,vid,a+1,vid,b+1]
eid=add(504,1,ev); faces=[]
for face,tri in enumerate([(0,2,3,1),(4,5,7,6),(0,1,5,4),(2,6,7,3),(0,4,6,2),(1,3,7,5)]):
 p,a,q,b=[verts[i] for i in tri]
 cu,cv=(3,2) if face in (1,2) else ((2,3) if face==3 else (2,2))
 poles=[]
 for v in range(cv):
  for u in range(cu):
   if face==0: p=(v,u,0)
   elif face==1: p=(u/2,v,2 if u==2 else 1)
   elif face==2: p=(u/2,0,v*(2 if u==2 else 1))
   elif face==3: p=(v/2,1,u*(2 if v==2 else 1))
   elif face==4:p=(0,v,u)
   else:p=(1,u,2*v)
   poles.extend(p)
 ku=[0]*cu+[1]*cu;kv=[0]*cv+[1]*cv
 sid=add(128,0,[128,cu-1,cv-1,cu-1,cv-1,0,0,1,0,0,*ku,*kv,*([1]*(cu*cv)),*poles,0,1,0,1])
 uv={tri[0]:(0,0,0),tri[1]:(1,0,0),tri[2]:(1,1,0),tri[3]:(0,1,0)}
 lv=[508,4]
 for aa,bb in zip(tri,tri[1:]+tri[:1]):
  edge=tuple(sorted((aa,bb)));i=edges.index(edge);pc=curve(uv[edge[0]],uv[edge[1]])
  lv += [0,eid,i+1,int(aa<bb),1,0,pc]
 loop=add(508,1,lv);faces.append(add(510,1,[510,sid,1,1,loop]))
shell=add(514,1,[514,6,*sum(([x,1] for x in faces),[])])
add(186,0,[186,shell,1,0])
# Independently place model-space geometry; parameter curves stay in UV.
placed=[]
for index,(kind,form,data) in enumerate(records):
 p=data[:-1].split(',');offset=None;count=0
 if kind==502:offset=2;count=8
 elif kind==126 and 2*index+1 in mids:
  count=int(p[1])+1;offset=7+3*count
 elif kind==128:
  cu=int(p[1])+1;cv=int(p[2])+1;count=cu*cv;offset=10+2*cu+2*cv+count
 if offset is not None:
  for i in range(count):
   for axis,shift in enumerate((.25,-2,3)):
    j=offset+3*i+axis;p[j]=str(float(p[j])+shift)
 placed.append((kind,form,','.join(p)+';'))
records=placed
# Independently permute all topology identifiers and list roles before writing.
vertex_order=[7,0,5,2,6,1,4,3]
vmap={old+1:new+1 for new,old in enumerate(vertex_order)}
edge_order=list(reversed(range(12)))
emap={old+1:new+1 for new,old in enumerate(edge_order)}
idmap={2*i+1:2*(len(records)-1-i)+1 for i in range(len(records))}
changed=[]
for kind,form,data in records:
 p=data[:-1].split(',')
 if kind==502:
  groups=[p[2+3*i:5+3*i] for i in range(8)]
  p=p[:2]+sum((groups[i] for i in vertex_order),[])
 elif kind==504:
  groups=[p[2+5*i:7+5*i] for i in range(12)]
  for g in groups:
   for j in (0,1,3):g[j]=str(idmap[int(g[j])])
   for j in (2,4):g[j]=str(vmap[int(g[j])])
  p=p[:2]+sum((groups[i] for i in edge_order),[])
 elif kind==508:
  groups=[p[2+7*i:9+7*i] for i in range(4)]
  for g in groups:
   g[1]=str(idmap[int(g[1])]);g[2]=str(emap[int(g[2])]);g[6]=str(idmap[int(g[6])])
  p=p[:2]+sum(groups[1:]+groups[:1],[])
 elif kind==510:
  for j in (1,4):p[j]=str(idmap[int(p[j])])
 elif kind==514:
  groups=[p[2+2*i:4+2*i] for i in range(6)]
  for g in groups:g[0]=str(idmap[int(g[0])])
  p=p[:2]+sum(list(reversed(groups)),[])
 elif kind==186:p[1]=str(idmap[int(p[1])])
 changed.append((kind,form,','.join(p)+';'))
records=list(reversed(changed))
modelids=set()
for kind,form,data in records:
 if kind==504:
  vals=data[:-1].split(',');modelids={int(vals[2+5*j]) for j in range(12)}
for j,(kind,form,data) in enumerate(records):
 if kind==502 and not root_only:
  vals=data[:-1].split(',')
  for k in range(8):
   x,y,z=map(float,vals[2+3*k:5+3*k]);vals[2+3*k:5+3*k]=map(str,(z+7,x-4,y+2))
  records[j]=(kind,form,','.join(vals)+';')
records.append((124,0,'124,0,0,1,7,1,0,0,-4,0,1,0,2;'))
global_lines=[x[:72] for x in open('exchange/iges/testdata/cylinder-solid.igs').read().splitlines() if x[72]=='G']
out=[]
def line(s,k,n):out.append(f'{s:<72}{k}{n:7d}')
line('Independent exact parabolic-roof topology','S',1)
for i,g in enumerate(global_lines):line(g,'G',i+1)
params=[]
for i,(kind,form,s) in enumerate(records):
 chunks=[]
 while len(s)>64:
  cut=s.rfind(',',0,64)+1
  if cut==0: raise ValueError('parameter token exceeds record')
  chunks.append(s[:cut]);s=s[cut:]
 chunks.append(s);start=len(params)+1
 params += [(v,2*i+1) for v in chunks]
 status=0 if kind==186 else (10001 if kind==504 else 10000)
 line(''.join(f'{x:8d}' for x in [kind,start,0,0,0,0,(117 if (kind==186 if root_only else kind==128 or 2*i+1 in modelids) else 0),0])+f'{status:08d}','D',2*i+1)
 line(''.join(f'{x:8d}' for x in [kind,0,0,len(chunks),form,0,0])+' '*8+f'{0:8d}','D',2*i+2)
for i,(v,owner) in enumerate(params):line(f'{v:<64}{owner:8d}','P',i+1)
line(f'S{1:7d}G{len(global_lines):7d}D{2*len(records):7d}P{len(params):7d}','T',1)
open('exchange/iges/testdata/'+('placed-parabolic-roof.igs' if root_only else 'leaf-placed-parabolic-roof.igs'),'w').write('\n'.join(out)+'\n')
