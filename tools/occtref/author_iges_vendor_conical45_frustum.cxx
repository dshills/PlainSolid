// Independent OCCT analytic frustum, kept without source normalization.
#include <IGESControl_Writer.hxx>
#include <BRepPrimAPI_MakeCone.hxx>
int main(int argc,char**argv){if(argc!=2)return 2;auto s=BRepPrimAPI_MakeCone(1,3,2).Shape();IGESControl_Writer w("MM",1);if(!w.AddShape(s))return 3;return w.Write(argv[1])?0:4;}
