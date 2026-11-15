#include <BRepAlgoAPI_Cut.hxx>
#include <BRepBuilderAPI_MakeEdge.hxx>
#include <BRepBuilderAPI_MakeFace.hxx>
#include <BRepBuilderAPI_MakeWire.hxx>
#include <BRepCheck_Analyzer.hxx>
#include <BRepGProp.hxx>
#include <BRepPrimAPI_MakeBox.hxx>
#include <BRepPrimAPI_MakeCylinder.hxx>
#include <BRepPrimAPI_MakePrism.hxx>
#include <GProp_GProps.hxx>
#include <Geom_Circle.hxx>
#include <TopoDS_Shape.hxx>
#include <gp_Ax2.hxx>
#include <gp_Dir.hxx>
#include <gp_Pnt.hxx>
#include <gp_Vec.hxx>
#include <cmath>
#include <iomanip>
#include <iostream>
#include <string_view>

int main(int argc, char** argv) {
    if (argc != 2) return 1;
    TopoDS_Shape solid;
    const std::string_view mode(argv[1]);
    if (mode == "major") {
        const Handle(Geom_Circle) circle =
            new Geom_Circle(gp_Ax2(gp_Pnt(0.,0.,0.), gp_Dir(0.,0.,1.)), 5.);
        // Author one original 270 degree analytic edge, without
        // reproducing the Go implementation's quarter-circle splits.
        BRepBuilderAPI_MakeWire wire;
        wire.Add(BRepBuilderAPI_MakeEdge(circle, 0., 1.5*std::acos(-1.)));
        wire.Add(BRepBuilderAPI_MakeEdge(gp_Pnt(0.,-5.,0.),gp_Pnt(5.,0.,0.)));
        if (!wire.IsDone()) return 2;
        BRepBuilderAPI_MakeFace face(wire.Wire());
        if (!face.IsDone()) return 2;
        solid = BRepPrimAPI_MakePrism(face.Face(), gp_Vec(0.,0.,2.)).Shape();
    } else if (mode == "full") {
        const auto box = BRepPrimAPI_MakeBox(gp_Pnt(-8.,-8.,-1.),16.,16.,2.).Shape();
        const auto cylinder = BRepPrimAPI_MakeCylinder(gp_Ax2(gp_Pnt(0.,0.,-2.),gp_Dir(0.,0.,1.)),5.,4.).Shape();
        BRepAlgoAPI_Cut cut(box,cylinder);
        if (!cut.IsDone()) return 2;
        solid = cut.Shape();
    } else {
        return 1;
    }
    if (solid.IsNull() || !BRepCheck_Analyzer(solid).IsValid()) return 3;
    GProp_GProps volume, area;
    BRepGProp::VolumeProperties(solid,volume,1e-12);
    BRepGProp::SurfaceProperties(solid,area,1e-12);
    std::cout << std::setprecision(17) << "{\"Volume\":" << volume.Mass() << ",\"Area\":" << area.Mass() << "}\n";
}
