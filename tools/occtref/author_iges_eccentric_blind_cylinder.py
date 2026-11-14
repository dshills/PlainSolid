# Independently specified exact five-face IGES topology, not Go writer output.
from pathlib import Path
r=[]
def add(k,f,v):
 i=2*len(r)+1;r.append((k,f,','.join(map(str,[k]+v))+';'));return i
for xyz in [(0,0,0),(0,0,4),(1,0.5,2)]:add(116,0,[*xyz,0])
add(123,0,[0,0,1]);add(123,0,[1,0,0])
add(192,1,[1,7,3,9]);add(192,1,[5,7,1,9])
for point in [3,1,5]:add(190,1,[point,7,9])
add(502,1,[4,3,0,4,3,0,0,2,0.5,4,2,0.5,2])
add(100,0,[4,0,0,3,0,3,0]);add(110,0,[3,0,0,3,0,4]);add(100,0,[0,0,0,3,0,3,0])
add(100,0,[4,1,0.5,2,0.5,2,0.5]);add(110,0,[2,0.5,2,2,0.5,4]);add(100,0,[2,1,0.5,2,0.5,2,0.5])
add(504,1,[6,23,21,1,21,1,25,21,2,21,1,27,21,2,21,2,29,21,3,21,3,31,21,4,21,3,33,21,4,21,4])
for height,offset in [(4,0),(2,3)]:
 pc=[]
 for a,b in [((360,height,0),(0,height,0)),((0,height,0),(0,0,0)),((0,0,0),(360,0,0)),((360,0,0),(360,height,0))]:pc.append(add(110,0,[*a,*b]))
 v=[4]
 for edge,sense,p in zip([1,2,3,2],[0,0,1,1],pc):v += [0,35,edge+offset,sense,1,0,p]
 add(508,1,v)
for edge,sense in [(1,1),(4,0),(3,1),(6,1)]:add(508,1,[1,0,35,edge,sense,0])
add(510,1,[11,1,1,45]);add(510,1,[13,1,1,55]);add(510,1,[15,2,1,57,59]);add(510,1,[17,1,1,61]);add(510,1,[19,1,1,63])
add(514,1,[5,65,1,67,0,69,1,71,0,73,1]);add(186,0,[75,1,0])
g=[x[:72] for x in Path('exchange/iges/testdata/cylinder-solid.igs').read_text().splitlines() if x[72]=='G'];out=[]
def emit(s,k,n):out.append(f'{s:<72}{k}{n:7d}')
emit('Independent eccentric blind bore with exact cavity floor','S',1)
for i,s in enumerate(g):emit(s,'G',i+1)
p=[]
for i,(kind,form,s) in enumerate(r):
 chunks=[]
 while len(s)>64:
  cut=s.rfind(',',0,64)+1;chunks.append(s[:cut]);s=s[cut:]
 chunks.append(s);start=len(p)+1;p.extend((v,2*i+1) for v in chunks);status=0 if kind==186 else (10001 if kind==504 else 10000)
 emit(''.join(f'{x:8d}' for x in (kind,start,0,0,0,0,0,0))+f'{status:08d}','D',2*i+1)
 emit(''.join(f'{x:8d}' for x in (kind,0,0,len(chunks),form,0,0))+' '*8+f'{0:8d}','D',2*i+2)
for i,(v,owner) in enumerate(p):emit(f'{v:<64}{owner:8d}','P',i+1)
emit(f'S{1:7d}G{len(g):7d}D{2*len(r):7d}P{len(p):7d}','T',1)
Path('exchange/iges/testdata/eccentric-blind-cylinder.igs').write_text('\n'.join(out)+'\n')
