// Independent OCCT polygons/prisms: no native mesh, topology or section recipe.
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
static gp_Pnt p(double x,double y,double z,bool b){return b?gp_Pnt(x-z/8,y+x/8+z/4,z+y/4):gp_Pnt(x+z/4,y+x/8,z+y/4);}
static TopoDS_Face cap(double z,bool b){BRepBuilderAPI_MakePolygon q;if(b){q.Add(p(.5,.5,z,b));q.Add(p(2.75,.5,z,b));q.Add(p(.5,2.75,z,b));}else{q.Add(p(0,0,z,b));q.Add(p(2,0,z,b));q.Add(p(3,1,z,b));q.Add(p(2,2,z,b));q.Add(p(0,2,z,b));}q.Close();return BRepBuilderAPI_MakeFace(q.Wire()).Face();}
int main(){auto a=BRepPrimAPI_MakePrism(cap(-2,false),gp_Vec(.5,0,2)).Shape();auto b=BRepPrimAPI_MakePrism(cap(0,true),gp_Vec(-.25,.5,2)).Shape();BRepAlgoAPI_Common material(a,b),interface(cap(0,false),cap(0,true));BRepAlgoAPI_Fuse fuse(a,b);if(!material.IsDone()||!interface.IsDone()||!fuse.IsDone())return 2;GProp_GProps v,zero,area;BRepGProp::VolumeProperties(fuse.Shape(),v);BRepGProp::VolumeProperties(material.Shape(),zero);BRepGProp::SurfaceProperties(interface.Shape(),area);int solids=0;for(TopExp_Explorer i(fuse.Shape(),TopAbs_SOLID);i.More();i.Next())++solids;std::cout<<std::setprecision(17)<<"{\"Valid\":"<<(BRepCheck_Analyzer(fuse.Shape()).IsValid()?"true":"false")<<",\"Solids\":"<<solids<<",\"Volume\":"<<v.Mass()<<",\"CommonVolume\":"<<zero.Mass()<<",\"CapAreaSquared\":"<<area.Mass()*area.Mass()<<"}\n";}
