// Independent source-transfer classifier. Tolerance belongs only to the oracle.
#include <STEPControl_Reader.hxx>
#include <BRepClass3d_SolidClassifier.hxx>
#include <TopExp_Explorer.hxx>
#include <TopoDS.hxx>
#include <iostream>
#include <cstdlib>
int main(int argc,char**argv){
 if(argc!=5)return 2;STEPControl_Reader r;
 if(r.ReadFile(argv[1])!=IFSelect_RetDone||r.TransferRoots()<1)return 3;
 gp_Pnt p(std::strtod(argv[2],nullptr),std::strtod(argv[3],nullptr),std::strtod(argv[4],nullptr));
 int inside=0,boundary=0,outside=0;
 for(TopExp_Explorer ex(r.OneShape(),TopAbs_SOLID);ex.More();ex.Next()){
 BRepClass3d_SolidClassifier c(ex.Current(),p,1e-8);
 switch(c.State()){case TopAbs_IN:++inside;break;case TopAbs_ON:++boundary;break;case TopAbs_OUT:++outside;break;default:return 4;}}
 std::cout<<"{\"Inside\":"<<inside<<",\"Boundary\":"<<boundary<<",\"Outside\":"<<outside<<"}\n";
}
