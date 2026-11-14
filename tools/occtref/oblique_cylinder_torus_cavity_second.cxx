#include <BRepAdaptor_Surface.hxx>
#include <BRepAlgoAPI_Cut.hxx>
#include <BRepCheck_Analyzer.hxx>
#include <BRepGProp.hxx>
#include <BRepPrimAPI_MakeCylinder.hxx>
#include <BRepPrimAPI_MakeTorus.hxx>
#include <GProp_GProps.hxx>
#include <IntCurvesFace_ShapeIntersector.hxx>
#include <STEPControl_Reader.hxx>
#include <TopExp_Explorer.hxx>
#include <TopoDS.hxx>
#include <algorithm>
#include <cmath>
#include <gp_Lin.hxx>
#include <gp_Torus.hxx>
#include <iomanip>
#include <iostream>
#include <vector>
int main(int argc, char **argv) {
  if (argc != 2)
    return 1;
  gp_Pnt center(-.5, .75, .25);
  gp_Dir x(.36, .48, -.8), z(.48, .64, .6);
  auto tor = BRepPrimAPI_MakeTorus(gp_Ax2(center, z, x), 2, .5).Shape();
  auto cylinder =
      BRepPrimAPI_MakeCylinder(gp_Ax2(gp_Pnt(0, 0, -4), gp_Dir(0, 0, 1)), 6, 9)
          .Shape();
  auto reference = BRepAlgoAPI_Cut(cylinder, tor).Shape();
  STEPControl_Reader reader;
  if (reader.ReadFile(argv[1]) != IFSelect_RetDone ||
      reader.TransferRoots() != 1)
    return 2;
  auto imported = reader.OneShape();
  for (auto body : {reference, imported}) {
    if (!BRepCheck_Analyzer(body).IsValid())
      return 3;
    int ns = 0, nh = 0, nt = 0;
    for (TopExp_Explorer it(body, TopAbs_SOLID); it.More(); it.Next())
      ns++;
    for (TopExp_Explorer it(body, TopAbs_SHELL); it.More(); it.Next())
      nh++;
    for (TopExp_Explorer it(body, TopAbs_FACE); it.More(); it.Next()) {
      auto f = TopoDS::Face(it.Current());
      BRepAdaptor_Surface s(f);
      if (s.GetType() == GeomAbs_Torus) {
        nt++;
        if (f.Orientation() != TopAbs_REVERSED ||
            s.Torus().Location().Distance(center) > 1e-9 ||
            fabs(s.Torus().Axis().Direction().Dot(z) - 1) > 1e-12)
          return 4;
      }
    }
    if (ns != 1 || nh != 2 || nt != 1)
      return 5;
  }
  GProp_GProps v, a, iv, ia;
  BRepGProp::VolumeProperties(reference, v);
  BRepGProp::SurfaceProperties(reference, a);
  BRepGProp::VolumeProperties(imported, iv);
  BRepGProp::SurfaceProperties(imported, ia);
  if (fabs(v.Mass() - iv.Mass()) > 1e-6 || fabs(a.Mass() - ia.Mass()) > 1e-6 ||
      v.CentreOfMass().Distance(iv.CentreOfMass()) > 1e-8)
    return 6;
  IntCurvesFace_ShapeIntersector line;
  line.Load(reference, 1e-10);
  line.Perform(gp_Lin(center, x), -10, 10);
  if (!line.IsDone())
    return 7;
  std::vector<double> roots;
  for (int i = 1; i <= line.NbPnt(); i++)
    roots.push_back(line.WParameter(i));
  std::sort(roots.begin(), roots.end());
  roots.erase(
      std::unique(roots.begin(), roots.end(),
                  [](double a, double b) { return fabs(a - b) < 1e-8; }),
      roots.end());
  if (roots.size() != 6)
    return 8;
  std::cout << std::setprecision(17) << "RESULT " << v.Mass() << " " << a.Mass()
            << " " << v.CentreOfMass().X() << " " << v.CentreOfMass().Y() << " "
            << v.CentreOfMass().Z();
  for (auto t : roots)
    std::cout << " " << t;
  std::cout << "\n";
}
