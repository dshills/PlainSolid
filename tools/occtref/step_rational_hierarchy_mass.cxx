// Independent adaptive OCCT volume/inertia of actual transferred STEP solids.
#include <STEPControl_Reader.hxx>
#include <BRepCheck_Analyzer.hxx>
#include <BRepGProp.hxx>
#include <GProp_GProps.hxx>
#include <TopExp_Explorer.hxx>
#include <gp_Mat.hxx>
#include <iostream>
#include <iomanip>
#include <string>
int main(int argc,char**argv){
 if(argc!=3||std::string(argv[1])!="read")return 2;
 STEPControl_Reader r;if(r.ReadFile(argv[2])!=IFSelect_RetDone||r.TransferRoots()<1)return 3;
 auto s=r.OneShape();if(!BRepCheck_Analyzer(s).IsValid())return 4;
 GProp_GProps p;double err=BRepGProp::VolumeProperties(s,p,1e-12,true,false);
 auto c=p.CentreOfMass();auto I=p.MatrixOfInertia();double v=p.Mass();int solids=0;
 for(TopExp_Explorer ex(s,TopAbs_SOLID);ex.More();ex.Next())++solids;
 std::cout<<std::setprecision(17)<<"{\"Volume\":"<<v<<",\"Solids\":"<<solids<<",\"IntegrationError\":"<<err<<",\"Center\":["<<c.X()<<","<<c.Y()<<","<<c.Z()<<"],\"CentralInertia\":[";
 for(int i=1;i<=3;i++){if(i>1)std::cout<<",";std::cout<<"[";for(int j=1;j<=3;j++){if(j>1)std::cout<<",";std::cout<<I.Value(i,j);}std::cout<<"]";}
 std::cout<<"],\"OriginInertia\":[";
 for(int i=1;i<=3;i++){if(i>1)std::cout<<",";std::cout<<"[";for(int j=1;j<=3;j++){if(j>1)std::cout<<",";double shift=-c.Coord(i)*c.Coord(j);if(i==j)shift+=c.X()*c.X()+c.Y()*c.Y()+c.Z()*c.Z();std::cout<<I.Value(i,j)+v*shift;}std::cout<<"]";}
 std::cout<<"]}\n";
}
