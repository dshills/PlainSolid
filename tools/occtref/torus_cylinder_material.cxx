// Independent original analytic torus/cylinder Boolean and oblique sections.
#include <BRepPrimAPI_MakeTorus.hxx>
#include <BRepPrimAPI_MakeCylinder.hxx>
#include <BRepAlgoAPI_Common.hxx>
#include <BRepAlgoAPI_Cut.hxx>
#include <BRepCheck_Analyzer.hxx>
#include <IntCurvesFace_ShapeIntersector.hxx>
#include <BRepClass3d_SolidClassifier.hxx>
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
int main(int argc,char**argv){try{
 if(argc!=2)throw std::runtime_error("usage: torus_cylinder_material OUTPUT_JSONL");std::ofstream out(argv[1]);out.exceptions(std::ios::failbit|std::ios::badbit);out<<std::setprecision(17);
 auto a=BRepPrimAPI_MakeTorus(5,2).Shape(),b=BRepPrimAPI_MakeCylinder(gp_Ax2(gp_Pnt(0,0,-3),gp_Dir(0,0,1)),6,7).Shape();TopoDS_Shape bodies[2]={BRepAlgoAPI_Common(a,b).Shape(),BRepAlgoAPI_Cut(a,b).Shape()};
 for(int i=0;i<2;i++){if(!BRepCheck_Analyzer(bodies[i]).IsValid())throw std::runtime_error("invalid Boolean");int solids=0;for(TopExp_Explorer x(bodies[i],TopAbs_SOLID);x.More();x.Next())solids++;if(solids!=1)throw std::runtime_error("solid count");
 for(int sign:{1,-1}){IntCurvesFace_ShapeIntersector q;q.Load(bodies[i],1e-10);double speed=std::sqrt(1.01);q.Perform(gp_Lin(gp_Pnt(0,0,0),gp_Dir(sign,0,sign*.1)),-8*speed,8*speed);if(!q.IsDone())throw std::runtime_error("intersection failed");std::vector<double> ts;for(int k=1;k<=q.NbPnt();k++)ts.push_back(q.WParameter(k)/speed);std::sort(ts.begin(),ts.end());ts.erase(std::unique(ts.begin(),ts.end(),[](double x,double y){return std::abs(x-y)<1e-8;}),ts.end());if(ts.size()!=4)throw std::runtime_error("four crossings expected");for(int j:{0,2}){double t=(ts[j]+ts[j+1])/2;BRepClass3d_SolidClassifier c(bodies[i],gp_Pnt(sign*t,0,sign*t/10),1e-10);if(c.State()!=TopAbs_IN)throw std::runtime_error("interval not inside");}
 out<<"{\"Difference\":"<<(i?"true":"false")<<",\"Sign\":"<<sign<<",\"Parameters\":["<<ts[0]<<','<<ts[1]<<','<<ts[2]<<','<<ts[3]<<"]}\n";
 }}return 0;
}catch(const Standard_Failure&e){std::cerr<<e.what()<<'\n';return 1;}catch(const std::exception&e){std::cerr<<e.what()<<'\n';return 1;}}
