#include <STEPControl_Reader.hxx>
#include <BRepCheck_Analyzer.hxx>
#include <BRepGProp.hxx>
#include <GProp_GProps.hxx>
#include <BRepPrimAPI_MakeTorus.hxx>
#include <BRepPrimAPI_MakeCylinder.hxx>
#include <BRepAlgoAPI_Cut.hxx>
#include <BRepAdaptor_Surface.hxx>
#include <TopExp_Explorer.hxx>
#include <TopoDS.hxx>
#include <cmath>
#include <gp_Torus.hxx>
int main(int argc,char**argv){if(argc!=2&&argc!=3)return 1;gp_Pnt origin(0,0,0);gp_Dir x(1,0,0),z(0,0,1);if(argc==3){origin=gp_Pnt(.1,-.4,.375);x=gp_Dir(.6,0,-.8);z=gp_Dir(.8,0,.6);}auto point=[&](double a,double b,double c){return argc==3?gp_Pnt(.1+.6*a+.8*c,-.4+b,.375-.8*a+.6*c):gp_Pnt(a,b,c);};auto direction=[&](double a,double b,double c){return argc==3?gp_Dir(.6*a+.8*c,b,-.8*a+.6*c):gp_Dir(a,b,c);};STEPControl_Reader reader;if(reader.ReadFile(argv[1])!=IFSelect_RetDone||reader.TransferRoots()!=1)return 2;auto s=reader.OneShape();if(!BRepCheck_Analyzer(s).IsValid())return 3;int solids=0,shells=0,torus=0;for(TopExp_Explorer x(s,TopAbs_SOLID);x.More();x.Next())solids++;for(TopExp_Explorer x(s,TopAbs_SHELL);x.More();x.Next())shells++;for(TopExp_Explorer x(s,TopAbs_FACE);x.More();x.Next()){auto face=TopoDS::Face(x.Current());if(BRepAdaptor_Surface(face).GetType()==GeomAbs_Torus){torus++;auto axis=BRepAdaptor_Surface(face).Torus().Axis().Direction();if(fabs(axis.Dot(direction(.8,0,.6))-1)>1e-12)return 7;if(face.Orientation()!=TopAbs_REVERSED)return 4;}}if(solids!=1||shells!=2||torus!=1)return 5;auto t=BRepPrimAPI_MakeTorus(gp_Ax2(point(1,0,0),direction(.8,0,.6),direction(.6,0,-.8)),3,1).Shape();auto c=BRepPrimAPI_MakeCylinder(gp_Ax2(point(0,0,-6),direction(0,0,1),direction(1,0,0)),8,12).Shape();auto ref=BRepAlgoAPI_Cut(c,t).Shape();GProp_GProps v,a,rv,ra;BRepGProp::VolumeProperties(s,v);BRepGProp::SurfaceProperties(s,a);BRepGProp::VolumeProperties(ref,rv);BRepGProp::SurfaceProperties(ref,ra);if(fabs(v.Mass()-rv.Mass())>1e-6||fabs(a.Mass()-ra.Mass())>1e-6||v.CentreOfMass().Distance(rv.CentreOfMass())>1e-7)return 6;return 0;}
