// Independently authored pentagonal stock and oblique tool; optional full width.
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
#include <TopoDS.hxx>
#include <vector>
#include <BRep_Tool.hxx>
#include <iostream>
#include <iomanip>
#include <string>
static gp_Pnt tool(double s,double t,double w){return gp_Pnt(s+w/4,t+s/2-w/8,s+w);}
static TopoDS_Shape stock(){BRepBuilderAPI_MakePolygon p;p.Add(gp_Pnt(-2,-1,-4));p.Add(gp_Pnt(2,-1,-4));p.Add(gp_Pnt(2,1,-4));p.Add(gp_Pnt(-1.75,1,-4));p.Add(gp_Pnt(-2,.75,-4));p.Close();return BRepPrimAPI_MakePrism(BRepBuilderAPI_MakeFace(p.Wire()).Face(),gp_Vec(0,0,4)).Shape();}
static TopoDS_Shape cutter(bool full){double lo=full?-1:0;BRepBuilderAPI_MakePolygon p;p.Add(tool(-1,lo,-2));p.Add(tool(1,lo,-2));p.Add(tool(1,1,-2));p.Add(tool(-1,1,-2));p.Close();return BRepPrimAPI_MakePrism(BRepBuilderAPI_MakeFace(p.Wire()).Face(),gp_Vec(.5,-.25,2)).Shape();}
int main(int argc,char**argv){if(argc>2)return 2;bool full=argc==2;if(full&&std::string(argv[1])!="full")return 2;auto a=stock(),b=cutter(full);std::cout<<std::setprecision(17)<<"[";bool first=true;for(int order=0;order<2;++order){auto x=order?b:a,y=order?a:b;for(int op=0;op<3;++op){TopoDS_Shape s;if(op==0){BRepAlgoAPI_Common f(x,y);if(!f.IsDone())return 2;s=f.Shape();}else if(op==1){BRepAlgoAPI_Fuse f(x,y);if(!f.IsDone())return 2;s=f.Shape();}else{BRepAlgoAPI_Cut f(x,y);if(!f.IsDone())return 2;s=f.Shape();}GProp_GProps v;BRepGProp::VolumeProperties(s,v);int solids=0;for(TopExp_Explorer i(s,TopAbs_SOLID);i.More();i.Next())++solids;if(!first)std::cout<<",";first=false;std::cout<<"{\"Order\":"<<order<<",\"Operation\":"<<op<<",\"Valid\":"<<(BRepCheck_Analyzer(s).IsValid()?"true":"false")<<",\"Solids\":"<<solids<<",\"Volume\":"<<v.Mass()<<",\"ComponentVolumes\":[";bool initial=true;for(TopExp_Explorer i(s,TopAbs_SOLID);i.More();i.Next()){GProp_GProps component;BRepGProp::VolumeProperties(i.Current(),component);if(!initial)std::cout<<",";initial=false;std::cout<<component.Mass();}std::cout<<"],\"PinchVertices\":";std::vector<TopoDS_Shape> vertices;for(TopExp_Explorer vi(s,TopAbs_VERTEX);vi.More();vi.Next()){bool found=false;for(const auto& prior:vertices)if(prior.IsSame(vi.Current())){found=true;break;}if(!found)vertices.push_back(vi.Current());}int pinch=0;for(const auto& vertex:vertices){auto point=BRep_Tool::Pnt(TopoDS::Vertex(vertex));if(point.Distance(gp_Pnt(0,-1,0))<1e-10)++pinch;}std::cout<<pinch<<"}";}}std::cout<<"]\n";}
