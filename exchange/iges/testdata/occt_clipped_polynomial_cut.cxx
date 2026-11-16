#include <BRepCheck_Analyzer.hxx>
#include <BRepGProp.hxx>
#include <BRep_Tool.hxx>
#include <GProp_GProps.hxx>
#ifdef PLAINSOLID_OCCT_STEP_COMPONENT_READER
#include <STEPControl_Reader.hxx>
using ComponentReader = STEPControl_Reader;
#else
#include <IGESControl_Reader.hxx>
using ComponentReader = IGESControl_Reader;
#endif
#include <NCollection_IndexedMap.hxx>
#include <Standard_Failure.hxx>
#include <TopExp.hxx>
#include <TopExp_Explorer.hxx>
#include <TopTools_ShapeMapHasher.hxx>
#include <TopoDS.hxx>
#include <TopoDS_Vertex.hxx>
#include <TopoDS_Shape.hxx>
#include <iomanip>
#include <iostream>

using IndexedShapes = NCollection_IndexedMap<TopoDS_Shape, TopTools_ShapeMapHasher>;

static int count(const TopoDS_Shape& body, TopAbs_ShapeEnum kind) {
    IndexedShapes shapes;
    TopExp::MapShapes(body, kind, shapes);
    return shapes.Extent();
}

int main(int argc, char** argv) {
    if (argc != 2) return 2;
    try {
        ComponentReader reader;
        if (reader.ReadFile(argv[1]) != IFSelect_RetDone) return 3;
        const int transferred = reader.TransferRoots();
        if (transferred == 0) return 4;
        const auto shape = reader.OneShape();
        if (shape.IsNull() || !BRepCheck_Analyzer(shape).IsValid()) return 5;
        std::cout << std::setprecision(17)
                  << "{\"TransferredRoots\":" << transferred
                  << ",\"Faces\":" << count(shape, TopAbs_FACE)
                  << ",\"Solids\":" << count(shape, TopAbs_SOLID)
                  << ",\"IndexedVertices\":" << count(shape, TopAbs_VERTEX)
                  << ",\"Components\":[";
        bool first = true;
        for (TopExp_Explorer part(shape, TopAbs_SOLID); part.More(); part.Next()) {
            const auto body = part.Current();
            IndexedShapes points, edges;
            TopExp::MapShapes(body, TopAbs_VERTEX, points);
            TopExp::MapShapes(body, TopAbs_EDGE, edges);
            GProp_GProps volume, area;
            const double error = BRepGProp::VolumeProperties(body, volume, 1e-10, true);
            BRepGProp::SurfaceProperties(body, area, 1e-10);
            if (!first) std::cout << ',';
            first = false;
            std::cout << "{\"Faces\":" << count(body, TopAbs_FACE)
                      << ",\"Edges\":" << edges.Extent()
                      << ",\"Shells\":" << count(body, TopAbs_SHELL)
                      << ",\"Volume\":" << volume.Mass()
                      << ",\"Area\":" << area.Mass()
                      << ",\"RelativeIntegrationError\":" << error
                      << ",\"Vertices\":[";
            for (int j = 1; j <= points.Extent(); ++j) {
                const auto p = BRep_Tool::Pnt(TopoDS::Vertex(points(j)));
                if (j > 1) std::cout << ',';
                std::cout << '[' << p.X() << ',' << p.Y() << ',' << p.Z() << ']';
            }
            std::cout << "],\"EdgeVertices\":[";
            for (int j = 1; j <= edges.Extent(); ++j) {
                TopoDS_Vertex start, end;
                TopExp::Vertices(TopoDS::Edge(edges(j)), start, end);
                if (j > 1) std::cout << ',';
                std::cout << '[' << points.FindIndex(start) - 1 << ',' << points.FindIndex(end) - 1 << ']';
            }
            std::cout << "]}";
        }
        std::cout << "]}" << std::endl;
        return 0;
    } catch (const Standard_Failure& e) {
        std::cerr << e.what() << std::endl;
        return 6;
    }
}
