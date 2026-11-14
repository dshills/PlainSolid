from pathlib import Path
r=[]
def add(k,f,v):
 i=2*len(r)+1;r.append((k,f,','.join(map(str,v))+';'));return i
# Independently specified radius2, height3 positive quarter solid.
O=add(116,0,[116,0,0,0,0]);T=add(116,0,[116,0,0,3,0])
Z=add(123,0,[123,0,0,1]);X=add(123,0,[123,1,0,0]);Y=add(123,0,[123,0,1,0]);NY=add(123,0,[123,0,-1,0]);NX=add(123,0,[123,-1,0,0])
S=add(192,1,[192,O,Z,2,X]);PT=add(190,1,[190,T,Z,X]);PB=add(190,1,[190,O,Z,X]);PX=add(190,1,[190,O,NY,X]);PY=add(190,1,[190,O,NX,Y])
V=add(502,1,[502,6,0,0,0,2,0,0,0,2,0,0,0,3,2,0,3,0,2,3])
curves=[(110,[0,0,0,2,0,0]),(100,[0,0,0,2,0,0,2]),(110,[0,2,0,0,0,0]),(110,[0,0,3,2,0,3]),(100,[3,0,0,2,0,0,2]),(110,[0,2,3,0,0,3]),(110,[0,0,0,0,0,3]),(110,[2,0,0,2,0,3]),(110,[0,2,0,0,2,3])]
C=[add(k,0,[k]+v) for k,v in curves]
edges=[504,9]
for c,(i,j) in zip(C,[(1,2),(2,3),(3,1),(4,5),(5,6),(6,4),(1,4),(2,5),(3,6)]):edges += [c,V,i,V,j]
E=add(504,1,edges)
loops=[]
for wire in [[2,9,-5,-8],[4,5,6],[1,2,3],[1,8,-4,-7],[3,7,-6,-9]]:
 v=[508,len(wire)]
 for i in wire:v += [0,E,abs(i),int(i>0),0]
 loops.append(add(508,1,v))
faces=[add(510,1,[510,s,1,1,l]) for s,l in zip([S,PT,PB,PX,PY],loops)]
SH=add(514,1,[514,5,faces[0],1,faces[1],1,faces[2],0,faces[3],1,faces[4],1]);add(186,0,[186,SH,1,0])
g=[x[:72] for x in Path('exchange/iges/testdata/cylinder-solid.igs').read_text().splitlines() if x[72]=='G'];out=[]
def emit(s,k,n):out.append(f'{s:<72}{k}{n:7d}')
emit('Independent model-only cardinal quarter cylinder','S',1)
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
Path('exchange/iges/testdata/quarter-cylinder.igs').write_text('\n'.join(out)+'\n')
