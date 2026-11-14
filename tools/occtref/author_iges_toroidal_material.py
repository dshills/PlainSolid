# Independent IGES186 one-face ring torus: R=4,r=1, two full circular seams.
from pathlib import Path
records=[(116,0,'116,0,0,0,0;'),(123,0,'123,0,0,1;'),(123,0,'123,1,0,0;'),
(198,1,'198,1,3,4,1,5,0,0;'),(502,1,'502,1,5,0,0;'),
(100,0,'100,0,0,0,5,0,5,0;'),(100,0,'100,0,0,0,1,0,1,0;'),
(124,0,'124,1,0,0,4,0,0,-1,0,0,1,0,0;'),
(504,1,'504,2,11,9,1,9,1,13,9,1,9,1;'),
(508,1,'508,4,0,17,1,1,0,0,17,2,1,0,0,17,1,0,0,0,17,2,0,0;'),
(510,1,'510,7,1,1,19;'),(514,1,'514,1,21,1;'),(186,0,'186,23,1,0;')]
base=Path('exchange/iges/testdata/conical45-frustum.igs').read_text().splitlines();g=[x[:72] for x in base if x[72]=='G'];out=[];params=[]
def emit(s,k,n):out.append(f'{s:<72}{k}{n:7d}')
emit('Independent exact ring torus material with two circular model seams','S',1)
for i,s in enumerate(g):emit(s,'G',i+1)
for i,(kind,form,s) in enumerate(records):
 chunks=[]
 while len(s)>64:
  cut=s.rfind(',',0,64)+1;chunks.append(s[:cut]);s=s[cut:]
 chunks.append(s);start=len(params)+1;params.extend((x,2*i+1) for x in chunks)
 status=0 if kind==186 else (10001 if kind==504 else 10000);tr=15 if 2*i+1==13 else 0
 emit(''.join(f'{x:8d}' for x in (kind,start,0,0,0,0,tr,0))+f'{status:08d}','D',2*i+1)
 emit(''.join(f'{x:8d}' for x in (kind,0,0,len(chunks),form,0,0))+' '*8+f'{0:8d}','D',2*i+2)
for i,(s,owner) in enumerate(params):emit(f'{s:<64}{owner:8d}','P',i+1)
emit(f'S{1:7d}G{len(g):7d}D{2*len(records):7d}P{len(params):7d}','T',1)
Path('exchange/iges/testdata/toroidal-material.igs').write_text('\n'.join(out)+'\n')
