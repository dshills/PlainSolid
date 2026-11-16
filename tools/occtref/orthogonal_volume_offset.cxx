#include <BRepAlgoAPI_Cut.hxx>
#include <BRepAlgoAPI_Fuse.hxx>
#include <BRepCheck_Analyzer.hxx>
#include <BRepGProp.hxx>
#include <BRepPrimAPI_MakeBox.hxx>
#include <GProp_GProps.hxx>
#include <TopExp_Explorer.hxx>
#include <TopoDS_Shape.hxx>
#include <gp_Pnt.hxx>

#include <iomanip>
#include <iostream>
#include <string_view>

namespace {
TopoDS_Shape box(double x0, double x1, double y0, double y1,
                 double z0, double z1) {
  return BRepPrimAPI_MakeBox(gp_Pnt(x0, y0, z0), gp_Pnt(x1, y1, z1)).Shape();
}

int count(const TopoDS_Shape& s, TopAbs_ShapeEnum kind) {
  int n = 0;
  for (TopExp_Explorer i(s, kind); i.More(); i.Next()) ++n;
  return n;
}
}  // namespace

// Independently build the expected regularized cubical offsets from the
// analytic moved support stations. This is not a Euclidean ball offset.
int main(int argc, char** argv) {
  if (argc != 2) return 2;
  const std::string_view fixture(argv[1]);
  TopoDS_Shape result;
  if (fixture == "step-outward") {
    BRepAlgoAPI_Fuse op(box(-.5, 4.5, -.5, 2.5, -.5, 2.5),
                       box(-.5, 2.5, -.5, 2.5, 1.5, 4.5));
    if (!op.IsDone()) return 3;
    result = op.Shape();
  } else if (fixture == "step-inward") {
    BRepAlgoAPI_Cut op(box(.5, 3.5, .5, 1.5, .5, 3.5),
                      box(1.5, 4., 0., 2., 1.5, 4.));
    if (!op.IsDone()) return 3;
    result = op.Shape();
  } else if (fixture == "cavity-outward") {
    BRepAlgoAPI_Cut op(box(-1., 11., -1., 11., -1., 11.),
                      box(4., 6., 4., 6., 4., 6.));
    if (!op.IsDone()) return 3;
    result = op.Shape();
  } else if (fixture == "cavity-inward") {
    BRepAlgoAPI_Cut op(box(1., 9., 1., 9., 1., 9.),
                      box(2., 8., 2., 8., 2., 8.));
    if (!op.IsDone()) return 3;
    result = op.Shape();
  } else if (fixture == "step-wall" || fixture == "step-open-top") {
    BRepAlgoAPI_Fuse source(box(0., 4., 0., 2., 0., 2.),
                           box(0., 2., 0., 2., 2., 4.));
    BRepAlgoAPI_Cut core(box(.5, 3.5, .5, 1.5, .5, 3.5),
                        box(1.5, 4., 0., 2., 1.5, 4.));
    if (!source.IsDone() || !core.IsDone()) return 3;
    TopoDS_Shape tool = core.Shape();
    if (fixture == "step-open-top") {
      BRepAlgoAPI_Fuse opened(tool, box(.5, 1.5, .5, 1.5, 3.5, 4.5));
      if (!opened.IsDone()) return 3;
      tool = opened.Shape();
    }
    BRepAlgoAPI_Cut wall(source.Shape(), tool);
    if (!wall.IsDone()) return 3;
    result = wall.Shape();
  } else if (fixture == "cavity-wall" || fixture == "cavity-open-top") {
    BRepAlgoAPI_Cut source(box(0., 10., 0., 10., 0., 10.),
                          box(3., 7., 3., 7., 3., 7.));
    BRepAlgoAPI_Cut core(box(1., 9., 1., 9., 1., 9.),
                        box(2., 8., 2., 8., 2., 8.));
    if (!source.IsDone() || !core.IsDone()) return 3;
    TopoDS_Shape tool = core.Shape();
    if (fixture == "cavity-open-top") {
      BRepAlgoAPI_Fuse opened(tool, box(1., 9., 1., 9., 9., 11.));
      if (!opened.IsDone()) return 3;
      tool = opened.Shape();
    }
    BRepAlgoAPI_Cut wall(source.Shape(), tool);
    if (!wall.IsDone()) return 3;
    result = wall.Shape();
  } else {
    return 2;
  }
  if (result.IsNull()) return 4;
  GProp_GProps volume, area;
  BRepGProp::VolumeProperties(result, volume);
  BRepGProp::SurfaceProperties(result, area);
  std::cout << BRepCheck_Analyzer(result).IsValid() << ' '
            << count(result, TopAbs_SOLID) << ' ' << count(result, TopAbs_SHELL)
            << ' ' << std::setprecision(17) << volume.Mass() << ' '
            << area.Mass() << '\n';
}
