// Pinned OCCT 4dbfdcc: ordinary transfer and adaptive mixed-surface measures.
#include <STEPControl_Reader.hxx>
#include <BRepCheck_Analyzer.hxx>
#include <BRepGProp.hxx>
#include <GProp_GProps.hxx>
#include <BRepBndLib.hxx>
#include <Bnd_Box.hxx>
#include <TopExp_Explorer.hxx>
#include <iostream>
#include <iomanip>
#include <string>
#include <cmath>
int main(int argc,char**argv){
 if(argc!=3||std::string(argv[1])!="read")return 2;
 STEPControl_Reader r;
 if(r.ReadFile(argv[2])!=IFSelect_RetDone||r.TransferRoots()<1)return 5;
 auto s=r.OneShape();if(!BRepCheck_Analyzer(s).IsValid())return 6;
 int faces=0,solids=0;
 for(TopExp_Explorer e(s,TopAbs_SOLID);e.More();e.Next())++solids;
 for(TopExp_Explorer e(s,TopAbs_FACE);e.More();e.Next())++faces;
 GProp_GProps volume,area;
 double ve=BRepGProp::VolumeProperties(s,volume,1e-12,true,false);
 double ae=BRepGProp::SurfaceProperties(s,area,1e-12,false);
 if(!std::isfinite(ve)||!std::isfinite(ae)||!std::isfinite(volume.Mass())||!std::isfinite(area.Mass()))return 7;
 Bnd_Box box;BRepBndLib::AddOptimal(s,box,false,false);double x0,y0,z0,x1,y1,z1;box.Get(x0,y0,z0,x1,y1,z1);
 std::cout<<std::setprecision(17)<<"{\"Area\":"<<area.Mass()<<",\"Volume\":"<<volume.Mass()<<",\"Faces\":"<<faces<<",\"Solids\":"<<solids<<",\"AreaError\":"<<ae<<",\"VolumeError\":"<<ve<<",\"Bounds\":["<<x0<<","<<y0<<","<<z0<<","<<x1<<","<<y1<<","<<z1<<"]}\n";
}
