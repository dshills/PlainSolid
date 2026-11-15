// Independently build the seven original faces over z=x*y, with the roof
// separated by the two-piece cubic coedge and an independent refinement cut.
#include <BRepBuilderAPI_MakeEdge.hxx>
#include <BRepBuilderAPI_MakeFace.hxx>
#include <BRepBuilderAPI_MakePolygon.hxx>
#include <BRepBuilderAPI_MakeSolid.hxx>
#include <BRepBuilderAPI_MakeWire.hxx>
#include <BRepBuilderAPI_Sewing.hxx>
#include <BRepCheck_Analyzer.hxx>
#include <BRepGProp.hxx>
#include <BRepLib.hxx>
#include <GProp_GProps.hxx>
#include <Geom2d_BezierCurve.hxx>
#include <Geom_BezierSurface.hxx>
#include <NCollection_Array1.hxx>
#include <NCollection_Array2.hxx>
#include <TopExp.hxx>
#include <TopExp_Explorer.hxx>
#include <TopoDS.hxx>
#include <TopoDS_Face.hxx>
#include <TopoDS_Solid.hxx>
#include <NCollection_IndexedMap.hxx>
#include <TopTools_ShapeMapHasher.hxx>
#include <array>
#include <iomanip>
#include <iostream>
#include <stdexcept>
#include <vector>

using Cubic = std::array<gp_Pnt2d, 4>;

static std::array<Cubic, 2> split(const Cubic& p) {
  auto middle = [](const gp_Pnt2d& a, const gp_Pnt2d& b) {
    return gp_Pnt2d((a.X() + b.X()) / 2, (a.Y() + b.Y()) / 2);
  };
  const auto a = middle(p[0], p[1]);
  const auto b = middle(p[1], p[2]);
  const auto c = middle(p[2], p[3]);
  const auto d = middle(a, b);
  const auto e = middle(b, c);
  const auto m = middle(d, e);
  return {Cubic{p[0], a, d, m}, Cubic{m, e, c, p[3]}};
}

static TopoDS_Face roof(bool south) {
  NCollection_Array2<gp_Pnt> poles(1, 2, 1, 2);
  for (int i = 0; i < 2; ++i) {
    for (int j = 0; j < 2; ++j) {
      poles.SetValue(i + 1, j + 1, gp_Pnt(south ? j : i, south ? i : j, i * j));
    }
  }
  occ::handle<Geom_BezierSurface> support = new Geom_BezierSurface(poles);
  BRepBuilderAPI_MakeWire wire;
  auto use = [&](const std::vector<gp_Pnt2d>& points) {
    NCollection_Array1<gp_Pnt2d> net(1, static_cast<int>(points.size()));
    for (int i = 1; i <= net.Upper(); ++i) {
      const auto& p = points[i - 1];
      net.SetValue(i, south ? gp_Pnt2d(p.Y(), p.X()) : p);
    }
    occ::handle<Geom2d_BezierCurve> curve = new Geom2d_BezierCurve(net);
    BRepBuilderAPI_MakeEdge edge(curve, support);
    if (!edge.IsDone() || !BRepLib::BuildCurve3d(edge.Edge(), 1e-11)) {
      throw std::runtime_error("could not lift the independent source edge");
    }
    wire.Add(edge.Edge());
    if (!wire.IsDone()) {
      throw std::runtime_error("could not join the independent roof wire");
    }
  };
  const Cubic a{gp_Pnt2d(0, .25), gp_Pnt2d(.125, .125), gp_Pnt2d(.375, .125), gp_Pnt2d(.5, .25)};
  const Cubic b{gp_Pnt2d(.5, .25), gp_Pnt2d(.625, .5), gp_Pnt2d(.875, .5), gp_Pnt2d(1, .25)};
  const auto ab = split(a);
  for (const auto& p : {ab[0], ab[1], b}) {
    use(std::vector<gp_Pnt2d>(p.begin(), p.end()));
  }
  const double outer = south ? 0 : .75;
  use({gp_Pnt2d(1, .25), gp_Pnt2d(1, outer)});
  use({gp_Pnt2d(1, outer), gp_Pnt2d(0, outer)});
  use({gp_Pnt2d(0, outer), gp_Pnt2d(0, .25)});
  BRepBuilderAPI_MakeFace builder(support, wire.Wire(), true);
  if (!builder.IsDone()) {
    throw std::runtime_error("could not build the independent roof face");
  }
  auto result = builder.Face();
  if (south) {
    result.Reverse();
  }
  return result;
}

static TopoDS_Face planar(const std::vector<gp_Pnt>& points, bool reverse) {
  BRepBuilderAPI_MakePolygon wire;
  for (const auto& p : points) {
    wire.Add(p);
  }
  wire.Close();
  BRepBuilderAPI_MakeFace face(wire.Wire(), true);
  if (!face.IsDone()) {
    throw std::runtime_error("could not build the independent planar face");
  }
  auto result = face.Face();
  if (reverse) {
    result.Reverse();
  }
  return result;
}

int main() {
  try {
    BRepBuilderAPI_Sewing sewing(1e-8);
    sewing.Add(roof(false));
    sewing.Add(roof(true));
    sewing.Add(planar({{1, 0, -1}, {1, .75, -1}, {1, .75, .75}, {1, .25, .25}, {1, 0, 0}}, false));
    sewing.Add(planar({{0, 0, -1}, {0, .75, -1}, {0, .75, 0}, {0, .25, 0}, {0, 0, 0}}, true));
    sewing.Add(planar({{0, .75, -1}, {1, .75, -1}, {1, .75, .75}, {0, .75, 0}}, true));
    sewing.Add(planar({{0, 0, -1}, {1, 0, -1}, {1, 0, 0}, {0, 0, 0}}, false));
    sewing.Add(planar({{0, 0, -1}, {1, 0, -1}, {1, .75, -1}, {0, .75, -1}}, true));
    sewing.Perform();
    TopExp_Explorer shell(sewing.SewedShape(), TopAbs_SHELL);
    if (!shell.More() || sewing.NbFreeEdges() || sewing.NbMultipleEdges()) {
      throw std::runtime_error("independent source shell is not closed");
    }
    const auto solid = BRepBuilderAPI_MakeSolid(TopoDS::Shell(shell.Current())).Solid();
    shell.Next();
    if (shell.More() || !BRepCheck_Analyzer(solid).IsValid()) {
      throw std::runtime_error("independent source solid is invalid");
    }

    GProp_GProps volume, area;
    BRepGProp::VolumeProperties(solid, volume, 1e-10);
    BRepGProp::SurfaceProperties(solid, area, 1e-10);
    NCollection_IndexedMap<TopoDS_Shape, TopTools_ShapeMapHasher> vertices, edges, faces;
    TopExp::MapShapes(solid, TopAbs_VERTEX, vertices);
    TopExp::MapShapes(solid, TopAbs_EDGE, edges);
    TopExp::MapShapes(solid, TopAbs_FACE, faces);
    const auto c = volume.CentreOfMass();
    std::cout << std::setprecision(17)
              << "{\"Vertices\":" << vertices.Extent() << ",\"Edges\":" << edges.Extent()
              << ",\"Faces\":" << faces.Extent() << ",\"Volume\":" << volume.Mass()
              << ",\"Area\":" << area.Mass() << ",\"Center\":[" << c.X() << "," << c.Y() << ","
              << c.Z() << "]}\n";
    return volume.Mass() > 0 ? 0 : 2;
  } catch (const std::exception& error) {
    std::cerr << error.what() << "\n";
    return 2;
  }
}
