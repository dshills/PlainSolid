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
#include <Interface_Static.hxx>
#include <BRepAdaptor_Curve.hxx>
#include <BRepAdaptor_Surface.hxx>
#include <BRepAlgoAPI_Cut.hxx>
#include <BRepCheck_Analyzer.hxx>
#include <BRepFilletAPI_MakeChamfer.hxx>
#include <BRepGProp.hxx>
#include <BRepPrimAPI_MakeCone.hxx>
#include <GProp_GProps.hxx>
#include <TopExp.hxx>
#include <TopExp_Explorer.hxx>
#include <NCollection_IndexedMap.hxx>
#include <TopTools_ShapeMapHasher.hxx>
#include <TopoDS.hxx>
#include <gp_Ax2.hxx>
#include <gp_Dir.hxx>

#include <array>
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

// OCCT authors and transfers the complete shape before selecting the
// rational support and seam carriers. Both source axes, all vertex-list
// data, full analytic rims and original loop incidences stay in the graph.
static bool selectExactCarriers(IGESControl_Writer& writer)
{
    const auto model = writer.Model();
    occ::handle<IGESBasic_Group> root;
    int cones = 0;
    for (int i = 1; i <= model->NbEntities(); ++i) {
        const auto entity = model->Value(i);
        if (const auto group = occ::down_cast<IGESBasic_Group>(entity); !group.IsNull()) {
            if (!root.IsNull() || group->NbEntities() != 1) return false;
            root = group;
        }
        const auto face = occ::down_cast<IGESSolid_Face>(entity);
        if (face.IsNull()) continue;
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
            a.Y() != center.Y() || b.Y() != center.Y() || a.X() <= center.X() ||
            b.X() <= center.X() || a.Z() == b.Z()) return false;
        line->Init(a.XYZ(), b.XYZ());
        auto loops = new NCollection_HArray1<occ::handle<IGESSolid_Loop>>(1, 1);
        loops->SetValue(1, loop);
        const auto low = a.Z() < b.Z() ? a : b;
        const auto high = a.Z() < b.Z() ? b : a;
        face->Init(coneNet(center, low, high, axis.Z() > 0 ? 1 : -1), true, loops);
        ++cones;
    }
    if (cones != 6 || root.IsNull()) return false;
    model->ClearEntities();
    return writer.AddEntity(root);
}

int main(int argc, char** argv) {
    if (argc != 3) return 1;
    const bool diagonal = true;
    const double z0 = 2.0, height = 8.0;
    const std::array<std::array<double, 2>, 2> radii{{{5, 7}, {2, 3}}};
    const std::array<std::array<double, 2>, 2> radial{{{.25, .5}, {.1875, .375}}};
    const std::array<std::array<double, 2>, 2> axial{{{2, 1}, {1, 2}}};
    const auto outer = BRepPrimAPI_MakeCone(gp_Ax2(gp_Pnt(0, 0, z0), gp::DZ()), radii[0][0], radii[0][1], height).Shape();
    const auto inner = BRepPrimAPI_MakeCone(gp_Ax2(gp_Pnt(diagonal ? 1.5 : 2.5, diagonal ? 2.0 : 0.0, z0), gp::DZ()), radii[1][0], radii[1][1], height).Shape();
    BRepAlgoAPI_Cut subtract(outer, inner);
    const auto tube = subtract.Shape();
    if (!subtract.IsDone() || tube.IsNull() || !BRepCheck_Analyzer(tube).IsValid()) return 2;
    BRepFilletAPI_MakeChamfer chamfer(tube);
    std::array<std::array<bool, 2>, 2> selected{};
    for (TopExp_Explorer faces(tube, TopAbs_FACE); faces.More(); faces.Next()) {
        const auto cap = TopoDS::Face(faces.Current());
        if (BRepAdaptor_Surface(cap).GetType() != GeomAbs_Plane) continue;
        for (TopExp_Explorer edges(cap, TopAbs_EDGE); edges.More(); edges.Next()) {
            const auto rim = TopoDS::Edge(edges.Current());
            const BRepAdaptor_Curve curve(rim);
            if (curve.GetType() != GeomAbs_Circle) continue;
            const auto circle = curve.Circle();
            const int side = circle.Location().X() > 1 ? 1 : 0;
            const int end = circle.Location().Z() > z0 + height / 2 ? 1 : 0;
            if (selected[side][end] || std::abs(circle.Radius() - radii[side][end]) > 1e-10) return 3;
            const double slope = (radii[side][1] - radii[side][0]) / height;
            chamfer.Add(radial[side][end], axial[side][end] * std::hypot(1., slope), rim, cap);
            selected[side][end] = true;
        }
    }
    for (const auto& side : selected) for (bool end : side) if (!end) return 4;
    chamfer.Build();
    if (!chamfer.IsDone()) return 5;
    const auto result = chamfer.Shape();
    if (result.IsNull() || !BRepCheck_Analyzer(result).IsValid()) return 6;


    IGESControl_Writer iw("MM", 1);
    if (!Interface_Static::SetIVal("write.surfacecurve.mode", 0)) return 9;
    if (!iw.AddShape(result) || !iw.Write(argv[2]) || !selectExactCarriers(iw) || !iw.Write(argv[1])) return 10;

    NCollection_IndexedMap<TopoDS_Shape, TopTools_ShapeMapHasher> solids, faces, edges;
    TopExp::MapShapes(result, TopAbs_SOLID, solids);
    TopExp::MapShapes(result, TopAbs_FACE, faces);
    TopExp::MapShapes(result, TopAbs_EDGE, edges);
    GProp_GProps volume, area;
    BRepGProp::VolumeProperties(result, volume, 1e-10, true);
    BRepGProp::SurfaceProperties(result, area, 1e-10);
    std::cout << std::setprecision(17) << "{\"Solids\":" << solids.Extent()
              << ",\"Faces\":" << faces.Extent() << ",\"Volume\":" << volume.Mass()
              << ",\"Area\":" << area.Mass() << ",\"Rims\":[";
    bool first = true;
    for (int i = 1; i <= edges.Extent(); ++i) {
        const BRepAdaptor_Curve curve(TopoDS::Edge(edges(i)));
        if (curve.GetType() != GeomAbs_Circle) continue;
        const auto c = curve.Circle();
        if (!first) std::cout << ',';
        first = false;
        std::cout << '[' << c.Location().X() << ',' << c.Location().Y() << ','
                  << c.Location().Z() << ',' << c.Radius() << ']';
    }
    std::cout << "]}\n";
}
