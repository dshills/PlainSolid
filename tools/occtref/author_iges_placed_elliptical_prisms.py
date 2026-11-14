# Independent leaf placement with world122 terminal points and502 vertices.
from pathlib import Path
import sys
names=("conic-circular-prism-topology","conic-circular-annular-prism-topology") if "--circle" in sys.argv else ("elliptical-prism-topology","elliptical-annular-prism-topology")
if "--cone" in sys.argv:names=("conical45-frustum",)
for name in names:
 lines=Path('exchange/iges/testdata/'+name+'.igs').read_text().splitlines();ds=[x for x in lines if x[72]=='D'];ps=[x for x in lines if x[72]=='P'];records=[]
 def pose(v):
  x,y,z=map(float,v);return [str(z+7.5),str(x-4.25),str(y+2.125)]
 for i in range(0,len(ds),2):
  a,b=ds[i],ds[i+1];kind=int(a[:8]);start=int(a[8:16]);n=int(b[24:32]);form=int(b[32:40]);v=''.join(x[:64] for x in ps[start-1:start-1+n]).strip().rstrip(';').split(',');v=[x.strip() for x in v]
  if kind==502:
   for j in range(int(v[1])):v[2+3*j:5+3*j]=pose(v[2+3*j:5+3*j])
  if kind==122:v[2:5]=pose(v[2:5])
  records.append((kind,form,','.join(v)+';'))
 tid=2*len(records)+1;records.append((124,0,'124,0,0,1,7.5,1,0,0,-4.25,0,1,0,2.125;'));g=[x[:72] for x in lines if x[72]=='G'];out=[]
 def emit(s,k,n):out.append(f'{s:<72}{k}{n:7d}')
 emit('Independent proper placement on exact elliptical model leaves','S',1)
 for i,s in enumerate(g):emit(s,'G',i+1)
 params=[]
 for i,(kind,form,s) in enumerate(records):
  chunks=[]
  while len(s)>64:
   cut=s.rfind(',',0,64)+1;chunks.append(s[:cut]);s=s[cut:]
  chunks.append(s);start=len(params)+1;params.extend((v,2*i+1) for v in chunks);status=0 if kind==186 else (10001 if kind==504 else 10000);tr=tid if kind in ((100,110,190,194) if "--cone" in sys.argv else (104,110,190)) else 0
  emit(''.join(f'{x:8d}' for x in (kind,start,0,0,0,0,tr,0))+f'{status:08d}','D',2*i+1);emit(''.join(f'{x:8d}' for x in (kind,0,0,len(chunks),form,0,0))+' '*8+f'{0:8d}','D',2*i+2)
 for i,(s,owner) in enumerate(params):emit(f'{s:<64}{owner:8d}','P',i+1)
 emit(f'S{1:7d}G{len(g):7d}D{2*len(records):7d}P{len(params):7d}','T',1)
 Path('exchange/iges/testdata/placed-'+name+'.igs').write_text('\n'.join(out)+'\n')
