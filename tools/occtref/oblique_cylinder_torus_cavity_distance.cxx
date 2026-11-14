#include <TopExp_Explorer.hxx>
#include <BRepBuilderAPI_Transform.hxx>
#include <gp_Trsf.hxx>

#include <BRepPrimAPI_MakeTorus.hxx>
#include <BRepPrimAPI_MakeCylinder.hxx>
#include <BRepAlgoAPI_Cut.hxx>
#include <BRepBuilderAPI_MakeVertex.hxx>
#include <BRepExtrema_DistShapeShape.hxx>
#include <iostream>
#include <iomanip>
int main(int argc,char**argv){if(argc!=4&&argc!=5)return 1;auto tor=BRepPrimAPI_MakeTorus(gp_Ax2(gp_Pnt(1,0,0),gp_Dir(.8,0,.6),gp_Dir(.6,0,-.8)),3,1).Shape();auto cyl=BRepPrimAPI_MakeCylinder(gp_Ax2(gp_Pnt(0,0,-6),gp_Dir(0,0,1)),8,12).Shape();auto body=BRepAlgoAPI_Cut(cyl,tor).Shape();if(argc==5){if(std::string(argv[4])!="oblique")return 3;gp_Trsf pose;pose.SetValues(.6,0,.8,.1,0,1,0,-.4,-.8,0,.6,.375);body=BRepBuilderAPI_Transform(body,pose,true).Shape();}auto point=BRepBuilderAPI_MakeVertex(gp_Pnt(std::stod(argv[1]),std::stod(argv[2]),std::stod(argv[3]))).Shape();double best=1e100;for(TopExp_Explorer x(body,TopAbs_FACE);x.More();x.Next()){BRepExtrema_DistShapeShape d(point,x.Current());if(!d.IsDone())return 2;best=std::min(best,d.Value());}std::cout<<std::setprecision(17)<<best*best;}
