// Independent IGES154 solid source construction and conventional material transfer.
#include <IGESControl_Writer.hxx>
#include <IGESControl_Reader.hxx>
#include <IGESSolid_Cylinder.hxx>
#include <BRepCheck_Analyzer.hxx>
#include <BRepGProp.hxx>
#include <GProp_GProps.hxx>
#include <TopExp_Explorer.hxx>
#include <Standard_Failure.hxx>
#include <iostream>
#include <iomanip>
#include <string>
int main(int argc,char**argv){if(argc!=3)return 2;try{
 std::string mode=argv[1];if(mode=="write"||mode=="write-negative"){
  occ::handle<IGESSolid_Cylinder> s=new IGESSolid_Cylinder;
  s->Init(3.,2.,gp_XYZ(2,3,4),gp_XYZ(mode=="write"?1:-1,0,0));
  IGESControl_Writer w("MM",0);if(!w.AddEntity(s))return 3;return w.Write(argv[2])?0:4;
 }
 IGESControl_Reader r;if(r.ReadFile(argv[2])!=IFSelect_RetDone)return 5;r.TransferRoots();auto s=r.OneShape();
 if(s.IsNull()){std::cerr<<"IGES154: no shape transferred\n";return 6;}if(!BRepCheck_Analyzer(s).IsValid())return 7;
 GProp_GProps v,a;BRepGProp::VolumeProperties(s,v);BRepGProp::SurfaceProperties(s,a);
 int faces=0,solids=0;for(TopExp_Explorer x(s,TopAbs_FACE);x.More();x.Next())++faces;for(TopExp_Explorer x(s,TopAbs_SOLID);x.More();x.Next())++solids;
 std::cout<<std::setprecision(17)<<"{\"Faces\":"<<faces<<",\"Solids\":"<<solids<<",\"Volume\":"<<v.Mass()<<",\"Area\":"<<a.Mass()<<",\"Center\":["<<v.CentreOfMass().X()<<","<<v.CentreOfMass().Y()<<","<<v.CentreOfMass().Z()<<"]}\n";return 0;
}catch(const Standard_Failure&e){std::cerr<<e.what();return 9;}}
