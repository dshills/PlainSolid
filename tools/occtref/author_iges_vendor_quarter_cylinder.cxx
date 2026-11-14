// Independent OCCT analytic quarter cylinder, kept without source normalization.
#include <IGESControl_Writer.hxx>
#include <BRepPrimAPI_MakeCylinder.hxx>
int main(int argc,char**argv){if(argc!=2)return 2;auto s=BRepPrimAPI_MakeCylinder(2,3,1.57079632679489661923).Shape();IGESControl_Writer w("MM",1);if(!w.AddShape(s))return 3;return w.Write(argv[1])?0:4;}
