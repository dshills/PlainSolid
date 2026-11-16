#include <BRepAdaptor_Curve.hxx>
#include <BRepAdaptor_Surface.hxx>
#include <BRepCheck_Analyzer.hxx>
#include <BRepGProp.hxx>
#include <BRep_Tool.hxx>
#include <GProp_GProps.hxx>
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
#include <cmath>
#include <iomanip>
#include <iostream>
#include <string_view>

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
            if (!close(p.X(), x + xy[0] * r) || !close(p.Y(), y + xy[1] * r) ||
                !close(p.Z(), z) || !close(s->Weight(u, v), base * weights[u - 1])) return false;
        }
        data[2 * v] = z;
        data[2 * v + 1] = r;
    }
    return data[2] < data[4];
}

static bool readRevolvedCone(const BRepAdaptor_Surface& s, Band& data) {
    gp_Pnt center;
    if (s.GetType() == GeomAbs_SurfaceOfRevolution) {
        const auto axis = s.AxeOfRevolution();
        center = axis.Location();
        if (axis.Direction().X() != 0 || axis.Direction().Y() != 0) return false;
    } else if (s.GetType() == GeomAbs_Cone) {
        const auto cone = s.Cone();
        center = cone.Location();
        if (cone.Axis().Direction().X() != 0 || cone.Axis().Direction().Y() != 0) return false;
    } else {
        return false;
    }
    auto a = s.Value(s.FirstUParameter(), s.FirstVParameter());
    auto b = s.Value(s.FirstUParameter(), s.LastVParameter());
    if (a.Z() > b.Z()) std::swap(a, b);
    auto radius = [&](const gp_Pnt& p) { return std::hypot(p.X() - center.X(), p.Y() - center.Y()); };
    data = {center.X(), center.Y(), a.Z(), radius(a), b.Z(), radius(b)};
    return a.Z() < b.Z();
}

int main(int argc, char** argv) {
    if (argc != 2) return 1;
    const std::string_view name(argv[1]);
    const bool iges = name.size() >= 4 && name.substr(name.size() - 4) == ".igs";
    TopoDS_Shape shape;
    if (iges) {
        IGESControl_Reader reader;
        if (reader.ReadFile(argv[1]) != IFSelect_RetDone || reader.TransferRoots() != 1) return 2;
        shape = reader.OneShape();
    } else {
        STEPControl_Reader reader;
        if (reader.ReadFile(argv[1]) != IFSelect_RetDone || reader.TransferRoots() != 1) return 2;
        shape = reader.OneShape();
    }
    if (shape.IsNull() || !BRepCheck_Analyzer(shape).IsValid()) return 3;
    GProp_GProps mass, area;
    BRepGProp::VolumeProperties(shape, mass, 1e-10, true);
    BRepGProp::SurfaceProperties(shape, area, 1e-10);
    ShapeMap solids, faces, edges, vertices;
    TopExp::MapShapes(shape, TopAbs_SOLID, solids);
    TopExp::MapShapes(shape, TopAbs_FACE, faces);
    TopExp::MapShapes(shape, TopAbs_EDGE, edges);
    TopExp::MapShapes(shape, TopAbs_VERTEX, vertices);
    std::cout << std::setprecision(17) << "{\"Solids\":" << solids.Extent()
              << ",\"Faces\":" << faces.Extent() << ",\"Volume\":" << mass.Mass()
              << ",\"Area\":" << area.Mass() << ",\"Vertices\":[";
    for (int i = 1; i <= vertices.Extent(); ++i) {
        if (i > 1) std::cout << ',';
        const auto p = BRep_Tool::Pnt(TopoDS::Vertex(vertices(i)));
        std::cout << '[' << p.X() << ',' << p.Y() << ',' << p.Z() << ']';
    }
    std::cout << "],\"Edges\":[";
    for (int i = 1; i <= edges.Extent(); ++i) {
        if (i > 1) std::cout << ',';
        const auto edge = TopoDS::Edge(edges(i));
        const BRepAdaptor_Curve curve(edge);
        const bool circle = curve.GetType() == GeomAbs_Circle;
        if (curve.GetType() != GeomAbs_Line && !circle) return 4;
        if (circle && std::abs(curve.LastParameter() - curve.FirstParameter() - std::acos(-1.) / 2) > 1e-8) return 4;
        std::cout << '[' << vertices.FindIndex(TopExp::FirstVertex(edge)) - 1 << ','
                  << vertices.FindIndex(TopExp::LastVertex(edge)) - 1 << ',' << (circle ? 1 : 0) << ']';
    }
    std::cout << "],\"Caps\":[";
    bool first = true;
    for (int i = 1; i <= faces.Extent(); ++i) {
        const auto face = TopoDS::Face(faces(i));
        const BRepAdaptor_Surface surface(face);
        if (surface.GetType() != GeomAbs_Plane) continue;
        int wires = 0;
        for (TopExp_Explorer e(face, TopAbs_WIRE); e.More(); e.Next()) ++wires;
        if (!first) std::cout << ',';
        first = false;
        std::cout << '[' << surface.Plane().Location().Z() << ',' << wires << ']';
    }
    std::cout << "],\"Bands\":[";
    first = true;
    for (int i = 1; i <= faces.Extent(); ++i) {
        const BRepAdaptor_Surface surface(TopoDS::Face(faces(i)));
        if (surface.GetType() == GeomAbs_Plane) continue;
        Band band{};
        if (!(iges ? readRationalCone(surface, band) : readRevolvedCone(surface, band))) return 5;
        if (!first) std::cout << ',';
        first = false;
        std::cout << '[';
        for (std::size_t j = 0; j < band.size(); ++j) {
            if (j) std::cout << ',';
            std::cout << band[j];
        }
        std::cout << ']';
    }
    std::cout << "]}\n";
}
