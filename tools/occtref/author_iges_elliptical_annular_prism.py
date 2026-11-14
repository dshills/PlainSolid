# Independent 36-record analytic through-bore graph; not Go output.
from pathlib import Path
r=[]
def add(k,f,v):
 i=2*len(r)+1;r.append((k,f,','.join(map(str,[k]+v))+';'));return i
add(116,0,[0,0,3,0]);add(116,0,[0,0,7,0]);add(123,0,[0,0,1]);add(123,0,[1,0,0])
add(122,0,[23,5,0,7]);add(122,0,[29,2,0,7]);add(190,1,[3,5,7]);add(190,1,[1,5,7])
add(502,1,[4,5,0,7,5,0,3,2,0,7,2,0,3])
for A,C,F,major in [(4,25,-100,5),(1,4,-4,2)]:
 add(104,1,[A,0,C,0,0,F,7,major,0,major,0]);add(110,0,[major,0,3,major,0,7]);add(104,1,[A,0,C,0,0,F,3,major,0,major,0])
add(504,1,[6,19,17,1,17,1,21,17,2,17,1,23,17,2,17,2,25,17,3,17,3,27,17,4,17,3,29,17,4,17,4])
for offset in [0,3]:
 v=[4]
 for edge,sense in zip([1,2,3,2],[0,0,1,1]):v += [0,31,edge+offset,sense,0]
 add(508,1,v)
for edge,sense in [(1,1),(4,0),(3,1),(6,0)]:add(508,1,[1,0,31,edge,sense,0])
add(510,1,[9,1,1,33]);add(510,1,[11,1,1,35]);add(510,1,[13,2,1,37,39]);add(510,1,[15,2,1,41,43])
add(514,1,[4,45,1,47,0,49,1,51,0]);add(186,0,[53,1,0])
g=[x[:72] for x in Path('exchange/iges/testdata/cylinder-solid.igs').read_text().splitlines() if x[72]=='G'];out=[]
def emit(s,k,n):out.append(f'{s:<72}{k}{n:7d}')
emit('Independent exact analytic elliptical annular prism','S',1)
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
Path('exchange/iges/testdata/elliptical-annular-prism-topology.igs').write_text('\n'.join(out)+'\n')
