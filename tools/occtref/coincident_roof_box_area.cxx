// Independent OCCT fixtures for regularized surface area at shared boundaries.
#include <BRepAlgoAPI_Common.hxx>
#include <BRepAlgoAPI_Cut.hxx>
#include <BRepAlgoAPI_Fuse.hxx>
#include <BRepCheck_Analyzer.hxx>
#include <BRepClass3d_SolidClassifier.hxx>
#include <BRepGProp.hxx>
#include <BRepPrimAPI_MakeBox.hxx>
#include <GProp.hxx>
#include <GProp_GProps.hxx>
#include <TopExp_Explorer.hxx>
#include <TopoDS.hxx>
#include <TopoDS_Shape.hxx>
#include <gp_Pnt.hxx>

#include <array>
#include <iomanip>
#include <iostream>
#include <string_view>

int main(int argc, char** argv) {
  const bool mass = argc == 2 && std::string_view(argv[1]) == "mass";
  const bool point = argc == 2 && std::string_view(argv[1]) == "point";
  if (argc != 1 && !mass && !point) {
    return 3;
  }
  const auto box = [](double x, double y, double z) {
    return BRepPrimAPI_MakeBox(gp_Pnt(x, y, z), 2., 2., 2.).Shape();
  };
  const TopoDS_Shape stock = box(0, 0, 0);
  const std::array<TopoDS_Shape, 3> tools{box(1, 0, 0), box(2, 1, 1), stock};
  const std::array<std::array<gp_Pnt, 4>, 3> points{{
      {gp_Pnt(1.5, 1, 2), gp_Pnt(1, 1, 1), gp_Pnt(0, 1, 2), gp_Pnt(2, 2, 2)},
      {gp_Pnt(2, 1.5, 1.5), gp_Pnt(2, 1, 1.5), gp_Pnt(2, 2, 2), gp_Pnt(2, .5, 1.5)},
      {gp_Pnt(0, 0, 0), gp_Pnt(1, 1, 2), gp_Pnt(1, 1, 1), gp_Pnt(-1, 1, 1)}}};
  std::cout << std::setprecision(17) << '[';
  for (size_t i = 0; i < tools.size(); ++i) {
    const auto& tool = tools[i];
    BRepAlgoAPI_Common common(stock, tool);
    BRepAlgoAPI_Fuse fuse(stock, tool);
    BRepAlgoAPI_Cut cut(stock, tool);
    if (!common.IsDone() || !fuse.IsDone() || !cut.IsDone()) {
      return 1;
    }
    std::cout << (i ? ",[" : "[");
    const std::array<TopoDS_Shape, 3> results{common.Shape(), fuse.Shape(), cut.Shape()};
    for (size_t j = 0; j < results.size(); ++j) {
      const auto& result = results[j];
      if (result.IsNull() || !BRepCheck_Analyzer(result).IsValid()) {
        return 2;
      }
      GProp_GProps prop;
      std::cout << (j ? "," : "");
      if (point) {
        std::cout << '[';
        for (size_t k = 0; k < points[i].size(); ++k) {
          int where = 0;
          for (TopExp_Explorer ex(result, TopAbs_SOLID); ex.More(); ex.Next()) {
            BRepClass3d_SolidClassifier c(TopoDS::Solid(ex.Current()), points[i][k], 1e-10);
            if (c.State() == TopAbs_IN) { where = 2; break; }
            if (c.State() == TopAbs_ON) where = 1;
            if (c.State() != TopAbs_OUT && c.State() != TopAbs_ON) return 4;
          }
          std::cout << (k ? "," : "") << where;
        }
        std::cout << ']';
        continue;
      }
      if (!mass) {
        BRepGProp::SurfaceProperties(result, prop);
        std::cout << prop.Mass();
        continue;
      }
      BRepGProp::VolumeProperties(result, prop);
      const auto c = prop.CentreOfMass();
      gp_Mat shift;
      GProp::HOperator(c, gp_Pnt(0, 0, 0), prop.Mass(), shift);
      const auto inertia = prop.MatrixOfInertia() + shift;
      std::cout << "{\"Volume\":" << prop.Mass() << ",\"Centroid\":["
                << c.X() << ',' << c.Y() << ',' << c.Z() << "],\"Inertia\":[";
      for (int r = 1; r <= 3; ++r) {
        std::cout << (r == 1 ? "[" : ",[") << inertia.Value(r, 1) << ','
                  << inertia.Value(r, 2) << ',' << inertia.Value(r, 3) << ']';
      }
      std::cout << "]}";
    }
    std::cout << ']';
  }
  std::cout << "]\n";
}
