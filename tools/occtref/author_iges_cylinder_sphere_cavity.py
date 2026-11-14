# Compose two independent hand-authored analytic shell graphs, no Go helpers.
from pathlib import Path
import sys
placed="--placed" in sys.argv

def read(name):
 lines=Path('exchange/iges/testdata/'+name+'.igs').read_text().splitlines()
 ds=[x for x in lines if x[72]=='D'];ps=[x for x in lines if x[72]=='P'];r=[]
 for i in range(0,len(ds),2):
  a,b=ds[i:i+2];start=int(a[8:16]);n=int(b[24:32]);v=''.join(x[:64] for x in ps[start-1:start-1+n]).strip().rstrip(';').split(',');r.append((int(a[:8]),int(b[32:40]),[x.strip() for x in v]))
 return r,[x[:72] for x in lines if x[72]=='G']
r,g=read('circular-cylinder-solid');inner,_=read('spherical-material')
updates={1:'116,0,0,-3,0',3:'116,0,0,4,0',9:'192,1,5,3,7',15:'502,2,3,0,4,3,0,-3',17:'100,4,0,0,3,0,3,0',19:'100,-3,0,0,3,0,3,0',21:'110,3,0,-3,3,0,4',25:'110,360,7,0,0,7,0',27:'110,0,7,0,0,0,0',31:'110,360,0,0,360,7,0',47:'186,45,1,1,49,0'}
for id,text in updates.items():k,f,_=r[(id-1)//2];r[(id-1)//2]=(k,f,text.split(','))
for k,f,v in inner[1:]:
 slots=[]
 if k==514:slots=[2]
 elif k==510:slots=[1,4]
 elif k==196:slots=[1,3,4];v[2]='1'
 elif k==508:slots=[3,8,10,15,17,22,24,29]
 elif k==504:slots=[2,3,5]
 elif k==502:v[4]='1';v[7]='-1'
 elif k==100:v[5]='-1';v[7]='1'
 for j in slots:v[j]=str(int(v[j])+46)
 r.append((k,f,v))
if placed:
 for i,(k,f,v) in enumerate(r):
  if k==502:
   for j in (2,5):
    x,y,z=map(int,v[j:j+3]);v[j:j+3]=map(str,(10+z,-3+y,5-x))
  if 2*i+1==71:v=['124','0','1','0','10','0','0','-1','-3','-1','0','0','5'];r[i]=(k,f,v)
 r.append((124,0,['124','0','0','1','10','0','1','0','-3','-1','0','0','5']))
out=[];params=[]
def emit(s,k,n):out.append(f'{s:<72}{k}{n:7d}')
emit('Independent cylindrical stock with concentric spherical cavity','S',1)
for i,s in enumerate(g):emit(s,'G',i+1)
for i,(kind,form,v) in enumerate(r):
 s=','.join(v)+';';chunks=[]
 while len(s)>64:
  cut=s.rfind(',',0,64)+1;chunks.append(s[:cut]);s=s[cut:]
 chunks.append(s);start=len(params)+1;params.extend((x,2*i+1) for x in chunks)
 status=0 if kind==186 else (10001 if kind==504 else 10000);tr=71 if 2*i+1==69 else (79 if placed and 2*i+1 in (9,11,13,17,19,21,53) else 0)
 emit(''.join(f'{x:8d}' for x in (kind,start,0,0,0,0,tr,0))+f'{status:08d}','D',2*i+1)
 emit(''.join(f'{x:8d}' for x in (kind,0,0,len(chunks),form,0,0))+' '*8+f'{0:8d}','D',2*i+2)
for i,(s,owner) in enumerate(params):emit(f'{s:<64}{owner:8d}','P',i+1)
emit(f'S{1:7d}G{len(g):7d}D{2*len(r):7d}P{len(params):7d}','T',1)
Path('exchange/iges/testdata/'+('placed-cylinder-sphere-cavity.igs' if placed else 'cylinder-sphere-cavity.igs')).write_text('\n'.join(out)+'\n')
