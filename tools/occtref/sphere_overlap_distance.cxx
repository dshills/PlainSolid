// Independent pinned OCCT overlapping sphere Boolean nearest reference.
#include <BRepPrimAPI_MakeSphere.hxx>
#include <BRepAlgoAPI_Fuse.hxx>
#include <BRepAlgoAPI_Cut.hxx>
#include <BRepAlgoAPI_Common.hxx>
#include <BRepExtrema_DistShapeShape.hxx>
#include <BRepBuilderAPI_MakeVertex.hxx>
#include <BRepCheck_Analyzer.hxx>
#include <BRepGProp.hxx>
#include <GProp_GProps.hxx>
#include <BRep_Builder.hxx>
#include <TopoDS_Compound.hxx>
#include <TopExp_Explorer.hxx>
#include <Standard_Failure.hxx>
#include <fstream>
#include <iostream>
#include <iomanip>
#include <stdexcept>
#include <string>
static void nearest(std::ostream& out,const char* name,const TopoDS_Shape& boundary,gp_Pnt p){
 BRepExtrema_DistShapeShape d(BRepBuilderAPI_MakeVertex(p).Shape(),boundary,1e-10);
 if(!d.IsDone()||d.NbSolution()<1)throw std::runtime_error("sphere boundary distance failed");
 auto w=d.PointOnShape2(1);out<<std::setprecision(17)<<"{\"Name\":\""<<name<<"\",\"SquaredDistance\":"<<d.Value()*d.Value()<<",\"Point\":["<<w.X()<<','<<w.Y()<<','<<w.Z()<<"],\"Solutions\":"<<d.NbSolution()<<"}\n";
}
int main(int argc,char**argv){try{
 if(argc!=2&&argc!=3)throw std::runtime_error("usage: sphere_overlap_distance OUTPUT_JSONL [--algebraic|--axis|--transverse]");
 bool algebraic=argc==3&&std::string(argv[2])=="--algebraic";
 bool axis=argc==3&&std::string(argv[2])=="--axis";
 bool transverse=argc==3&&std::string(argv[2])=="--transverse";
 if(argc==3&&!algebraic&&!axis&&!transverse)throw std::runtime_error("unknown mode");
 std::ofstream out(argv[1]);out.exceptions(std::ios::failbit|std::ios::badbit);
 auto a=BRepPrimAPI_MakeSphere(transverse?gp_Pnt(0,0,0):gp_Pnt(-3,0,0),5).Shape();
 auto b=BRepPrimAPI_MakeSphere(transverse?gp_Pnt(2,2,1):gp_Pnt(3,0,0),5).Shape();
 TopoDS_Shape bodies[3]={BRepAlgoAPI_Fuse(a,b).Shape(),BRepAlgoAPI_Common(a,b).Shape(),BRepAlgoAPI_Cut(a,b).Shape()};
 const char* names[3]={"unionRim","commonRim","cutRim"};
 gp_Pnt points[3]={gp_Pnt(0,2.25,0),gp_Pnt(0,7.2,0),gp_Pnt(3,4.5,0)};
 if(algebraic){points[0]=gp_Pnt(0,2,1);points[1]=gp_Pnt(0,7,1);points[2]=gp_Pnt(3,4,1);}
 if(transverse){points[0]=gp_Pnt(2,0,.5);points[1]=gp_Pnt(5,-3,.5);points[2]=gp_Pnt(6,-2,1);}
 for(int i=0;i<3;++i){
 if(!BRepCheck_Analyzer(bodies[i]).IsValid())throw std::runtime_error("invalid overlapping Boolean");
 int solids=0;for(TopExp_Explorer x(bodies[i],TopAbs_SOLID);x.More();x.Next())++solids;
 if(solids!=1)throw std::runtime_error("expected one solid");
 BRep_Builder builder;TopoDS_Compound boundary;builder.MakeCompound(boundary);
 for(TopExp_Explorer f(bodies[i],TopAbs_FACE);f.More();f.Next())builder.Add(boundary,f.Current());
 if(axis){
 if(i==0){nearest(out,"unionCapA",boundary,gp_Pnt(-3,0,0));nearest(out,"unionCapB",boundary,gp_Pnt(3,0,0));nearest(out,"unionCircle",boundary,gp_Pnt(0,0,0));nearest(out,"unionPole",boundary,gp_Pnt(-10,0,0));}
 if(i==1){nearest(out,"commonPoles",boundary,gp_Pnt(0,0,0));nearest(out,"commonCenter",boundary,gp_Pnt(-3,0,0));}
 if(i==2){nearest(out,"cutCap",boundary,gp_Pnt(3,0,0));nearest(out,"cutCenter",boundary,gp_Pnt(-3,0,0));}
 continue;
 }
 nearest(out,names[i],boundary,points[i]);
 if(i==0)nearest(out,"unionFaces",boundary,transverse?gp_Pnt(5,-3,.5):(algebraic?gp_Pnt(0,7,1):gp_Pnt(0,7.2,0)));
 if(i==1)nearest(out,"commonFaces",boundary,transverse?gp_Pnt(2,0,.5):(algebraic?gp_Pnt(0,2,1):gp_Pnt(0,2.25,0)));
 if(i==2)nearest(out,"cutFace",boundary,transverse?gp_Pnt(2,0,.5):(algebraic?gp_Pnt(0,2,1):gp_Pnt(0,2.25,0)));
 }
 if(axis||transverse){
 auto u=BRepAlgoAPI_Fuse(BRepPrimAPI_MakeSphere(gp_Pnt(-1,0,0),2).Shape(),BRepPrimAPI_MakeSphere(gp_Pnt(1,0,0),2).Shape()).Shape();
 if(!BRepCheck_Analyzer(u).IsValid())throw std::runtime_error("invalid irrational circle union");
 BRep_Builder builder;TopoDS_Compound boundary;builder.MakeCompound(boundary);for(TopExp_Explorer f(u,TopAbs_FACE);f.More();f.Next())builder.Add(boundary,f.Current());
 nearest(out,transverse?"irrationalCircleFoot":"irrationalCircle",boundary,transverse?gp_Pnt(0,1,1):gp_Pnt(0,0,0));
 }
 return 0;
}catch(const Standard_Failure&e){std::cerr<<e.what()<<'\n';return 1;}catch(const std::exception&e){std::cerr<<e.what()<<'\n';return 1;}}
