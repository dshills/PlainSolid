#include <BRepAdaptor_Curve.hxx>
#include <BRepAdaptor_Surface.hxx>
#include <BRepCheck_Analyzer.hxx>
#include <BRepGProp.hxx>
#include <BRep_Tool.hxx>
#include <GProp_GProps.hxx>
#include <IGESControl_Reader.hxx>
#include <NCollection_IndexedMap.hxx>
#include <STEPControl_Reader.hxx>
#include <TopExp.hxx>
#include <TopExp_Explorer.hxx>
#include <TopTools_ShapeMapHasher.hxx>
#include <TopoDS.hxx>

#include <algorithm>
#include <cmath>
#include <iomanip>
#include <iostream>
#include <string_view>

using ShapeMap = NCollection_IndexedMap<TopoDS_Shape, TopTools_ShapeMapHasher>;

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
    GProp_GProps mass, area;
    BRepGProp::VolumeProperties(shape, mass, 1e-10, true);
    BRepGProp::SurfaceProperties(shape, area, 1e-10);
    ShapeMap solids, faces, edges, vertices;
    TopExp::MapShapes(shape, TopAbs_SOLID, solids);
    TopExp::MapShapes(shape, TopAbs_FACE, faces);
    TopExp::MapShapes(shape, TopAbs_EDGE, edges);
    TopExp::MapShapes(shape, TopAbs_VERTEX, vertices);
    std::cout << std::setprecision(17)
              << "{\"Solids\":" << solids.Extent() << ",\"Faces\":" << faces.Extent()
              << ",\"Volume\":" << mass.Mass() << ",\"Area\":" << area.Mass() << ",\"Vertices\":[";
    for (int i = 1; i <= vertices.Extent(); ++i) {
        if (i > 1) std::cout << ',';
        const gp_Pnt p = BRep_Tool::Pnt(TopoDS::Vertex(vertices(i)));
        std::cout << '[' << p.X() << ',' << p.Y() << ',' << p.Z() << ']';
    }
    std::cout << "],\"Edges\":[";
    for (int i = 1; i <= edges.Extent(); ++i) {
        if (i > 1) std::cout << ',';
        const TopoDS_Edge edge = TopoDS::Edge(edges(i));
        const BRepAdaptor_Curve c(edge);
        const bool circular = c.GetType() == GeomAbs_Circle;
        if (c.GetType() != GeomAbs_Line && !circular) return 4;
        if (circular && std::abs(c.LastParameter() - c.FirstParameter() - std::acos(-1.) / 2) > 1e-8) return 5;
        const auto va = TopExp::FirstVertex(edge);
        const auto vb = TopExp::LastVertex(edge);
        std::cout << '[' << vertices.FindIndex(va) - 1 << ',' << vertices.FindIndex(vb) - 1 << ','
                  << (circular ? 1 : 0) << ']';
    }
    std::cout << "],\"Caps\":[";
    bool first = true;
    for (int i = 1; i <= faces.Extent(); ++i) {
        const auto f = TopoDS::Face(faces(i));
        const BRepAdaptor_Surface b(f);
        if (b.GetType() != GeomAbs_Plane) continue;
        if (!first) std::cout << ',';
        first = false;
        int wireCount = 0;
        for (TopExp_Explorer w(f, TopAbs_WIRE); w.More(); w.Next()) ++wireCount;
        std::cout << '[' << b.Plane().Location().Z() << ',' << wireCount << ']';
    }
    std::cout << "],\"Cylinders\":[";
    first = true;
    for (int i = 1; i <= faces.Extent(); ++i) {
        const BRepAdaptor_Surface b(TopoDS::Face(faces(i)));
        if (b.GetType() != GeomAbs_Cylinder) continue;
        if (!first) std::cout << ',';
        first = false;
        const auto surface = b.Cylinder();
        const auto center = surface.Location();
        const double za = b.Value(0., b.FirstVParameter()).Z(), zb = b.Value(0., b.LastVParameter()).Z();
        std::cout << '[' << center.X() << ',' << center.Y() << ',' << surface.Radius()
                  << ',' << std::min(za, zb) << ',' << std::max(za, zb) << ']';
    }
    std::cout << "],\"Toroids\":[";
    first = true;
    for (int i = 1; i <= faces.Extent(); ++i) {
        const BRepAdaptor_Surface b(TopoDS::Face(faces(i)));
        if (b.GetType() != GeomAbs_Torus) continue;
        if (!first) std::cout << ',';
        first = false;
        const auto surface = b.Torus();
        const auto center = surface.Location();
        const double lo = b.FirstVParameter(), hi = b.LastVParameter();
        if (std::abs(hi - lo - std::acos(-1.) / 2) > 1e-8) return 6;
        const auto a = b.Value(0., lo);
        const auto c = b.Value(0., hi);
        const double ra = std::hypot(a.X() - center.X(), a.Y() - center.Y());
        const double rb = std::hypot(c.X() - center.X(), c.Y() - center.Y());
        std::cout << '[' << center.X() << ',' << center.Y() << ',' << center.Z()
                  << ',' << surface.MajorRadius() << ',' << surface.MinorRadius()
                  << ',' << std::min(ra, rb) << ',' << std::max(ra, rb)
                  << ',' << std::min(a.Z(), c.Z()) << ',' << std::max(a.Z(), c.Z()) << ']';
    }
    std::cout << "]}\n";
}
