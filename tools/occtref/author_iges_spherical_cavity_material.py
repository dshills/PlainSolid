# Independently authored exact IGES186 sphere: poles, meridian, degree pcurves.
from pathlib import Path
records=[(186,0,'186,3,1,0;'),(514,1,'514,1,5,1;'),(510,1,'510,7,1,1,15;'),
(196,1,'196,9,2,11,13;'),(116,0,'116,0,0,0,0;'),(123,0,'123,0,0,1;'),(123,0,'123,1,0,0;'),
(508,1,'508,4,1,17,1,0,1,0,19,0,21,1,0,1,0,27,1,17,2,1,1,0,29,0,21,1,1,1,0,31;'),
(502,1,'502,2,0,0,2,0,0,-2;'),(110,0,'110,360,90,0,0,90,0;'),
(504,1,'504,1,23,17,2,17,1;'),(100,0,'100,0,0,0,0,-2,0,2;'),
(124,0,'124,1,0,0,0,0,0,-1,0,0,1,0,0;'),
(110,0,'110,0,90,0,0,-90,0;'),(110,0,'110,0,-90,0,360,-90,0;'),(110,0,'110,360,-90,0,360,90,0;')]
# Duplicate the independently authored complete shell with its own datums.
outer=records[:]
inner=[]
for kind,form,text in outer[1:]:
 v=text.rstrip(';').split(',');slots=[]
 if kind==514:slots=[2]
 elif kind==510:slots=[1,4]
 elif kind==196:slots=[1,3,4];v[2]='1'
 elif kind==508:slots=[3,8,10,15,17,22,24,29]
 elif kind==504:slots=[2,3,5]
 elif kind==502:v[4]='1';v[7]='-1'
 elif kind==100:v[5]='-1';v[7]='1'
 for j in slots:v[j]=str(int(v[j])+30)
 inner.append((kind,form,','.join(v)+';'))
records=outer+inner
records[0]=(186,0,'186,3,1,1,33,0;')
base=Path('exchange/iges/testdata/conical45-frustum.igs').read_text().splitlines();g=[x[:72] for x in base if x[72]=='G'];out=[];params=[]
def emit(s,k,n):out.append(f'{s:<72}{k}{n:7d}')
emit('Independent concentric sphere cavity with two complete original shells','S',1)
for i,s in enumerate(g):emit(s,'G',i+1)
for i,(kind,form,s) in enumerate(records):
 chunks=[]
 while len(s)>64:
  cut=s.rfind(',',0,64)+1;chunks.append(s[:cut]);s=s[cut:]
 chunks.append(s);start=len(params)+1;params.extend((x,2*i+1) for x in chunks)
 status=0 if kind==186 else (10001 if kind==504 else 10000);tr=25 if 2*i+1==23 else (55 if 2*i+1==53 else 0)
 emit(''.join(f'{x:8d}' for x in (kind,start,0,0,0,0,tr,0))+f'{status:08d}','D',2*i+1)
 emit(''.join(f'{x:8d}' for x in (kind,0,0,len(chunks),form,0,0))+' '*8+f'{0:8d}','D',2*i+2)
for i,(s,owner) in enumerate(params):emit(f'{s:<64}{owner:8d}','P',i+1)
emit(f'S{1:7d}G{len(g):7d}D{2*len(records):7d}P{len(params):7d}','T',1)
Path('exchange/iges/testdata/spherical-cavity-material.igs').write_text('\n'.join(out)+'\n')
