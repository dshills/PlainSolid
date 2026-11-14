// Independent placed analytic spheres: no PlainSolid shape/mesh input.
#include <BRepPrimAPI_MakeSphere.hxx>
#include <BRepAlgoAPI_Fuse.hxx>
#include <BRepAlgoAPI_Common.hxx>
#include <BRepAlgoAPI_Cut.hxx>
#include <BRepCheck_Analyzer.hxx>
#include <BRepExtrema_DistShapeShape.hxx>
#include <BRepBuilderAPI_MakeVertex.hxx>
#include <BRepClass3d_SolidClassifier.hxx>
#include <IntCurvesFace_ShapeIntersector.hxx>
#include <BRep_Builder.hxx>
#include <TopoDS_Compound.hxx>
#include <TopExp_Explorer.hxx>
#include <gp_Lin.hxx>
#include <Standard_Failure.hxx>
#include <algorithm>
#include <cmath>
#include <fstream>
#include <iomanip>
#include <iostream>
#include <stdexcept>
#include <vector>
static gp_Pnt world(double x,double y,double z){return gp_Pnt(3*x/13-12*y/13+4*z/13+1./7,36*x/65+5*y/13+48*z/65-2./9,-4*x/5+3*z/5+3./11);}
int main(int argc,char**argv){try{
 if(argc!=2)throw std::runtime_error("usage: sphere_pair_occurrence OUTPUT_JSONL");
 std::ofstream out(argv[1]);out.exceptions(std::ios::failbit|std::ios::badbit);out<<std::setprecision(17);
 auto a=BRepPrimAPI_MakeSphere(world(-3,0,0),5).Shape(),b=BRepPrimAPI_MakeSphere(world(3,0,0),5).Shape();
 TopoDS_Shape bodies[3]={BRepAlgoAPI_Fuse(a,b).Shape(),BRepAlgoAPI_Common(a,b).Shape(),BRepAlgoAPI_Cut(a,b).Shape()};
 const char*names[3]={"union","common","cut"};gp_Pnt qs[3]={world(0,2,1),world(0,7,1),world(3,4,1)};
 for(int i=0;i<3;i++){
 if(!BRepCheck_Analyzer(bodies[i]).IsValid())throw std::runtime_error("invalid Boolean");int solids=0;for(TopExp_Explorer x(bodies[i],TopAbs_SOLID);x.More();x.Next())solids++;if(solids!=1)throw std::runtime_error("solid count");
 BRep_Builder builder;TopoDS_Compound boundary;builder.MakeCompound(boundary);for(TopExp_Explorer x(bodies[i],TopAbs_FACE);x.More();x.Next())builder.Add(boundary,x.Current());
 BRepExtrema_DistShapeShape nearest(BRepBuilderAPI_MakeVertex(qs[i]).Shape(),boundary,1e-10);if(!nearest.IsDone()||nearest.NbSolution()<1)throw std::runtime_error("nearest failed");auto foot=nearest.PointOnShape2(1);
 for(int sign:{1,-1})for(int bound:{20,9}){
 gp_Lin line(world(0,0,0),gp_Dir(sign*3./13,sign*36./65,-sign*4./5));
 IntCurvesFace_ShapeIntersector q;q.Load(bodies[i],1e-10);q.Perform(line,-bound*2./3,bound*2./3);if(!q.IsDone())throw std::runtime_error("intersection failed");
 std::vector<double> ts;for(int k=1;k<=q.NbPnt();k++)ts.push_back(q.WParameter(k)*1.5);
 for(int end:{-bound,bound}){BRepClass3d_SolidClassifier c(bodies[i],world(sign*end*2./3,0,0),1e-10);if(c.State()==TopAbs_IN)ts.push_back(end);}
 std::sort(ts.begin(),ts.end());ts.erase(std::unique(ts.begin(),ts.end(),[](double x,double y){return std::abs(x-y)<1e-8;}),ts.end());if(ts.size()!=2)throw std::runtime_error("section endpoints");
 BRepClass3d_SolidClassifier mid(bodies[i],world(sign*(ts[0]+ts[1])/3,0,0),1e-10);if(mid.State()!=TopAbs_IN)throw std::runtime_error("midpoint outside");
 out<<"{\"Name\":\""<<names[i]<<"\",\"Sign\":"<<sign<<",\"Bound\":"<<bound<<",\"Parameters\":["<<ts[0]<<','<<ts[1]<<"],\"SquaredDistance\":"<<nearest.Value()*nearest.Value()<<",\"Point\":["<<foot.X()<<','<<foot.Y()<<','<<foot.Z()<<"],\"Solids\":"<<solids<<"}\n";
 }}return 0;
}catch(const Standard_Failure&e){std::cerr<<e.what()<<'\n';return 1;}catch(const std::exception&e){std::cerr<<e.what()<<'\n';return 1;}}
