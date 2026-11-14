#include <BRepPrimAPI_MakeTorus.hxx>
#include <BRepPrimAPI_MakeCylinder.hxx>
#include <BRepAlgoAPI_Common.hxx>
#include <BRepAlgoAPI_Cut.hxx>
#include <BRepAlgoAPI_Fuse.hxx>
#include <BRepCheck_Analyzer.hxx>
#include <BRepAdaptor_Surface.hxx>
#include <BRepExtrema_DistShapeShape.hxx>
#include <BRepBuilderAPI_MakeVertex.hxx>
#include <BRep_Builder.hxx>
#include <TopoDS_Compound.hxx>
#include <TopoDS.hxx>
#include <TopExp_Explorer.hxx>
#include <Standard_Failure.hxx>
#include <fstream>
#include <iomanip>
#include <iostream>
#include <stdexcept>
int main(int argc,char**argv){try{if(argc!=2)throw std::runtime_error("output path");std::ofstream out(argv[1]);out.exceptions(std::ios::failbit|std::ios::badbit);out<<std::setprecision(17);
 auto a=BRepPrimAPI_MakeTorus(5,2).Shape(),b=BRepPrimAPI_MakeCylinder(gp_Ax2(gp_Pnt(0,0,-3),gp_Dir(0,0,1)),6,7).Shape();TopoDS_Shape bodies[3]={BRepAlgoAPI_Common(a,b).Shape(),BRepAlgoAPI_Cut(a,b).Shape(),BRepAlgoAPI_Fuse(a,b).Shape()};
 for(int op=0;op<3;op++){if(!BRepCheck_Analyzer(bodies[op]).IsValid())throw std::runtime_error("invalid");BRep_Builder builder;TopoDS_Compound owners[4],boundary;bool present[4]={false,false,false,false};builder.MakeCompound(boundary);for(auto&x:owners)builder.MakeCompound(x);
 for(TopExp_Explorer f(bodies[op],TopAbs_FACE);f.More();f.Next()){auto face=TopoDS::Face(f.Current());BRepAdaptor_Surface surf(face);int i=surf.GetType()==GeomAbs_Torus?0:surf.GetType()==GeomAbs_Cylinder?1:surf.GetType()==GeomAbs_Plane?(surf.Plane().Location().Z()<0?2:3):-1;if(i<0)throw std::runtime_error("support");builder.Add(owners[i],face);builder.Add(boundary,face);present[i]=true;}
 gp_Pnt points[3]={gp_Pnt(4,1,1.5),gp_Pnt(0,0,0),gp_Pnt(6,0,2)};for(int query=0;query<3;query++){auto p=BRepBuilderAPI_MakeVertex(points[query]).Shape();BRepExtrema_DistShapeShape global(p,boundary,1e-10);if(!global.IsDone())throw std::runtime_error("distance");out<<"{\"Operation\":"<<op<<",\"Query\":"<<query<<",\"SquaredDistance\":"<<global.Value()*global.Value()<<",\"Supports\":[";for(int i=0;i<4;i++){if(i)out<<',';if(!present[i]){out<<-1;continue;}BRepExtrema_DistShapeShape d(p,owners[i],1e-10);if(!d.IsDone())throw std::runtime_error("owner distance");out<<d.Value()*d.Value();}out<<"]}\n";}
 }return 0;}catch(const Standard_Failure&e){std::cerr<<e.what()<<'\n';return 1;}catch(const std::exception&e){std::cerr<<e.what()<<'\n';return 1;}}
