#include <STEPControl_Reader.hxx>
#include <BRepCheck_Analyzer.hxx>
#include <BRepGProp.hxx>
#include <GProp_GProps.hxx>
#include <TopExp_Explorer.hxx>
#include <TopoDS.hxx>
#include <BRepAdaptor_Surface.hxx>
#include <cmath>
#include <iostream>
int main(int argc,char**argv){if(argc!=2)return 1;STEPControl_Reader r;if(r.ReadFile(argv[1])!=IFSelect_RetDone||r.TransferRoots()!=1)return 2;auto s=r.OneShape();if(!BRepCheck_Analyzer(s).IsValid())return 3;int solids=0,shells=0,torus=0;for(TopExp_Explorer x(s,TopAbs_SOLID);x.More();x.Next())solids++;for(TopExp_Explorer x(s,TopAbs_SHELL);x.More();x.Next())shells++;for(TopExp_Explorer x(s,TopAbs_FACE);x.More();x.Next()){auto f=TopoDS::Face(x.Current());if(BRepAdaptor_Surface(f).GetType()==GeomAbs_Torus){torus++;if(f.Orientation()!=TopAbs_REVERSED)return 4;}}
GProp_GProps v,a;BRepGProp::VolumeProperties(s,v);BRepGProp::SurfaceProperties(s,a);double pi=acos(-1.);std::cerr<<solids<<" "<<shells<<" "<<torus<<" "<<v.Mass()<<" "<<a.Mass()<<"\n";if(solids!=1||shells!=2||torus!=1||fabs(v.Mass()-(448*pi-40*pi*pi))>1e-6||fabs(a.Mass()-(240*pi+40*pi*pi))>1e-6)return 5;std::cout<<"cavity verified\n";return 0;}
