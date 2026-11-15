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
#include <StepGeom_BSplineCurveWithKnots.hxx>
#include <StepGeom_BSplineCurveWithKnotsAndRationalBSplineCurve.hxx>
#include <StepGeom_BSplineSurfaceWithKnots.hxx>
#include <StepGeom_BSplineSurfaceWithKnotsAndRationalBSplineSurface.hxx>
#include <StepGeom_Pcurve.hxx>
#include <StepGeom_SurfaceCurve.hxx>
#include <StepGeom_TrimmedCurve.hxx>
#include <StepGeom_TrimmingSelect.hxx>
#include <StepRepr_DefinitionalRepresentation.hxx>
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

static constexpr double additionalNorthKnot = 407. / 16384;
static constexpr double fifthSupportKnot = 1341. / 16384;
static constexpr double sixthSupportKnot = 2395. / 16384;
static constexpr double secondSharedKnot = 99. / 256;
static constexpr double secondCellSouthKnot = 917. / 2048;
static constexpr double thirdSharedKnot = .75;

enum class InferredSupportEvents { none, asynchronous, shared, sharedPair, offsetPair, nonplanar };

struct RoofSupportLayout {
  int cuts = 3;
  int storedCells = 1;
  bool separated = false;
  InferredSupportEvents inferred = InferredSupportEvents::none;
  bool rationalModel = false;
  bool independentOwnerClocks = false;
  bool rationalRoofs = false;
  bool joinedRationalCells = false;
  bool missingSouthJoint = false;
};

static std::vector<double> inferredSupportKnots(bool south, InferredSupportEvents events) {
  switch (events) {
  case InferredSupportEvents::shared:
    return {29. / 256};
  case InferredSupportEvents::sharedPair:
  case InferredSupportEvents::offsetPair:
    return {107. / 2048, south && events == InferredSupportEvents::offsetPair ? 29. / 256 : 369. / 2048};
  case InferredSupportEvents::nonplanar:
    return {1. / 16, south ? 1. / 8 : 3. / 16};
  case InferredSupportEvents::asynchronous:
    return {south ? 369. / 2048 : 107. / 2048};
  default:
    return {};
  }
}

static constexpr double nonplanarWidth = .75;
static constexpr double nonplanarAmplitude = 8.;

static double nonplanarSeamY(double u) {
  return .25 + .75 * u * (1. - u);
}

static std::array<gp_Pnt2d, 3> nonplanarSeamPoles(double a, double b) {
  return {gp_Pnt2d(a, nonplanarSeamY(a)),
          gp_Pnt2d((a + b) / 2., .25 + .75 * ((a + b) / 2. - a * b)),
          gp_Pnt2d(b, nonplanarSeamY(b))};
}

static Cubic nonplanarChartSeam(double a, double b) {
  const auto seam = nonplanarSeamPoles(a, b);
  const double xa = nonplanarWidth*a, xb = nonplanarWidth*b;
  return {gp_Pnt2d(xa, seam[0].Y()),
          gp_Pnt2d((2*xa+xb)/3., (seam[0].Y()+2*seam[1].Y())/3.),
          gp_Pnt2d((xa+2*xb)/3., (2*seam[1].Y()+seam[2].Y())/3.),
          gp_Pnt2d(xb, seam[2].Y())};
}

static double nonplanarRoofEnvelope(double u, double first, double last) {
  return (1. - last) * std::max(0., u - first) - (1. - first) * std::max(0., u - last);
}

