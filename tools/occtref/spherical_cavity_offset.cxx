#include <BRepAlgoAPI_Common.hxx>
#include <BRepAlgoAPI_Cut.hxx>
#include <BRepAlgoAPI_Fuse.hxx>
#include <BRep_Builder.hxx>
#include <BRepBuilderAPI_MakeSolid.hxx>
#include <BRepCheck_Analyzer.hxx>
#include <BRepGProp.hxx>
#include <BRepLib.hxx>
#include <BRepOffsetAPI_MakeOffsetShape.hxx>
#include <BRepPrimAPI_MakeSphere.hxx>
#include <BRep_Tool.hxx>
#include <GProp_GProps.hxx>
#include <Standard_Failure.hxx>
#include <TopExp_Explorer.hxx>
#include <TopoDS.hxx>
#include <TopoDS_Compound.hxx>
#include <TopoDS_Shape.hxx>
#include <TopoDS_Solid.hxx>
#include <gp_Pnt.hxx>

#include <cstdlib>
#include <iomanip>
#include <iostream>
#include <string_view>

static TopoDS_Shape cavity(double outer, double inner) {
  const gp_Pnt exterior(2, -3, 0.5);
  const gp_Pnt hole(3, -3, 0.5);
  TopoDS_Shape stock = BRepPrimAPI_MakeSphere(exterior, outer).Shape();
  if (inner <= 0.) return stock;
  BRepAlgoAPI_Cut cut(stock, BRepPrimAPI_MakeSphere(hole, inner).Shape());
  if (!cut.IsDone()) return {};
  return cut.Shape();
}

static TopoDS_Shape separated(double distance) {
  const double first = 1. + distance;
  const double second = 2. + distance;
  const gp_Pnt a(0, 0, 0);
  const gp_Pnt b(4, 0, 0);
  if (second <= 0.) {
    BRep_Builder builder;
    TopoDS_Compound empty;
    builder.MakeCompound(empty);
    return empty;
  }
  const TopoDS_Shape secondBall = BRepPrimAPI_MakeSphere(b, second).Shape();
  if (first <= 0.) return secondBall;
  BRepAlgoAPI_Fuse fuse(BRepPrimAPI_MakeSphere(a, first).Shape(), secondBall);
  if (!fuse.IsDone()) return {};
  return fuse.Shape();
}

int main(int argc, char** argv) {
  if (argc != 3) return 2;
  try {
    const std::string_view mode(argv[1]);
    const double distance = std::strtod(argv[2], nullptr);
    TopoDS_Shape shape;
    if (mode == "joined-offset" || mode == "simple-offset") {
      const TopoDS_Shape original = cavity(4., 1.);
      if (original.IsNull() || !BRepCheck_Analyzer(original).IsValid()) return 3;
      BRepOffsetAPI_MakeOffsetShape operation;
      if (mode == "joined-offset") {
        operation.PerformByJoin(original, distance, 1.e-9, BRepOffset_Skin,
                                false, false, GeomAbs_Arc, true);
      } else {
        operation.PerformBySimple(original, distance);
      }
      if (!operation.IsDone()) return 4;
      shape = operation.Shape();
    } else if (mode == "supports") {
      shape = cavity(4. + distance, 1. - distance);
    } else if (mode == "separated-supports") {
      shape = separated(distance);
    } else if (mode == "lens-joined-dilation") {
      const auto first = BRepPrimAPI_MakeSphere(gp_Pnt(0, 0, 0), 5.).Shape();
      const auto second = BRepPrimAPI_MakeSphere(gp_Pnt(0, 0, 6), 5.).Shape();
      BRepAlgoAPI_Common overlap(first, second);
      if (!overlap.IsDone() || !BRepCheck_Analyzer(overlap.Shape()).IsValid()) return 3;
      BRepOffsetAPI_MakeOffsetShape operation;
      operation.PerformByJoin(overlap.Shape(), distance, 1.e-9, BRepOffset_Skin,
                              false, false, GeomAbs_Arc, true);
      if (!operation.IsDone()) return 4;
      TopExp_Explorer shells(operation.Shape(), TopAbs_SHELL);
      if (!shells.More() || !BRep_Tool::IsClosed(shells.Current())) return 7;
      const auto shell = TopoDS::Shell(shells.Current());
      shells.Next();
      if (shells.More()) return 7;
      TopoDS_Solid solid = BRepBuilderAPI_MakeSolid(shell).Solid();
      if (!BRepLib::OrientClosedSolid(solid)) return 7;
      shape = solid;
    } else if (mode == "lens-supports") {
      const auto first = BRepPrimAPI_MakeSphere(gp_Pnt(0, 0, 0), 2. - distance).Shape();
      const auto second = BRepPrimAPI_MakeSphere(gp_Pnt(0, 2, 0), 2.5 - distance).Shape();
      BRepAlgoAPI_Common overlap(first, second);
      if (!overlap.IsDone()) return 4;
      shape = overlap.Shape();
    } else {
      return 2;
    }
    if (shape.IsNull()) return 5;
    const auto count = [&](TopAbs_ShapeEnum type) {
      int result = 0;
      for (TopExp_Explorer it(shape, type); it.More(); it.Next()) ++result;
      return result;
    };
    GProp_GProps mass, area;
    BRepGProp::VolumeProperties(shape, mass, 1.e-12, true);
    BRepGProp::SurfaceProperties(shape, area, 1.e-12);
    std::cout << BRepCheck_Analyzer(shape).IsValid() << ' '
              << count(TopAbs_SOLID) << ' ' << count(TopAbs_FACE) << ' '
              << std::setprecision(17) << mass.Mass() << ' ' << area.Mass() << '\n';
    return 0;
  } catch (const Standard_Failure& error) {
    std::cerr << error.what() << '\n';
    return 6;
  }
}
