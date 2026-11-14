"""Combine independently authored mm box with original OCCT metre box.
Run the corresponding C++ helper's write mode first. No Go writer is used.
"""
from pathlib import Path
import re,sys
root=Path('exchange/step/testdata')
sphere=Path(sys.argv[1]).read_text()
sphere='\n'.join(x.rstrip() for x in sphere.splitlines())+'\n'
(root/'occt-independent-metre-box.step').write_text(sphere)
base=(root/'occt-independent-exact-materials.step').read_text()
records={int(i):v for i,v in re.findall(r'#(\d+)\s*=\s*(.*?);',base,re.S)}
def refs(s):return list(map(int,re.findall(r'#(\d+)',s)))
def closure(i,seen):
 if i in seen:return
 seen.add(i)
 for x in refs(records[i]):closure(x,seen)
keep=set();closure(3,keep)
for i,v in records.items():
 if v.startswith(('APPLICATION_PROTOCOL_DEFINITION','PRODUCT_RELATED_PRODUCT_CATEGORY')) and any(x in keep for x in refs(v)):closure(i,keep)
extra=['#%d=%s;'%(i,records[i]) for i in sorted(keep)]
off=max(keep)
for i,v in re.findall(r'#(\d+)\s*=\s*(.*?);',sphere,re.S):extra.append('#'+str(int(i)+off)+'='+re.sub(r'#(\d+)',lambda m:'#'+str(int(m[1])+off),v)+';')
(root/'independent-planar-units.step').write_text(sphere[:sphere.index('DATA;')+5]+'\n'+'\n'.join(extra)+'\nENDSEC;END-ISO-10303-21;\n')
