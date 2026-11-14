#include <STEPControl_Writer.hxx>
#include <BRepPrimAPI_MakeBox.hxx>
int main(int argc,char**argv){if(argc!=3)return 2;for(int i=0;i<2;i++){STEPControl_Writer w;auto s=i==0?BRepPrimAPI_MakeBox(2,3,4).Shape():BRepPrimAPI_MakeBox(1,2,3).Shape();if(w.Transfer(s,STEPControl_AsIs)!=IFSelect_RetDone||w.Write(argv[i+1])!=IFSelect_RetDone)return 3;}return 0;}
