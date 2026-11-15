#include <BRepBuilderAPI_MakeEdge.hxx>
#include <BRepBuilderAPI_MakeFace.hxx>
#include <BRepBuilderAPI_MakePolygon.hxx>
#include <BRepBuilderAPI_MakeVertex.hxx>
#include <BRepBuilderAPI_MakeWire.hxx>
#include <BRepCheck_Analyzer.hxx>
#include <BRepGProp.hxx>
#include <BRepPrimAPI_MakePrism.hxx>
#include <Geom_Circle.hxx>
#include <GProp_GProps.hxx>
#include <IGESControl_Writer.hxx>
#include <Interface_Static.hxx>
#include <STEPControl_Writer.hxx>
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
#include <string_view>

int main(int argc, char** argv) {
    const std::string_view mode = argc == 3 ? argv[1] : "";
    const bool iges = mode == "iges" || mode == "iges-local-chart";
    if (argc > 2 && !iges) return 1;
    BRepBuilderAPI_MakePolygon stock;
    const std::array<gp_Pnt,4> outline{{{0.,0.,-1.},{6.,0.,-1.},{6.,4.,-1.},{0.,4.,-1.}}};
    for (const auto& p : outline) stock.Add(p);
    stock.Close();
    BRepBuilderAPI_MakeFace face(stock.Wire());
    constexpr double turn = 1.57079632679489661923;
    for (double x : {2.,4.}) {
        const occ::handle<Geom_Circle> circle = new Geom_Circle(gp_Ax2(gp_Pnt(x,2.,-1.),gp_Dir(0.,0.,1.)),.5);
        const std::array<gp_Pnt,4> cardinal{{{x+.5,2.,-1.},{x,2.5,-1.},{x-.5,2.,-1.},{x,1.5,-1.}}};
        std::array<TopoDS_Vertex,4> vertices;
        for (int i=0; i<4; ++i) vertices[i]=BRepBuilderAPI_MakeVertex(cardinal[i]).Vertex();
        BRepBuilderAPI_MakeWire hole;
        for (int i=0; i<4; ++i) {
            auto support = circle;
            double first = i*turn;
            if (mode == "iges-local-chart") {
                // Use an independent proper local frame for each quarter.
                const gp_Dir ray(cardinal[i].X()-x,cardinal[i].Y()-2.,0.);
                support = new Geom_Circle(gp_Ax2(gp_Pnt(x,2.,-1.),gp_Dir(0.,0.,1.),ray),.5);
                first = 0.;
            }
            BRepBuilderAPI_MakeEdge edge(support,vertices[i],vertices[(i+1)%4],first,first+turn);
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
    if (argc == 2) {
        STEPControl_Writer writer;
        Interface_Static::SetCVal("write.step.schema", "AP214IS");
        Interface_Static::SetIVal("write.surfacecurve.mode", 0);
        if (writer.Transfer(shape, STEPControl_AsIs) != IFSelect_RetDone || writer.Write(argv[1]) != IFSelect_RetDone) return 5;
    }
    if (iges) {
        IGESControl_Writer writer("MM", 1);
        if (!writer.AddShape(shape) || !writer.Write(argv[2])) return 6;
    }
}
