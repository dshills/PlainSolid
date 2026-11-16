#include <BRepAdaptor_Curve.hxx>
#include <BRepAlgoAPI_Cut.hxx>
#include <BRepCheck_Analyzer.hxx>
#include <BRepFilletAPI_MakeFillet.hxx>
#include <BRepGProp.hxx>
#include <BRepPrimAPI_MakeCone.hxx>
#include <BRepPrimAPI_MakeCylinder.hxx>
#include <GProp_GProps.hxx>
#include <Interface_Static.hxx>
#include <NCollection_IndexedMap.hxx>
#include <IGESControl_Writer.hxx>
#include <IGESBasic_Group.hxx>
#include <IGESData_IGESModel.hxx>
#include <IGESGeom_BSplineSurface.hxx>
#include <IGESGeom_Direction.hxx>
#include <IGESGeom_Line.hxx>
#include <IGESGeom_Point.hxx>
#include <IGESSolid_ConicalSurface.hxx>
#include <IGESSolid_EdgeList.hxx>
#include <IGESSolid_Face.hxx>
#include <IGESSolid_VertexList.hxx>
#include <IGESSolid_ToroidalSurface.hxx>
#include <IGESGeom_TransformationMatrix.hxx>
#include <array>
#include <TopExp.hxx>
#include <TopTools_ShapeMapHasher.hxx>
#include <TopoDS.hxx>
#include <cmath>
#include <iomanip>
#include <iostream>

static occ::handle<IGESGeom_BSplineSurface> coneNet(const gp_Pnt& center, const gp_Pnt& low,
                                                   const gp_Pnt& high, int direction)
{
    constexpr std::array<double, 12> knotsU{0,0,0,1,1,2,2,3,3,4,4,4};
    constexpr std::array<double, 4> knotsV{0,0,1,1};
    constexpr std::array<std::array<int, 2>, 9> meridians{{
        {1,0},{1,1},{0,1},{-1,1},{-1,0},{-1,-1},{0,-1},{1,-1},{1,0}
    }};
    // The rational half-angle parameter gives exact homogeneous quadrants.
    constexpr std::array<int, 9> weights{1,1,2,2,4,2,2,1,1};
    auto u = new NCollection_HArray1<double>(-2, 9);
    auto v = new NCollection_HArray1<double>(-1, 2);
    for (int i = 0; i < 12; ++i) u->SetValue(i-2, knotsU[i]);
    for (int i = 0; i < 4; ++i) v->SetValue(i-1, knotsV[i]);
    auto w = new NCollection_HArray2<double>(0, 8, 0, 1);
    auto p = new NCollection_HArray2<gp_XYZ>(0, 8, 0, 1);
    for (int j = 0; j < 2; ++j) {
        const gp_Pnt& end = (j == 0) == (direction > 0) ? low : high;
        const double radius = end.X() - center.X();
        for (int i = 0; i < 9; ++i) {
            w->SetValue(i, j, weights[i]);
            p->SetValue(i, j, gp_XYZ(center.X() + radius*meridians[i][0],
                                    center.Y() + direction*radius*meridians[i][1], end.Z()));
        }
    }
    auto out = new IGESGeom_BSplineSurface;
    out->Init(8, 1, 2, 1, true, false, false, false, false, u, v, w, p, 0, 4, 0, 1);
    return out;
}

