// Independent full-support boundary distance, not a manifold tangent cavity.
#include <BRepPrimAPI_MakeSphere.hxx>
#include <BRepAlgoAPI_Cut.hxx>
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
static void nearest(std::ostream& out,const char* name,const TopoDS_Shape& boundary,gp_Pnt p){
 BRepExtrema_DistShapeShape d(BRepBuilderAPI_MakeVertex(p).Shape(),boundary,1e-10);
 if(!d.IsDone()||d.NbSolution()<1)throw std::runtime_error("sphere boundary distance failed");
 auto w=d.PointOnShape2(1);out<<std::setprecision(17)<<"{\"Name\":\""<<name<<"\",\"SquaredDistance\":"<<d.Value()*d.Value()<<",\"Point\":["<<w.X()<<','<<w.Y()<<','<<w.Z()<<"],\"Solutions\":"<<d.NbSolution()<<"}\n";
}
static TopoDS_Compound supports(gp_Pnt a,double ra,gp_Pnt b,double rb){
 BRep_Builder builder;TopoDS_Compound boundary;builder.MakeCompound(boundary);
 auto outer=BRepPrimAPI_MakeSphere(a,ra).Shape(),inner=BRepPrimAPI_MakeSphere(b,rb).Shape();
 for(TopExp_Explorer f(outer,TopAbs_FACE);f.More();f.Next())builder.Add(boundary,f.Current());
 for(TopExp_Explorer f(inner,TopAbs_FACE);f.More();f.Next())builder.Add(boundary,f.Current());
 return boundary;
}
int main(int argc,char**argv){try{
 if(argc!=2)throw std::runtime_error("usage: sphere_touching_cavity_distance OUTPUT_JSONL");
 std::ofstream out(argv[1]);out.exceptions(std::ios::failbit|std::ios::badbit);
 // Both full original surfaces are the retained analytical boundary. No Cut
 // or valid manifold solid is asserted for their singular internal contact.
 auto boundary=supports(gp_Pnt(0,0,0),5,gp_Pnt(4,0,0),1);
 nearest(out,"sharedInside",boundary,gp_Pnt(4.5,0,0));
 nearest(out,"contact",boundary,gp_Pnt(5,0,0));
 nearest(out,"sharedOutside",boundary,gp_Pnt(6,0,0));
 nearest(out,"innerCenter",boundary,gp_Pnt(4,0,0));
 nearest(out,"distinctTie",boundary,gp_Pnt(20./7,15./7,0));
 nearest(out,"innerFoot",boundary,gp_Pnt(4,1,0));
 nearest(out,"outerCenter",boundary,gp_Pnt(0,0,0));
 auto oblique=supports(gp_Pnt(2,-1,3),6,gp_Pnt(5,3,3),1);
 nearest(out,"obliqueContact",oblique,gp_Pnt(53./10,17./5,3));
 return 0;
}catch(const Standard_Failure&e){std::cerr<<e.what()<<'\n';return 1;}catch(const std::exception&e){std::cerr<<e.what()<<'\n';return 1;}}
