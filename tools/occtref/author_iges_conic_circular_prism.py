# Independently specialize the hand-authored twenty-entity ellipse topology.
# A=C=4,F=-100 is precisely x*x+y*y=25; all seam/vertex data already lie on +X.
from pathlib import Path
source=Path('exchange/iges/testdata/elliptical-prism-topology.igs').read_text().splitlines()
out=[]
for line in source:
 if line[72]=='P' and line.startswith('104,'):
  values=line[:64].strip().rstrip(';').split(',')
  assert len(values)==12 and values[1]=='4' and values[3]=='25'
  values[3]='4'
  text=','.join(values)+';'
  line=f'{text:<64}'+line[64:]
 if line[72]=='S':line=f'{"Independent exact circular conic104 tabulated solid":<72}'+line[72:]
 out.append(line)
Path('exchange/iges/testdata/conic-circular-prism-topology.igs').write_text('\n'.join(out)+'\n')
