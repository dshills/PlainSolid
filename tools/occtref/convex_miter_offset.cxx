#include <BRepBuilderAPI_MakeFace.hxx>
#include <BRepBuilderAPI_MakePolygon.hxx>
#include <BRepCheck_Analyzer.hxx>
#include <BRepGProp.hxx>
#include <BRepOffsetAPI_MakeOffsetShape.hxx>
#include <BRepPrimAPI_MakePrism.hxx>
#include <GProp_GProps.hxx>
#include <TopExp_Explorer.hxx>
#include <gp_Pnt.hxx>
#include <gp_Vec.hxx>

#include <cstdlib>
#include <iomanip>
#include <iostream>
#include <string_view>

int main(int argc, char** argv) {
  if (argc != 3) return 2;
  BRepBuilderAPI_MakePolygon outline;
  outline.Add(gp_Pnt(0, 0, 0));
  outline.Add(gp_Pnt(4, 0, 0));
  if (std::string_view(argv[1]) == "triangle") {
    outline.Add(gp_Pnt(0, 3, 0));
  } else if (std::string_view(argv[1]) == "clipped") {
    outline.Add(gp_Pnt(4, 3.25, 0));
    outline.Add(gp_Pnt(3, 4, 0));
    outline.Add(gp_Pnt(0, 4, 0));
  } else {
    return 2;
  }
  outline.Close();
  const auto face = BRepBuilderAPI_MakeFace(outline.Wire()).Face();
  const auto source = BRepPrimAPI_MakePrism(face, gp_Vec(0, 0, 5)).Shape();
  BRepOffsetAPI_MakeOffsetShape offset;
  offset.PerformByJoin(source, std::strtod(argv[2], nullptr), 1e-9,
                       BRepOffset_Skin, false, false, GeomAbs_Intersection, true);
  if (!offset.IsDone()) return 3;
  const auto& shape = offset.Shape();
  if (shape.IsNull()) return 4;
  auto count = [&](TopAbs_ShapeEnum kind) {
    int n = 0;
    for (TopExp_Explorer it(shape, kind); it.More(); it.Next()) ++n;
    return n;
  };
  GProp_GProps volume, area;
  BRepGProp::VolumeProperties(shape, volume);
  BRepGProp::SurfaceProperties(shape, area);
  std::cout << BRepCheck_Analyzer(shape).IsValid() << ' ' << count(TopAbs_SOLID)
            << ' ' << count(TopAbs_FACE) << ' ' << std::setprecision(17)
            << volume.Mass() << ' ' << area.Mass() << '\n';
  return 0;
}
