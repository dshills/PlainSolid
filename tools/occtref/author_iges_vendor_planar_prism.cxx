#include <BRepAlgoAPI_Cut.hxx>
#include <BRepBuilderAPI_MakeFace.hxx>
#include <BRepBuilderAPI_MakePolygon.hxx>
#include <BRepCheck_Analyzer.hxx>
#include <BRepGProp.hxx>
#include <BRepPrimAPI_MakeBox.hxx>
#include <BRepPrimAPI_MakePrism.hxx>
#include <BRep_Builder.hxx>
#include <GProp_GProps.hxx>
#include <IGESControl_Reader.hxx>
#include <IGESControl_Writer.hxx>
#include <TopExp_Explorer.hxx>
#include <TopoDS_Compound.hxx>
#include <gp_Pnt.hxx>
#include <gp_Vec.hxx>
#include <iomanip>
#include <initializer_list>
#include <iostream>
#include <string>

static TopoDS_Wire wire(std::initializer_list<gp_Pnt> points) {
    BRepBuilderAPI_MakePolygon polygon;
    for (const auto& point : points) polygon.Add(point);
    polygon.Close();
    return polygon.Wire();
}

int main(int argc, char** argv) {
    if (argc != 3) return 2;
    const std::string mode = argv[1];
    if (mode == "write" || mode == "write-void" || mode == "write-two-voids" || mode == "write-assembly") {
        TopoDS_Shape shape;
        if (mode == "write") {
            BRepBuilderAPI_MakeFace face(wire({{0,0,0},{6,0,0},{6,5,0},{4,5,0},{4,4,0},{0,4,0}}));
            face.Add(wire({{1,1,0},{1,2,0},{2,2,0},{2,1,0}}));
            if (!face.IsDone()) return 3;
            shape = BRepPrimAPI_MakePrism(face.Face(), gp_Vec(0,0,2)).Shape();
        } else if (mode == "write-assembly") {
            BRep_Builder builder;
            TopoDS_Compound pieces;
            builder.MakeCompound(pieces);
            for (double p : {0., 1., 4.}) {
                builder.Add(pieces, BRepPrimAPI_MakeBox(gp_Pnt(p,p,p),1,1,1).Shape());
            }
            shape = pieces;
        } else {
            const auto box = BRepPrimAPI_MakeBox(8,8,8).Shape();
            if (mode == "write-void") {
                shape = BRepAlgoAPI_Cut(box, BRepPrimAPI_MakeBox(gp_Pnt(2,2,2),4,4,4).Shape()).Shape();
            } else {
                const auto first = BRepAlgoAPI_Cut(box, BRepPrimAPI_MakeBox(gp_Pnt(1,1,1),2,2,2).Shape()).Shape();
                shape = BRepAlgoAPI_Cut(first, BRepPrimAPI_MakeBox(gp_Pnt(5,5,5),2,2,2).Shape()).Shape();
            }
        }
        if (!BRepCheck_Analyzer(shape).IsValid()) return 4;
        IGESControl_Writer writer("MM", 1);
        if (!writer.AddShape(shape)) return 5;
        writer.ComputeModel();
        return writer.Write(argv[2]) ? 0 : 6;
    }
    if (mode != "read") return 2;
    IGESControl_Reader reader;
    if (reader.ReadFile(argv[2]) != IFSelect_RetDone || !reader.TransferRoots()) return 7;
    const auto shape = reader.OneShape();
    if (shape.IsNull() || !BRepCheck_Analyzer(shape).IsValid()) return 8;
    int solids = 0, shells = 0, faces = 0;
    for (TopExp_Explorer e(shape, TopAbs_SOLID); e.More(); e.Next()) ++solids;
    for (TopExp_Explorer e(shape, TopAbs_SHELL); e.More(); e.Next()) ++shells;
    for (TopExp_Explorer e(shape, TopAbs_FACE); e.More(); e.Next()) ++faces;
    GProp_GProps volume, surface;
    BRepGProp::VolumeProperties(shape, volume);
    BRepGProp::SurfaceProperties(shape, surface);
    const auto c = volume.CentreOfMass();
    std::cout << std::setprecision(17)
              << "{\"Solids\":" << solids << ",\"Shells\":" << shells << ",\"Faces\":" << faces
              << ",\"Volume\":" << volume.Mass() << ",\"Area\":" << surface.Mass()
              << ",\"Centroid\":[" << c.X() << "," << c.Y() << "," << c.Z() << "]}\n";
    return 0;
}
