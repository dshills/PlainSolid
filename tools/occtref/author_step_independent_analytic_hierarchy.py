"""Three-level shared analytic hierarchy authored independently of the Go writer.
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
box=copy_body('occt-independent-exact-materials.step');torus=copy_body('independent-model-torus.step');cylinder=copy_body('quarter-cylinder-model.step')
def frame(o,x=(1,0,0),z=(0,0,1)):
 p=emit("CARTESIAN_POINT('',("+','.join(str(v)+'.' for v in o)+"))")
 a=emit("DIRECTION('',("+','.join(str(v)+'.' for v in z)+"))")
 b=emit("DIRECTION('',("+','.join(str(v)+'.' for v in x)+"))")
 return emit(f"AXIS2_PLACEMENT_3D('',#{p},#{a},#{b})")
neutral=frame((0,0,0)); targets=[frame((0,0,0)),frame((20,0,0)),frame((0,20,0),(0,1,0),(1,0,0)),frame((8,0,0)),frame((0,0,5))]
unit=emit('(LENGTH_UNIT() NAMED_UNIT(*) SI_UNIT(.MILLI.,.METRE.))')
ang=emit('(NAMED_UNIT(*) PLANE_ANGLE_UNIT() SI_UNIT($,.RADIAN.))')
solid=emit("(NAMED_UNIT(*) SI_UNIT($,.STERADIAN.) SOLID_ANGLE_UNIT())")
ctx=emit(f"(GEOMETRIC_REPRESENTATION_CONTEXT(3) GLOBAL_UNIT_ASSIGNED_CONTEXT((#{unit},#{ang},#{solid})) REPRESENTATION_CONTEXT('',''))")
app=emit("APPLICATION_CONTEXT('core data for automotive mechanical design processes')")
emit(f"APPLICATION_PROTOCOL_DEFINITION('international standard','automotive_design',2000,#{app})")
pc=emit(f"PRODUCT_CONTEXT('',#{app},'mechanical')");dc=emit(f"PRODUCT_DEFINITION_CONTEXT('part definition',#{app},'design')")
def product(name,body=None,nodeframes=None):
 items=[neutral]+((nodeframes or []) if body is None else [body]);kind='SHAPE_REPRESENTATION' if body is None else 'ADVANCED_BREP_SHAPE_REPRESENTATION'
 rep=emit(f"{kind}('',("+','.join('#'+str(v) for v in items)+f"),#{ctx})")
 p=emit(f"PRODUCT('{name}','{name}','',(#{pc}))");formation=emit(f"PRODUCT_DEFINITION_FORMATION('','',#{p})")
 definition=emit(f"PRODUCT_DEFINITION('design','',#{formation},#{dc})");shape=emit(f"PRODUCT_DEFINITION_SHAPE('','',#{definition})")
 emit(f"SHAPE_DEFINITION_REPRESENTATION(#{shape},#{rep})")
 return definition,rep
parent,pr=product('root',nodeframes=targets[:3]);carrier,crp=product('carrier',nodeframes=[targets[4]]);group,gr=product('analytic group',nodeframes=[targets[3]])
bp,br=product('box',box);tp,tr=product('torus',torus);cp,cr=product('cylinder',cylinder)
uses=[(parent,pr,bp,br,targets[0]),(parent,pr,carrier,crp,targets[1]),(parent,pr,carrier,crp,targets[2]),(carrier,crp,group,gr,targets[4]),(group,gr,tp,tr,neutral),(group,gr,cp,cr,targets[3])]
for n,(owner,ownerrep,child,rep,target) in enumerate(uses):
 use=emit(f"NEXT_ASSEMBLY_USAGE_OCCURRENCE('use{n}','use{n}','',#{owner},#{child},$)")
 shape=emit(f"PRODUCT_DEFINITION_SHAPE('','',#{use})");transform=emit(f"ITEM_DEFINED_TRANSFORMATION('','',#{neutral},#{target})")
 relation=emit(f"(REPRESENTATION_RELATIONSHIP('','',#{rep},#{ownerrep}) REPRESENTATION_RELATIONSHIP_WITH_TRANSFORMATION(#{transform}) SHAPE_REPRESENTATION_RELATIONSHIP())")
 emit(f"CONTEXT_DEPENDENT_SHAPE_REPRESENTATION(#{relation},#{shape})")
header="ISO-10303-21;HEADER;FILE_DESCRIPTION(('independent assembly'),'2;1');FILE_NAME('','',(''),(''),'','','');FILE_SCHEMA(('AUTOMOTIVE_DESIGN'));ENDSEC;DATA;\n"
(root/'independent-analytic-hierarchy.step').write_text(header+'\n'.join(f'#{i+1}={v};' for i,v in enumerate(records))+'\nENDSEC;END-ISO-10303-21;\n')
