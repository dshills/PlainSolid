"""Independent AP214 registration of exact hand-authored cylinder models.
No Go output is used. Existing OCCT planar/ellipse roots are retained unchanged.
"""
from pathlib import Path
import re
base=Path('exchange/step/testdata/occt-independent-exact-materials.step').read_text()
next_id=max(map(int,re.findall(r'^#(\d+)\s*=',base,re.M)))+1
extra=[]
for name in ('split-cylinder-model','quarter-cylinder-model'):
 text=Path('exchange/step/testdata/'+name+'.step').read_text()
 records={int(i):v.strip() for i,v in re.findall(r'#(\d+)\s*=\s*(.*?);',text,re.S)}
 rep=next(i for i,v in records.items() if v.startswith('ADVANCED_BREP_SHAPE_REPRESENTATION'))
 body=next(i for i,v in records.items() if v.startswith('MANIFOLD_SOLID_BREP'))
 count=max(records)
 def add(s):
  global count
  count+=1;records[count]=s;return count
 point=add("CARTESIAN_POINT('',(0.,0.,0.))");z=add("DIRECTION('',(0.,0.,1.))");x=add("DIRECTION('',(1.,0.,0.))");neutral=add(f"AXIS2_PLACEMENT_3D('',#{point},#{z},#{x})")
 records[rep]=records[rep].replace(f'(#{body})',f'(#{neutral},#{body})')
 app=add("APPLICATION_CONTEXT('core data for automotive mechanical design processes')")
 add(f"APPLICATION_PROTOCOL_DEFINITION('international standard','automotive_design',2000,#{app})")
 pc=add(f"PRODUCT_CONTEXT('',#{app},'mechanical')");dc=add(f"PRODUCT_DEFINITION_CONTEXT('part definition',#{app},'design')")
 product=add(f"PRODUCT('{name}','{name}','',(#{pc}))")
 formation=add(f"PRODUCT_DEFINITION_FORMATION('','',#{product})")
 definition=add(f"PRODUCT_DEFINITION('design','',#{formation},#{dc})")
 shape=add(f"PRODUCT_DEFINITION_SHAPE('','',#{definition})")
 add(f"SHAPE_DEFINITION_REPRESENTATION(#{shape},#{rep})")
 add(f"PRODUCT_RELATED_PRODUCT_CATEGORY('part',$,(#{product}))")
 mapping={old:next_id+i for i,old in enumerate(sorted(records))};next_id+=len(mapping)
 for old,v in sorted(records.items()):extra.append(f"#{mapping[old]}="+re.sub(r'#(\d+)',lambda m:'#'+str(mapping[int(m.group(1))]),v)+';')
 end=base.rfind('ENDSEC;')
 Path('exchange/step/testdata/independent-cylinder-materials.step').write_text(base[:end]+'\n'.join(extra)+'\n'+base[end:])