// Each affine-XY bicubic roof adds 24*A(u)*y*(3/4-y)*(y-h(u)) to z=x*y,
// with u=4*x/3, h the common quadratic UV trace and A the owner's affine
// envelope on this span. A vanishes on the x walls. The other factors vanish
// on the y walls and along the shared trace. The rational form doubles this
// correction and divides it by W(v)=1+3v², v=4y/3. Its homogeneous x and y
// still represent the original affine XY chart, with dyadic Cartesian poles.
static occ::handle<Geom_BSplineSurface> nonplanarRoofSupport(bool south, const std::vector<double>& knots, bool rational) {
  const int spanCount = static_cast<int>(knots.size()) - 1;
  const int xPoles = 3 * spanCount + 1;
  NCollection_Array2<gp_Pnt> poles(1, south ? 4 : xPoles, 1, south ? xPoles : 4);
  NCollection_Array2<double> weights(1, south ? 4 : xPoles, 1, south ? xPoles : 4);
  const std::array<double, 4> homogeneousWeights{1., 1., 2., 4.};
  const std::array<double, 4> rationalY{0., .25, .25, .75};
  for (int span = 0; span < spanCount; ++span) {
    const double a = knots[span], b = knots[span + 1];
    const auto seam = nonplanarSeamPoles(a, b);
    const double a0 = nonplanarRoofEnvelope(a, knots[1], knots[2]);
    const double a1 = nonplanarRoofEnvelope(b, knots[1], knots[2]);
    // These are cubic controls of 3*A and 3*A*h. Scaling by three
    // cancels the denominator in the Bernstein product of A with h.
    const std::array<double, 4> envelope{3 * a0, 2 * a0 + a1, a0 + 2 * a1, 3 * a1};
    const std::array<double, 4> envelopeSeam{
      3 * a0 * seam[0].Y(), 2 * a0 * seam[1].Y() + a1 * seam[0].Y(),
      a0 * seam[2].Y() + 2 * a1 * seam[1].Y(), 3 * a1 * seam[2].Y()};
    for (int i = 0; i < 4; ++i) {
      const double x = nonplanarWidth * a + (b - a) * i / 4.;
      for (int j = 0; j < 4; ++j) {
        const double y = rational ? rationalY[j] : j / 4.;
        const double weight = rational ? homogeneousWeights[j] : 1.;
        double z = x * y;
        if (j == 1 || j == 2) {
          // 3/16 is the nonzero Bernstein coefficient of y(3/4-y)
          // after multiplying its quadratic by the linear y-seam factor.
          const double scale = rational ? 2. / weight : 1.;
          z += 3. / 16 * nonplanarAmplitude * scale * ((j == 2 ? .75 * envelope[i] : 0.) - envelopeSeam[i]);
        }
        const int xi = 3 * span + i + 1, yi = j + 1;
        poles.SetValue(south ? yi : xi, south ? xi : yi, gp_Pnt(x, y, z));
        weights.SetValue(south ? yi : xi, south ? xi : yi, weight);
      }
    }
  }
  NCollection_Array1<double> xKnots(1, spanCount + 1), yKnots(1, 2);
  NCollection_Array1<int> xMult(1, spanCount + 1), yMult(1, 2);
  for (int i = 1; i <= xKnots.Upper(); ++i) {
    xKnots.SetValue(i, nonplanarWidth * knots[i - 1]);
    xMult.SetValue(i, i == 1 || i == xKnots.Upper() ? 4 : 3);
  }
  yKnots.SetValue(1, 0.);
  yKnots.SetValue(2, .75);
  yMult.Init(4);
  if (rational) {
    return south ? new Geom_BSplineSurface(poles, weights, yKnots, xKnots, yMult, xMult, 3, 3)
                 : new Geom_BSplineSurface(poles, weights, xKnots, yKnots, xMult, yMult, 3, 3);
  }
  return south ? new Geom_BSplineSurface(poles, yKnots, xKnots, yMult, xMult, 3, 3)
               : new Geom_BSplineSurface(poles, xKnots, yKnots, xMult, yMult, 3, 3);
}

static occ::handle<Geom_BezierCurve> nonplanarModelSeam(const std::vector<gp_Pnt2d>& seam) {
  const double xa = nonplanarWidth * seam[0].X(), xb = nonplanarWidth * seam[2].X();
  const double ya = seam[0].Y(), ym = seam[1].Y(), yb = seam[2].Y();
  NCollection_Array1<gp_Pnt> poles(1, 4);
  poles.SetValue(1, gp_Pnt(xa, ya, xa * ya));
  poles.SetValue(2, gp_Pnt((2*xa+xb)/3., (ya+2*ym)/3., (2*xa*ym+xb*ya)/3.));
  poles.SetValue(3, gp_Pnt((xa+2*xb)/3., (2*ym+yb)/3., (xa*yb+2*xb*ym)/3.));
  poles.SetValue(4, gp_Pnt(xb, yb, xb * yb));
  return new Geom_BezierCurve(poles);
}

static std::vector<double> nonplanarSupportEvents(bool south, RoofSupportLayout layout) {
  if (layout.joinedRationalCells) {
    return south ? std::vector<double>{layout.missingSouthJoint ? .125 : .25, .625}
                 : std::vector<double>{.0625, .25};
  }
  return inferredSupportKnots(south, layout.inferred);
}

