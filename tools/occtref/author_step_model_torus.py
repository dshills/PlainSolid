"""Independently author a natural ring torus with exact model-only seam circles.
Not a repair of the retained OCCT rounded-pcurve source; no Go writer is used.
"""
from pathlib import Path
r=[]
def emit(x):r.append(x);return len(r)
def p(x,y,z):return emit(f"CARTESIAN_POINT('',({x}.,{y}.,{z}.))")
def d(x,y,z):return emit(f"DIRECTION('',({x}.,{y}.,{z}.))")
o=p(1,2,3);z=d(0,0,1);x=d(1,0,0);a=emit(f"AXIS2_PLACEMENT_3D('',#{o},#{z},#{x})")
surface=emit(f"TOROIDAL_SURFACE('',#{a},3.,1.)")
major=emit(f"CIRCLE('',#{a},4.)")
minorcenter=p(4,2,3);negativey=d(0,-1,0);minoraxis=emit(f"AXIS2_PLACEMENT_3D('',#{minorcenter},#{negativey},#{x})")
minor=emit(f"CIRCLE('',#{minoraxis},1.)")
point=p(5,2,3);vertex=emit(f"VERTEX_POINT('',#{point})")
edges=[emit(f"EDGE_CURVE('',#{vertex},#{vertex},#{c},.T.)") for c in (major,minor)]
uses=[emit(f"ORIENTED_EDGE('',*,*,#{edges[i%2]},.{sense}.)") for i,sense in enumerate(('F','F','T','T'))]
loop=emit("EDGE_LOOP('',("+','.join('#'+str(i) for i in uses)+"))")
bound=emit(f"FACE_BOUND('',#{loop},.T.)");face=emit(f"ADVANCED_FACE('',(#{bound}),#{surface},.T.)");shell=emit(f"CLOSED_SHELL('',(#{face}))");body=emit(f"MANIFOLD_SOLID_BREP('exact model torus',#{shell})")
unit=emit('(LENGTH_UNIT() NAMED_UNIT(*) SI_UNIT(.MILLI.,.METRE.))');angle=emit('(NAMED_UNIT(*) PLANE_ANGLE_UNIT() SI_UNIT($,.RADIAN.))');context=emit(f"(GEOMETRIC_REPRESENTATION_CONTEXT(3) GLOBAL_UNIT_ASSIGNED_CONTEXT((#{unit},#{angle})) REPRESENTATION_CONTEXT('',''))")
zero=p(0,0,0);neutral=emit(f"AXIS2_PLACEMENT_3D('',#{zero},#{z},#{x})");rep=emit(f"ADVANCED_BREP_SHAPE_REPRESENTATION('',(#{neutral},#{body}),#{context})")
app=emit("APPLICATION_CONTEXT('core data for automotive mechanical design processes')");emit(f"APPLICATION_PROTOCOL_DEFINITION('international standard','automotive_design',2000,#{app})");pc=emit(f"PRODUCT_CONTEXT('',#{app},'mechanical')");dc=emit(f"PRODUCT_DEFINITION_CONTEXT('part definition',#{app},'design')");product=emit(f"PRODUCT('torus','torus','',(#{pc}))");formation=emit(f"PRODUCT_DEFINITION_FORMATION('','',#{product})");definition=emit(f"PRODUCT_DEFINITION('design','',#{formation},#{dc})");shape=emit(f"PRODUCT_DEFINITION_SHAPE('','',#{definition})");emit(f"SHAPE_DEFINITION_REPRESENTATION(#{shape},#{rep})");emit(f"PRODUCT_RELATED_PRODUCT_CATEGORY('part',$,(#{product}))")
text="ISO-10303-21;HEADER;FILE_DESCRIPTION(('independent model torus'),'2;1');FILE_NAME('','',(''),(''),'','','');FILE_SCHEMA(('AUTOMOTIVE_DESIGN'));ENDSEC;DATA;\n"+'\n'.join(f'#{i+1}={v};' for i,v in enumerate(r))+"\nENDSEC;END-ISO-10303-21;\n"
Path('exchange/step/testdata/independent-model-torus.step').write_text(text)
