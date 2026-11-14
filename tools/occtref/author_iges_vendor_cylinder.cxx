// Independent native OCCT analytic cylinder. Its type186 output has rounded
// seam coordinates and an alternate transformed-circle graph; retained negative
// evidence for the exact canonical Go cylinder reader.
#include <IGESControl_Writer.hxx>
#include <BRepPrimAPI_MakeCylinder.hxx>
int main(int argc,char**argv){if(argc!=2)return 2;IGESControl_Writer w("MM",1);if(!w.AddShape(BRepPrimAPI_MakeCylinder(2,3).Shape()))return 3;return w.Write(argv[1])?0:4;}
