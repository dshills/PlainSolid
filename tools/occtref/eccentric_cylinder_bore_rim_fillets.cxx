#include <BRepAdaptor_Curve.hxx>
#include <BRepAdaptor_Surface.hxx>
#include <BRepAlgoAPI_Cut.hxx>
#include <BRepCheck_Analyzer.hxx>
#include <BRepFilletAPI_MakeFillet.hxx>
#include <BRepGProp.hxx>
#include <BRepPrimAPI_MakeCylinder.hxx>
#include <GProp_GProps.hxx>
#include <NCollection_IndexedMap.hxx>
#include <TopExp.hxx>
#include <TopTools_ShapeMapHasher.hxx>
#include <TopoDS.hxx>
#include <gp_Ax2.hxx>

#include <array>
#include <cmath>
#include <iomanip>
#include <iostream>
#include <string_view>

using ShapeMap = NCollection_IndexedMap<TopoDS_Shape, TopTools_ShapeMapHasher>;

int main(int argc, char** argv) {
    if (argc != 2) return 1;
    const std::string_view mode(argv[1]);
    if (mode != "lower" && mode != "upper" && mode != "both") return 1;
    constexpr double cx = 1.5, cy = 2, z0 = 2, height = 8, outerRadius = 6.25, boreRadius = 2;
    const std::array<double, 2> radii{{mode == "upper" ? 0. : .5, mode == "lower" ? 0. : .75}};
    const auto outer = BRepPrimAPI_MakeCylinder(gp_Ax2(gp_Pnt(0, 0, z0), gp::DZ()), outerRadius, height).Shape();
    const auto tool = BRepPrimAPI_MakeCylinder(gp_Ax2(gp_Pnt(cx, cy, z0), gp::DZ()), boreRadius, height).Shape();
    BRepAlgoAPI_Cut cut(outer, tool);
    const auto source = cut.Shape();
    if (!cut.IsDone() || source.IsNull() || !BRepCheck_Analyzer(source).IsValid()) return 2;
    BRepFilletAPI_MakeFillet fillet(source);
    ShapeMap sourceEdges;
    TopExp::MapShapes(source, TopAbs_EDGE, sourceEdges);
    std::array<bool, 2> added{};
    auto close = [](double a, double b) { return std::abs(a - b) < 1e-9; };
    for (int index = 1; index <= sourceEdges.Extent(); ++index) {
        const auto edge = TopoDS::Edge(sourceEdges(index));
        const BRepAdaptor_Curve curve(edge);
        if (curve.GetType() != GeomAbs_Circle) continue;
        const auto circle = curve.Circle();
        const auto center = circle.Location();
        if (!close(circle.Radius(), boreRadius) || !close(center.X(), cx) || !close(center.Y(), cy)) continue;
        const int end = close(center.Z(), z0) ? 0 : close(center.Z(), z0 + height) ? 1 : -1;
        if (end < 0 || added[end]) return 3;
        if (radii[end] > 0) fillet.Add(radii[end], edge);
        added[end] = true;
    }
    if (!added[0] || !added[1]) return 4;
    fillet.Build();
    if (!fillet.IsDone()) return 5;
    const auto shape = fillet.Shape();
    if (shape.IsNull() || !BRepCheck_Analyzer(shape).IsValid()) return 6;

    ShapeMap solids, faces, edges;
    TopExp::MapShapes(shape, TopAbs_SOLID, solids);
    TopExp::MapShapes(shape, TopAbs_FACE, faces);
    TopExp::MapShapes(shape, TopAbs_EDGE, edges);
    GProp_GProps mass, area;
    BRepGProp::VolumeProperties(shape, mass, 1e-10, true);
    BRepGProp::SurfaceProperties(shape, area, 1e-10);
    std::cout << std::setprecision(17)
              << "{\"Solids\":" << solids.Extent() << ",\"Faces\":" << faces.Extent()
              << ",\"Volume\":" << mass.Mass() << ",\"Area\":" << area.Mass() << ",\"Rims\":[";
    bool first = true;
    for (int index = 1; index <= edges.Extent(); ++index) {
        const BRepAdaptor_Curve curve(TopoDS::Edge(edges(index)));
        if (curve.GetType() != GeomAbs_Circle) continue;
        const auto c = curve.Circle();
        if (!first) std::cout << ',';
        first = false;
        std::cout << '[' << c.Location().X() << ',' << c.Location().Y() << ',' << c.Location().Z() << ',' << c.Radius() << ']';
    }
    std::cout << "],\"Toroids\":[";
    first = true;
    for (int index = 1; index <= faces.Extent(); ++index) {
        const BRepAdaptor_Surface surface(TopoDS::Face(faces(index)));
        if (surface.GetType() != GeomAbs_Torus) continue;
        const auto torus = surface.Torus();
        if (!first) std::cout << ',';
        first = false;
        std::cout << '[' << torus.Location().X() << ',' << torus.Location().Y() << ',' << torus.Location().Z()
                  << ',' << torus.MajorRadius() << ',' << torus.MinorRadius() << ']';
    }
    std::cout << "]}\n";
}
