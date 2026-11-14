#include <BRepPrimAPI_MakeTorus.hxx>
#include <BRepPrimAPI_MakeCylinder.hxx>
#include <BRepAlgoAPI_Cut.hxx>
#include <BRepCheck_Analyzer.hxx>
#include <BRepClass3d_SolidClassifier.hxx>
#include <BRepGProp.hxx>
#include <GProp_GProps.hxx>
#include <TopExp_Explorer.hxx>
#include <IntCurvesFace_ShapeIntersector.hxx>
#include <gp_Lin.hxx>
#include <algorithm>
#include <vector>
#include <cmath>
#include <iostream>
#include <iomanip>
int main(){auto tor=BRepPrimAPI_MakeTorus(gp_Ax2(gp_Pnt(1,0,0),gp_Dir(.8,0,.6),gp_Dir(.6,0,-.8)),3,1).Shape();auto cyl=BRepPrimAPI_MakeCylinder(gp_Ax2(gp_Pnt(0,0,-6),gp_Dir(0,0,1)),8,12).Shape();auto body=BRepAlgoAPI_Cut(cyl,tor).Shape();if(!BRepCheck_Analyzer(body).IsValid())return 1;int ns=0,nh=0;for(TopExp_Explorer x(body,TopAbs_SOLID);x.More();x.Next())ns++;for(TopExp_Explorer x(body,TopAbs_SHELL);x.More();x.Next())nh++;if(ns!=1||nh!=2)return 2;IntCurvesFace_ShapeIntersector line;line.Load(body,1e-10);line.Perform(gp_Lin(gp_Pnt(1,0,0),gp_Dir(.6,0,-.8)),-11,11);if(!line.IsDone())return 3;std::vector<double> roots;for(int j=1;j<=line.NbPnt();j++)roots.push_back(line.WParameter(j));std::sort(roots.begin(),roots.end());roots.erase(std::unique(roots.begin(),roots.end(),[](double a,double b){return fabs(a-b)<1e-8;}),roots.end());double expected[]={-7.5,-4,-2,2,4,7.5};if(roots.size()!=6)return 4;for(int i=0;i<6;i++)if(fabs(roots[i]-expected[i])>1e-7)return 5;for(int i=0;i<3;i++){double t=i==0?0:i==1?3:4;BRepClass3d_SolidClassifier classifier(body,gp_Pnt(1+.6*t,0,-.8*t),1e-9);TopAbs_State want=i==0?TopAbs_IN:i==1?TopAbs_OUT:TopAbs_ON;if(classifier.State()!=want)return 6;}GProp_GProps v,a;BRepGProp::VolumeProperties(body,v);BRepGProp::SurfaceProperties(body,a);auto p=v.CentreOfMass();std::cout<<std::setprecision(17)<<v.Mass()<<" "<<a.Mass()<<" "<<p.X()<<" "<<p.Y()<<" "<<p.Z();}
