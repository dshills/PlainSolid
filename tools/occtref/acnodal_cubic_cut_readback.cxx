#include <BRepAdaptor_Curve.hxx>
#include <BRepAdaptor_Surface.hxx>
#include <BRepCheck_Analyzer.hxx>
#include <BRepClass3d_SolidClassifier.hxx>
#include <BRep_Tool.hxx>
#include <Geom2d_Curve.hxx>
#include <IGESControl_Reader.hxx>
#include <NCollection_IndexedMap.hxx>
#include <STEPControl_Reader.hxx>
#include <Standard_Failure.hxx>
#include <TopExp.hxx>
#include <TopExp_Explorer.hxx>
#include <TopTools_ShapeMapHasher.hxx>
#include <TopoDS.hxx>
#include <TopoDS_Edge.hxx>
#include <TopoDS_Face.hxx>
#include <gp_Pnt.hxx>
#include <gp_Pnt2d.hxx>
#include <gp_Vec.hxx>
#include <gp_Vec2d.hxx>

#include <iomanip>
#include <iostream>
#include <string>
#include <vector>

constexpr double coincidence = 1e-12;

struct OwnerUse {
    int orientation;
    gp_Pnt2d uv;
    gp_Vec2d first;
};

struct Germ {
    gp_Pnt exit;
    gp_Vec first;
    std::vector<OwnerUse> owners;
};

template <typename Reader>
static int diagnose(const char* path, const gp_Pnt& turn, const gp_Pnt& isolated) {
    Reader reader;
    if (reader.ReadFile(path) != IFSelect_RetDone || reader.TransferRoots() == 0) return 3;
    const TopoDS_Shape shape = reader.OneShape();
    if (shape.IsNull() || !BRepCheck_Analyzer(shape).IsValid()) return 4;

    TopExp_Explorer solids(shape, TopAbs_SOLID);
    if (!solids.More()) return 5;
    BRepClass3d_SolidClassifier classify(solids.Current(), isolated, coincidence);
    if (classify.State() != TopAbs_OUT) return 5;
    solids.Next();
    if (solids.More()) return 5;

    NCollection_IndexedMap<TopoDS_Shape, TopTools_ShapeMapHasher> edges;
    TopExp::MapShapes(shape, TopAbs_EDGE, edges);
    std::vector<Germ> germs;
    for (int i = 1; i <= edges.Extent(); ++i) {
        const TopoDS_Edge edge = TopoDS::Edge(edges(i));
        BRepAdaptor_Curve model(edge);
        const double lo = model.FirstParameter(), hi = model.LastParameter();
        const bool atStart = model.Value(lo).Distance(turn) <= coincidence;
        const bool atEnd = model.Value(hi).Distance(turn) <= coincidence;
        if (atStart && atEnd) return 6;
        if (!atStart && !atEnd) continue;

        Germ germ;
        gp_Pnt p;
        model.D1(atStart ? lo : hi, p, germ.first);
        germ.first.Multiply(atStart ? hi - lo : lo - hi);
        germ.exit = model.Value(atStart ? hi : lo);
        for (TopExp_Explorer faceUses(shape, TopAbs_FACE); faceUses.More(); faceUses.Next()) {
            const TopoDS_Face face = TopoDS::Face(faceUses.Current());
            for (TopExp_Explorer edgeUses(face, TopAbs_EDGE); edgeUses.More(); edgeUses.Next()) {
                const TopoDS_Edge use = TopoDS::Edge(edgeUses.Current());
                if (!use.IsSame(edge)) continue;
                double first, last;
                const auto curve = BRep_Tool::CurveOnSurface(use, face, first, last);
                if (curve.IsNull() || first >= last) return 7;
                BRepAdaptor_Surface support(face);
                const gp_Pnt2d a = curve->Value(first), b = curve->Value(last);
                const bool firstAtTurn = support.Value(a.X(), a.Y()).Distance(turn) <= coincidence;
                const bool lastAtTurn = support.Value(b.X(), b.Y()).Distance(turn) <= coincidence;
                if (firstAtTurn == lastAtTurn) return 7;
                const gp_Pnt2d opposite = firstAtTurn ? b : a;
                if (support.Value(opposite.X(), opposite.Y()).Distance(germ.exit) > coincidence) return 7;
                OwnerUse owner;
                if (use.Orientation() == TopAbs_FORWARD) owner.orientation = 1;
                else if (use.Orientation() == TopAbs_REVERSED) owner.orientation = -1;
                else return 7;
                curve->D1(firstAtTurn ? first : last, owner.uv, owner.first);
                owner.first.Multiply(firstAtTurn ? last - first : first - last);
                germ.owners.push_back(owner);
            }
        }
        if (germ.owners.size() != 2 || germ.owners[0].orientation == germ.owners[1].orientation) return 8;
        germs.push_back(germ);
    }
    if (germs.size() != 2) return 6;
    std::cout << std::setprecision(17) << "{\"IsolatedOutside\":true,\"Germs\":[";
    for (std::size_t i = 0; i < germs.size(); ++i) {
        const Germ& germ = germs[i];
        if (i) std::cout << ',';
        std::cout << "{\"Exit\":[" << germ.exit.X() << ',' << germ.exit.Y() << ',' << germ.exit.Z()
                  << "],\"First\":[" << germ.first.X() << ',' << germ.first.Y() << ',' << germ.first.Z()
                  << "],\"Owners\":[";
        for (std::size_t j = 0; j < germ.owners.size(); ++j) {
            const OwnerUse& owner = germ.owners[j];
            if (j) std::cout << ',';
            std::cout << "{\"Orientation\":" << owner.orientation << ",\"UV\":[" << owner.uv.X() << ','
                      << owner.uv.Y() << "],\"First\":[" << owner.first.X() << ',' << owner.first.Y() << "]}";
        }
        std::cout << "]}";
    }
    std::cout << "]}\n";
    return 0;
}

int main(int argc, char** argv) {
    if (argc != 9) return 2;
    try {
        const gp_Pnt turn(std::stod(argv[3]), std::stod(argv[4]), std::stod(argv[5]));
        const gp_Pnt isolated(std::stod(argv[6]), std::stod(argv[7]), std::stod(argv[8]));
        const std::string format(argv[1]);
        if (format == "step") return diagnose<STEPControl_Reader>(argv[2], turn, isolated);
        if (format == "iges") return diagnose<IGESControl_Reader>(argv[2], turn, isolated);
    } catch (const Standard_Failure& e) {
        std::cerr << e.what() << '\n';
        return 9;
    }
    return 2;
}
