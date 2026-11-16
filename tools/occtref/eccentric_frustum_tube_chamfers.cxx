#include <BRepAdaptor_Curve.hxx>
#include <BRepAdaptor_Surface.hxx>
#include <BRepAlgoAPI_Cut.hxx>
#include <BRepCheck_Analyzer.hxx>
#include <BRepFilletAPI_MakeChamfer.hxx>
#include <BRepGProp.hxx>
#include <BRepPrimAPI_MakeCone.hxx>
#include <GProp_GProps.hxx>
#include <TopExp.hxx>
#include <TopExp_Explorer.hxx>
#include <NCollection_IndexedMap.hxx>
#include <TopTools_ShapeMapHasher.hxx>
#include <TopoDS.hxx>
#include <gp_Ax2.hxx>
#include <gp_Dir.hxx>

#include <array>
#include <cmath>
#include <iomanip>
#include <iostream>
#include <string_view>

int main(int argc, char** argv) {
    const bool diagonal = argc == 2 && std::string_view(argv[1]) == "diagonal";
    if (argc > 2 || (argc == 2 && !diagonal)) return 1;
    const double z0 = 2.0, height = 8.0;
    const std::array<std::array<double, 2>, 2> radii{{{5, 7}, {2, 3}}};
    const std::array<std::array<double, 2>, 2> radial{{{.25, .5}, {.1875, .375}}};
    const std::array<std::array<double, 2>, 2> axial{{{2, 1}, {1, 2}}};
    const auto outer = BRepPrimAPI_MakeCone(gp_Ax2(gp_Pnt(0, 0, z0), gp::DZ()), radii[0][0], radii[0][1], height).Shape();
    const auto inner = BRepPrimAPI_MakeCone(gp_Ax2(gp_Pnt(diagonal ? 1.5 : 2.5, diagonal ? 2.0 : 0.0, z0), gp::DZ()), radii[1][0], radii[1][1], height).Shape();
    BRepAlgoAPI_Cut subtract(outer, inner);
    const auto tube = subtract.Shape();
    if (!subtract.IsDone() || tube.IsNull() || !BRepCheck_Analyzer(tube).IsValid()) return 2;
    BRepFilletAPI_MakeChamfer chamfer(tube);
    std::array<std::array<bool, 2>, 2> selected{};
    for (TopExp_Explorer faces(tube, TopAbs_FACE); faces.More(); faces.Next()) {
        const auto cap = TopoDS::Face(faces.Current());
        if (BRepAdaptor_Surface(cap).GetType() != GeomAbs_Plane) continue;
        for (TopExp_Explorer edges(cap, TopAbs_EDGE); edges.More(); edges.Next()) {
            const auto rim = TopoDS::Edge(edges.Current());
            const BRepAdaptor_Curve curve(rim);
            if (curve.GetType() != GeomAbs_Circle) continue;
            const auto circle = curve.Circle();
            const int side = circle.Location().X() > 1 ? 1 : 0;
            const int end = circle.Location().Z() > z0 + height / 2 ? 1 : 0;
            if (selected[side][end] || std::abs(circle.Radius() - radii[side][end]) > 1e-10) return 3;
            const double slope = (radii[side][1] - radii[side][0]) / height;
            chamfer.Add(radial[side][end], axial[side][end] * std::hypot(1., slope), rim, cap);
            selected[side][end] = true;
        }
    }
    for (const auto& side : selected) for (bool end : side) if (!end) return 4;
    chamfer.Build();
    if (!chamfer.IsDone()) return 5;
    const auto result = chamfer.Shape();
    if (result.IsNull() || !BRepCheck_Analyzer(result).IsValid()) return 6;

    NCollection_IndexedMap<TopoDS_Shape, TopTools_ShapeMapHasher> solids, faces, edges;
    TopExp::MapShapes(result, TopAbs_SOLID, solids);
    TopExp::MapShapes(result, TopAbs_FACE, faces);
    TopExp::MapShapes(result, TopAbs_EDGE, edges);
    GProp_GProps volume, area;
    BRepGProp::VolumeProperties(result, volume, 1e-10, true);
    BRepGProp::SurfaceProperties(result, area, 1e-10);
    std::cout << std::setprecision(17) << "{\"Solids\":" << solids.Extent()
              << ",\"Faces\":" << faces.Extent() << ",\"Volume\":" << volume.Mass()
              << ",\"Area\":" << area.Mass() << ",\"Rims\":[";
    bool first = true;
    for (int i = 1; i <= edges.Extent(); ++i) {
        const BRepAdaptor_Curve curve(TopoDS::Edge(edges(i)));
        if (curve.GetType() != GeomAbs_Circle) continue;
        const auto c = curve.Circle();
        if (!first) std::cout << ',';
        first = false;
        std::cout << '[' << c.Location().X() << ',' << c.Location().Y() << ','
                  << c.Location().Z() << ',' << c.Radius() << ']';
    }
    std::cout << "]}\n";
}
