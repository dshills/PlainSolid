// Independent original OCCT spherical Booleans and finite line intersections.
#include <BRepPrimAPI_MakeSphere.hxx>
#include <BRepAlgoAPI_Fuse.hxx>
#include <BRepAlgoAPI_Common.hxx>
#include <BRepAlgoAPI_Cut.hxx>
#include <BRepCheck_Analyzer.hxx>
#include <IntCurvesFace_ShapeIntersector.hxx>
#include <BRepClass3d_SolidClassifier.hxx>
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
 if(argc!=2)throw std::runtime_error("usage: sphere_pair_rational_line OUTPUT_JSONL");
 std::ofstream out(argv[1]);out.exceptions(std::ios::failbit|std::ios::badbit);out<<std::setprecision(17);
 auto a=BRepPrimAPI_MakeSphere(gp_Pnt(0,0,0),5).Shape(),b=BRepPrimAPI_MakeSphere(gp_Pnt(2,2,1),5).Shape();
 TopoDS_Shape bodies[3]={BRepAlgoAPI_Fuse(a,b).Shape(),BRepAlgoAPI_Common(a,b).Shape(),BRepAlgoAPI_Cut(a,b).Shape()};
 const char*names[3]={"union","common","cut"};
 for(int i=0;i<3;++i){if(!BRepCheck_Analyzer(bodies[i]).IsValid())throw std::runtime_error("invalid Boolean");
 for(int sign:{1,-1}){
 gp_Pnt origin(1./3,1./3,1./6);gp_Lin line(origin,gp_Dir(2*sign,2*sign,sign));
 IntCurvesFace_ShapeIntersector query;query.Load(bodies[i],1e-10);query.Perform(line,-10,10);if(!query.IsDone())throw std::runtime_error("line intersection failed");
 std::vector<double> ts;for(int k=1;k<=query.NbPnt();++k)ts.push_back(query.WParameter(k)*3); // original raw direction length1/3
 std::sort(ts.begin(),ts.end());ts.erase(std::unique(ts.begin(),ts.end(),[](double x,double y){return std::abs(x-y)<1e-8;}),ts.end());
 if(ts.size()!=2)throw std::runtime_error("expected two retained crossings");
 double mid=(ts[0]+ts[1])/2;gp_Pnt p(origin.X()+sign*2*mid/9,origin.Y()+sign*2*mid/9,origin.Z()+sign*mid/9);
 BRepClass3d_SolidClassifier classifier(bodies[i],p,1e-10);if(classifier.State()!=TopAbs_IN)throw std::runtime_error("material interval not inside");
 out<<"{\"Name\":\""<<names[i]<<(sign==1?"Forward":"Reverse")<<"\",\"Parameters\":["<<ts[0]<<','<<ts[1]<<"],\"MidInside\":true}\n";
 }}return 0;
}catch(const Standard_Failure&e){std::cerr<<e.what()<<'\n';return 1;}catch(const std::exception&e){std::cerr<<e.what()<<'\n';return 1;}}
