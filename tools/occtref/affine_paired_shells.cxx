// Independent analytic OCCT source solids; no PlainSolid tessellation/import.
#include <BRepPrimAPI_MakeBox.hxx>
#include <BRepPrimAPI_MakePrism.hxx>
#include <BRepBuilderAPI_MakePolygon.hxx>
#include <BRepBuilderAPI_MakeFace.hxx>
#include <BRepAlgoAPI_Common.hxx>
#include <BRepAlgoAPI_Fuse.hxx>
#include <BRepAlgoAPI_Cut.hxx>
#include <BRepCheck_Analyzer.hxx>
#include <BRepGProp.hxx>
#include <GProp_GProps.hxx>
#include <TopExp_Explorer.hxx>
#include <iostream>
#include <iomanip>

static gp_Pnt point(double s,double t,double w) {return gp_Pnt(s+w/4,t+s/2-w/8,s+w);}
int main(){std::cout<<std::setprecision(17);for(int regular=0;regular<2;++regular){
 TopoDS_Shape a=BRepPrimAPI_MakeBox(gp_Pnt(-2,-1,-4),4,2,4).Shape();
 BRepBuilderAPI_MakePolygon p;p.Add(point(-1,regular?0:-1,-2));p.Add(point(1,regular?0:-1,-2));p.Add(point(1,1,-2));p.Add(point(-1,1,-2));p.Close();
 TopoDS_Shape b=BRepPrimAPI_MakePrism(BRepBuilderAPI_MakeFace(p.Wire()).Face(),gp_Vec(.5,-.25,2)).Shape();
 for(int order=0;order<2;++order){auto x=order?b:a,y=order?a:b;for(int op=0;op<3;++op){TopoDS_Shape shape;if(op==0){BRepAlgoAPI_Common c(x,y);if(!c.IsDone())return 2;shape=c.Shape();}else if(op==1){BRepAlgoAPI_Fuse c(x,y);if(!c.IsDone())return 3;shape=c.Shape();}else{BRepAlgoAPI_Cut c(x,y);if(!c.IsDone())return 4;shape=c.Shape();}
 GProp_GProps mass;BRepGProp::VolumeProperties(shape,mass);int solids=0;for(TopExp_Explorer it(shape,TopAbs_SOLID);it.More();it.Next())++solids;

 std::cout<<"{\"Regular\":"<<regular<<",\"Order\":"<<order<<",\"Operation\":"<<op<<",\"Valid\":"<<(BRepCheck_Analyzer(shape).IsValid()?"true":"false")<<",\"Solids\":"<<solids<<",\"Volume\":"<<mass.Mass()<<"}\n";
 }}}
}
