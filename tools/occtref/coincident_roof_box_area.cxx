// Independent OCCT fixtures for regularized surface area at shared boundaries.
#include <BRepAlgoAPI_Common.hxx>
#include <BRepAlgoAPI_Cut.hxx>
#include <BRepAlgoAPI_Fuse.hxx>
#include <BRepCheck_Analyzer.hxx>
#include <BRepGProp.hxx>
#include <BRepPrimAPI_MakeBox.hxx>
#include <GProp_GProps.hxx>
#include <TopoDS_Shape.hxx>
#include <gp_Pnt.hxx>

#include <array>
#include <iomanip>
#include <iostream>

int main() {
  const auto box = [](double x, double y, double z) {
    return BRepPrimAPI_MakeBox(gp_Pnt(x, y, z), 2., 2., 2.).Shape();
  };
  const TopoDS_Shape stock = box(0, 0, 0);
  const std::array<TopoDS_Shape, 3> tools{box(1, 0, 0), box(2, 1, 1), stock};
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
      BRepGProp::SurfaceProperties(result, prop);
      std::cout << (j ? "," : "") << prop.Mass();
    }
    std::cout << ']';
  }
  std::cout << "]\n";
}
