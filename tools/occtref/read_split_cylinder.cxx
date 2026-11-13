// Independent model-only STEP transfer/validity and mass qualification.
#include <STEPControl_Reader.hxx>
#include <StepData_StepModel.hxx>
#include <StepShape_ManifoldSolidBrep.hxx>
#include <BRepCheck_Analyzer.hxx>
#include <BRepGProp.hxx>
#include <GProp_GProps.hxx>
#include <TopExp_Explorer.hxx>
#include <iostream>
#include <iomanip>
#include <string>
int main(int argc,char**argv){if(argc!=2&&argc!=3)return 2;STEPControl_Reader r;if(r.ReadFile(argv[1])!=IFSelect_RetDone)return 3;bool transferred=false;if(argc==3){if(std::string(argv[2])!="roots"||r.TransferRoots()!=1)return 7;transferred=true;}else{auto model=r.StepModel();for(int i=1;i<=model->NbEntities();++i){auto body=occ::down_cast<StepShape_ManifoldSolidBrep>(model->Entity(i));if(!body.IsNull()){if(transferred||!r.TransferEntity(body))return 4;transferred=true;}}}if(!transferred)return 4;auto s=r.OneShape();if(s.IsNull()||!BRepCheck_Analyzer(s).IsValid())return 5;GProp_GProps v,a;BRepGProp::VolumeProperties(s,v);BRepGProp::SurfaceProperties(s,a);int n=0;for(TopExp_Explorer x(s,TopAbs_FACE);x.More();x.Next())++n;std::cout<<std::setprecision(17)<<"{\"Faces\":"<<n<<",\"Volume\":"<<v.Mass()<<",\"Area\":"<<a.Mass()<<",\"Center\":["<<v.CentreOfMass().X()<<","<<v.CentreOfMass().Y()<<","<<v.CentreOfMass().Z()<<"]}\n";return n==4?0:6;}
