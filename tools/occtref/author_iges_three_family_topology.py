# Combine independently authored reference materials, remapping only IGES's
# declared topology-pointer positions. Neither input was emitted by Go.
from pathlib import Path
records=[]
for path,dx in (('tetrahedron-solid.igs',0),('tetrahedron-solid.igs',10),('hexahedron-solid.igs',0),('mapped-parabolic-roof.igs',0)):
 lines=Path('exchange/iges/testdata/'+path).read_text().splitlines()
 ds=[x for x in lines if x[72]=='D'];ps=[x for x in lines if x[72]=='P'];offset=2*len(records)
 modelids=set()
 for i in range(0,len(ds),2):
  if int(ds[i][:8])==504:
   start=int(ds[i][8:16]);count=int(ds[i+1][24:32]);q=''.join(x[:64] for x in ps[start-1:start-1+count]).strip().rstrip(';').split(',');modelids={int(q[2+5*j]) for j in range(int(q[1]))}
 for i in range(0,len(ds),2):
  a,b=ds[i],ds[i+1];kind=int(a[:8]);start=int(a[8:16]);count=int(b[24:32]);form=int(b[32:40]);v=''.join(x[:64] for x in ps[start-1:start-1+count]).strip().rstrip(';').split(',');v=[x.strip() for x in v]
  if dx:
   n,at=0,0
   if kind==502:n=int(v[1]);at=2
   elif kind==128:
    nu,nv,du,dv=map(int,v[1:5]);n=(nu+1)*(nv+1);at=10+(nu+du+2)+(nv+dv+2)+n
   elif kind==126 and i+1 in modelids:
    last,degree=map(int,v[1:3]);n=last+1;at=7+(last+degree+2)+n
   for j in range(n):v[at+3*j]=str(float(v[at+3*j])+dx)
  slots=[]
  if kind==186:slots=[1]
  elif kind==514:slots=[2+2*j for j in range(int(v[1]))]
  elif kind==510:slots=[1]+[4+j for j in range(int(v[2]))]
  elif kind==508:slots=sum(([3+7*j,8+7*j] for j in range(int(v[1]))),[])
  elif kind==504:slots=sum(([2+5*j,3+5*j,5+5*j] for j in range(int(v[1]))),[])
  for j in slots:v[j]=str(int(v[j])+offset)
  records.append((kind,form,','.join(v)+';'))
g=[x[:72] for x in lines if x[72]=='G'];out=[]
def emit(s,k,n):out.append(f'{s:<72}{k}{n:7d}')
emit('Independent mixed tetrahedral and curved original material roots','S',1)
for i,s in enumerate(g):emit(s,'G',i+1)
params=[]
for i,(kind,form,s) in enumerate(records):
 chunks=[]
 while len(s)>64:
  cut=s.rfind(',',0,64)+1
  if not cut:raise ValueError('long numeric token')
  chunks.append(s[:cut]);s=s[cut:]
 chunks.append(s);start=len(params)+1;params.extend((v,2*i+1) for v in chunks)
 status=0 if kind==186 else (10001 if kind==504 else 10000)
 emit(''.join(f'{x:8d}' for x in (kind,start,0,0,0,0,0,0))+f'{status:08d}','D',2*i+1)
 emit(''.join(f'{x:8d}' for x in (kind,0,0,len(chunks),form,0,0))+' '*8+f'{0:8d}','D',2*i+2)
for i,(v,owner) in enumerate(params):emit(f'{v:<64}{owner:8d}','P',i+1)
emit(f'S{1:7d}G{len(g):7d}D{2*len(records):7d}P{len(params):7d}','T',1)
Path('exchange/iges/testdata/three-family-topology-solids.igs').write_text('\n'.join(out)+'\n')
