#include <BRepAdaptor_Curve.hxx>
#include <BRepCheck_Analyzer.hxx>
#include <BRep_Tool.hxx>
#include <IGESControl_Reader.hxx>
#include <STEPControl_Reader.hxx>
#include <Standard_Failure.hxx>
#include <NCollection_IndexedMap.hxx>
#include <TopTools_ShapeMapHasher.hxx>
#include <TopExp_Explorer.hxx>
#include <TopExp.hxx>
#include <TopoDS.hxx>
#include <TopoDS_Vertex.hxx>
#include <gp_Pnt.hxx>
#include <gp_Vec.hxx>
#include <iomanip>
#include <iostream>
#include <string>

template <typename Reader>
static int diagnose(const char* path, const gp_Pnt& cusp) {
    Reader reader;
    if (reader.ReadFile(path) != IFSelect_RetDone || reader.TransferRoots() == 0) return 3;
    const TopoDS_Shape shape = reader.OneShape();
    if (shape.IsNull() || !BRepCheck_Analyzer(shape).IsValid()) return 4;
    NCollection_IndexedMap<TopoDS_Shape, TopTools_ShapeMapHasher> edges;
    TopExp::MapShapes(shape, TopAbs_EDGE, edges);
    std::cout << std::setprecision(17) << "{\"Germs\":[";
    int count = 0;
    for (int i = 1; i <= edges.Extent(); ++i) {
        const TopoDS_Edge edge = TopoDS::Edge(edges(i));
        BRepAdaptor_Curve curve(edge);
        const double first = curve.FirstParameter(), last = curve.LastParameter();
        const bool atFirst = curve.Value(first).Distance(cusp) < 1e-12;
        const bool atLast = curve.Value(last).Distance(cusp) < 1e-12;
        if (atFirst && atLast) return 5;
        if (!atFirst && !atLast) continue;
        gp_Pnt point;
        gp_Vec v1, v2, v3;
        curve.D3(atFirst ? first : last, point, v1, v2, v3);
        if (atLast) { v1.Reverse(); v3.Reverse(); }
        if (count++) std::cout << ',';
        auto print = [](const gp_Vec& d) { std::cout << '[' << d.X() << ',' << d.Y() << ',' << d.Z() << ']'; };
        std::cout << "{\"First\":"; print(v1);
        std::cout << ",\"Second\":"; print(v2);
        std::cout << ",\"Third\":"; print(v3);
        std::cout << '}';
    }
    std::cout << "]}" << std::endl;
    return count == 2 ? 0 : 6;
}

int main(int argc, char** argv) {
    if (argc != 6) return 2;
    try {
        const gp_Pnt cusp(std::stod(argv[3]), std::stod(argv[4]), std::stod(argv[5]));
        if (std::string(argv[1]) == "step") return diagnose<STEPControl_Reader>(argv[2], cusp);
        if (std::string(argv[1]) == "iges") return diagnose<IGESControl_Reader>(argv[2], cusp);
    } catch (const Standard_Failure& e) {
        std::cerr << e.what() << std::endl;
        return 7;
    }
    return 2;
}
