// Independently authored inclined planar caps and unequal extrusion directions.
#include <BRepBuilderAPI_MakePolygon.hxx>
#include <BRepBuilderAPI_MakeFace.hxx>
#include <BRepPrimAPI_MakePrism.hxx>
#include <BRepAlgoAPI_Common.hxx>
#include <BRepAlgoAPI_Fuse.hxx>
#include <BRepCheck_Analyzer.hxx>
#include <BRepGProp.hxx>
#include <GProp_GProps.hxx>
#include <TopExp_Explorer.hxx>
#include <TopoDS_Face.hxx>
#include <iostream>
#include <iomanip>
#include <string>
static bool diamond=false;
static gp_Pnt p(double u,double v,double w,bool second){if(second){if(diamond){double x=.5+.5*u-.5*v,y=-.25+.5*u+.5*v;u=x;v=y;}else{u+=.5;v+=.25;}return gp_Pnt(2*u-.25*w,2*v+.5*w,u+v+1.5*w);}return gp_Pnt(2*u+.5*w,2*v,u+v+2*w);}
static TopoDS_Face face(double w,bool second){BRepBuilderAPI_MakePolygon b;b.Add(p(0,0,w,second));b.Add(p(1,0,w,second));b.Add(p(1,1,w,second));b.Add(p(0,1,w,second));b.Close();return BRepBuilderAPI_MakeFace(b.Wire()).Face();}
int main(int argc,char**argv){if(argc>2)return 2;if(argc==2){if(std::string(argv[1])!="diamond")return 2;diamond=true;}auto a=BRepPrimAPI_MakePrism(face(-1,false),gp_Vec(.5,0,2)).Shape();auto b=BRepPrimAPI_MakePrism(face(0,true),gp_Vec(-.25,.5,1.5)).Shape();BRepAlgoAPI_Fuse fuse(a,b);BRepAlgoAPI_Common common(a,b),cap(face(0,false),face(0,true));if(!fuse.IsDone()||!common.IsDone()||!cap.IsDone())return 2;GProp_GProps volume,zero,area;BRepGProp::VolumeProperties(fuse.Shape(),volume);BRepGProp::VolumeProperties(common.Shape(),zero);BRepGProp::SurfaceProperties(cap.Shape(),area);int solids=0;for(TopExp_Explorer i(fuse.Shape(),TopAbs_SOLID);i.More();i.Next())++solids;std::cout<<std::setprecision(17)<<"{\"Valid\":"<<(BRepCheck_Analyzer(fuse.Shape()).IsValid()?"true":"false")<<",\"Solids\":"<<solids<<",\"Volume\":"<<volume.Mass()<<",\"CommonVolume\":"<<zero.Mass()<<",\"CapAreaSquared\":"<<area.Mass()*area.Mass()<<"}\n";}
