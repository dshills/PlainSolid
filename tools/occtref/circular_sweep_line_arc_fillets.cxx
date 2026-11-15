#include <BRepBuilderAPI_MakeEdge.hxx>
#include <BRepBuilderAPI_MakeFace.hxx>
#include <BRepBuilderAPI_MakeWire.hxx>
#include <BRepCheck_Analyzer.hxx>
#include <BRepFilletAPI_MakeFillet.hxx>
#include <BRepGProp.hxx>
#include <BRepPrimAPI_MakePrism.hxx>
#include <BRep_Tool.hxx>
#include <GProp_GProps.hxx>
#include <Geom_Circle.hxx>
#include <TopExp.hxx>
#include <TopExp_Explorer.hxx>
#include <TopoDS.hxx>
#include <TopoDS_Vertex.hxx>
#include <gp_Ax2.hxx>
#include <gp_Dir.hxx>
#include <gp_Pnt.hxx>
#include <gp_Vec.hxx>
#include <cmath>
#include <iomanip>
#include <iostream>
#include <string_view>
#include <vector>

int main(int argc, char** argv) {
    if (argc != 2 || (std::string_view(argv[1]) != "internal" && std::string_view(argv[1]) != "external"))
        return 1;
    const bool internal = std::string_view(argv[1]) == "internal";
    const gp_Pnt corner(internal ? 6. : -4., 0., 0.);
    const gp_Pnt end(internal ? 0. : 5., internal ? 18. : 3., 0.);
    const gp_Pnt center(0., internal ? 8. : 3., 0.);
    std::vector<gp_Pnt> rest = internal ? std::vector<gp_Pnt>{{-12., 18., 0.}, {-12., 0., 0.}} :
                                         std::vector<gp_Pnt>{{5., 16., 0.}, {-20., 16., 0.}, {-20., 0., 0.}};
    const Handle(Geom_Circle) circle = new Geom_Circle(gp_Ax2(center, gp_Dir(0., 0., 1.)), internal ? 10. : 5.);
    BRepBuilderAPI_MakeWire wire;
    wire.Add(BRepBuilderAPI_MakeEdge(circle, corner, end));
    gp_Pnt from = end;
    for (const auto& to : rest) {
        wire.Add(BRepBuilderAPI_MakeEdge(from, to));
        from = to;
    }
    wire.Add(BRepBuilderAPI_MakeEdge(from, corner));
    if (!wire.IsDone()) return 2;
    BRepBuilderAPI_MakeFace face(wire.Wire());
    if (!face.IsDone()) return 2;
    BRepPrimAPI_MakePrism sweep(face.Face(), gp_Vec(0., 0., 2.));
    const auto stock = sweep.Shape();
    if (!BRepCheck_Analyzer(stock).IsValid()) return 3;
    BRepFilletAPI_MakeFillet fillet(stock);
    bool found = false;
    for (TopExp_Explorer ex(stock, TopAbs_EDGE); ex.More(); ex.Next()) {
        const auto edge = TopoDS::Edge(ex.Current());
        TopoDS_Vertex first, last;
        TopExp::Vertices(edge, first, last);
        if (first.IsNull() || last.IsNull()) continue;
        const auto p = BRep_Tool::Pnt(first), q = BRep_Tool::Pnt(last);
        if (std::abs(p.Z()-q.Z()) > 1. &&
            std::hypot(p.X()-corner.X(), p.Y()) < 1e-8 && std::hypot(q.X()-corner.X(), q.Y()) < 1e-8) {
            fillet.Add(5., edge);
            found = true;
            break;
        }
    }
    if (!found) return 4;
    fillet.Build();
    if (!fillet.IsDone() || !BRepCheck_Analyzer(fillet.Shape()).IsValid()) return 5;
    GProp_GProps v0, v1, a0, a1;
    BRepGProp::VolumeProperties(stock, v0);
    BRepGProp::VolumeProperties(fillet.Shape(), v1);
    BRepGProp::SurfaceProperties(stock, a0);
    BRepGProp::SurfaceProperties(fillet.Shape(), a1);
    std::cout << std::setprecision(17) << "{\"volumeChange\":" << v1.Mass()-v0.Mass()
              << ",\"areaChange\":" << a1.Mass()-a0.Mass() << "}\n";
}
