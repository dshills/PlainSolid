#include <BRepBuilderAPI_MakeEdge.hxx>
#include <BRepBuilderAPI_MakeFace.hxx>
#include <BRepBuilderAPI_MakePolygon.hxx>
#include <BRepBuilderAPI_MakeWire.hxx>
#include <BRepCheck_Analyzer.hxx>
#include <BRepGProp.hxx>
#include <BRepPrimAPI_MakePrism.hxx>
#include <GProp_GProps.hxx>
#include <TopExp_Explorer.hxx>
#include <TopoDS.hxx>
#include <TopoDS_Wire.hxx>
#include <gp_Ax2.hxx>
#include <gp_Circ.hxx>
#include <gp_Dir.hxx>
#include <gp_Pnt.hxx>
#include <gp_Vec.hxx>
#include <array>
#include <iomanip>
#include <iostream>

int main() {
    BRepBuilderAPI_MakePolygon stock;
    const std::array<gp_Pnt,4> outline{{{0.,0.,-1.},{6.,0.,-1.},{6.,4.,-1.},{0.,4.,-1.}}};
    for (const auto& p : outline) stock.Add(p);
    stock.Close();
    BRepBuilderAPI_MakeFace face(stock.Wire());
    constexpr double turn = 1.57079632679489661923;
    for (double x : {2.,4.}) {
        const gp_Circ circle(gp_Ax2(gp_Pnt(x,2.,-1.),gp_Dir(0.,0.,1.)),.5);
        BRepBuilderAPI_MakeWire hole;
        for (int i=0; i<4; ++i) {
            BRepBuilderAPI_MakeEdge edge(circle,i*turn,(i+1)*turn);
            if (!edge.IsDone()) return 2;
            hole.Add(edge.Edge());
        }
        if (!hole.IsDone()) return 2;
        face.Add(TopoDS::Wire(hole.Wire().Reversed()));
    }
    if (!face.IsDone() || !BRepCheck_Analyzer(face.Face()).IsValid()) return 3;
    const auto shape = BRepPrimAPI_MakePrism(face.Face(),gp_Vec(0.,0.,2.)).Shape();
    if (shape.IsNull() || !BRepCheck_Analyzer(shape).IsValid()) return 4;
    int faces = 0;
    for (TopExp_Explorer it(shape,TopAbs_FACE); it.More(); it.Next()) ++faces;
    GProp_GProps volume,area;
    BRepGProp::VolumeProperties(shape,volume);
    BRepGProp::SurfaceProperties(shape,area);
    std::cout << std::setprecision(17) << "{\"Faces\":" << faces << ",\"Volume\":"
              << volume.Mass() << ",\"Area\":" << area.Mass() << "}\n";
}
