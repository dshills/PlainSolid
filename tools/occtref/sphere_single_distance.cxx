// Independent pinned OCCT reduced full-sphere Boolean reference.
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
 if(argc!=2)throw std::runtime_error("usage: sphere_single_distance OUTPUT_JSONL");
 std::ofstream out(argv[1]);out.exceptions(std::ios::failbit|std::ios::badbit);
 struct Case{const char*name;double ra,rb,bx;int op;gp_Pnt query;bool empty;};
 Case cases[]={
 {"strictUnion",5,1,1,0,gp_Pnt(1,1,0),false},
 {"internalUnion",5,1,4,0,gp_Pnt(5,0,0),false},
 {"internalCommon",5,1,4,1,gp_Pnt(4,0,0),false},
 {"identicalUnion",5,5,0,0,gp_Pnt(1,1,0),false},
 {"identicalCommon",5,5,0,1,gp_Pnt(0,0,0),false},
 {"externalCut",2,2,4,2,gp_Pnt(2,0,0),false},
 {"disjointCut",2,1,5,2,gp_Pnt(0,0,0),false},
 {"emptyCommon",2,2,4,1,gp_Pnt(0,0,0),true},
 {"emptyCut",5,5,0,2,gp_Pnt(0,0,0),true}};
 for(const auto& c:cases){
 auto a=BRepPrimAPI_MakeSphere(gp_Pnt(0,0,0),c.ra).Shape(),b=BRepPrimAPI_MakeSphere(gp_Pnt(c.bx,0,0),c.rb).Shape();
 TopoDS_Shape body;if(c.op==0)body=BRepAlgoAPI_Fuse(a,b).Shape();else if(c.op==1)body=BRepAlgoAPI_Common(a,b).Shape();else body=BRepAlgoAPI_Cut(a,b).Shape();
 if(!BRepCheck_Analyzer(body).IsValid())throw std::runtime_error("invalid reduced Boolean");
 int solids=0;for(TopExp_Explorer x(body,TopAbs_SOLID);x.More();x.Next())++solids;
 if(c.empty){if(solids!=0)throw std::runtime_error("expected empty material");out<<"{\"Name\":\""<<c.name<<"\",\"Solids\":0}\n";continue;}
 if(solids!=1)throw std::runtime_error("expected single sphere material");
 BRep_Builder builder;TopoDS_Compound boundary;builder.MakeCompound(boundary);for(TopExp_Explorer f(body,TopAbs_FACE);f.More();f.Next())builder.Add(boundary,f.Current());
 nearest(out,c.name,boundary,c.query);
 }
 return 0;
}catch(const Standard_Failure&e){std::cerr<<e.what()<<'\n';return 1;}catch(const std::exception&e){std::cerr<<e.what()<<'\n';return 1;}}
