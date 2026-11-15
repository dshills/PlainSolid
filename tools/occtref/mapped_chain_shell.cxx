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
#include <BRep_Builder.hxx>
#include <BRep_Tool.hxx>
#include <GProp_GProps.hxx>
#ifdef PLAINSOLID_AUTHORED_STEP
#include <STEPControl_Writer.hxx>
#include <Interface_Static.hxx>
#include <StepData_Protocol.hxx>
#include <StepData_StepModel.hxx>
#include <StepData_StepWriter.hxx>
#include <fstream>
#endif
#include <Geom2d_BezierCurve.hxx>
#include <Geom2d_BSplineCurve.hxx>
#include <Geom_Plane.hxx>
#include <Geom_BezierCurve.hxx>
#include <Geom_BezierSurface.hxx>
#include <Geom_BSplineSurface.hxx>
#include <Geom_BSplineCurve.hxx>
#include <gp_Ax3.hxx>
#include <gp_Pln.hxx>
#include <NCollection_Array1.hxx>
#include <NCollection_Array2.hxx>
#include <TopExp.hxx>
#include <TopExp_Explorer.hxx>
#include <TopoDS.hxx>
#include <TopoDS_Face.hxx>
#include <TopoDS_Solid.hxx>
#include <NCollection_IndexedMap.hxx>
#include <TopTools_ShapeMapHasher.hxx>
#include <algorithm>
#include <array>
#include <iomanip>
#include <iostream>
#include <optional>
#include <stdexcept>
#include <string>
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

static std::vector<double> threeRoofKnots(bool south) {
  return south ? std::vector<double>{0., 29. / 256, 369. / 2048, 1.}
               : std::vector<double>{0., 107. / 2048, 29. / 256, 1.};
}

static double roofHeight(double x, double y, bool crease, bool three = false, bool south = false) {
  if (three) {
    const double separate = south ? 369. / 2048 : 107. / 2048;
    return x * y + .125 * std::max(0., x - 29. / 256) + .125 * (y - .25) * std::max(0., x - separate);
  }
  return x * y + (crease ? .5 * std::max(0., x - .5) : 0.);
}