// The pinned kernel authors and transfers the complete model before
// exact cone and circular charts are selected. The world points, analytic
// toroid axes, original edge-list endpoints and all loop uses are retained.
static bool selectExactCarriers(IGESControl_Writer& writer)
{
    const auto model = writer.Model();
    occ::handle<IGESBasic_Group> root;
    int cones = 0, toroids = 0, matrices = 0;
    for (int i = 1; i <= model->NbEntities(); ++i) {
        const auto entity = model->Value(i);
        if (const auto group = occ::down_cast<IGESBasic_Group>(entity); !group.IsNull()) {
            if (!root.IsNull() || group->NbEntities() != 1) return false;
            root = group;
        }
        const auto matrix = occ::down_cast<IGESGeom_TransformationMatrix>(entity);
        if (!matrix.IsNull()) {
            // The stored translation retains the original center. Snap
            // the observed near-cardinal chart axes to their signed basis.
            auto basis = new NCollection_HArray2<double>(1, 3, 1, 4);
            for (int row = 1; row <= 3; ++row) {
                for (int col = 1; col <= 3; ++col) {
                    const double actual = matrix->Data(row, col);
                    const double cardinal = std::round(actual);
                    if (std::abs(actual - cardinal) > 1e-14) return false;
                    basis->SetValue(row, col, cardinal);
                }
                basis->SetValue(row, 4, matrix->Data(row, 4));
            }
            matrix->Init(basis);
            ++matrices;
        }
        const auto face = occ::down_cast<IGESSolid_Face>(entity);
        if (face.IsNull()) continue;
        const auto torus = occ::down_cast<IGESSolid_ToroidalSurface>(face->Surface());
        if (!torus.IsNull()) {
            const auto reference = torus->ReferenceDir();
            const auto x = reference->Value();
            if (x.X() != 1 || std::abs(x.Y()) > 1e-14 || x.Z() != 0) return false;
            reference->Init(gp_XYZ(1, 0, 0));
            ++toroids;
        }
        const auto cone = occ::down_cast<IGESSolid_ConicalSurface>(face->Surface());
        if (cone.IsNull()) continue;
        if (face->NbLoops() != 1 || !face->HasOuterLoop()) return false;
        const auto loop = face->Loop(1);
        if (loop->NbEdges() != 4) return false;
        int seam = 0;
        occ::handle<IGESSolid_EdgeList> edgeList;
        for (int j = 1; j <= loop->NbEdges(); ++j) {
            const auto edges = occ::down_cast<IGESSolid_EdgeList>(loop->Edge(j));
            const int idx = loop->ListIndex(j);
            if (edges.IsNull() || edges->StartVertexList(idx) != edges->EndVertexList(idx)) return false;
            if (edges->StartVertexIndex(idx) == edges->EndVertexIndex(idx)) continue;
            if (seam != 0 && (seam != idx || edgeList != edges)) return false;
            seam = idx;
            edgeList = edges;
        }
        if (seam == 0) return false;
        const auto line = occ::down_cast<IGESGeom_Line>(edgeList->Curve(seam));
        if (line.IsNull() || line->HasTransf()) return false;
        const auto vertices = edgeList->StartVertexList(seam);
        const auto a = vertices->Vertex(edgeList->StartVertexIndex(seam));
        const auto b = vertices->Vertex(edgeList->EndVertexIndex(seam));
        const auto center = cone->LocationPoint()->Value();
        const auto axis = cone->Axis()->Value();
        const auto reference = cone->ReferenceDir()->Value();
        if (axis.X() != 0 || axis.Y() != 0 || std::abs(axis.Z()) != 1 ||
            reference.X() != 1 || reference.Y() != 0 || reference.Z() != 0 ||
            std::abs(a.Y()-center.Y()) > 1e-14 || std::abs(b.Y()-center.Y()) > 1e-14 || a.X() <= center.X() ||
            b.X() <= center.X() || a.Z() == b.Z()) return false;
        auto loops = new NCollection_HArray1<occ::handle<IGESSolid_Loop>>(1, 1);
        loops->SetValue(1, loop);
        const auto low = a.Z() < b.Z() ? a : b;
        const auto high = a.Z() < b.Z() ? b : a;
        face->Init(coneNet(center, low, high, axis.Z() > 0 ? 1 : -1), true, loops);
        ++cones;
    }
    if (cones != 1 || toroids != 2 || matrices != 8 || root.IsNull()) return false;
    model->ClearEntities();
    return writer.AddEntity(root);
}


int main(int argc, char** argv) {
    if (argc != 3) return 1;
    const gp_Ax2 stockAxis(gp_Pnt(0,2,0), gp::DZ(), gp::DX());
    const auto cone = BRepPrimAPI_MakeCone(stockAxis, 8, 16, 6).Shape();
    BRepFilletAPI_MakeFillet fillets(cone);
    NCollection_IndexedMap<TopoDS_Shape, TopTools_ShapeMapHasher> edges;
    TopExp::MapShapes(cone, TopAbs_EDGE, edges);
    int selected = 0;
    for (int i = 1; i <= edges.Extent(); ++i) {
        auto edge = TopoDS::Edge(edges(i));
        BRepAdaptor_Curve curve(edge);
        if (curve.GetType() != GeomAbs_Circle) continue;
        double z = curve.Circle().Location().Z();
        if (std::abs(z) < 1e-12) { fillets.Add(15./16, edge); ++selected; }
        if (std::abs(z - 6) < 1e-12) { fillets.Add(15./8, edge); ++selected; }
    }
    fillets.Build();
    if (selected != 2 || !fillets.IsDone()) return 2;
    const auto body = fillets.Shape();
    const gp_Ax2 boreAxis(gp_Pnt(1.5,4,-1), gp::DZ(), gp::DX());
    const auto cutter = BRepPrimAPI_MakeCylinder(boreAxis, 1, 8).Shape();
    const auto withBore = BRepAlgoAPI_Cut(body, cutter).Shape();
    if (body.IsNull() || withBore.IsNull() || !BRepCheck_Analyzer(body).IsValid() ||
        !BRepCheck_Analyzer(withBore).IsValid()) return 3;
    IGESControl_Writer writer("MM", 1);
    if (!Interface_Static::SetIVal("write.surfacecurve.mode", 0) ||
        !writer.AddShape(withBore) || !writer.Write(argv[2]) ||
        !selectExactCarriers(writer) || !writer.Write(argv[1])) return 4;
    GProp_GProps v, a;
    BRepGProp::VolumeProperties(withBore, v, 1e-10, true);
    BRepGProp::SurfaceProperties(withBore, a, 1e-10);
    std::cout << std::setprecision(17) << "{\"volume\":" << v.Mass() << ",\"area\":" << a.Mass() << "}\n";
}
