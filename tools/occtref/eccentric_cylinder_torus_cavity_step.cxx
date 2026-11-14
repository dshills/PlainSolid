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
int main(int argc,char**argv){if(argc!=2)return 1;STEPControl_Reader reader;if(reader.ReadFile(argv[1])!=IFSelect_RetDone||reader.TransferRoots()!=1)return 2;auto s=reader.OneShape();if(!BRepCheck_Analyzer(s).IsValid())return 3;int solids=0,shells=0,torus=0;for(TopExp_Explorer x(s,TopAbs_SOLID);x.More();x.Next())solids++;for(TopExp_Explorer x(s,TopAbs_SHELL);x.More();x.Next())shells++;for(TopExp_Explorer x(s,TopAbs_FACE);x.More();x.Next()){auto face=TopoDS::Face(x.Current());if(BRepAdaptor_Surface(face).GetType()==GeomAbs_Torus){torus++;if(face.Orientation()!=TopAbs_REVERSED)return 4;}}if(solids!=1||shells!=2||torus!=1)return 5;auto t=BRepPrimAPI_MakeTorus(gp_Ax2(gp_Pnt(1,-1,0),gp_Dir(0,0,1)),5,2).Shape();auto c=BRepPrimAPI_MakeCylinder(gp_Ax2(gp_Pnt(0,0,-3),gp_Dir(0,0,1)),10,7).Shape();auto ref=BRepAlgoAPI_Cut(c,t).Shape();GProp_GProps v,a,rv,ra;BRepGProp::VolumeProperties(s,v);BRepGProp::SurfaceProperties(s,a);BRepGProp::VolumeProperties(ref,rv);BRepGProp::SurfaceProperties(ref,ra);if(fabs(v.Mass()-rv.Mass())>1e-6||fabs(a.Mass()-ra.Mass())>1e-6||v.CentreOfMass().Distance(rv.CentreOfMass())>1e-7)return 6;return 0;}
