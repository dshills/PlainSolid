#include <BRepAlgoAPI_Cut.hxx>
#include <BRepBuilderAPI_MakeFace.hxx>
#include <BRepBuilderAPI_MakePolygon.hxx>
#include <BRepCheck_Analyzer.hxx>
#include <BRepFilletAPI_MakeChamfer.hxx>
#include <BRepFilletAPI_MakeFillet.hxx>
#include <BRepGProp.hxx>
#include <BRepOffsetAPI_MakeOffsetShape.hxx>
#include <BRepOffsetAPI_MakeThickSolid.hxx>
#include <BRepPrimAPI_MakePrism.hxx>
#include <GProp_GProps.hxx>
#include <TopExp_Explorer.hxx>
#include <TopoDS.hxx>
#include <TopoDS_Shape.hxx>
#include <gp_Pnt.hxx>
#include <gp_Vec.hxx>

#include <cmath>
#include <cstdlib>
#include <iomanip>
#include <iostream>
#include <string_view>

int main(int argc, char** argv) {
  if (argc != 3) return 2;
  BRepBuilderAPI_MakePolygon outline;
  outline.Add(gp_Pnt(0, 0, 0));
  outline.Add(gp_Pnt(4, 0, 0));
  const std::string_view operation = argv[1];
  if (operation == "triangle" || operation == "triangle-open-top" ||
      operation == "triangle-open-both" || operation == "triangle-closed" ||
      operation == "triangle-chamfer" || operation == "triangle-fillet" ||
      operation == "triangle-fillet-both") {
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
  const double distance = std::strtod(argv[2], nullptr);
  TopoDS_Shape shape;
  if (operation == "triangle-chamfer" || operation == "triangle-fillet" ||
      operation == "triangle-fillet-both") {
    TopoDS_Edge target;
    TopoDS_Face side;
    for (TopExp_Explorer it(source, TopAbs_FACE); it.More(); it.Next()) {
      GProp_GProps surface;
      BRepGProp::SurfaceProperties(it.Current(), surface);
      if (std::abs(surface.CentreOfMass().X() - 2) > 1e-9 ||
          std::abs(surface.CentreOfMass().Y() - 1.5) > 1e-9) continue;
      for (TopExp_Explorer edge(it.Current(), TopAbs_EDGE); edge.More(); edge.Next()) {
        GProp_GProps line;
        BRepGProp::LinearProperties(edge.Current(), line);
        if (std::abs(line.CentreOfMass().X() - 4) > 1e-9) continue;
        target = TopoDS::Edge(edge.Current());
        side = TopoDS::Face(it.Current());
      }
    }
    if (target.IsNull() || side.IsNull()) return 5;
    if (operation == "triangle-chamfer") {
      BRepFilletAPI_MakeChamfer chamfer(source);
      chamfer.Add(.625, distance, target, side);
      chamfer.Build();
      if (!chamfer.IsDone()) return 3;
      shape = chamfer.Shape();
    } else {
      BRepFilletAPI_MakeFillet fillet(source);
      fillet.Add(distance, target);
      if (operation == "triangle-fillet-both") {
        TopoDS_Edge other;
        for (TopExp_Explorer it(source, TopAbs_EDGE); it.More(); it.Next()) {
          GProp_GProps line;
          BRepGProp::LinearProperties(it.Current(), line);
          const gp_Pnt midpoint = line.CentreOfMass();
          if (midpoint.Distance(gp_Pnt(0, 0, 2.5)) < 1e-9) other = TopoDS::Edge(it.Current());
        }
        if (other.IsNull()) return 5;
        fillet.Add(.25, other);
      }
      fillet.Build();
      if (!fillet.IsDone()) return 3;
      shape = fillet.Shape();
    }
  } else if (operation == "triangle-open-top" || operation == "triangle-open-both") {
    NCollection_List<TopoDS_Shape> opening;
    for (TopExp_Explorer it(source, TopAbs_FACE); it.More(); it.Next()) {
      GProp_GProps properties;
      BRepGProp::SurfaceProperties(it.Current(), properties);
      const double z = properties.CentreOfMass().Z();
      if (z > 4.99 || (operation == "triangle-open-both" && z < .01)) opening.Append(it.Current());
    }
    if (opening.Size() != (operation == "triangle-open-top" ? 1 : 2)) return 5;
    BRepOffsetAPI_MakeThickSolid thick;
    thick.MakeThickSolidByJoin(source, opening, distance, 1e-9, BRepOffset_Skin,
                              false, false, GeomAbs_Intersection, true);
    if (!thick.IsDone()) return 3;
    shape = thick.Shape();
  } else if (operation == "triangle-closed") {
    if (distance != -.5) return 2;
    BRepBuilderAPI_MakePolygon inner;
    inner.Add(gp_Pnt(.5, .5, .5));
    inner.Add(gp_Pnt(2.5, .5, .5));
    inner.Add(gp_Pnt(.5, 2, .5));
    inner.Close();
    const auto cavity = BRepPrimAPI_MakePrism(BRepBuilderAPI_MakeFace(inner.Wire()).Face(), gp_Vec(0, 0, 4)).Shape();
    BRepAlgoAPI_Cut cut(source, cavity);
    if (!cut.IsDone()) return 3;
    shape = cut.Shape();
  } else {
    BRepOffsetAPI_MakeOffsetShape offset;
    offset.PerformByJoin(source, distance, 1e-9, BRepOffset_Skin, false, false,
                         GeomAbs_Intersection, true);
    if (!offset.IsDone()) return 3;
    shape = offset.Shape();
  }
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
