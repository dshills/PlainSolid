// Standalone pinned OCCT separated-sphere union oracle; no PlainSolid input or mesh.
#include <BRepPrimAPI_MakeSphere.hxx>
#include <BRepAlgoAPI_Fuse.hxx>
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
int main(int argc,char**argv){try{
 if(argc!=2)throw std::runtime_error("usage: sphere_union_distance OUTPUT_JSONL");
 auto outer=BRepPrimAPI_MakeSphere(gp_Pnt(0,0,0),2).Shape();
 auto inner=BRepPrimAPI_MakeSphere(gp_Pnt(6,0,0),1).Shape();
 auto body=BRepAlgoAPI_Fuse(outer,inner).Shape();
 if(!BRepCheck_Analyzer(body).IsValid())throw std::runtime_error("invalid sphere union");
 int shells=0,solids=0;for(TopExp_Explorer x(body,TopAbs_SHELL);x.More();x.Next())++shells;for(TopExp_Explorer x(body,TopAbs_SOLID);x.More();x.Next())++solids;
 GProp_GProps volume,area;BRepGProp::VolumeProperties(body,volume,1e-10);BRepGProp::SurfaceProperties(body,area,1e-10);
 std::ofstream out(argv[1]);out.exceptions(std::ios::failbit|std::ios::badbit);
 out<<std::setprecision(17)<<"{\"Name\":\"union\",\"Volume\":"<<volume.Mass()<<",\"Area\":"<<area.Mass()<<",\"Shells\":"<<shells<<",\"Solids\":"<<solids<<"}\n";
 // A face compound avoids treating interior material points as distance zero.
 BRep_Builder builder;TopoDS_Compound boundary;builder.MakeCompound(boundary);
 for(TopExp_Explorer f(body,TopAbs_FACE);f.More();f.Next())builder.Add(boundary,f.Current());
 nearest(out,"outer",boundary,gp_Pnt(3.5,.5,0));
 nearest(out,"inner",boundary,gp_Pnt(4,.5,0));
 nearest(out,"tie",boundary,gp_Pnt(3.5,0,0));
 nearest(out,"center",boundary,gp_Pnt(6,0,0));
 auto tangent=BRepAlgoAPI_Fuse(BRepPrimAPI_MakeSphere(gp_Pnt(0,0,0),2).Shape(),BRepPrimAPI_MakeSphere(gp_Pnt(4,0,0),2).Shape()).Shape();
 TopoDS_Compound tb;builder.MakeCompound(tb);for(TopExp_Explorer f(tangent,TopAbs_FACE);f.More();f.Next())builder.Add(tb,f.Current());
 nearest(out,"contact",tb,gp_Pnt(2,0,0));
 nearest(out,"tangentTie",tb,gp_Pnt(2,1,0));
 return 0;
}catch(const Standard_Failure&e){std::cerr<<e.what()<<'\n';return 1;}catch(const std::exception&e){std::cerr<<e.what()<<'\n';return 1;}}
