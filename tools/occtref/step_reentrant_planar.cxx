#include <BRepBuilderAPI_MakePolygon.hxx>
#include <BRepBuilderAPI_MakeFace.hxx>
#include <BRepPrimAPI_MakePrism.hxx>
#include <STEPControl_Writer.hxx>
#include <Interface_Static.hxx>
#include <TopoDS_Wire.hxx>
int main(int argc,char**argv){if(argc!=2)return 2;BRepBuilderAPI_MakePolygon wire;for(auto p:{gp_Pnt(-10,-4,-2),gp_Pnt(-6,-4,-2),gp_Pnt(-6,-3,-2),gp_Pnt(-9,-3,-2),gp_Pnt(-9,0,-2),gp_Pnt(-10,0,-2)})wire.Add(p);wire.Close();auto face=BRepBuilderAPI_MakeFace(wire.Wire()).Face();auto solid=BRepPrimAPI_MakePrism(face,gp_Vec(0,0,4)).Shape();STEPControl_Writer writer;Interface_Static::SetCVal("write.step.schema","AP214IS");if(writer.Transfer(solid,STEPControl_AsIs)!=IFSelect_RetDone)return 3;return writer.Write(argv[1])==IFSelect_RetDone?0:4;}
