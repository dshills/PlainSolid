#include <BRepAlgoAPI_Common.hxx>
#include <BRepBuilderAPI_MakeFace.hxx>
#include <BRepCheck_Analyzer.hxx>
#include <BRepGProp.hxx>
#include <BRepPrimAPI_MakeBox.hxx>
#include <BRepPrimAPI_MakeHalfSpace.hxx>
#include <GProp_GProps.hxx>
#include <TopExp.hxx>
#include <NCollection_IndexedMap.hxx>
#include <TopTools_ShapeMapHasher.hxx>
#include <TopoDS_Face.hxx>
#include <TopoDS_Solid.hxx>
#include <gp_Pln.hxx>
#include <iomanip>
#include <iostream>

int main() {
  TopoDS_Shape stock = BRepPrimAPI_MakeBox(1, 1, 1).Shape();
  const gp_Pnt inside(.875, .125, .6875);
  const gp_Pln planes[] = {
    gp_Pln(gp_Pnt(0, 0, .375), gp_Dir(-.25, -.25, 1)),
    gp_Pln(gp_Pnt(0, 0, .75), gp_Dir(0, 0, 1)),
    gp_Pln(gp_Pnt(.75, 0, 0), gp_Dir(1, 0, 0))
  };
  for (const auto& plane : planes) {
    const auto face = BRepBuilderAPI_MakeFace(plane).Face();
    const auto tool = BRepPrimAPI_MakeHalfSpace(face, inside).Solid();
    BRepAlgoAPI_Common cut(stock, tool);
    if (!cut.IsDone() || cut.Shape().IsNull() || !BRepCheck_Analyzer(cut.Shape()).IsValid()) return 1;
    stock = cut.Shape();
  }
  NCollection_IndexedMap<TopoDS_Shape, TopTools_ShapeMapHasher> solids, faces, edges, vertices;
  TopExp::MapShapes(stock, TopAbs_SOLID, solids);
  TopExp::MapShapes(stock, TopAbs_FACE, faces);
  TopExp::MapShapes(stock, TopAbs_EDGE, edges);
  TopExp::MapShapes(stock, TopAbs_VERTEX, vertices);
  if (solids.Extent() != 1) return 2;
  GProp_GProps m, a;
  BRepGProp::VolumeProperties(stock, m, 1e-12);
  BRepGProp::SurfaceProperties(stock, a, 1e-12);
  const gp_Pnt c = m.CentreOfMass();
  std::cout << std::setprecision(17) << '[' << m.Mass() << ',' << a.Mass()
            << ',' << c.X() << ',' << c.Y() << ',' << c.Z() << ',' << vertices.Extent()
            << ',' << edges.Extent() << ',' << faces.Extent() << "]\n";
}
