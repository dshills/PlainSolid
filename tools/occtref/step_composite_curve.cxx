// Independent authored AP203 composite fixture, normalized by OCCT's reader/writer.
// No PlainSolid graph generation or geometry implementation is used.
#include <STEPControl_Reader.hxx>
#include <STEPControl_Writer.hxx>
#include <BRepCheck_Analyzer.hxx>
#include <BRepGProp.hxx>
#include <GProp_GProps.hxx>
#include <TopExp_Explorer.hxx>
#include <TopoDS.hxx>
#include <TopoDS_Wire.hxx>
#include <TopoDS_Vertex.hxx>

#include <BRepTools_WireExplorer.hxx>
#include <BRep_Tool.hxx>
#include <TopExp.hxx>
#include <sstream>
#include <iostream>
#include <iomanip>
#include <string>
int main(int argc,char**argv){if(argc!=3)return 2;std::string mode=argv[1];if(mode=="write"||mode=="write-closed"){
 bool closed=mode=="write-closed";std::ostringstream s;
 s<<"ISO-10303-21;HEADER;FILE_DESCRIPTION(('independent composite'),'2;1');FILE_NAME('','',(''),(''),'OCCT fixture','','');FILE_SCHEMA(('CONFIG_CONTROL_DESIGN'));ENDSEC;DATA;\n"
 "#1=(LENGTH_UNIT() NAMED_UNIT(*) SI_UNIT(.MILLI.,.METRE.));\n"
 "#2=(NAMED_UNIT(*) PLANE_ANGLE_UNIT() SI_UNIT($,.RADIAN.));\n"
 "#3=(NAMED_UNIT(*) SI_UNIT($,.STERADIAN.) SOLID_ANGLE_UNIT());\n"
 "#4=(GEOMETRIC_REPRESENTATION_CONTEXT(3) GLOBAL_UNIT_ASSIGNED_CONTEXT((#1,#2,#3)) REPRESENTATION_CONTEXT('',''));\n"
 "#10=CARTESIAN_POINT('',(0.,0.,0.));#11=CARTESIAN_POINT('',(1.,0.,0.));\n"
 "#12=B_SPLINE_CURVE_WITH_KNOTS('line',1,(#10,#11),.UNSPECIFIED.,.F.,.U.,(2,2),(0.,1.),.UNSPECIFIED.);\n"
 "#13=CARTESIAN_POINT('',(2.,1.,0.));#14=CARTESIAN_POINT('',(2.,0.,0.));#15=CARTESIAN_POINT('',(1.,0.,0.));\n"
 "#16=(BOUNDED_CURVE() B_SPLINE_CURVE(2,(#13,#14,#15),.UNSPECIFIED.,.F.,.U.) B_SPLINE_CURVE_WITH_KNOTS((3,3),(0.,1.),.UNSPECIFIED.) CURVE() GEOMETRIC_REPRESENTATION_ITEM() RATIONAL_B_SPLINE_CURVE((1.,0.75,1.)) REPRESENTATION_ITEM('reversed rational bend'));\n"
 "#21=COMPOSITE_CURVE_SEGMENT(.CONTINUOUS.,.T.,#12);\n";
 s<<"#22=COMPOSITE_CURVE_SEGMENT(."<<(closed?"CONTINUOUS":"DISCONTINUOUS")<<".,.F.,#16);\n";
 if(closed)s<<"#17=CARTESIAN_POINT('',(2.,1.,0.));#18=CARTESIAN_POINT('',(0.,0.,0.));#19=B_SPLINE_CURVE_WITH_KNOTS('return',1,(#17,#18),.UNSPECIFIED.,.F.,.U.,(2,2),(0.,1.),.UNSPECIFIED.);#23=COMPOSITE_CURVE_SEGMENT(.CONTINUOUS.,.T.,#19);\n";
 s<<"#24=COMPOSITE_CURVE('composite',(#21,#22"<<(closed?",#23":"")<<"),.U.);\n"
 "#25=GEOMETRIC_CURVE_SET('ordered set',(#24));#26=GEOMETRICALLY_BOUNDED_WIREFRAME_SHAPE_REPRESENTATION('wire geometry',(#25),#4);\n"
 "#30=APPLICATION_CONTEXT('configuration controlled 3d designs of mechanical parts and assemblies');\n"
 "#31=APPLICATION_PROTOCOL_DEFINITION('international standard','config_control_design',1994,#30);\n"
 "#32=MECHANICAL_CONTEXT('',#30,'mechanical');#33=DESIGN_CONTEXT('',#30,'design');\n"
 "#34=PRODUCT('1','composite product','',(#32));#35=PRODUCT_DEFINITION_FORMATION_WITH_SPECIFIED_SOURCE('','',#34,.NOT_KNOWN.);\n"
 "#36=PRODUCT_DEFINITION('design','',#35,#33);#37=PRODUCT_DEFINITION_SHAPE('','',#36);#38=SHAPE_DEFINITION_REPRESENTATION(#37,#26);ENDSEC;END-ISO-10303-21;\n";
 std::istringstream input(s.str());STEPControl_Reader reader;if(reader.ReadStream("independent composite",input)!=IFSelect_RetDone)return 3;STEPControl_Writer writer(reader.WS(),false);return writer.Write(argv[2])==IFSelect_RetDone?0:4;
 }
 STEPControl_Reader reader;if(reader.ReadFile(argv[2])!=IFSelect_RetDone||reader.TransferRoots()!=1)return 5;auto shape=reader.OneShape();if(shape.IsNull()||!BRepCheck_Analyzer(shape).IsValid())return 6;
 int edges=0,wires=0,closed=0;gp_Pnt first,last;for(TopExp_Explorer x(shape,TopAbs_EDGE);x.More();x.Next())++edges;
 for(TopExp_Explorer x(shape,TopAbs_WIRE);x.More();x.Next()){++wires;auto wire=TopoDS::Wire(x.Current());if(wire.Closed())++closed;bool start=true;for(BRepTools_WireExplorer e(wire);e.More();e.Next()){TopoDS_Vertex a,b;TopExp::Vertices(e.Current(),a,b,true);if(start){first=BRep_Tool::Pnt(a);start=false;}last=BRep_Tool::Pnt(b);}}
 GProp_GProps p;BRepGProp::LinearProperties(shape,p);std::cout<<std::setprecision(17)<<"{\"Edges\":"<<edges<<",\"Wires\":"<<wires<<",\"Closed\":"<<closed<<",\"Length\":"<<p.Mass()<<",\"Start\":["<<first.X()<<","<<first.Y()<<","<<first.Z()<<"],\"End\":["<<last.X()<<","<<last.Y()<<","<<last.Z()<<"]}\n";return 0;
}
