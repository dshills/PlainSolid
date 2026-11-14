// Independent original analytic cavity torus/cylinder Boolean fixtures.
#include <BRepPrimAPI_MakeTorus.hxx>
#include <BRepPrimAPI_MakeCylinder.hxx>
#include <BRepAlgoAPI_Common.hxx>
#include <BRepAlgoAPI_Cut.hxx>
#include <BRepAlgoAPI_Fuse.hxx>
#include <BRepCheck_Analyzer.hxx>
#include <BRepGProp.hxx>
#include <GProp_GProps.hxx>
#include <TopExp_Explorer.hxx>
#include <IntCurvesFace_ShapeIntersector.hxx>
#include <gp_Lin.hxx>
#include <Standard_Failure.hxx>
#include <algorithm>
#include <cmath>
#include <fstream>
#include <iomanip>
#include <iostream>
#include <stdexcept>
#include <vector>
int main(int argc,char**argv){try{
 if(argc!=2)throw std::runtime_error("usage: torus_cylinder_cavity OUT_JSONL");std::ofstream out(argv[1]);out.exceptions(std::ios::failbit|std::ios::badbit);out<<std::setprecision(17);
 for(int reflected=0;reflected<2;reflected++){double lower=reflected?-4:-3,upper=reflected?3:4;auto a=BRepPrimAPI_MakeTorus(5,2).Shape(),b=BRepPrimAPI_MakeCylinder(gp_Ax2(gp_Pnt(0,0,lower),gp_Dir(0,0,1)),8,upper-lower).Shape();
 for(int i=0;i<1;i++){auto shape=BRepAlgoAPI_Cut(b,a).Shape();int solids=0,shells=0;for(TopExp_Explorer x(shape,TopAbs_SOLID);x.More();x.Next())solids++;for(TopExp_Explorer x(shape,TopAbs_SHELL);x.More();x.Next())shells++;if(solids!=1||shells!=2)throw std::runtime_error("separate component count");if(!BRepCheck_Analyzer(shape).IsValid())throw std::runtime_error("invalid Boolean");
 GProp_GProps v,a;BRepGProp::VolumeProperties(shape,v);BRepGProp::SurfaceProperties(shape,a);std::vector<double> roots;
 if(i!=1){IntCurvesFace_ShapeIntersector line;line.Load(shape,1e-10);line.Perform(gp_Lin(gp_Pnt(0,0,0),gp_Dir(1,0,0)),-9,9);if(!line.IsDone())throw std::runtime_error("section");for(int j=1;j<=line.NbPnt();j++)roots.push_back(line.WParameter(j));std::sort(roots.begin(),roots.end());roots.erase(std::unique(roots.begin(),roots.end(),[](double x,double y){return std::abs(x-y)<1e-8;}),roots.end());if(roots.size()!=6u)throw std::runtime_error("section counts");}
 out<<"{\"Operation\":"<<i+1<<",\"Lower\":"<<lower<<",\"Upper\":"<<upper<<",\"Solids\":"<<solids<<",\"Shells\":"<<shells<<",\"Volume\":"<<v.Mass()<<",\"Area\":"<<a.Mass()<<",\"CentroidZ\":"<<(i!=1?v.CentreOfMass().Z():0)<<",\"Parameters\":[";for(size_t j=0;j<roots.size();j++){if(j)out<<',';out<<roots[j];}out<<"]}\n";
 }}return 0;
}catch(const Standard_Failure&e){std::cerr<<e.what()<<'\n';return 1;}catch(const std::exception&e){std::cerr<<e.what()<<'\n';return 1;}}
