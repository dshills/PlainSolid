#include <BRepPrimAPI_MakeTorus.hxx>
#include <BRepPrimAPI_MakeCylinder.hxx>
#include <BRepAlgoAPI_Cut.hxx>
#include <BRepGProp.hxx>
#include <GProp_GProps.hxx>
#include <iostream>
#include <iomanip>
int main(int argc,char**argv){if(argc!=2)return 1;bool reflected=std::stoi(argv[1]);double lo=reflected?-4:-3,hi=reflected?3:4;gp_Pnt origin(.1,-.4,.375);gp_Dir x(.6,0,-.8),z(.8,0,.6);auto tor=BRepPrimAPI_MakeTorus(gp_Ax2(origin,z,x),5,2).Shape();auto cyl=BRepPrimAPI_MakeCylinder(gp_Ax2(origin.Translated(gp_Vec(z)*lo),z,x),8,hi-lo).Shape();auto shape=BRepAlgoAPI_Cut(cyl,tor).Shape();GProp_GProps m;BRepGProp::VolumeProperties(shape,m);auto c=m.CentreOfMass();auto I=m.MatrixOfInertia();std::cout<<std::setprecision(17)<<m.Mass()<<" "<<c.X()<<" "<<c.Y()<<" "<<c.Z();for(int i=1;i<=3;i++)for(int j=1;j<=3;j++)std::cout<<" "<<I.Value(i,j);}
