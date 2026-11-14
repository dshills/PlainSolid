#include <IGESControl_Writer.hxx>
#include <BRepPrimAPI_MakeSphere.hxx>
int main(int argc,char**argv){if(argc!=2)return 2;IGESControl_Writer w("MM",1);w.AddShape(BRepPrimAPI_MakeSphere(2).Shape());return w.Write(argv[1])?0:1;}
