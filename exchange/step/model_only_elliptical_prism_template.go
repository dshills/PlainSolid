package step

const modelOnlyEllipticalPrismTemplate = `#1 = APPLICATION_PROTOCOL_DEFINITION('international standard',
  'automotive_design',2000,#2);
#2 = APPLICATION_CONTEXT(
  'core data for automotive mechanical design processes');
#3 = SHAPE_DEFINITION_REPRESENTATION(#4,#10);
#4 = PRODUCT_DEFINITION_SHAPE('','',#5);
#5 = PRODUCT_DEFINITION('design','',#6,#9);
#6 = PRODUCT_DEFINITION_FORMATION('','',#7);
#7 = PRODUCT('Open CASCADE STEP translator 8.1 1',
  'Open CASCADE STEP translator 8.1 1','',(#8));
#8 = PRODUCT_CONTEXT('',#2,'mechanical');
#9 = PRODUCT_DEFINITION_CONTEXT('part definition',#2,'design');
#10 = ADVANCED_BREP_SHAPE_REPRESENTATION('',(#11,#15),#116);
#11 = AXIS2_PLACEMENT_3D('',#12,#13,#14);
#12 = CARTESIAN_POINT('',(0.,0.,0.));
#13 = DIRECTION('',(0.,0.,1.));
#14 = DIRECTION('',(1.,0.,-0.));
#15 = MANIFOLD_SOLID_BREP('',#16);
#16 = CLOSED_SHELL('',(#17,#108,#112));
#17 = ADVANCED_FACE('',(#18),#32,.F.);
#18 = FACE_BOUND('',#19,.F.);
#19 = EDGE_LOOP('',(#20,#53,#80,#81));
#20 = ORIENTED_EDGE('',*,*,#21,.T.);
#21 = EDGE_CURVE('',#22,#24,#27,.T.);
#22 = VERTEX_POINT('',#23);
#23 = CARTESIAN_POINT('',(1.,2.,-1.));
#24 = VERTEX_POINT('',#25);
#25 = CARTESIAN_POINT('',(1.,5.,-1.));
#27 = LINE('',#28,#29);
#28 = CARTESIAN_POINT('',(1.,2.,-1.));
#29 = VECTOR('',#30,1.);
#30 = DIRECTION('',(0.,1.,0.));
#32 = SURFACE_OF_LINEAR_EXTRUSION('',#33,#38);
#33 = ELLIPSE('',#34,4.,2.);
#34 = AXIS2_PLACEMENT_3D('',#35,#36,#37);
#35 = CARTESIAN_POINT('',(1.,2.,3.));
#36 = DIRECTION('',(0.,1.,0.));
#37 = DIRECTION('',(0.,0.,-1.));
#38 = VECTOR('',#39,1.);
#39 = DIRECTION('',(-0.,-1.,-0.));
#53 = ORIENTED_EDGE('',*,*,#54,.T.);
#54 = EDGE_CURVE('',#24,#24,#56,.T.);
#56 = ELLIPSE('',#57,4.,2.);
#57 = AXIS2_PLACEMENT_3D('',#58,#59,#60);
#58 = CARTESIAN_POINT('',(1.,5.,3.));
#59 = DIRECTION('',(0.,1.,0.));
#60 = DIRECTION('',(0.,0.,-1.));
#69 = PLANE('',#70);
#70 = AXIS2_PLACEMENT_3D('',#71,#72,#73);
#71 = CARTESIAN_POINT('',(1.,5.,3.));
#72 = DIRECTION('',(0.,1.,0.));
#73 = DIRECTION('',(0.,0.,-1.));
#80 = ORIENTED_EDGE('',*,*,#21,.F.);
#81 = ORIENTED_EDGE('',*,*,#82,.F.);
#82 = EDGE_CURVE('',#22,#22,#84,.T.);
#84 = ELLIPSE('',#85,4.,2.);
#85 = AXIS2_PLACEMENT_3D('',#86,#87,#88);
#86 = CARTESIAN_POINT('',(1.,2.,3.));
#87 = DIRECTION('',(0.,1.,0.));
#88 = DIRECTION('',(0.,0.,-1.));
#97 = PLANE('',#98);
#98 = AXIS2_PLACEMENT_3D('',#99,#100,#101);
#99 = CARTESIAN_POINT('',(1.,2.,3.));
#100 = DIRECTION('',(0.,1.,0.));
#101 = DIRECTION('',(0.,0.,-1.));
#108 = ADVANCED_FACE('',(#109),#97,.F.);
#109 = FACE_BOUND('',#110,.F.);
#110 = EDGE_LOOP('',(#111));
#111 = ORIENTED_EDGE('',*,*,#82,.T.);
#112 = ADVANCED_FACE('',(#113),#69,.T.);
#113 = FACE_BOUND('',#114,.T.);
#114 = EDGE_LOOP('',(#115));
#115 = ORIENTED_EDGE('',*,*,#54,.T.);
#116 = ( GEOMETRIC_REPRESENTATION_CONTEXT(3)
GLOBAL_UNCERTAINTY_ASSIGNED_CONTEXT((#120)) GLOBAL_UNIT_ASSIGNED_CONTEXT
((#117,#118,#119)) REPRESENTATION_CONTEXT('Context #1',
  '3D Context with UNIT and UNCERTAINTY') );
#117 = ( LENGTH_UNIT() NAMED_UNIT(*) SI_UNIT(.MILLI.,.METRE.) );
#118 = ( NAMED_UNIT(*) PLANE_ANGLE_UNIT() SI_UNIT($,.RADIAN.) );
#119 = ( NAMED_UNIT(*) SI_UNIT($,.STERADIAN.) SOLID_ANGLE_UNIT() );
#120 = UNCERTAINTY_MEASURE_WITH_UNIT(LENGTH_MEASURE(1.E-07),#117,
  'distance_accuracy_value','confusion accuracy');
#121 = PRODUCT_RELATED_PRODUCT_CATEGORY('part',$,(#7));
`
