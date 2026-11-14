#include <BRepPrimAPI_MakeTorus.hxx>
#include <BRepPrimAPI_MakeCylinder.hxx>
#include <BRepAlgoAPI_Cut.hxx>
#include <BRepGProp.hxx>
#include <GProp_GProps.hxx>
#include <BRepBndLib.hxx>
#include <Bnd_Box.hxx>
#include <cmath>
#include <iostream>
#include <iomanip>
int main(int argc,char**argv){if(argc!=2)return 1;bool reflected=std::stoi(argv[1]);double lo=reflected?-4:-3,hi=reflected?3:4;gp_Pnt origin(.1,-.4,.375);gp_Dir x(.6,0,-.8),z(.8,0,.6);auto tor=BRepPrimAPI_MakeTorus(gp_Ax2(origin,z,x),5,2).Shape();auto cyl=BRepPrimAPI_MakeCylinder(gp_Ax2(origin.Translated(gp_Vec(z)*lo),z,x),8,hi-lo).Shape();auto shape=BRepAlgoAPI_Cut(cyl,tor).Shape();Bnd_Box box;BRepBndLib::AddOptimal(shape,box,false,false);double xmin,ymin,zmin,xmax,ymax,zmax;box.Get(xmin,ymin,zmin,xmax,ymax,zmax);double expectedMin[3]={.1+.8*lo-4.8,-8.4,.375+.6*lo-6.4},expectedMax[3]={.1+.8*hi+4.8,7.6,.375+.6*hi+6.4};double actualMin[3]={xmin,ymin,zmin},actualMax[3]={xmax,ymax,zmax};for(int i=0;i<3;i++)if(fabs(actualMin[i]-expectedMin[i])>1e-6||fabs(actualMax[i]-expectedMax[i])>1e-6)return 3;GProp_GProps m;BRepGProp::VolumeProperties(shape,m);auto c=m.CentreOfMass();auto I=m.MatrixOfInertia();std::cout<<std::setprecision(17)<<m.Mass()<<" "<<c.X()<<" "<<c.Y()<<" "<<c.Z();for(int i=1;i<=3;i++)for(int j=1;j<=3;j++)std::cout<<" "<<I.Value(i,j);}
