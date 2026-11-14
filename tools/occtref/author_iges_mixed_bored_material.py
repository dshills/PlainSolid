# Combine independently authored reference materials, remapping only IGES's
# declared topology-pointer positions. Neither input was emitted by Go.
from pathlib import Path
records=[]
for path in ('tetrahedron-solid.igs','blind-circular-cylinder.igs','through-circular-cylinder.igs'):
 lines=Path('exchange/iges/testdata/'+path).read_text().splitlines()
 ds=[x for x in lines if x[72]=='D'];ps=[x for x in lines if x[72]=='P'];offset=2*len(records)
 for i in range(0,len(ds),2):
  a,b=ds[i],ds[i+1];kind=int(a[:8]);start=int(a[8:16]);count=int(b[24:32]);form=int(b[32:40]);v=''.join(x[:64] for x in ps[start-1:start-1+count]).strip().rstrip(';').split(',');v=[x.strip() for x in v]
  if path in ('blind-circular-cylinder.igs','through-circular-cylinder.igs'):
   shift=-10 if path.startswith('blind') else 10
   if kind==116:v[1]=str(float(v[1])+shift)
   elif kind==100:
    for j in (2,4,6):v[j]=str(float(v[j])+shift)
   elif kind==502:
    for j in range(int(v[1])):v[2+3*j]=str(float(v[2+3*j])+shift)
   elif kind==110 and i+1 in ((25,31) if shift<0 else (21,27)):
    for j in (1,4):v[j]=str(float(v[j])+shift)
  slots=[]
  if kind==190:slots=[1,2,3]
  elif kind==192:slots=[1,2,4]
  elif kind==186:slots=[1]
  elif kind==514:slots=[2+2*j for j in range(int(v[1]))]
  elif kind==510:slots=[1]+[4+j for j in range(int(v[2]))]
  elif kind==508:
   at=2
   for j in range(int(v[1])):
    slots.append(at+1);count=int(v[at+4])
    for k in range(count):slots.append(at+6+2*k)
    at+=5+2*count
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
Path('exchange/iges/testdata/mixed-bored-material.igs').write_text('\n'.join(out)+'\n')
