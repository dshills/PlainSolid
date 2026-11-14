"""Standard flat NAUO assembly authored independently of the Go assembly writer.
Local geometry: untouched genuine OCCT box, separately exact hand-authored torus.
No rounded vendor torus is repaired or promoted. All local IDs are rebased.
"""
from pathlib import Path
import re
root=Path('exchange/step/testdata'); records=[]
def emit(v):
 records.append(v);return len(records)
def copy_body(name):
 text=(root/name).read_text()
 source={int(i):v.strip() for i,v in re.findall(r'#(\d+)\s*=\s*(.*?);',text,re.S)}
 body=next(i for i,v in source.items() if v.startswith('MANIFOLD_SOLID_BREP'))
 keep=set()
 def visit(i):
  if i in keep:return
  keep.add(i)
  for j in re.findall(r'#(\d+)',source[i]):visit(int(j))
 visit(body)
 ids={i:len(records)+n+1 for n,i in enumerate(sorted(keep))}
 for i in sorted(keep):emit(re.sub(r'#(\d+)',lambda m:'#'+str(ids[int(m[1])]),source[i]))
 return ids[body]
box=copy_body('occt-independent-exact-materials.step');torus=copy_body('independent-model-torus.step')
def frame(o,x=(1,0,0),z=(0,0,1)):
 p=emit("CARTESIAN_POINT('',("+','.join(str(v)+'.' for v in o)+"))")
 a=emit("DIRECTION('',("+','.join(str(v)+'.' for v in z)+"))")
 b=emit("DIRECTION('',("+','.join(str(v)+'.' for v in x)+"))")
 return emit(f"AXIS2_PLACEMENT_3D('',#{p},#{a},#{b})")
neutral=frame((0,0,0)); targets=[frame((0,0,0)),frame((20,0,0)),frame((0,20,0),(0,1,0),(1,0,0))]
unit=emit('(LENGTH_UNIT() NAMED_UNIT(*) SI_UNIT(.MILLI.,.METRE.))')
ang=emit('(NAMED_UNIT(*) PLANE_ANGLE_UNIT() SI_UNIT($,.RADIAN.))')
ctx=emit(f"(GEOMETRIC_REPRESENTATION_CONTEXT(3) GLOBAL_UNIT_ASSIGNED_CONTEXT((#{unit},#{ang})) REPRESENTATION_CONTEXT('',''))")
app=emit("APPLICATION_CONTEXT('core data for automotive mechanical design processes')")
emit(f"APPLICATION_PROTOCOL_DEFINITION('international standard','automotive_design',2000,#{app})")
pc=emit(f"PRODUCT_CONTEXT('',#{app},'mechanical')");dc=emit(f"PRODUCT_DEFINITION_CONTEXT('part definition',#{app},'design')")
def product(name,body=None):
 items=[neutral]+(targets if body is None else [body]);kind='SHAPE_REPRESENTATION' if body is None else 'ADVANCED_BREP_SHAPE_REPRESENTATION'
 rep=emit(f"{kind}('',("+','.join('#'+str(v) for v in items)+f"),#{ctx})")
 p=emit(f"PRODUCT('{name}','{name}','',(#{pc}))");formation=emit(f"PRODUCT_DEFINITION_FORMATION('','',#{p})")
 definition=emit(f"PRODUCT_DEFINITION('design','',#{formation},#{dc})");shape=emit(f"PRODUCT_DEFINITION_SHAPE('','',#{definition})")
 emit(f"SHAPE_DEFINITION_REPRESENTATION(#{shape},#{rep})")
 return definition,rep
parent,pr=product('root');bp,br=product('box',box);tp,tr=product('torus',torus)
for n,(child,rep,target) in enumerate([(bp,br,targets[0]),(tp,tr,targets[1]),(tp,tr,targets[2])]):
 use=emit(f"NEXT_ASSEMBLY_USAGE_OCCURRENCE('use{n}','use{n}','',#{parent},#{child},$)")
 shape=emit(f"PRODUCT_DEFINITION_SHAPE('','',#{use})");transform=emit(f"ITEM_DEFINED_TRANSFORMATION('','',#{neutral},#{target})")
 relation=emit(f"(REPRESENTATION_RELATIONSHIP('','',#{rep},#{pr}) REPRESENTATION_RELATIONSHIP_WITH_TRANSFORMATION(#{transform}) SHAPE_REPRESENTATION_RELATIONSHIP())")
 emit(f"CONTEXT_DEPENDENT_SHAPE_REPRESENTATION(#{relation},#{shape})")
header="ISO-10303-21;HEADER;FILE_DESCRIPTION(('independent assembly'),'2;1');FILE_NAME('','',(''),(''),'','','');FILE_SCHEMA(('AUTOMOTIVE_DESIGN'));ENDSEC;DATA;\n"
(root/'independent-torus-assembly.step').write_text(header+'\n'.join(f'#{i+1}={v};' for i,v in enumerate(records))+'\nENDSEC;END-ISO-10303-21;\n')
