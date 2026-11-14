"""Combine retained independent mm cylinder and genuine OCCT metre sphere/box.
Only STEP record IDs are rebased; no Go writer or numeric changes are used.
"""
from pathlib import Path
import re
root=Path('exchange/step/testdata')
base=(root/'independent-round-units.step').read_text()
box=(root/'occt-independent-metre-box.step').read_text()
off=max(map(int,re.findall(r'^#(\d+)\s*=',base,re.M)))
extra=[]
for i,v in re.findall(r'#(\d+)\s*=\s*(.*?);',box,re.S):
 extra.append('#'+str(int(i)+off)+'='+re.sub(r'#(\d+)',lambda m:'#'+str(int(m[1])+off),v)+';')
pos=base.rfind('ENDSEC;')
(root/'independent-heterogeneous-units.step').write_text(base[:pos]+'\n'.join(extra)+'\n'+base[pos:])
