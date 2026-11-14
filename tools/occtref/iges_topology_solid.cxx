// Independent conventional IGES topology material transfer and mass.
#include <IGESControl_Writer.hxx>
#include <IGESControl_Reader.hxx>
#include <BRepCheck_Analyzer.hxx>
#include <BRepGProp.hxx>
#include <GProp_GProps.hxx>
#include <TopExp_Explorer.hxx>
#include <Standard_Failure.hxx>
#include <iostream>
#include <iomanip>
#include <string>
int main(int argc,char**argv){if(argc!=2)return 2;try{
  IGESControl_Reader r;if(r.ReadFile(argv[1])!=IFSelect_RetDone)return 5;r.TransferRoots();auto s=r.OneShape();
 if(s.IsNull()||!BRepCheck_Analyzer(s).IsValid())return 6;
 GProp_GProps v,a;BRepGProp::VolumeProperties(s,v);BRepGProp::SurfaceProperties(s,a);
 int faces=0,solids=0;for(TopExp_Explorer x(s,TopAbs_FACE);x.More();x.Next())++faces;for(TopExp_Explorer x(s,TopAbs_SOLID);x.More();x.Next())++solids;
 std::cout<<std::setprecision(17)<<"{\"Faces\":"<<faces<<",\"Solids\":"<<solids<<",\"Volume\":"<<v.Mass()<<",\"Area\":"<<a.Mass()<<",\"Center\":["<<v.CentreOfMass().X()<<","<<v.CentreOfMass().Y()<<","<<v.CentreOfMass().Z()<<"]}\n";return 0;
}catch(const Standard_Failure&e){std::cerr<<e.what();return 9;}}
