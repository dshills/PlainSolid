// Independent planar geometry baseline; mapped graph is authored separately.
#include <STEPControl_Writer.hxx>
#include <BRepPrimAPI_MakeBox.hxx>
int main(int argc,char**argv){if(argc!=2)return 2;STEPControl_Writer w;if(w.Transfer(BRepPrimAPI_MakeBox(2,3,4).Shape(),STEPControl_AsIs)!=IFSelect_RetDone)return 3;return w.Write(argv[1])==IFSelect_RetDone?0:4;}
