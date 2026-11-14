from pathlib import Path
import sys
placed="--placed" in sys.argv
r=[]
def add(k,f,v):
 i=2*len(r)+1;r.append((k,f,','.join(map(str,v))+';'));return i
O=add(116,0,[116,0,0,0,0]);T=add(116,0,[116,0,0,4,0]);Z=add(123,0,[123,0,0,1]);X=add(123,0,[123,1,0,0])
SO=add(194,1,[194,O,Z,3,45,X,0,0]);SI=add(192,1,[192,O,Z,1,X]);PT=add(190,1,[190,T,Z,X]);PB=add(190,1,[190,O,Z,X])
V=add(502,1,[502,4,7,0,4,3,0,0,1,0,4,1,0,0])
curves=[]
for lo,hi in ((3,7),(1,1)):
 curves += [add(100,0,[100,4,0,0,hi,0,hi,0]),add(110,0,[110,lo,0,0,hi,0,4]),add(100,0,[100,0,0,0,lo,0,lo,0])]
ev=[504,6]
for c,(a,b) in zip(curves,((1,1),(2,1),(2,2),(3,3),(4,3),(4,4))):ev += [c,V,a,V,b]
E=add(504,1,ev)
def wall(offset):
 v=[508,4]
 for edge,sense in zip((1,2,3,2),(0,0,1,1)):v += [0,E,edge+offset,sense,0]
 return add(508,1,v)
LO=wall(0);LI=wall(3)
def loop(edge,sense):return add(508,1,[508,1,0,E,edge,sense,0])
LT=loop(1,1);LTH=loop(4,0);LB=loop(3,1);LBH=loop(6,0)
FO=add(510,1,[510,SO,1,1,LO]);FI=add(510,1,[510,SI,1,1,LI]);FT=add(510,1,[510,PT,2,1,LT,LTH]);FB=add(510,1,[510,PB,2,1,LB,LBH])
SH=add(514,1,[514,4,FO,1,FI,0,FT,1,FB,0]);add(186,0,[186,SH,1,0])

if placed:
 for i,(kind,form,text) in enumerate(r):
  if kind!=502:continue
  v=text.rstrip(';').split(',')
  for j in (2,5,8,11):
   x,y,z=map(int,v[j:j+3]);v[j:j+3]=map(str,(10+z,-3+y,5-x))
  r[i]=(kind,form,','.join(v)+';')
 add(124,0,[124,0,0,1,10,0,1,0,-3,-1,0,0,5])

g=[x[:72] for x in Path('exchange/iges/testdata/cylinder-solid.igs').read_text().splitlines() if x[72]=='G'];out=[]
def emit(s,k,n):out.append(f'{s:<72}{k}{n:7d}')
emit('Independent exact cone with cylindrical through aperture','S',1)
for i,s in enumerate(g):emit(s,'G',i+1)
p=[]
for i,(kind,form,s) in enumerate(r):
 chunks=[]
 while len(s)>64:
  cut=s.rfind(',',0,64)+1;chunks.append(s[:cut]);s=s[cut:]
 chunks.append(s);start=len(p)+1;p.extend((v,2*i+1) for v in chunks);status=0 if kind==186 else (10001 if kind==504 else 10000)
 emit(''.join(f'{x:8d}' for x in (kind,start,0,0,0,0,57 if placed and kind in (100,110,190,192,194) else 0,0))+f'{status:08d}','D',2*i+1)
 emit(''.join(f'{x:8d}' for x in (kind,0,0,len(chunks),form,0,0))+' '*8+f'{0:8d}','D',2*i+2)
for i,(v,owner) in enumerate(p):emit(f'{v:<64}{owner:8d}','P',i+1)
emit(f'S{1:7d}G{len(g):7d}D{2*len(r):7d}P{len(p):7d}','T',1)
Path('exchange/iges/testdata/'+('placed-conical45-cylinder-through.igs' if placed else 'conical45-cylinder-through.igs')).write_text('\n'.join(out)+'\n')