static std::vector<double> roofSupportKnots(bool south, RoofSupportLayout layout) {
  if (layout.inferred != InferredSupportEvents::none) {
    auto knots = nonplanarSupportEvents(south, layout);
    knots.insert(knots.begin(), 0.);
    knots.push_back(1.);
    return knots;
  }
  if (layout.separated) {
    return south ? std::vector<double>{0., 29. / 256, .75, 1.}
                 : std::vector<double>{0., 107. / 2048, 29. / 256, 1.};
  }
  if (layout.storedCells >= 2) {
    auto knots = south ? std::vector<double>{0., 29. / 256, secondSharedKnot, secondCellSouthKnot, 1.}
                       : std::vector<double>{0., 107. / 2048, 29. / 256, secondSharedKnot, 1.};
    if (layout.storedCells == 3) {
      knots.insert(knots.end() - 1, thirdSharedKnot);
    }
    return knots;
  }
  const int supportCuts = layout.cuts;
  if (south && supportCuts == 6) {
    return {0., 29. / 256, sixthSupportKnot, 369. / 2048, 1.};
  }
  if (!south && supportCuts >= 5) {
    return {0., additionalNorthKnot, 107. / 2048, fifthSupportKnot, 29. / 256, 1.};
  }
  if (!south && supportCuts == 4) {
    return {0., additionalNorthKnot, 107. / 2048, 29. / 256, 1.};
  }
  return south ? std::vector<double>{0., 29. / 256, 369. / 2048, 1.}
               : std::vector<double>{0., 107. / 2048, 29. / 256, 1.};
}

static double roofHeight(double x, double y, bool crease, bool three = false, bool south = false, RoofSupportLayout layout = {}) {
  if (layout.inferred == InferredSupportEvents::nonplanar) {
    const auto knots = nonplanarSupportEvents(south, layout);
    const double v = y / .75;
    const double scale = layout.rationalRoofs ? 2. / (1. + 3. * v * v) : 1.;
    return nonplanarWidth*x*y + 3.*nonplanarAmplitude*scale*nonplanarRoofEnvelope(x, knots[0], knots[1])
      * y*(.75-y)*(y-nonplanarSeamY(x));
  }
  if (three) {
    if (layout.inferred != InferredSupportEvents::none) {
      double z = x * y;
      for (const double knot : inferredSupportKnots(south, layout.inferred)) {
        z += .125 * (y - .25) * std::max(0., x - knot);
      }
      return z;
    }
    if (layout.separated) {
      const double separate = south ? .75 : 107. / 2048;
      return x * y + .125 * std::max(0., x - 29. / 256) + .125 * (y - .25) * std::max(0., x - separate);
    }
    if (layout.storedCells >= 2) {
      const double separate = south ? secondCellSouthKnot : 107. / 2048;
      const double third = layout.storedCells == 3 ? .125 * std::max(0., x - thirdSharedKnot) : 0.;
      return x * y + .125 * (std::max(0., x - 29. / 256) + std::max(0., x - secondSharedKnot))
        + .125 * (y - .25) * std::max(0., x - separate) + third;
    }
    const int supportCuts = layout.cuts;
    const double separate = south ? 369. / 2048 : 107. / 2048;
    double extra = supportCuts >= 4 && !south ? .25 * (y - .25) * std::max(0., x - additionalNorthKnot) : 0.;
    if (supportCuts >= 5 && !south) {
      extra += .25 * (y - .25) * std::max(0., x - fifthSupportKnot);
    }
    if (supportCuts == 6 && south) {
      extra += .25 * (y - .25) * std::max(0., x - sixthSupportKnot);
    }
    return x * y + .125 * std::max(0., x - 29. / 256) + .125 * (y - .25) * std::max(0., x - separate) + extra;
  }
  return x * y + (crease ? .5 * std::max(0., x - .5) : 0.);
}

