#include <BRepAdaptor_Surface.hxx>
#include <BRepAlgoAPI_Cut.hxx>
#include <BRepBuilderAPI_MakeVertex.hxx>
#include <BRepBuilderAPI_Transform.hxx>
#include <BRepExtrema_DistShapeShape.hxx>
#include <BRepPrimAPI_MakeCylinder.hxx>
#include <BRepPrimAPI_MakeTorus.hxx>
#include <ElSLib.hxx>
#include <TopExp_Explorer.hxx>
#include <TopoDS.hxx>
#include <gp_Trsf.hxx>
#include <iomanip>
#include <iostream>
int main(int argc, char **argv) {
  if (argc != 4 && argc != 5)
    return 1;
  auto tor =
      BRepPrimAPI_MakeTorus(
          gp_Ax2(gp_Pnt(1, 0, 0), gp_Dir(.8, 0, .6), gp_Dir(.6, 0, -.8)), 3, 1)
          .Shape();
  auto cyl =
      BRepPrimAPI_MakeCylinder(gp_Ax2(gp_Pnt(0, 0, -6), gp_Dir(0, 0, 1)), 8, 12)
          .Shape();
  auto body = BRepAlgoAPI_Cut(cyl, tor).Shape();
  gp_Trsf pose;
  if (argc == 5) {
    if (std::string(argv[4]) != "oblique")
      return 2;
    pose.SetValues(.6, 0, .8, .1, 0, 1, 0, -.4, -.8, 0, .6, .375);
    body = BRepBuilderAPI_Transform(body, pose, true).Shape();
  }
  auto point =
      BRepBuilderAPI_MakeVertex(
          gp_Pnt(std::stod(argv[1]), std::stod(argv[2]), std::stod(argv[3])))
          .Shape();
  std::cout << std::setprecision(17);
  for (TopExp_Explorer it(body, TopAbs_FACE); it.More(); it.Next()) {
    auto face = TopoDS::Face(it.Current());
    BRepExtrema_DistShapeShape d(point, face);
    if (!d.IsDone())
      return 3;
    if (d.Value() > 1e-7)
      continue;
    BRepAdaptor_Surface s(face);
    double u, v;
    if (s.GetType() == GeomAbs_Torus)
      ElSLib::Parameters(s.Torus(), d.PointOnShape2(1), u, v);
    else if (s.GetType() == GeomAbs_Cylinder)
      ElSLib::Parameters(s.Cylinder(), d.PointOnShape2(1), u, v);
    else if (s.GetType() == GeomAbs_Plane)
      ElSLib::Parameters(s.Plane(), d.PointOnShape2(1), u, v);
    else
      return 4;
    gp_Pnt p;
    gp_Vec du, dv;
    s.D1(u, v, p, du, dv);
    gp_Vec n = du.Crossed(dv);
    n.Normalize();
    if (face.Orientation() == TopAbs_REVERSED)
      n.Reverse();
    int owner = 0;
    if (s.GetType() == GeomAbs_Torus)
      owner = 1;
    else if (s.GetType() == GeomAbs_Cylinder)
      owner = 2;
    else if (s.GetType() == GeomAbs_Plane) {
      auto p = s.Plane().Location();
      p.Transform(pose.Inverted());
      owner = p.Z() < 0 ? 4 : 8;
    } else
      return 4;
    std::cout << owner << " " << n.X() << " " << n.Y() << " " << n.Z() << "\n";
  }
}