static TopoDS_Face roof(bool south, int rotate = 0, bool crease = false, bool three = false) {
  const std::vector<double> x = three ? threeRoofKnots(south)
    : crease ? std::vector<double>{0., .5, 1.} : std::vector<double>{0., 1.};
  const std::vector<double> y{0., 1.};
  const auto& u = south ? y : x;
  const auto& v = south ? x : y;
  NCollection_Array2<gp_Pnt> poles(1, static_cast<int>(u.size()), 1, static_cast<int>(v.size()));
  for (int i = 0; i < poles.UpperRow(); ++i) {
    for (int j = 0; j < poles.UpperCol(); ++j) {
      const double a = south ? v[j] : u[i];
      const double b = south ? u[i] : v[j];
      poles.SetValue(i + 1, j + 1, gp_Pnt(a, b, roofHeight(a, b, crease, three, south)));
    }
  }
  occ::handle<Geom_Surface> support;
  if (crease || three) {
    NCollection_Array1<double> ku(1, static_cast<int>(u.size())), kv(1, static_cast<int>(v.size()));
    NCollection_Array1<int> mu(1, ku.Upper()), mv(1, kv.Upper());
    for (int i = 1; i <= ku.Upper(); ++i) {
      ku.SetValue(i, u[i - 1]);
      mu.SetValue(i, i == 1 || i == ku.Upper() ? 2 : 1);
    }
    for (int j = 1; j <= kv.Upper(); ++j) {
      kv.SetValue(j, v[j - 1]);
      mv.SetValue(j, j == 1 || j == kv.Upper() ? 2 : 1);
    }
    support = new Geom_BSplineSurface(poles, ku, kv, mu, mv, 1, 1);
  } else {
    support = new Geom_BezierSurface(poles);
  }
  BRepBuilderAPI_MakeWire wire;
  auto use = [&](const std::vector<gp_Pnt2d>& points) {
    NCollection_Array1<gp_Pnt2d> net(1, static_cast<int>(points.size()));
    for (int i = 1; i <= net.Upper(); ++i) {
      const auto& p = points[i - 1];
      net.SetValue(i, south ? gp_Pnt2d(p.Y(), p.X()) : p);
    }
    occ::handle<Geom2d_BezierCurve> curve = new Geom2d_BezierCurve(net);
    BRepBuilderAPI_MakeEdge edge(curve, support);
    if (!edge.IsDone()) {
      throw std::runtime_error("could not lift the independent source edge");
    }
    if (three) {
      // Each trace has one fixed chart coordinate and lies within one original
      // support cell. Its model controls are the exact affine lifts of its UV
      // controls in that cell.
      NCollection_Array1<gp_Pnt> model(1, net.Upper());
      for (int i = 1; i <= model.Upper(); ++i) {
        const auto& p = points[i - 1];
        model.SetValue(i, gp_Pnt(p.X(), p.Y(), roofHeight(p.X(), p.Y(), crease, three, south)));
      }
      BRep_Builder builder;
      occ::handle<Geom_BezierCurve> lifted = new Geom_BezierCurve(model);
      builder.UpdateEdge(edge.Edge(), lifted, 1e-11);
      builder.Range(edge.Edge(), 0., 1., true);
      builder.SameRange(edge.Edge(), true);
      builder.SameParameter(edge.Edge(), true);
    } else if (!BRepLib::BuildCurve3d(edge.Edge(), 1e-11)) {
      throw std::runtime_error("could not lift the independent source edge");
    }
    wire.Add(edge.Edge());
    if (!wire.IsDone()) {
      throw std::runtime_error("could not join the independent roof wire");
    }
  };
  const Cubic a{gp_Pnt2d(0, .25), gp_Pnt2d(.125, three ? .25 : .125), gp_Pnt2d(.375, three ? .25 : .125), gp_Pnt2d(.5, .25)};
  const Cubic b{gp_Pnt2d(.5, .25), gp_Pnt2d(.625, three ? .25 : .5), gp_Pnt2d(.875, three ? .25 : .5), gp_Pnt2d(1, .25)};
  const auto ab = split(a);
  std::vector<std::vector<gp_Pnt2d>> traces;
  if (three) {
    const auto quarters = split(ab[0]);
    const auto left = split(quarters[0]);
    const auto right = split(quarters[1]);
    for (const auto& p : {left[0], left[1], right[0], right[1], ab[1], b}) {
      traces.emplace_back(p.begin(), p.end());
    }
  } else {
    for (const auto& p : {ab[0], ab[1], b}) {
      traces.emplace_back(p.begin(), p.end());
    }
  }
  const double outer = south ? 0 : .75;
  traces.push_back({gp_Pnt2d(1, .25), gp_Pnt2d(1, outer)});
  for (int i = static_cast<int>(x.size()) - 1; i > 0; --i) {
    traces.push_back({gp_Pnt2d(x[i], outer), gp_Pnt2d(x[i - 1], outer)});
  }
  traces.push_back({gp_Pnt2d(0, outer), gp_Pnt2d(0, .25)});
  std::rotate(traces.begin(), traces.begin() + rotate, traces.end());
  for (const auto& p : traces) {
    use(p);
  }
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

static TopoDS_Face planar(const std::vector<gp_Pnt>& points, bool reverse,
                          std::optional<gp_Ax3> frame = std::nullopt) {
  BRepBuilderAPI_MakePolygon wire;
  for (const auto& p : points) {
    wire.Add(p);
  }
  wire.Close();
  BRepBuilderAPI_MakeFace face = frame
    ? BRepBuilderAPI_MakeFace(gp_Pln(*frame), wire.Wire(), true)
    : BRepBuilderAPI_MakeFace(wire.Wire(), true);
  if (!face.IsDone()) {
    throw std::runtime_error("could not build the independent planar face");
  }
  auto result = face.Face();
  if (reverse) {
    result.Reverse();
  }
  return result;
}

// Author the straight planar incidences explicitly in their own charts and
// 3D. A normalized LINE direction would round the slope of an oblique edge
// when written as a finite STEP decimal.
static void setOriginalPlaneCurves(const TopoDS_Solid& solid) {
  BRep_Builder builder;
  NCollection_IndexedMap<TopoDS_Shape, TopTools_ShapeMapHasher> prepared;
  for (TopExp_Explorer current(solid, TopAbs_FACE); current.More(); current.Next()) {
    const auto face = TopoDS::Face(current.Current());
    auto plane = occ::down_cast<Geom_Plane>(BRep_Tool::Surface(face));
    if (plane.IsNull()) {
      continue;
    }
    const auto frame = plane->Position();
    auto chart = [&](const TopoDS_Vertex& vertex) {
      const gp_Vec delta(frame.Location(), BRep_Tool::Pnt(vertex));
      return gp_Pnt2d(delta.Dot(gp_Vec(frame.XDirection())), delta.Dot(gp_Vec(frame.YDirection())));
    };
    for (TopExp_Explorer use(face, TopAbs_EDGE); use.More(); use.Next()) {
      auto edge = TopoDS::Edge(use.Current());
      edge.Orientation(TopAbs_FORWARD);
      TopoDS_Vertex start, end;
      TopExp::Vertices(edge, start, end, true);
      double low, high;
      BRep_Tool::Range(edge, low, high);
      NCollection_Array1<gp_Pnt2d> poles(1, 2);
      poles.SetValue(1, chart(start));
      poles.SetValue(2, chart(end));
      NCollection_Array1<double> knots(1, 2);
      knots.SetValue(1, low);
      knots.SetValue(2, high);
      NCollection_Array1<int> mult(1, 2);
      mult.Init(2);
      if (!prepared.Contains(edge)) {
        NCollection_Array1<gp_Pnt> model(1, 2);
        model.SetValue(1, BRep_Tool::Pnt(start));
        model.SetValue(2, BRep_Tool::Pnt(end));
        occ::handle<Geom_BSplineCurve> curve = new Geom_BSplineCurve(model, knots, mult, 1);
        builder.UpdateEdge(edge, curve, BRep_Tool::Tolerance(edge));
        prepared.Add(edge);
      }
      occ::handle<Geom2d_BSplineCurve> pcurve = new Geom2d_BSplineCurve(poles, knots, mult, 1);
      builder.UpdateEdge(edge, pcurve, face, BRep_Tool::Tolerance(edge));
      builder.Range(edge, face, low, high);
    }
  }
}

int main(int argc, char** argv) {
  try {
    const std::string mode = argc > 1 ? argv[1] : "";
    const bool three = mode == "write-three-knots";
    const bool authoredPlanes = mode == "write-knot" || three;
    const bool crease = authoredPlanes || mode == "write-knot-default-planes";
    auto frame = [&](const gp_Pnt& origin, const gp_Dir& normal, const gp_Dir& u) -> std::optional<gp_Ax3> {
      if (!authoredPlanes) {
        return std::nullopt;
      }
      return gp_Ax3(origin, normal, u);
    };
    const bool reordered = mode == "write-reordered" || crease;
    auto side = [&](double y, bool reverse) {
      const bool south = y == 0.;
      std::vector<gp_Pnt> points{{0, y, -1}, {1, y, -1}, {1, y, roofHeight(1, y, crease, three, south)}};
      if (three) {
        const auto xs = threeRoofKnots(south);
        for (int i = static_cast<int>(xs.size()) - 2; i > 0; --i) {
          points.emplace_back(xs[i], y, roofHeight(xs[i], y, crease, true, south));
        }
      } else if (crease) {
        points.emplace_back(.5, y, roofHeight(.5, y, true));
      }
      points.emplace_back(0, y, 0);
      return planar(points, reverse, frame({0, y, 0}, {0, -1, 0}, {0, 0, -1}));
    };
    const std::array<TopoDS_Face, 7> source{
      roof(false, reordered ? 4 : 0, crease, three),
      roof(true, reordered ? 5 : 0, crease, three),
      planar({{1, 0, -1}, {1, .75, -1}, {1, .75, roofHeight(1, .75, crease, three)}, {1, .25, roofHeight(1, .25, crease, three)}, {1, 0, roofHeight(1, 0, crease, three, true)}}, false,
             frame({1, 0, 0}, {1, 0, 0}, {0, 0, 1})),
      planar({{0, 0, -1}, {0, .75, -1}, {0, .75, 0}, {0, .25, 0}, {0, 0, 0}}, true,
             frame({0, 0, 0}, {1, 0, 0}, {0, 0, 1})),
      side(.75, true),
      side(0, false),
      planar({{0, 0, -1}, {1, 0, -1}, {1, .75, -1}, {0, .75, -1}}, true,
             frame({0, 0, -1}, {0, 0, 1}, {1, 0, 0}))
    };
    const std::array<int, 7> order = reordered
      ? std::array<int, 7>{6, 2, 1, 4, 3, 0, 5}
      : std::array<int, 7>{0, 1, 2, 3, 4, 5, 6};
    BRepBuilderAPI_Sewing sewing(1e-8);
    for (int i : order) {
      sewing.Add(source[i]);
    }
    sewing.Perform();
    TopExp_Explorer shell(sewing.SewedShape(), TopAbs_SHELL);
    if (!shell.More() || sewing.NbFreeEdges() || sewing.NbMultipleEdges()) {
      throw std::runtime_error("independent source shell is not closed");
    }
    const auto solid = BRepBuilderAPI_MakeSolid(TopoDS::Shell(shell.Current())).Solid();
    if (authoredPlanes) {
      setOriginalPlaneCurves(solid);
    }
    shell.Next();
    if (shell.More() || !BRepCheck_Analyzer(solid).IsValid()) {
      throw std::runtime_error("independent source solid is invalid");
    }

#ifdef PLAINSOLID_AUTHORED_STEP
    if (argc == 3 && (mode == "write" || reordered)) {
      STEPControl_Writer writer;
      Interface_Static::SetIVal("write.surfacecurve.mode", 1);
      Interface_Static::SetCVal("write.step.schema", "AP242DIS");
      if (writer.Transfer(solid, STEPControl_ManifoldSolidBrep) != IFSelect_RetDone) {
        throw std::runtime_error("independent STEP writer failed");
      }
      if (three) {
        // The authored geometric coefficients are dyadic with at most 18
        // fractional decimal digits. Retain every one in the pinned writer.
        auto model = writer.Model();
        auto protocol = occ::down_cast<StepData_Protocol>(model->Protocol());
        if (protocol.IsNull()) {
          throw std::runtime_error("independent STEP model has no protocol");
        }
        StepData_StepWriter exactWriter(model);
        exactWriter.FloatWriter().SetFormat("%.18f", true);
        exactWriter.SendModel(protocol);
        std::ofstream output(argv[2]);
        if (!output || !exactWriter.Print(output)) {
          throw std::runtime_error("independent exact STEP writer failed");
        }
      } else if (writer.Write(argv[2]) != IFSelect_RetDone) {
        throw std::runtime_error("independent STEP writer failed");
      }
      return 0;
    }
#endif
    if (argc != 1) {
      return 2;
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