static TopoDS_Face roof(bool south, int rotate, bool crease, bool three, bool unsplit, RoofSupportLayout layout) {
  const std::vector<double> x = three ? roofSupportKnots(south, layout)
    : crease ? std::vector<double>{0., .5, 1.} : std::vector<double>{0., 1.};
  const bool nonplanar = layout.inferred == InferredSupportEvents::nonplanar;
  occ::handle<Geom_Surface> support;
  if (nonplanar) {
    support = nonplanarRoofSupport(south, x, layout.rationalRoofs);
  } else {
    const std::vector<double> y{0., 1.};
    const auto& u = south ? y : x;
    const auto& v = south ? x : y;
    NCollection_Array2<gp_Pnt> poles(1, static_cast<int>(u.size()), 1, static_cast<int>(v.size()));
    for (int i = 0; i < poles.UpperRow(); ++i) {
      for (int j = 0; j < poles.UpperCol(); ++j) {
        const double a = south ? v[j] : u[i];
        const double b = south ? u[i] : v[j];
        poles.SetValue(i + 1, j + 1, gp_Pnt(a, b, roofHeight(a, b, crease, three, south, layout)));
      }
    }
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
  }
  BRepBuilderAPI_MakeWire wire;
  auto use = [&](const std::vector<gp_Pnt2d>& points) {
    const bool spatialSeam = nonplanar && points.size() == 3;
    NCollection_Array1<gp_Pnt2d> net(1, spatialSeam ? 4 : static_cast<int>(points.size()));
    if (spatialSeam) {
      const auto chart = nonplanarChartSeam(points.front().X(), points.back().X());
      for (int i = 1; i <= net.Upper(); ++i) {
        const auto& p = chart[i - 1];
        net.SetValue(i, south ? gp_Pnt2d(p.Y(), p.X()) : p);
      }
    } else {
      for (int i = 1; i <= net.Upper(); ++i) {
        const auto& p = points[i - 1];
        const double x = nonplanar ? nonplanarWidth * p.X() : p.X();
        net.SetValue(i, south ? gp_Pnt2d(p.Y(), x) : gp_Pnt2d(x, p.Y()));
      }
    }
    auto lift = [&](const gp_Pnt2d& p) {
      const double modelX = nonplanar ? nonplanarWidth * p.X() : p.X();
      return gp_Pnt(modelX, p.Y(), roofHeight(p.X(), p.Y(), crease, three, south, layout));
    };
    occ::handle<Geom2d_BezierCurve> curve = new Geom2d_BezierCurve(net);
    BRepBuilderAPI_MakeEdge edge;
    if (nonplanar) {
      // Author exact endpoints along with the model curve so STEP vertices
      // retain the same represented points as the independently stored poles.
      edge.Init(curve, support, lift(points.front()), lift(points.back()), 0., 1.);
    } else {
      edge.Init(curve, support);
    }
    if (!edge.IsDone()) {
      throw std::runtime_error("could not lift the independent source edge");
    }
    if (three) {
      occ::handle<Geom_Curve> lifted;
      const double lastStoredStart = layout.storedCells == 3 ? .5 : layout.storedCells == 2 ? .25 : 0.;
      if (spatialSeam) {
        lifted = nonplanarModelSeam(points);
      } else if (unsplit && layout.storedCells > 0 && points.size() == 4 && points.front().X() <= lastStoredStart) {
        // The full first UV cubic crosses the original support stations.
        // The authored stored cut normally follows the shared material kink.
        // The same-cell control mode instead authors a redundant exact join.
        const auto pieces = split(Cubic{points[0], points[1], points[2], points[3]});
        NCollection_Array1<gp_Pnt> model(1, 7);
        for (int i = 0; i < 4; ++i) {
          model.SetValue(i + 1, lift(pieces[0][i]));
          model.SetValue(i + 4, lift(pieces[1][i]));
        }
        NCollection_Array1<int> mult(1, 3);
        mult.Init(4);
        mult.SetValue(2, 3);
        NCollection_Array1<double> knots(1, 3);
        knots.SetValue(1, 0.);
        knots.SetValue(2, .5);
        knots.SetValue(3, 1.);
        lifted = new Geom_BSplineCurve(model, knots, mult, 3);
      } else {
        NCollection_Array1<gp_Pnt> model(1, net.Upper());
        for (int i = 1; i <= model.Upper(); ++i) {
          model.SetValue(i, lift(points[i - 1]));
        }
        lifted = new Geom_BezierCurve(model);
      }
      BRep_Builder builder;
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
  if (nonplanar) {
    const auto ends = layout.joinedRationalCells ? std::vector<double>{0., .25, 1.}
                                                 : std::vector<double>{0., .25, .5, 1.};
    for (std::size_t i = 0; i + 1 < ends.size(); ++i) {
      const auto seam = nonplanarSeamPoles(ends[i], ends[i + 1]);
      traces.emplace_back(seam.begin(), seam.end());
    }
  } else if (three && !unsplit) {
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

#ifdef PLAINSOLID_AUTHORED_STEP
template<class T, std::size_t N>
static occ::handle<NCollection_HArray1<T>> stepArray(const std::array<T, N>& values) {
  occ::handle<NCollection_HArray1<T>> items = new NCollection_HArray1<T>(1, N);
  for (std::size_t i = 0; i < N; ++i) {
    items->SetValue(i + 1, values[i]);
  }
  return items;
}

template <std::size_t N>
static auto nonplanarSouthPoles(const occ::handle<TCollection_HAsciiString>& name, const std::array<double, N>& ends) {
  std::array<occ::handle<StepGeom_CartesianPoint>, 3*(N-1)+1> poles;
  for (std::size_t i = 0; i + 1 < N; ++i) {
    const auto seam = nonplanarChartSeam(ends[i], ends[i + 1]);
    for (std::size_t j = 0; j < seam.size(); ++j) {
      auto point = new StepGeom_CartesianPoint;
      point->Init2D(name, seam[j].Y(), seam[j].X());
      poles[3*i+j] = point;
    }
  }
  return stepArray(poles);
}

static occ::handle<StepGeom_BSplineSurfaceWithKnots> stepRoofKnots(const occ::handle<StepGeom_Surface>& support) {
  auto rational = occ::down_cast<StepGeom_BSplineSurfaceWithKnotsAndRationalBSplineSurface>(support);
  if (!rational.IsNull()) {
    return rational->BSplineSurfaceWithKnots();
  }
  return occ::down_cast<StepGeom_BSplineSurfaceWithKnots>(support);
}

// Retain every source pole and coedge but falsify one strictly interior
// south-roof weight. The exact changed value remains a finite STEP decimal.
static void authorFalseSouthRoofWeight(const occ::handle<StepData_StepModel>& model) {
  bool changed = false;
  for (int i = 1; i <= model->NbEntities(); ++i) {
    auto roof = occ::down_cast<StepGeom_BSplineSurfaceWithKnotsAndRationalBSplineSurface>(model->Entity(i));
    if (roof.IsNull() || roof->NbUKnots() != 2 || roof->NbVKnots() != 4) {
      continue;
    }
    auto weights = roof->WeightsData();
    if (changed || weights->Value(3, 8) != 2.) {
      throw std::runtime_error("unexpected original south roof weights");
    }
    weights->SetValue(3, 8, 2. + 1. / 131072);
    changed = true;
  }
  if (!changed) {
    throw std::runtime_error("could not author the false south roof weight");
  }
}

// Author one original south basis with a decreasing raw parameter and
// nonuniform weights. The north roof keeps the six OCCT-generated bases.
static occ::handle<StepData_StepModel> authorMixedBasis(const occ::handle<StepData_StepModel>& model,
  const occ::handle<StepData_Protocol>& protocol, bool unsplit, RoofSupportLayout layout) {
  const int supportCuts = layout.cuts;
  const bool nonplanar = layout.inferred == InferredSupportEvents::nonplanar;
  const bool localCells = layout.storedCells >= 2 || layout.separated || layout.inferred != InferredSupportEvents::none;
  const double lastSouthParameter = layout.storedCells == 3 || layout.separated || layout.inferred != InferredSupportEvents::none ? -2. : 0.;
  occ::handle<TCollection_HAsciiString> name = new TCollection_HAsciiString("");
  occ::handle<StepGeom_BSplineCurveWithKnotsAndRationalBSplineCurve> shared =
    new StepGeom_BSplineCurveWithKnotsAndRationalBSplineCurve;
  const double firstSouthParameter = layout.independentOwnerClocks ? 181. : 48.;
  const auto southKnots = localCells ? std::array{lastSouthParameter, 1., 16., firstSouthParameter}
    : supportCuts >= 4 ? std::array{2., 8., 16., 48.} : std::array{10., 17., 23., 478.};
  if (layout.joinedRationalCells) {
    const auto poles = nonplanarSouthPoles(name, std::array{1., .25, 0.});
    shared->Init(name, 3, poles, StepGeom_bscfUnspecified, StepData_LFalse, StepData_LFalse,
                 stepArray(std::array{4, 3, 4}), stepArray(std::array{1., 16., 181.}), StepGeom_ktUnspecified,
                 stepArray(std::array{8., 16., 32., 64., 96., 144., 216.}));
  } else if (nonplanar) {
    const auto poles = nonplanarSouthPoles(name, std::array{1., .5, .25, 0.});
    // The last stored span is the first in model traversal. Its new
    // increasing-raw geometric weight ratio 3/2 acts in reverse traversal.
    const auto weights = layout.independentOwnerClocks
      ? std::array{1., 2., 4., 8., 16., 32., 64., 96., 144., 216.}
      : std::array{1., 2., 4., 8., 16., 32., 64., 64., 64., 64.};
    shared->Init(name, 3, poles, StepGeom_bscfUnspecified, StepData_LFalse, StepData_LFalse,
                 stepArray(std::array{4, 3, 3, 4}), stepArray(southKnots), StepGeom_ktUnspecified, stepArray(weights));
  } else {
    const std::array<double, 10> southX{1., .875, .625, .5, .4375, .34375, .25, .15625, .0625, 0.};
    std::array<occ::handle<StepGeom_CartesianPoint>, 10> poles;
    for (std::size_t i = 0; i < poles.size(); ++i) {
      poles[i] = new StepGeom_CartesianPoint;
      poles[i]->Init2D(name, .25, southX[i]);
    }
    const auto southWeights = (localCells || supportCuts >= 4) ? std::array{1., 2., 4., 8., 4., 2., 1., 1., 1., 1.}
      : std::array{1./64, 1./16, 1./4, 1., 1./2, 1./4, 1./8, 1./32, 1./128, 1./512};
    shared->Init(name, 3, stepArray(poles), StepGeom_bscfUnspecified, StepData_LFalse, StepData_LFalse,
                 stepArray(std::array{4, 3, 3, 4}), stepArray(southKnots), StepGeom_ktUnspecified, stepArray(southWeights));
  }
  const auto x = layout.joinedRationalCells ? std::vector<double>{0., .25, 1.}
                : unsplit ? std::vector<double>{0., .25, .5, 1.}
                         : std::vector<double>{0., 107. / 2048, 29. / 256, 369. / 2048, .25, .5, 1.};
  const auto raw = layout.joinedRationalCells ? std::vector<double>{181., 16., 1.}
                  : localCells ? std::vector<double>{firstSouthParameter, 16., 1., lastSouthParameter}
                  : supportCuts >= 4 ? std::vector<double>{48., 16., 8., 2.}
                  : unsplit ? std::vector<double>{478., 23., 17., 10.}
                           : std::vector<double>{478., 443., 387., 283., 23., 17., 10.};
  const auto station = layout.joinedRationalCells ? std::vector<double>{0., 8., 32.}
                      : (localCells || supportCuts >= 4) ? std::vector<double>{0., 8., 16., 32.}
                      : unsplit ? std::vector<double>{0., 4., 8., 16.}
                               : std::vector<double>{0., 1., 2., 3., 4., 8., 16.};
  const auto northKnots = static_cast<int>(roofSupportKnots(false, layout).size());
  const auto southKnotCount = static_cast<int>(roofSupportKnots(true, layout).size());
  std::vector<bool> changed(x.size() - 1);
  for (int i = 1; i <= model->NbEntities(); ++i) {
    auto curve = occ::down_cast<StepGeom_SurfaceCurve>(model->Entity(i));
    if (curve.IsNull() || curve->NbAssociatedGeometry() != 2) {
      continue;
    }
    occ::handle<StepGeom_Pcurve> north, south;
    for (int j = 1; j <= 2; ++j) {
      auto owner = curve->AssociatedGeometryValue(j).Pcurve();
      if (owner.IsNull()) {
        continue;
      }
      const auto roof = stepRoofKnots(owner->BasisSurface());
      if (!roof.IsNull() && roof->NbUKnots() == northKnots && roof->NbVKnots() == 2) {
        north = owner;
      }
      if (!roof.IsNull() && roof->NbUKnots() == 2 && roof->NbVKnots() == southKnotCount) {
        south = owner;
      }
    }
    if (north.IsNull() || south.IsNull()) {
      continue;
    }
    auto spatial = occ::down_cast<StepGeom_BSplineCurveWithKnots>(curve->Curve3d());
    auto isolated = occ::down_cast<StepGeom_BSplineCurveWithKnots>(north->ReferenceToCurve()->ItemsValue(1));
    if (spatial.IsNull() || isolated.IsNull() || isolated->NbControlPointsList() != 4 || isolated->NbKnots() != 2) {
      throw std::runtime_error("unexpected independently authored seam bases");
    }
    const double scale = nonplanar ? nonplanarWidth : 1.;
    const double a = spatial->ControlPointsListValue(1)->CoordinatesValue(1) / scale;
    const double b = spatial->ControlPointsListValue(spatial->NbControlPointsList())->CoordinatesValue(1) / scale;
    const auto cell = static_cast<std::size_t>(std::find(x.begin(), x.end(), std::min(a, b)) - x.begin());
    if (cell >= changed.size() || changed[cell] || x[cell + 1] != std::max(a, b)) {
      throw std::runtime_error("unexpected independently authored seam junction");
    }
    changed[cell] = true;
    if (unsplit && cell < static_cast<std::size_t>(layout.storedCells)) {
      if (spatial->NbControlPointsList() != 7 || spatial->NbKnots() != 3) {
        throw std::runtime_error("the unsplit model edge needs its original material knot");
      }
      spatial->SetKnots(stepArray(std::array{station[cell], (station[cell] + station[cell + 1]) / 2., station[cell + 1]}));
    } else {
      if (spatial->NbControlPointsList() != 4 || spatial->NbKnots() != 2) {
        throw std::runtime_error("an explicit seam edge needs one original cubic span");
      }
      spatial->SetKnots(stepArray(std::array{station[cell], station[cell + 1]}));
    }
    if (layout.rationalModel && (cell == 0 || layout.joinedRationalCells)) {
      // The geometric homogeneous weights induce independent positive
      // projective parameters on the original nonplanar Bézier cells.
      auto weights = cell == 0 ? std::array{1., 2., 4., 8.} : std::array{1., 4., 16., 64.};
      if (a > b) {
        std::reverse(weights.begin(), weights.end());
      }
      occ::handle<StepGeom_BSplineCurveWithKnotsAndRationalBSplineCurve> rational =
        new StepGeom_BSplineCurveWithKnotsAndRationalBSplineCurve;
      rational->Init(name, 3, spatial->ControlPointsList(), StepGeom_bscfUnspecified, StepData_LFalse, StepData_LFalse,
                     spatial->KnotMultiplicities(), stepArray(std::array{station[cell], station[cell + 1]}),
                     StepGeom_ktUnspecified, stepArray(weights));
      curve->SetCurve3d(rational);
    }
    if (unsplit) {
      // The first two Cartesian nets are exact half restrictions of the
      // original north cubic. The homogeneous weights express the original
      // rational raw map with a common dyadic scale on each basis.
      const double firstRawEnd = layout.independentOwnerClocks ? 167.
        : supportCuts >= 5 ? 4622. : (supportCuts == 4 || localCells) ? 107. : 457.;
      const auto firstWeights = layout.independentOwnerClocks ? std::array{8., 12., 18., 27.}
        : supportCuts >= 5 ? std::array{1., 3., 9., 27.}
        : (supportCuts == 4 || localCells) ? std::array{1./8, 1./4, 1./2, 1.} : std::array{27./64, 9./16, 3./4, 1.};
      const std::array<std::array<double, 2>, 3> domains{{{2., firstRawEnd}, {700., localCells ? 715. : 706.}, {850., 862.}}};
      const std::array<std::array<double, 4>, 3> weights{{
        firstWeights, {8., 12., 18., 27.}, {1., 1./2, 1./4, 1./8}}};
      auto w = weights[cell];
      if (a > b) {
        std::reverse(w.begin(), w.end());
      }
      occ::handle<StepGeom_BSplineCurveWithKnotsAndRationalBSplineCurve> rational =
        new StepGeom_BSplineCurveWithKnotsAndRationalBSplineCurve;
      rational->Init(name, 3, isolated->ControlPointsList(), StepGeom_bscfUnspecified, StepData_LFalse, StepData_LFalse,
                     isolated->KnotMultiplicities(), stepArray(domains[cell]), StepGeom_ktUnspecified, stepArray(w));
      north->ReferenceToCurve()->Items()->SetValue(1, rational);
    } else {
      // Distinct complete raw intervals make the independent ownership visible.
      const double domain = 10. + 8. * cell;
      isolated->SetKnots(stepArray(std::array{domain, domain + 7.}));
    }
    StepGeom_TrimmingSelect lo, hi;
    lo.SetParameterValue(raw[cell + 1]);
    hi.SetParameterValue(raw[cell]);
    occ::handle<StepGeom_TrimmedCurve> trim = new StepGeom_TrimmedCurve;
    trim->Init(name, shared, stepArray(std::array{lo}), stepArray(std::array{hi}), a > b, StepGeom_tpParameter);
    south->ReferenceToCurve()->Items()->SetValue(1, trim);
    curve->SetMasterRepresentation(StepGeom_pscrCurve3d);
  }
  if (std::find(changed.begin(), changed.end(), false) != changed.end()) {
    throw std::runtime_error("could not author all mixed-basis coedges");
  }
  // Retain the original product assembly and rebuild the reference closure
  // after replacing the six individual south coedge bases.
  occ::handle<StepData_StepModel> out = new StepData_StepModel;
  out->GetFromAnother(model);
  for (int i = 1; i <= model->NbEntities(); ++i) {
    const auto entity = model->Entity(i);
    const std::string type = entity->DynamicType()->Name();
    if (type == "StepShape_ShapeDefinitionRepresentation" || type == "StepBasic_ApplicationProtocolDefinition"
        || type == "StepBasic_ProductRelatedProductCategory") {
      out->AddWithRefs(entity, protocol);
    }
  }
  return out;
}
#endif

int main(int argc, char** argv) {
  try {
    const std::string mode = argc > 1 ? argv[1] : "";
    const int supportCuts = mode == "write-six-knots-mixed-unsplit" ? 6
      : mode == "write-five-knots-mixed-unsplit" ? 5 : mode == "write-four-knots-mixed-unsplit" ? 4 : 3;
    const bool sameCellAsynchronous = mode == "write-same-cell-asynchronous" || mode == "write-same-cell-redundant-knot";
    const bool sharedInferred = mode == "write-shared-inferred-knot" || mode == "write-shared-inferred-redundant-knot";
    const bool sharedPair = mode == "write-shared-inferred-knot-pair" || mode == "write-shared-inferred-pair-redundant-knot";
    const bool offsetPair = mode == "write-shared-inferred-offset-pair" || mode == "write-mixed-inferred-redundant-knot";
    const bool missingSouthJoint = mode == "write-joined-rational-cells-missing-south-joint";
    const bool measureJoined = mode == "measure-joined-rational-cells";
    const bool joinedRationalCells = mode == "write-joined-rational-cells" || missingSouthJoint || measureJoined;
    const bool falseRoofWeight = mode == "write-nonplanar-false-roof-weight";
    const bool rationalRoofs = mode == "write-nonplanar-rational-roofs" || falseRoofWeight || joinedRationalCells;
    const bool independentOwnerClocks = mode == "write-nonplanar-independent-owner-clocks" || rationalRoofs;
    const bool rationalModel = mode == "write-nonplanar-rational-inferred-mixed" || independentOwnerClocks;
    const bool nonplanar = mode == "write-nonplanar-inferred-mixed" || rationalModel;
    const auto inferred = nonplanar ? InferredSupportEvents::nonplanar
      : offsetPair ? InferredSupportEvents::offsetPair : sharedPair ? InferredSupportEvents::sharedPair
      : sharedInferred ? InferredSupportEvents::shared
      : sameCellAsynchronous ? InferredSupportEvents::asynchronous : InferredSupportEvents::none;
    const bool noModelKnot = mode == "write-same-cell-asynchronous" || mode == "write-shared-inferred-knot"
      || mode == "write-shared-inferred-knot-pair" || mode == "write-shared-inferred-offset-pair" || nonplanar;
    const int storedCells = mode == "write-three-support-cells" ? 3 : mode == "write-two-support-cells" ? 2
      : noModelKnot ? 0 : 1;
    const RoofSupportLayout layout{supportCuts, storedCells, mode == "write-separated-support-cells", inferred,
                                   rationalModel, independentOwnerClocks, rationalRoofs, joinedRationalCells, missingSouthJoint};
    const bool unsplitBasis = mode == "write-three-knots-mixed-unsplit" || supportCuts > 3 || storedCells >= 2
      || layout.separated || inferred != InferredSupportEvents::none;
    const bool mixedBasis = mode == "write-three-knots-mixed-basis" || unsplitBasis;
    const bool three = mode == "write-three-knots" || mixedBasis;
    const bool authoredPlanes = mode == "write-knot" || three;
    const bool crease = authoredPlanes || mode == "write-knot-default-planes";
    auto frame = [&](const gp_Pnt& origin, const gp_Dir& normal, const gp_Dir& u) -> std::optional<gp_Ax3> {
      if (!authoredPlanes) {
        return std::nullopt;
      }
      return gp_Ax3(origin, normal, u);
    };
    const bool reordered = mode == "write-reordered" || crease;
    const double width = nonplanar ? nonplanarWidth : 1.;
    auto side = [&](double y, bool reverse) {
      const bool south = y == 0.;
      std::vector<gp_Pnt> points{{0, y, -1}, {width, y, -1}, {width, y, roofHeight(1, y, crease, three, south, layout)}};
      if (three) {
        const auto xs = roofSupportKnots(south, layout);
        for (int i = static_cast<int>(xs.size()) - 2; i > 0; --i) {
          points.emplace_back(width * xs[i], y, roofHeight(xs[i], y, crease, true, south, layout));
        }
      } else if (crease) {
        points.emplace_back(.5, y, roofHeight(.5, y, true));
      }
      points.emplace_back(0, y, 0);
      return planar(points, reverse, frame({0, y, 0}, {0, -1, 0}, {0, 0, -1}));
    };
    const std::array<TopoDS_Face, 7> source{
      roof(false, reordered ? 4 : 0, crease, three, unsplitBasis, layout),
      roof(true, reordered ? 5 : 0, crease, three, unsplitBasis, layout),
      planar({{width, 0, -1}, {width, .75, -1}, {width, .75, roofHeight(1, .75, crease, three, false, layout)},
             {width, .25, roofHeight(1, .25, crease, three, false, layout)}, {width, 0, roofHeight(1, 0, crease, three, true, layout)}}, false,
             frame({width, 0, 0}, {1, 0, 0}, {0, 0, 1})),
      planar({{0, 0, -1}, {0, .75, -1}, {0, .75, 0}, {0, .25, 0}, {0, 0, 0}}, true,
             frame({0, 0, 0}, {1, 0, 0}, {0, 0, 1})),
      side(.75, true),
      side(0, false),
      planar({{0, 0, -1}, {width, 0, -1}, {width, .75, -1}, {0, .75, -1}}, true,
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
        // Retain all authored dyadic coefficients in the pinned writer.
        auto model = writer.Model();
        auto protocol = occ::down_cast<StepData_Protocol>(model->Protocol());
        if (protocol.IsNull()) {
          throw std::runtime_error("independent STEP model has no protocol");
        }
        if (mixedBasis) {
          model = authorMixedBasis(model, protocol, unsplitBasis, layout);
        }
        if (falseRoofWeight) {
          authorFalseSouthRoofWeight(model);
        }
        StepData_StepWriter exactWriter(model);
        exactWriter.FloatWriter().SetFormat("%.18f", true);
        if (supportCuts >= 5) {
          // The new chart coefficients use up to 19 fractional digits.
          // They have magnitude below 2, while the raw knots can be larger
          // integers. The separate format keeps both within OCCT's buffer.
          exactWriter.FloatWriter().SetFormatForRange("%.19f", 0., 2.);
        }
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
    if (argc != 1 && !(argc == 2 && measureJoined)) {
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
