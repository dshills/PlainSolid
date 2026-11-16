#include <BRepAdaptor_Curve.hxx>
#include <BRepAdaptor_Surface.hxx>
#include <BRepCheck_Analyzer.hxx>
#include <BRepGProp.hxx>
#include <BRep_Tool.hxx>
#include <GProp_GProps.hxx>
#include <Geom_BSplineCurve.hxx>
#include <Geom_BSplineSurface.hxx>
#include <IGESControl_Reader.hxx>
#include <NCollection_IndexedMap.hxx>
#include <STEPControl_Reader.hxx>
#include <TopExp.hxx>
#include <TopExp_Explorer.hxx>
#include <TopTools_ShapeMapHasher.hxx>
#include <TopoDS.hxx>
#include <algorithm>
#include <array>
#include <string_view>
#include <cmath>
#include <iomanip>
#include <iostream>

using ShapeMap = NCollection_IndexedMap<TopoDS_Shape, TopTools_ShapeMapHasher>;

using Band = std::array<double, 6>;

static bool readRationalCone(const BRepAdaptor_Surface& face, Band& data) {
    const auto s = face.BSpline();
    if (s.IsNull() || s->UDegree() != 2 || s->VDegree() != 1 ||
        s->NbUPoles() != (s->IsUPeriodic() ? 8 : 9) || s->NbVPoles() != 2 ||
        (!s->IsURational() && !s->IsVRational()) || s->NbUKnots() != 5 || s->NbVKnots() != 2) return false;
    const std::array<std::array<int, 2>, 9> azimuth{{
        {1, 0}, {1, 1}, {0, 1}, {-1, 1}, {-1, 0}, {-1, -1}, {0, -1}, {1, -1}, {1, 0},
    }};
    constexpr std::array<int, 9> weights{{1, 1, 2, 2, 4, 2, 2, 1, 1}};
    auto close = [](double a, double b) { return std::abs(a - b) < 1e-10; };
    const double x = (s->Pole(1, 1).X() + s->Pole(5, 1).X()) / 2;
    const double y = s->Pole(1, 1).Y();
    const double base = s->Weight(1, 1);
    const double sign = s->Pole(1, 2).Z() > s->Pole(1, 1).Z() ? 1. : -1.;
    for (int u = 1; u <= 5; ++u) {
        const int multiplicity = !s->IsUPeriodic() && (u == 1 || u == 5) ? 3 : 2;
        if (s->UKnot(u) != u - 1 || s->UMultiplicity(u) != multiplicity) return false;
    }
    if (s->VKnot(1) != 0 || s->VKnot(2) != 1 || s->VMultiplicity(1) != 2 || s->VMultiplicity(2) != 2) return false;
    data[0] = x;
    data[1] = y;
    for (int v = 1; v <= 2; ++v) {
        const double z = s->Pole(1, v).Z(), r = s->Pole(1, v).X() - x;
        if (r <= 0) return false;
        for (int u = 1; u <= s->NbUPoles(); ++u) {
            const auto p = s->Pole(u, v);
            const auto xy = azimuth[u - 1];
            if (!close(p.X(), x + xy[0] * r) || !close(p.Y(), y + sign * xy[1] * r) ||
                !close(p.Z(), z) || !close(s->Weight(u, v), base * weights[u - 1])) return false;
        }
        data[2 * v] = z;
        data[2 * v + 1] = r;
    }
    if (data[2] == data[4]) return false;
    if (sign < 0) {
        std::swap(data[2], data[4]);
        std::swap(data[3], data[5]);
    }
    return true;
}

