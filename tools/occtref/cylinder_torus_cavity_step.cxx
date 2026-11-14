#include <STEPControl_Reader.hxx>
#include <BRepCheck_Analyzer.hxx>
#include <BRepGProp.hxx>
#include <GProp_GProps.hxx>
#include <TopExp_Explorer.hxx>
#include <TopoDS.hxx>
#include <BRepAdaptor_Surface.hxx>
#include <BRepPrimAPI_MakeTorus.hxx>
#include <BRepPrimAPI_MakeCylinder.hxx>
#include <BRepAlgoAPI_Cut.hxx>
#include <IntCurvesFace_ShapeIntersector.hxx>
#include <gp_Lin.hxx>
#include <algorithm>
#include <vector>
#include <cmath>
#include <iostream>
int main(int argc,char**argv){if(argc!=2&&argc!=7&&argc!=8)return 1;double R=5,radius=2,c=8,lo=-3,hi=4;if(argc>=7){R=std::stod(argv[2]);radius=std::stod(argv[3]);c=std::stod(argv[4]);lo=std::stod(argv[5]);hi=std::stod(argv[6]);}STEPControl_Reader r;if(r.ReadFile(argv[1])!=IFSelect_RetDone||r.TransferRoots()!=1)return 2;auto s=r.OneShape();if(!BRepCheck_Analyzer(s).IsValid())return 3;int solids=0,shells=0,torus=0;for(TopExp_Explorer x(s,TopAbs_SOLID);x.More();x.Next())solids++;for(TopExp_Explorer x(s,TopAbs_SHELL);x.More();x.Next())shells++;for(TopExp_Explorer x(s,TopAbs_FACE);x.More();x.Next()){auto f=TopoDS::Face(x.Current());if(BRepAdaptor_Surface(f).GetType()==GeomAbs_Torus){torus++;if(f.Orientation()!=TopAbs_REVERSED)return 4;}}
GProp_GProps v,a;BRepGProp::VolumeProperties(s,v);BRepGProp::SurfaceProperties(s,a);double pi=acos(-1.);std::cerr<<solids<<" "<<shells<<" "<<torus<<" "<<v.Mass()<<" "<<a.Mass()<<"\n";if(solids!=1||shells!=2||torus!=1||fabs(v.Mass()-(c*c*(hi-lo)*pi-2*R*radius*radius*pi*pi))>1e-6||fabs(a.Mass()-((2*c*(hi-lo)+2*c*c)*pi+4*R*radius*pi*pi))>1e-6)return 5;if(argc==8){
 gp_Pnt origin(.1,-.4,.375);gp_Dir x(.6,0,-.8),z(.8,0,.6);gp_Ax2 frame(origin,z,x);
 auto tor=BRepPrimAPI_MakeTorus(frame,R,radius).Shape();gp_Pnt base=origin.Translated(gp_Vec(z)*lo);auto cyl=BRepPrimAPI_MakeCylinder(gp_Ax2(base,z,x),c,hi-lo).Shape();auto reference=BRepAlgoAPI_Cut(cyl,tor).Shape();if(!BRepCheck_Analyzer(reference).IsValid())return 6;
 GProp_GProps rv,ra;BRepGProp::VolumeProperties(reference,rv);BRepGProp::SurfaceProperties(reference,ra);if(fabs(v.Mass()-rv.Mass())>1e-6||fabs(a.Mass()-ra.Mass())>1e-6||v.CentreOfMass().Distance(rv.CentreOfMass())>1e-7)return 7;
 for(auto shape:{s,reference}){IntCurvesFace_ShapeIntersector hit;hit.Load(shape,1e-10);hit.Perform(gp_Lin(origin,x),-9,9);if(!hit.IsDone())return 8;std::vector<double> roots;for(int j=1;j<=hit.NbPnt();j++)roots.push_back(hit.WParameter(j));std::sort(roots.begin(),roots.end());roots.erase(std::unique(roots.begin(),roots.end(),[](double a,double b){return fabs(a-b)<1e-8;}),roots.end());double expected[6]={-8,-7,-3,3,7,8};if(roots.size()!=6)return 9;for(int j=0;j<6;j++)if(fabs(roots[j]-expected[j])>1e-7)return 10;}
 }
 std::cout<<"cavity verified\n";return 0;}
