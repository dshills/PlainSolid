#include <BRepPrimAPI_MakeTorus.hxx>
#include <BRepPrimAPI_MakeCylinder.hxx>
#include <BRepAlgoAPI_Cut.hxx>
#include <BRepBuilderAPI_Transform.hxx>
#include <BRepGProp.hxx>
#include <BRepBndLib.hxx>
#include <Bnd_Box.hxx>
#include <GProp_GProps.hxx>
#include <gp_Trsf.hxx>
#include <iostream>
#include <iomanip>
int main(){auto tor=BRepPrimAPI_MakeTorus(gp_Ax2(gp_Pnt(1,0,0),gp_Dir(.8,0,.6),gp_Dir(.6,0,-.8)),3,1).Shape();auto cyl=BRepPrimAPI_MakeCylinder(gp_Ax2(gp_Pnt(0,0,-6),gp_Dir(0,0,1)),8,12).Shape();auto source=BRepAlgoAPI_Cut(cyl,tor).Shape();gp_Trsf pose;pose.SetValues(.36,-.8,.48,.1,.48,.6,.64,-.4,-.8,0,.6,.375);auto body=BRepBuilderAPI_Transform(source,pose,true).Shape();GProp_GProps v,a;BRepGProp::VolumeProperties(body,v);BRepGProp::SurfaceProperties(body,a);Bnd_Box box;BRepBndLib::AddOptimal(body,box,false,false);double x,y,z,X,Y,Z;box.Get(x,y,z,X,Y,Z);std::cout<<std::setprecision(17)<<v.Mass()<<" "<<a.Mass()<<" "<<x<<" "<<y<<" "<<z<<" "<<X<<" "<<Y<<" "<<Z;}
