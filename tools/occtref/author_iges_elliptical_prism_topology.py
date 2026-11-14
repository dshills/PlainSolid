from pathlib import Path
r=[]
def add(k,f,v):
 i=2*len(r)+1;r.append((k,f,','.join(map(str,v))+';'));return i
O=add(116,0,[116,0,0,3,0]);T=add(116,0,[116,0,0,7,0]);Z=add(123,0,[123,0,0,1]);X=add(123,0,[123,1,0,0]);CB=add(104,1,[104,4,0,25,0,0,-100,3,5,0,5,0]);CT=add(104,1,[104,4,0,25,0,0,-100,7,5,0,5,0]);L=add(110,0,[110,5,0,3,5,0,7]);S=add(122,0,[122,CB,5,0,7]);PT=add(190,1,[190,T,Z,X]);PB=add(190,1,[190,O,Z,X]);V=add(502,1,[502,2,5,0,7,5,0,3]);E=add(504,1,[504,3,CT,V,1,V,1,L,V,2,V,1,CB,V,2,V,2]);ls=[508,4]
for edge,sense in zip((1,2,3,2),(0,0,1,1)):ls += [0,E,edge,sense,0]
LS=add(508,1,ls);LT=add(508,1,[508,1,0,E,1,1,0]);LB=add(508,1,[508,1,0,E,3,1,0]);FS=add(510,1,[510,S,1,1,LS]);FT=add(510,1,[510,PT,1,1,LT]);FB=add(510,1,[510,PB,1,1,LB]);SH=add(514,1,[514,3,FS,1,FT,1,FB,0]);B=add(186,0,[186,SH,1,0]);g=[x[:72] for x in Path('exchange/iges/testdata/cylinder-solid.igs').read_text().splitlines() if x[72]=='G'];out=[]
def emit(s,k,n):out.append(f'{s:<72}{k}{n:7d}')
emit('Independent exact analytic elliptical prism topology','S',1)
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
Path('exchange/iges/testdata/elliptical-prism-topology.igs').write_text('\n'.join(out)+'\n')
