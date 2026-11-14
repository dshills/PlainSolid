#include <BRepPrimAPI_MakeTorus.hxx>
#include <BRepPrimAPI_MakeCylinder.hxx>
#include <BRepAlgoAPI_Cut.hxx>
#include <BRepBuilderAPI_Transform.hxx>
#include <IntCurvesFace_ShapeIntersector.hxx>
#include <gp_Trsf.hxx>
#include <gp_Lin.hxx>
#include <algorithm>
#include <cmath>
#include <iostream>
#include <iomanip>
#include <vector>
int main(int argc,char**argv){if(argc!=9&&argc!=10)return 1;auto tor=BRepPrimAPI_MakeTorus(gp_Ax2(gp_Pnt(1,0,0),gp_Dir(.8,0,.6),gp_Dir(.6,0,-.8)),3,1).Shape();auto cyl=BRepPrimAPI_MakeCylinder(gp_Ax2(gp_Pnt(0,0,-6),gp_Dir(0,0,1)),8,12).Shape();auto body=BRepAlgoAPI_Cut(cyl,tor).Shape();if(argc==10){if(std::string(argv[9])!="oblique")return 2;gp_Trsf pose;pose.SetValues(.6,0,.8,.1,0,1,0,-.4,-.8,0,.6,.375);body=BRepBuilderAPI_Transform(body,pose,true).Shape();}gp_Pnt p(std::stod(argv[1]),std::stod(argv[2]),std::stod(argv[3]));gp_Vec v(std::stod(argv[4]),std::stod(argv[5]),std::stod(argv[6]));double length=v.Magnitude();if(length==0)return 3;IntCurvesFace_ShapeIntersector line;line.Load(body,1e-10);line.Perform(gp_Lin(p,gp_Dir(v)),std::stod(argv[7])*length,std::stod(argv[8])*length);if(!line.IsDone())return 4;std::vector<double> roots;for(int i=1;i<=line.NbPnt();i++)roots.push_back(line.WParameter(i)/length);std::sort(roots.begin(),roots.end());roots.erase(std::unique(roots.begin(),roots.end(),[](double a,double b){return std::abs(a-b)<1e-8;}),roots.end());for(double t:roots)std::cout<<std::setprecision(17)<<t<<" ";}
