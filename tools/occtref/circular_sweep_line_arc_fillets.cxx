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
    if (argc != 2) return 1;
    const std::string_view mode(argv[1]);
    const bool obliqueLine = mode == "oblique-lines", obliqueArc = mode == "oblique-line-arc";
    const bool internal = mode == "internal" || obliqueArc, mixed = mode == "mixed-arc-pair";
    const bool pair = mode == "arc-pair" || mixed;
    if (!internal && !pair && !obliqueLine && mode != "external") return 1;
    auto placed = [&](const gp_Pnt& p) {
        return obliqueArc ? gp_Pnt(3.*p.X()-4.*p.Y(), 4.*p.X()+3.*p.Y(), p.Z()) : p;
    };
    gp_Pnt initial, corner, end;
    std::vector<gp_Pnt> rest;
    BRepBuilderAPI_MakeWire wire;
    auto addArc = [&](const gp_Pnt& a, const gp_Pnt& b, const gp_Pnt& c, double radius, double sense) {
        const Handle(Geom_Circle) circle = new Geom_Circle(gp_Ax2(c, gp_Dir(0., 0., sense)), radius);
        wire.Add(BRepBuilderAPI_MakeEdge(circle, a, b));
    };
    if (pair) {
        initial = mixed ? gp_Pnt(-1.25, -6.25, 0.) : gp_Pnt(-1.5, -6.5, 0.);
        corner = mixed ? gp_Pnt(3.75, 3.75, 0.) : gp_Pnt(4.5, 2.5, 0.);
        end = mixed ? gp_Pnt(2.5, 5., 0.) : gp_Pnt(-1.5, 5.5, 0.);
        rest = mixed ? std::vector<gp_Pnt>{{2.5, 10., 0.}, {-12., 10., 0.}, {-12., -6.25, 0.}} :
                       std::vector<gp_Pnt>{{-12., 5.5, 0.}, {-12., -6.5, 0.}};
        addArc(initial, corner, gp_Pnt(initial.X(), 0., 0.), -initial.Y(), 1.);
        addArc(corner, end, mixed ? gp_Pnt(3.75, 5., 0.) : gp_Pnt(-1.5, -2., 0.),
               mixed ? 1.25 : 7.5, mixed ? -1. : 1.);
    } else if (obliqueLine) {
        initial = corner = gp_Pnt(0., 0., 0.);
        end = gp_Pnt(18., 24., 0.);
        rest = {{-24., 24., 0.}, {-24., 0., 0.}};
        wire.Add(BRepBuilderAPI_MakeEdge(corner, end));
    } else {
        initial = corner = gp_Pnt(internal ? 6. : -4., 0., 0.);
        end = gp_Pnt(internal ? 0. : 5., internal ? 18. : 3., 0.);
        rest = internal ? std::vector<gp_Pnt>{{-12., 18., 0.}, {-12., 0., 0.}} :
                          std::vector<gp_Pnt>{{5., 16., 0.}, {-20., 16., 0.}, {-20., 0., 0.}};
        initial = corner = placed(corner);
        end = placed(end);
        for (auto& p : rest) p = placed(p);
        addArc(corner, end, placed(gp_Pnt(0., internal ? 8. : 3., 0.)), obliqueArc ? 50. : internal ? 10. : 5., 1.);
    }
    gp_Pnt from = end;
    for (const auto& to : rest) {
        wire.Add(BRepBuilderAPI_MakeEdge(from, to));
        from = to;
    }
    wire.Add(BRepBuilderAPI_MakeEdge(from, initial));
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
            std::hypot(p.X()-corner.X(), p.Y()-corner.Y()) < 1e-8 &&
            std::hypot(q.X()-corner.X(), q.Y()-corner.Y()) < 1e-8) {
            fillet.Add(obliqueArc ? 25. : obliqueLine ? 10. : 5., edge);
            found = true;
            break;
        }
    }
    if (!found) return 4;
    fillet.Build();
    if (!fillet.IsDone() || !BRepCheck_Analyzer(fillet.Shape()).IsValid()) return 5;
    GProp_GProps v0, v1, a0, a1;
    BRepGProp::VolumeProperties(stock, v0, 1e-12);
    BRepGProp::VolumeProperties(fillet.Shape(), v1, 1e-12);
    BRepGProp::SurfaceProperties(stock, a0, 1e-12);
    BRepGProp::SurfaceProperties(fillet.Shape(), a1, 1e-12);
    std::cout << std::setprecision(17) << "{\"volumeChange\":" << v1.Mass()-v0.Mass()
              << ",\"areaChange\":" << a1.Mass()-a0.Mass() << "}\n";
}
