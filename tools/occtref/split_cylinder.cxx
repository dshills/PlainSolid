#include <BRepPrimAPI_MakeCylinder.hxx>
#include <ShapeUpgrade_ShapeDivideClosed.hxx>
#include <STEPControl_Writer.hxx>
#include <BRepCheck_Analyzer.hxx>
#include <BRepGProp.hxx>
#include <GProp_GProps.hxx>
#include <TopExp_Explorer.hxx>
#include <iostream>
int main(int argc,char**argv){if(argc!=2)return 2;auto s=BRepPrimAPI_MakeCylinder(2.,3.).Shape();ShapeUpgrade_ShapeDivideClosed split(s);split.SetNbSplitPoints(1);split.Perform();s=split.Result();if(!BRepCheck_Analyzer(s).IsValid())return 3;GProp_GProps v,a;BRepGProp::VolumeProperties(s,v);BRepGProp::SurfaceProperties(s,a);int n=0;for(TopExp_Explorer x(s,TopAbs_FACE);x.More();x.Next())++n;std::cout<<"faces="<<n<<" volume="<<v.Mass()<<" area="<<a.Mass()<<"\n";STEPControl_Writer w;w.Transfer(s,STEPControl_AsIs);return w.Write(argv[1])==IFSelect_RetDone?0:4;}
