#include <BRep_Tool.hxx>
#include <BRepBuilderAPI_MakeFace.hxx>
#include <BRepBuilderAPI_MakePolygon.hxx>
#include <BRepCheck_Analyzer.hxx>
#include <BRepFilletAPI_MakeFillet.hxx>
#include <BRepGProp.hxx>
#include <BRepPrimAPI_MakePrism.hxx>
#include <GProp_GProps.hxx>
#include <TopExp.hxx>
#include <TopExp_Explorer.hxx>
#include <NCollection_IndexedMap.hxx>
#include <TopTools_ShapeMapHasher.hxx>
#include <TopoDS.hxx>
#include <TopoDS_Vertex.hxx>
#include <gp_Pnt.hxx>
#include <gp_Vec.hxx>
#include <array>
#include <iomanip>
#include <iostream>

int main() {
    const std::array<gp_Pnt,6> outline{{{0.,0.,-1.},{4.,0.,-1.},{4.,1.,-1.},
                                       {1.,1.,-1.},{1.,4.,-1.},{0.,4.,-1.}}};
    BRepBuilderAPI_MakePolygon wire;
    for (const auto& p : outline) wire.Add(p);
    wire.Close();
    BRepBuilderAPI_MakeFace face(wire.Wire());
    if (!face.IsDone()) return 2;
    const auto stock = BRepPrimAPI_MakePrism(face.Face(),gp_Vec(0.,0.,2.)).Shape();
    BRepFilletAPI_MakeFillet round(stock);
    NCollection_IndexedMap<TopoDS_Shape,TopTools_ShapeMapHasher> edges;
    TopExp::MapShapes(stock,TopAbs_EDGE,edges);
    int selected = 0;
    for (int i=1; i<=edges.Extent(); ++i) {
        const auto edge = TopoDS::Edge(edges(i));
        TopoDS_Vertex from,to;
        TopExp::Vertices(edge,from,to);
        const auto a=BRep_Tool::Pnt(from), b=BRep_Tool::Pnt(to);
        if (a.X()!=b.X() || a.Y()!=b.Y() || a.Z()==b.Z()) continue;
        if ((a.X()==4. && a.Y()==0.) || (a.X()==1. && a.Y()==1.)) {
            round.Add(.25,edge);
            ++selected;
        }
    }
    if (selected!=2) return 3;
    round.Build();
    if (!round.IsDone()) return 4;
    const auto result=round.Shape();
    if (result.IsNull() || !BRepCheck_Analyzer(result).IsValid()) return 5;
    int faces=0;
    for (TopExp_Explorer x(result,TopAbs_FACE); x.More(); x.Next()) ++faces;
    GProp_GProps volume,area;
    BRepGProp::VolumeProperties(result,volume);
    BRepGProp::SurfaceProperties(result,area);
    std::cout << std::setprecision(17) << "{\"Faces\":" << faces
              << ",\"Volume\":" << volume.Mass()
              << ",\"Area\":" << area.Mass() << "}\n";
}
