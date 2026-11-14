"""Combine retained three-family collection and independently authored metre torus.
Only STEP record IDs are rebased; no Go writer or numeric changes are used.
"""
from pathlib import Path
import re
root=Path('exchange/step/testdata')
base=(root/'independent-heterogeneous-units.step').read_text()
box=(root/'independent-model-metre-torus.step').read_text()
off=max(map(int,re.findall(r'^#(\d+)\s*=',base,re.M)))
extra=[]
for i,v in re.findall(r'#(\d+)\s*=\s*(.*?);',box,re.S):
 extra.append('#'+str(int(i)+off)+'='+re.sub(r'#(\d+)',lambda m:'#'+str(int(m[1])+off),v)+';')
pos=base.rfind('ENDSEC;')
(root/'independent-toroidal-units.step').write_text(base[:pos]+'\n'.join(extra)+'\n'+base[pos:])
