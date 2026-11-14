# Independently authored IGES topological parabolic-roof, explicit polynomial charts.
import itertools
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
 line(''.join(f'{x:8d}' for x in [kind,start,0,0,0,0,0,0])+f'{status:08d}','D',2*i+1)
 line(''.join(f'{x:8d}' for x in [kind,0,0,len(chunks),form,0,0])+' '*8+f'{0:8d}','D',2*i+2)
for i,(v,owner) in enumerate(params):line(f'{v:<64}{owner:8d}','P',i+1)
line(f'S{1:7d}G{len(global_lines):7d}D{2*len(records):7d}P{len(params):7d}','T',1)
open('exchange/iges/testdata/parabolic-roof.igs','w').write('\n'.join(out)+'\n')
