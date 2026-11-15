#include <BRepBuilderAPI_MakePolygon.hxx>
#include <BRepBuilderAPI_MakeFace.hxx>
#include <BRepPrimAPI_MakePrism.hxx>
#include <BRepFilletAPI_MakeFillet.hxx>
#include <BRepGProp.hxx>
#include <BRepCheck_Analyzer.hxx>
#include <GProp_GProps.hxx>
#include <STEPControl_Writer.hxx>
#include <STEPControl_Reader.hxx>
#include <Interface_Static.hxx>
#include <TopExp_Explorer.hxx>
#include <TopoDS.hxx>
#include <iostream>
#include <iomanip>
#include <string>
int main(int argc,char**argv){if(argc!=3)return 2;TopoDS_Shape shape;
if(std::string(argv[1])=="write"){
 BRepBuilderAPI_MakePolygon p;p.Add(gp_Pnt(0,0,0));p.Add(gp_Pnt(4,0,0));p.Add(gp_Pnt(0,3,0));p.Close();auto stock=BRepPrimAPI_MakePrism(BRepBuilderAPI_MakeFace(p.Wire()).Face(),gp_Vec(0,0,5)).Shape();TopoDS_Edge target;
 for(TopExp_Explorer e(stock,TopAbs_EDGE);e.More();e.Next()){GProp_GProps g;BRepGProp::LinearProperties(e.Current(),g);if(g.CentreOfMass().Distance(gp_Pnt(4,0,2.5))<1e-12)target=TopoDS::Edge(e.Current());}
 if(target.IsNull())return 3;BRepFilletAPI_MakeFillet fillet(stock);fillet.Add(5.0/16,target);fillet.Build();if(!fillet.IsDone())return 4;shape=fillet.Shape();STEPControl_Writer w;Interface_Static::SetCVal("write.step.schema","AP214IS");Interface_Static::SetIVal("write.surfacecurve.mode",0);if(w.Transfer(shape,STEPControl_AsIs)!=IFSelect_RetDone||w.Write(argv[2])!=IFSelect_RetDone)return 5;
}else if(std::string(argv[1])=="read"){STEPControl_Reader r;if(r.ReadFile(argv[2])!=IFSelect_RetDone||r.TransferRoots()==0)return 6;shape=r.OneShape();}else return 2;
if(!BRepCheck_Analyzer(shape).IsValid())return 7;GProp_GProps v,a;BRepGProp::VolumeProperties(shape,v,1e-12);BRepGProp::SurfaceProperties(shape,a,1e-12);int faces=0,solids=0;for(TopExp_Explorer e(shape,TopAbs_FACE);e.More();e.Next())faces++;for(TopExp_Explorer e(shape,TopAbs_SOLID);e.More();e.Next())solids++;
std::cout<<std::setprecision(17)<<"{\"Volume\":"<<v.Mass()<<",\"Area\":"<<a.Mass()<<",\"Faces\":"<<faces<<",\"Solids\":"<<solids<<"}\n";
}
