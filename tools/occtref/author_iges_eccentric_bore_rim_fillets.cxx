#include <BRepAdaptor_Curve.hxx>
#include <BRepAlgoAPI_Cut.hxx>
#include <BRepCheck_Analyzer.hxx>
#include <BRepFilletAPI_MakeFillet.hxx>
#include <BRepPrimAPI_MakeCylinder.hxx>
#include <IGESControl_Writer.hxx>
#include <Interface_Static.hxx>
#include <NCollection_IndexedMap.hxx>
#include <TopExp.hxx>
#include <TopTools_ShapeMapHasher.hxx>
#include <TopoDS.hxx>
#include <gp_Ax2.hxx>

#include <array>
#include <cmath>
#include <string_view>

int main(int argc, char** argv) {
    if (argc != 4) return 1;
    const std::string_view fillets(argv[1]), source(argv[3]);
    if ((fillets != "lower" && fillets != "upper" && fillets != "both") ||
        (source != "shifted" && source != "origin")) return 1;
    const double y = source == "shifted" ? 64. : 0.;
    const auto stock = BRepPrimAPI_MakeCylinder(gp_Ax2(gp_Pnt(0, y, 2), gp::DZ()), 6.25, 8).Shape();
    const auto cutter = BRepPrimAPI_MakeCylinder(gp_Ax2(gp_Pnt(1.5, y + 2, 2), gp::DZ()), 2, 8).Shape();
    BRepAlgoAPI_Cut cut(stock, cutter);
    const auto bore = cut.Shape();
    if (!cut.IsDone() || bore.IsNull() || !BRepCheck_Analyzer(bore).IsValid()) return 2;
    BRepFilletAPI_MakeFillet result(bore);
    NCollection_IndexedMap<TopoDS_Shape, TopTools_ShapeMapHasher> edges;
    TopExp::MapShapes(bore, TopAbs_EDGE, edges);
    const std::array<double, 2> radii{{fillets == "upper" ? 0. : .5, fillets == "lower" ? 0. : .75}};
    std::array<bool, 2> found{};
    auto matches = [](double a, double b) { return std::abs(a - b) < 1e-9; };
    for (int i = 1; i <= edges.Extent(); ++i) {
        const auto edge = TopoDS::Edge(edges(i));
        const BRepAdaptor_Curve curve(edge);
        if (curve.GetType() != GeomAbs_Circle) continue;
        const auto circle = curve.Circle();
        const auto center = circle.Location();
        if (!matches(circle.Radius(), 2) || !matches(center.X(), 1.5) || !matches(center.Y(), y + 2)) continue;
        const int end = matches(center.Z(), 2) ? 0 : matches(center.Z(), 10) ? 1 : -1;
        if (end < 0 || found[end]) return 3;
        found[end] = true;
        if (radii[end] > 0) result.Add(radii[end], edge);
    }
    if (!found[0] || !found[1]) return 4;
    result.Build();
    if (!result.IsDone() || result.Shape().IsNull() || !BRepCheck_Analyzer(result.Shape()).IsValid()) return 5;
    IGESControl_Writer writer("MM", 1);
    if (!Interface_Static::SetIVal("write.surfacecurve.mode", 0)) return 6;
    return writer.AddShape(result.Shape()) && writer.Write(argv[2]) ? 0 : 7;
}
