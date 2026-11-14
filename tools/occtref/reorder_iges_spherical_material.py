# Independent pointer-aware reversal of all directory records; no Go helpers.
from pathlib import Path
import sys
reorder_loops = True
reorder_vertices = True
reorder_edges = False
reorder_faces = False
names=("spherical-material","placed-spherical-material")
for name in names:
 lines=Path('exchange/iges/testdata/'+name+'.igs').read_text().splitlines();ds=[x for x in lines if x[72]=='D'];ps=[x for x in lines if x[72]=='P'];records=[]
 for i in range(0,len(ds),2):
  a,b=ds[i],ds[i+1];start=int(a[8:16]);n=int(b[24:32]);v=''.join(x[:64] for x in ps[start-1:start-1+n]).strip().rstrip(';').split(',');records.append((i+1,a,b,[x.strip() for x in v]))
 if reorder_vertices:
  vertex_count=next(int(v[1]) for _,_,_,v in records if int(v[0])==502)
  for _,_,_,v in records:
   if int(v[0])==502:
    entries=[v[j:j+3] for j in range(2,len(v),3)];v[2:]=sum(entries[::-1],[])
   if int(v[0])==504:
    for j in range(2,len(v),5):
     v[j+2]=str(vertex_count+1-int(v[j+2]));v[j+4]=str(vertex_count+1-int(v[j+4]))
  # Pole ordinals in vertex-use loop entries must follow the502 permutation.
 for _,_,_,v in records:
  if int(v[0])==508:
   for j in range(2,len(v),7):
    if int(v[j])==1:v[j+2]=str(vertex_count+1-int(v[j+2]))
 if reorder_loops:
  for _,_,_,v in records:
   if int(v[0])==508 and int(v[1])==4:
    entries=[v[j:j+7] for j in range(2,len(v),7)];v[2:]=sum(entries[1:]+entries[:1],[])
 edge_count=next(int(v[1]) for _,_,_,v in records if int(v[0])==504)
 if reorder_edges:
  for _,_,_,v in records:
   if int(v[0])==504:
    entries=[v[j:j+5] for j in range(2,len(v),5)];v[2:]=sum(entries[::-1],[])
   if int(v[0])==508:
    for j in range(2,len(v),5):v[j+2]=str(edge_count+1-int(v[j+2]))
 records.reverse();mapping={old:2*i+1 for i,(old,a,b,v) in enumerate(records)};out=[];params=[];g=[x[:72] for x in lines if x[72]=='G']
 def emit(s,k,n):out.append(f'{s:<72}{k}{n:7d}')
 emit('Independent sphere record, pole ordinal and cyclic-use permutation','S',1)
 for i,s in enumerate(g):emit(s,'G',i+1)
 for i,(old,a,b,v) in enumerate(records):
  kind=int(v[0]);slots=[]
  if kind in (122,186):slots=[1]
  elif kind==190:slots=[1,2,3]
  elif kind==196:slots=[1,3,4]
  elif kind in (194,198):slots=[1,2,5]
  elif kind==514:slots=[2+2*j for j in range(int(v[1]))]
  elif kind==510:slots=[1]+list(range(4,4+int(v[2])))
  elif kind==504:slots=sum(([2+5*j,3+5*j,5+5*j] for j in range(int(v[1]))),[])
  elif kind==508:
   pos=2
   for j in range(int(v[1])):
    slots.append(pos+1);n=int(v[pos+4]);slots += [pos+6+2*k for k in range(n)];pos+=5+2*n
  for j in slots:v[j]=str(mapping[int(v[j])])
  if kind==514 and reorder_faces:
   pairs=[v[j:j+2] for j in range(2,len(v),2)]
   v=v[:2]+sum(pairs[::-1],[])
  text=','.join(v)+';';chunks=[]
  while len(text)>64:
   cut=text.rfind(',',0,64)+1;chunks.append(text[:cut]);text=text[cut:]
  chunks.append(text);start=len(params)+1;params.extend((x,2*i+1) for x in chunks)
  transform=int(a[48:56]);transform=mapping[transform] if transform else 0
  a=a[:8]+f'{start:8d}'+a[16:48]+f'{transform:8d}'+a[56:72]
  b=b[:24]+f'{len(chunks):8d}'+b[32:72]
  emit(a,'D',2*i+1);emit(b,'D',2*i+2)
 for i,(s,owner) in enumerate(params):emit(f'{s:<64}{owner:8d}','P',i+1)
 emit(f'S{1:7d}G{len(g):7d}D{2*len(records):7d}P{len(params):7d}','T',1)
 Path('exchange/iges/testdata/'+('loop-start-' if reorder_loops else 'vertex-order-' if reorder_vertices else 'edge-order-' if reorder_edges else 'face-order-' if reorder_faces else 'reindexed-')+name+'.igs').write_text('\n'.join(out)+'\n')
