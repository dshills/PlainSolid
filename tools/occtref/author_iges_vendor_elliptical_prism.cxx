#include <IGESControl_Writer.hxx>
#include <BRepBuilderAPI_MakeEdge.hxx>
#include <BRepBuilderAPI_MakeWire.hxx>
#include <BRepBuilderAPI_MakeFace.hxx>
#include <BRepPrimAPI_MakePrism.hxx>
#include <TopoDS_Wire.hxx>
#include <TopoDS_Face.hxx>
#include <gp_Elips.hxx>
#include <gp_Pln.hxx>
int main(int argc,char**argv){if(argc!=2)return 2;gp_Elips e(gp_Ax2(gp_Pnt(0,0,3),gp_Dir(0,0,1),gp_Dir(1,0,0)),5,2);auto edge=BRepBuilderAPI_MakeEdge(e).Edge();auto wire=BRepBuilderAPI_MakeWire(edge).Wire();auto disk=BRepBuilderAPI_MakeFace(gp_Pln(e.Position()),wire,true).Face();auto solid=BRepPrimAPI_MakePrism(disk,gp_Vec(0,0,4),false,true).Shape();IGESControl_Writer w("MM",1);if(!w.AddShape(solid))return 3;return w.Write(argv[1])?0:4;}
