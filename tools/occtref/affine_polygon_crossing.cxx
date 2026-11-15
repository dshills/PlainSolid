// Independent original triangular prisms under a common nonorthogonal map.
#include <BRepBuilderAPI_MakePolygon.hxx>
#include <BRepBuilderAPI_MakeFace.hxx>
#include <BRepPrimAPI_MakePrism.hxx>
#include <BRepAlgoAPI_Common.hxx>
#include <BRepAlgoAPI_Fuse.hxx>
#include <BRepAlgoAPI_Cut.hxx>
#include <BRepCheck_Analyzer.hxx>
#include <BRepGProp.hxx>
#include <GProp_GProps.hxx>
#include <TopExp_Explorer.hxx>
#include <TopoDS_Face.hxx>
#include <iostream>
#include <iomanip>
static gp_Pnt p(double x,double y,double z){return gp_Pnt(x+z/4,y+x/8,z+y/4);}
static TopoDS_Shape prism(bool b){double x=b?.5:0,y=b?.25:0,z=b?.5:0;BRepBuilderAPI_MakePolygon q;q.Add(p(x,y,z));q.Add(p(x+2,y,z));q.Add(p(x,y+2,z));q.Close();auto face=BRepBuilderAPI_MakeFace(q.Wire()).Face();return BRepPrimAPI_MakePrism(face,gp_Vec(.5,0,2)).Shape();}
int main(){auto a=prism(false),b=prism(true);std::cout<<std::setprecision(17)<<"[";bool first=true;for(int order=0;order<2;++order){auto x=order?b:a,y=order?a:b;for(int op=0;op<3;++op){TopoDS_Shape s;if(op==0){BRepAlgoAPI_Common f(x,y);if(!f.IsDone())return 2;s=f.Shape();}else if(op==1){BRepAlgoAPI_Fuse f(x,y);if(!f.IsDone())return 2;s=f.Shape();}else{BRepAlgoAPI_Cut f(x,y);if(!f.IsDone())return 2;s=f.Shape();}GProp_GProps volume;BRepGProp::VolumeProperties(s,volume);int solids=0;for(TopExp_Explorer i(s,TopAbs_SOLID);i.More();i.Next())++solids;if(!first)std::cout<<",";first=false;std::cout<<"{\"Order\":"<<order<<",\"Operation\":"<<op<<",\"Valid\":"<<(BRepCheck_Analyzer(s).IsValid()?"true":"false")<<",\"Solids\":"<<solids<<",\"Volume\":"<<volume.Mass()<<"}";}}std::cout<<"]\n";}