int main(int argc, char** argv) {
    if (argc != 2) return 1;
    const std::string_view path(argv[1]);
    TopoDS_Shape shape;
    if (path.size() >= 4 && path.substr(path.size() - 4) == ".igs") {
        IGESControl_Reader reader;
        if (reader.ReadFile(argv[1]) != IFSelect_RetDone || reader.TransferRoots() != 1) return 2;
        shape = reader.OneShape();
    } else {
        STEPControl_Reader reader;
        if (reader.ReadFile(argv[1]) != IFSelect_RetDone || reader.TransferRoots() != 1) return 2;
        shape = reader.OneShape();
    }
    if (shape.IsNull() || !BRepCheck_Analyzer(shape).IsValid()) return 3;
    ShapeMap solids, faces, edges, vertices;
    TopExp::MapShapes(shape, TopAbs_SOLID, solids);
    TopExp::MapShapes(shape, TopAbs_FACE, faces);
    TopExp::MapShapes(shape, TopAbs_EDGE, edges);
    TopExp::MapShapes(shape, TopAbs_VERTEX, vertices);
    if (solids.Extent() != 1 || faces.Extent() != 6 || edges.Extent() != 10 || vertices.Extent() != 6) return 4;

    GProp_GProps volume, area;
    BRepGProp::VolumeProperties(shape, volume, 1e-10, true);
    BRepGProp::SurfaceProperties(shape, area, 1e-10);
    std::cout << std::setprecision(17) << "{\"volume\":" << volume.Mass()
              << ",\"area\":" << area.Mass() << ",\"vertices\":[";
    for (int i = 1; i <= vertices.Extent(); ++i) {
        if (i != 1) std::cout << ',';
        const auto p = BRep_Tool::Pnt(TopoDS::Vertex(vertices(i)));
        std::cout << '[' << p.X() << ',' << p.Y() << ',' << p.Z() << ']';
    }
    const double pi = std::acos(-1.);
    int circles = 0, meridians = 0, generators = 0;
    for (int i = 1; i <= edges.Extent(); ++i) {
        const BRepAdaptor_Curve c(TopoDS::Edge(edges(i)));
        if (c.GetType() == GeomAbs_Circle) {
            const double span = c.LastParameter() - c.FirstParameter();
            if (std::abs(c.Circle().Axis().Direction().Z()) == 1 && std::abs(span - 2*pi) < 1e-8)
                ++circles;
            else if (std::abs(c.Circle().Axis().Direction().Y()) == 1 && span > 0 && span < pi)
                ++meridians;
            else
                return 5;
        } else if (c.GetType() == GeomAbs_Line ||
                   (c.GetType() == GeomAbs_BSplineCurve && c.BSpline()->Degree() == 1)) {
            ++generators;
        } else {
            return 5;
        }
    }
    if (circles != 6 || meridians != 2 || generators != 2) return 5;
    std::cout << "],\"edges\":[" << circles << ',' << meridians << ',' << generators << "],\"faces\":[";
    for (int i = 1; i <= faces.Extent(); ++i) {
        const auto face = TopoDS::Face(faces(i));
        const BRepAdaptor_Surface s(face);
        if (i != 1) std::cout << ',';
        switch (s.GetType()) {
        case GeomAbs_Plane: {
            int wires = 0;
            for (TopExp_Explorer w(face, TopAbs_WIRE); w.More(); w.Next()) ++wires;
            std::cout << "[0," << s.Plane().Location().Z() << ',' << wires << ']';
            break;
        }
        case GeomAbs_Cylinder: {
            const auto cyl = s.Cylinder();
            const auto c = cyl.Location();
            const auto a = s.Value(0, s.FirstVParameter()), b = s.Value(0, s.LastVParameter());
            if (std::abs(cyl.Axis().Direction().Z()) != 1) return 6;
            std::cout << "[1," << c.X() << ',' << c.Y() << ',' << cyl.Radius()
                      << ',' << std::min(a.Z(),b.Z()) << ',' << std::max(a.Z(),b.Z()) << ']';
            break;
        }
        case GeomAbs_Torus: {
            const auto torus = s.Torus();
            const auto c = torus.Location();
            const auto a = s.Value(0, s.FirstVParameter()), b = s.Value(0, s.LastVParameter());
            if (std::abs(torus.Axis().Direction().Z()) != 1) return 6;
            std::cout << "[2," << c.X() << ',' << c.Y() << ',' << c.Z() << ',' << torus.MajorRadius()
                      << ',' << torus.MinorRadius() << ',' << std::min(a.Z(),b.Z()) << ',' << std::max(a.Z(),b.Z()) << ']';
            break;
        }
        case GeomAbs_SurfaceOfRevolution: {
            const auto axis = s.AxeOfRevolution();
            auto a = s.Value(0, s.FirstVParameter()), b = s.Value(0, s.LastVParameter());
            if (std::abs(axis.Direction().Z()) != 1) return 6;
            if (a.Z() > b.Z()) std::swap(a,b);
            const auto c = axis.Location();
            std::cout << "[3," << c.X() << ',' << c.Y() << ',' << a.Z() << ',' << a.X()-c.X()
                      << ',' << b.Z() << ',' << b.X()-c.X() << ']';
            break;
        }
        case GeomAbs_BSplineSurface: {
            Band band{};
            if (!readRationalCone(s, band)) return 6;
            std::cout << "[3";
            for (double scalar : band) std::cout << ',' << scalar;
            std::cout << ']';
            break;
        }
        default:
            return 6;
        }
    }
    std::cout << "]}\n";
}
