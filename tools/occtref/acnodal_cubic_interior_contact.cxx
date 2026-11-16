#include <BRepClass_FaceClassifier.hxx>
#include <BRepCheck_Analyzer.hxx>
#include <BRep_Tool.hxx>
#include <GeomAPI_ProjectPointOnSurf.hxx>
#include <IGESControl_Reader.hxx>
#include <STEPControl_Reader.hxx>
#include <Standard_Failure.hxx>
#include <TopExp_Explorer.hxx>
#include <TopoDS.hxx>
#include <TopoDS_Face.hxx>
#include <gp_Pnt.hxx>
#include <gp_Pnt2d.hxx>
#include <iostream>
#include <string>

template<typename Reader>
static int check(const char* path, const gp_Pnt& point) {
    Reader reader;
    if (reader.ReadFile(path) != IFSelect_RetDone || reader.TransferRoots() == 0) return 3;
    const auto shape = reader.OneShape();
    if (shape.IsNull() || !BRepCheck_Analyzer(shape).IsValid()) return 4;
    constexpr double precision = 1e-10;
    int interior = 0, boundary = 0;
    for (TopExp_Explorer faces(shape, TopAbs_FACE); faces.More(); faces.Next()) {
        const TopoDS_Face face = TopoDS::Face(faces.Current());
        GeomAPI_ProjectPointOnSurf nearest(point, BRep_Tool::Surface(face));
        if (!nearest.IsDone() || nearest.NbPoints() == 0 || nearest.LowerDistance() > precision) continue;
        double u, v;
        nearest.LowerDistanceParameters(u, v);
        const BRepClass_FaceClassifier classify(face, gp_Pnt2d(u, v), precision);
        interior += classify.State() == TopAbs_IN;
        boundary += classify.State() == TopAbs_ON;
    }
    std::cout << "{\"InteriorFaces\":" << interior << ",\"BoundaryFaces\":" << boundary << "}\n";
    return interior == 1 && boundary == 0 ? 0 : 5;
}

int main(int argc, char** argv) {
    if (argc != 6) return 2;
    try {
        const gp_Pnt point(std::stod(argv[3]), std::stod(argv[4]), std::stod(argv[5]));
        const auto format = std::string(argv[1]);
        if (format == "step") return check<STEPControl_Reader>(argv[2], point);
        if (format == "iges") return check<IGESControl_Reader>(argv[2], point);
    } catch (const Standard_Failure& e) {
        std::cerr << e.what() << '\n';
        return 6;
    }
    return 2;
}
