"""Insert the independent OCCT reentrant product into retained AP214 hierarchies.
Only the planar body's owned geometry closure changes; relationship frames,
product IDs and the other three local materials remain original.
"""
from pathlib import Path
import re
root=Path('exchange/step/testdata')
def records(s):return {int(k):v.strip() for k,v in re.findall(r'#(\d+)\s*=\s*(.*?);',s,re.S)}
def refs(v):return [int(k) for k in re.findall(r'#(\d+)',v)]
def closure(r,seed):
 seen=set();todo=list(seed)
 while todo:
  k=todo.pop()
  if k in seen:continue
  seen.add(k);todo.extend(refs(r[k]))
 return seen
for product,suffix in [('occt-reentrant-planar-product.step','reentrant'),('occt-holed-planar-product.step','holed'),('occt-affine-pcurve-holed-product.step','affine-pcurve-holed')]:
 raw=(root/product).read_text();new=records(raw);body=next(k for k,v in new.items() if v.startswith('MANIFOLD_SOLID_BREP'));owned=closure(new,[body])
 for base in ['rational-spherical-hierarchy','occt-rational-spherical-hierarchy']:
  text=(root/(base+'.step')).read_text();r=records(text);old=closure(r,[1]);incoming={q for k,v in r.items() if k not in old for q in refs(v) if q in old and q!=1};keep=closure(r,incoming)
  for k in old-keep:r.pop(k)
  mapping={body:1};n=max(r)+1
  for k in sorted(owned-{body}):mapping[k]=n;n+=1
  for k in sorted(owned):r[mapping[k]]=re.sub(r'#(\d+)',lambda m:'#'+str(mapping[int(m[1])]),new[k])
  out=text[:text.index('DATA;')+5]+'\n'+'\n'.join(f'#{k}={v};' for k,v in r.items())+'\nENDSEC;END-ISO-10303-21;\n'
  (root/(base+'-'+suffix+'.step')).write_text('\n'.join(line.rstrip() for line in out.splitlines())+'\n')
 (root/product).write_text('\n'.join(line.rstrip() for line in raw.splitlines())+'\n')
