#include <BRepAdaptor_Curve.hxx>
#include <BRepAdaptor_Surface.hxx>
#include <BRepCheck_Analyzer.hxx>
#include <BRepGProp.hxx>
#include <BRep_Tool.hxx>
#include <GProp_GProps.hxx>
#include <Geom_BSplineCurve.hxx>
#include <NCollection_IndexedMap.hxx>
#include <STEPControl_Reader.hxx>
#include <TopExp.hxx>
#include <TopExp_Explorer.hxx>
#include <TopTools_ShapeMapHasher.hxx>
#include <TopoDS.hxx>
#include <algorithm>
#include <cmath>
#include <iomanip>
#include <iostream>

using ShapeMap = NCollection_IndexedMap<TopoDS_Shape, TopTools_ShapeMapHasher>;

int main(int argc, char** argv) {
    if (argc != 2) return 1;
    STEPControl_Reader reader;
    if (reader.ReadFile(argv[1]) != IFSelect_RetDone || reader.TransferRoots() != 1) return 2;
    const auto shape = reader.OneShape();
    if (shape.IsNull() || !BRepCheck_Analyzer(shape).IsValid()) return 3;
    ShapeMap solids, faces, edges, vertices;
    TopExp::MapShapes(shape, TopAbs_SOLID, solids);
    TopExp::MapShapes(shape, TopAbs_FACE, faces);
    TopExp::MapShapes(shape, TopAbs_EDGE, edges);
    TopExp::MapShapes(shape, TopAbs_VERTEX, vertices);
    if (solids.Extent() != 1 || faces.Extent() != 6 || edges.Extent() != 10 || vertices.Extent() != 6) return 4;

    GProp_GProps volume, area;
    BRepGProp::VolumeProperties(shape, volume, 1e-10, true);
    BRepGProp::SurfaceProperties(shape, area, 1e-10);
    std::cout << std::setprecision(17) << "{\"volume\":" << volume.Mass()
              << ",\"area\":" << area.Mass() << ",\"vertices\":[";
    for (int i = 1; i <= vertices.Extent(); ++i) {
        if (i != 1) std::cout << ',';
        const auto p = BRep_Tool::Pnt(TopoDS::Vertex(vertices(i)));
        std::cout << '[' << p.X() << ',' << p.Y() << ',' << p.Z() << ']';
    }
    const double pi = std::acos(-1.);
    int circles = 0, meridians = 0, generators = 0;
    for (int i = 1; i <= edges.Extent(); ++i) {
        const BRepAdaptor_Curve c(TopoDS::Edge(edges(i)));
        if (c.GetType() == GeomAbs_Circle) {
            const double span = c.LastParameter() - c.FirstParameter();
            if (std::abs(c.Circle().Axis().Direction().Z()) == 1 && std::abs(span - 2*pi) < 1e-8)
                ++circles;
            else if (std::abs(c.Circle().Axis().Direction().Y()) == 1 && span > 0 && span < pi)
                ++meridians;
            else
                return 5;
        } else if (c.GetType() == GeomAbs_Line ||
                   (c.GetType() == GeomAbs_BSplineCurve && c.BSpline()->Degree() == 1)) {
            ++generators;
        } else {
            return 5;
        }
    }
    if (circles != 6 || meridians != 2 || generators != 2) return 5;
    std::cout << "],\"edges\":[" << circles << ',' << meridians << ',' << generators << "],\"faces\":[";
    for (int i = 1; i <= faces.Extent(); ++i) {
        const auto face = TopoDS::Face(faces(i));
        const BRepAdaptor_Surface s(face);
        if (i != 1) std::cout << ',';
        switch (s.GetType()) {
        case GeomAbs_Plane: {
            int wires = 0;
            for (TopExp_Explorer w(face, TopAbs_WIRE); w.More(); w.Next()) ++wires;
            std::cout << "[0," << s.Plane().Location().Z() << ',' << wires << ']';
            break;
        }
        case GeomAbs_Cylinder: {
            const auto cyl = s.Cylinder();
            const auto c = cyl.Location();
            const auto a = s.Value(0, s.FirstVParameter()), b = s.Value(0, s.LastVParameter());
            if (std::abs(cyl.Axis().Direction().Z()) != 1) return 6;
            std::cout << "[1," << c.X() << ',' << c.Y() << ',' << cyl.Radius()
                      << ',' << std::min(a.Z(),b.Z()) << ',' << std::max(a.Z(),b.Z()) << ']';
            break;
        }
        case GeomAbs_Torus: {
            const auto torus = s.Torus();
            const auto c = torus.Location();
            const auto a = s.Value(0, s.FirstVParameter()), b = s.Value(0, s.LastVParameter());
            if (std::abs(torus.Axis().Direction().Z()) != 1) return 6;
            std::cout << "[2," << c.X() << ',' << c.Y() << ',' << c.Z() << ',' << torus.MajorRadius()
                      << ',' << torus.MinorRadius() << ',' << std::min(a.Z(),b.Z()) << ',' << std::max(a.Z(),b.Z()) << ']';
            break;
        }
        case GeomAbs_SurfaceOfRevolution: {
            const auto axis = s.AxeOfRevolution();
            auto a = s.Value(0, s.FirstVParameter()), b = s.Value(0, s.LastVParameter());
            if (std::abs(axis.Direction().Z()) != 1) return 6;
            if (a.Z() > b.Z()) std::swap(a,b);
            const auto c = axis.Location();
            std::cout << "[3," << c.X() << ',' << c.Y() << ',' << a.Z() << ',' << a.X()-c.X()
                      << ',' << b.Z() << ',' << b.X()-c.X() << ']';
            break;
        }
        default:
            return 6;
        }
    }
    std::cout << "]}\n";
}
