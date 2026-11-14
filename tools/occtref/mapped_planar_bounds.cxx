#include <STEPControl_Reader.hxx>
#include <BRepCheck_Analyzer.hxx>
#include <BRepBndLib.hxx>
#include <Bnd_Box.hxx>
#include <iostream>
#include <iomanip>
int main(int argc,char**argv){if(argc!=2)return 2;STEPControl_Reader r;if(r.ReadFile(argv[1])!=IFSelect_RetDone||!r.TransferRoots())return 3;auto s=r.OneShape();if(s.IsNull()||!BRepCheck_Analyzer(s).IsValid())return 4;Bnd_Box b;BRepBndLib::AddOptimal(s,b,false,false);double x,y,z,X,Y,Z;b.Get(x,y,z,X,Y,Z);std::cout<<std::setprecision(17)<<"{\"Min\":["<<x<<","<<y<<","<<z<<"],\"Max\":["<<X<<","<<Y<<","<<Z<<"]}\n";}
