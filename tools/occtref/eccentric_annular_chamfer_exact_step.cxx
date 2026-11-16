#include <STEPControl_Writer.hxx>
#include <Interface_EntityIterator.hxx>
#include <Interface_Graph.hxx>
#include <StepData_StepModel.hxx>
#include <StepGeom_Axis1Placement.hxx>
#include <StepGeom_Axis2Placement3d.hxx>
#include <StepGeom_BSplineCurveWithKnots.hxx>
#include <StepGeom_CartesianPoint.hxx>
#include <StepGeom_ConicalSurface.hxx>
#include <StepGeom_Direction.hxx>
#include <StepGeom_SurfaceOfRevolution.hxx>
#include <StepShape_AdvancedFace.hxx>
#include <StepShape_EdgeCurve.hxx>
#include <StepShape_EdgeLoop.hxx>
#include <StepShape_FaceBound.hxx>
#include <StepShape_OrientedEdge.hxx>
#include <StepShape_VertexPoint.hxx>
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


using Point = occ::handle<StepGeom_CartesianPoint>;

static occ::handle<StepGeom_BSplineCurveWithKnots> segment(const Point& a, const Point& b)
{
    auto poles = new NCollection_HArray1<Point>(1, 2);
    poles->SetValue(1, a);
    poles->SetValue(2, b);
    auto multiplicities = new NCollection_HArray1<int>(1, 2, 2);
    auto knots = new NCollection_HArray1<double>(1, 2);
    knots->SetValue(1, 0);
    knots->SetValue(2, 1);
    auto curve = new StepGeom_BSplineCurveWithKnots;
    curve->Init(new TCollection_HAsciiString(""), 1, poles, StepGeom_bscfUnspecified,
                StepData_LFalse, StepData_LFalse, multiplicities, knots, StepGeom_ktPiecewiseBezierKnots);
    return curve;
}

// OCCT authors the entire solid and transfers the original graph first.
// Select the exact generator and seam representations using the graph's
// unchanged shared vertices, analytic circles, original axis and senses.
static bool selectExactCarriers(const occ::handle<StepData_StepModel>& model)
{
    const auto roots = Interface_Graph(model).RootEntities();
    int cones = 0;
    for (int i = 1; i <= model->NbEntities(); ++i) {
        const auto face = occ::down_cast<StepShape_AdvancedFace>(model->Value(i));
        if (face.IsNull()) continue;
        const auto cone = occ::down_cast<StepGeom_ConicalSurface>(face->FaceGeometry());
        if (cone.IsNull()) continue;
        if (face->NbBounds() != 1) return false;
        const auto loop = occ::down_cast<StepShape_EdgeLoop>(face->BoundsValue(1)->Bound());
        if (loop.IsNull() || loop->NbEdgeList() != 4) return false;
        occ::handle<StepShape_EdgeCurve> seam;
        for (int j = 1; j <= loop->NbEdgeList(); ++j) {
            const auto edge = occ::down_cast<StepShape_EdgeCurve>(loop->EdgeListValue(j)->EdgeElement());
            if (edge.IsNull()) return false;
            if (edge->EdgeStart() == edge->EdgeEnd()) continue;
            if (!seam.IsNull() && seam != edge) return false;
            seam = edge;
        }
        if (seam.IsNull() || !seam->SameSense()) return false;
        const auto a = occ::down_cast<StepShape_VertexPoint>(seam->EdgeStart());
        const auto b = occ::down_cast<StepShape_VertexPoint>(seam->EdgeEnd());
        if (a.IsNull() || b.IsNull()) return false;
        const auto p = occ::down_cast<StepGeom_CartesianPoint>(a->VertexGeometry());
        const auto q = occ::down_cast<StepGeom_CartesianPoint>(b->VertexGeometry());
        const auto placement = cone->Position();
        const auto axis = placement->Axis();
        if (p.IsNull() || q.IsNull() || axis.IsNull() || !placement->HasRefDirection()) return false;
        if (axis->DirectionRatiosValue(1) != 0 || axis->DirectionRatiosValue(2) != 0 ||
            placement->RefDirection()->DirectionRatiosValue(1) != 1 ||
            p->CoordinatesValue(2) != placement->Location()->CoordinatesValue(2) ||
            q->CoordinatesValue(2) != placement->Location()->CoordinatesValue(2)) return false;
        const auto sense = (q->CoordinatesValue(3) - p->CoordinatesValue(3)) * axis->DirectionRatiosValue(3);
        if (sense == 0) return false;
        seam->SetEdgeGeometry(segment(p, q));
        occ::handle<StepGeom_Axis1Placement> revolutionAxis = new StepGeom_Axis1Placement;
        revolutionAxis->Init(new TCollection_HAsciiString(""), placement->Location(), true, axis);
        occ::handle<StepGeom_SurfaceOfRevolution> revolution = new StepGeom_SurfaceOfRevolution;
        revolution->Init(new TCollection_HAsciiString(""), sense > 0 ? segment(p, q) : segment(q, p), revolutionAxis);
        face->SetFaceGeometry(revolution);
        ++cones;
    }
    if (cones != 6) return false;
    // Keep the original product roots and only their currently reachable entities.
    model->ClearEntities();
    auto iterator = roots;
    for (iterator.Start(); iterator.More(); iterator.Next()) model->AddWithRefs(iterator.Value());
    return model->NbEntities() > 0;
}

int main(int argc, char** argv) {
    if (argc != 3) return 1;
    const double z0 = 2.0, height = 8.0;
    const std::array<std::array<double, 2>, 2> radii{{{5, 7}, {2, 3}}};
    const std::array<std::array<double, 2>, 2> radial{{{.25, .5}, {.1875, .375}}};
    const std::array<std::array<double, 2>, 2> axial{{{2, 1}, {1, 2}}};
    const auto outer = BRepPrimAPI_MakeCone(gp_Ax2(gp_Pnt(0, 0, z0), gp::DZ()), radii[0][0], radii[0][1], height).Shape();
    const auto inner = BRepPrimAPI_MakeCone(gp_Ax2(gp_Pnt(1.5, 2.0, z0), gp::DZ()), radii[1][0], radii[1][1], height).Shape();
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


    STEPControl_Writer sw;
    if (!Interface_Static::SetIVal("write.surfacecurve.mode", 0)) return 7;
    if (sw.Transfer(result, STEPControl_AsIs) != IFSelect_RetDone || sw.Write(argv[2]) != IFSelect_RetDone ||
        !selectExactCarriers(sw.Model()) || sw.Write(argv[1]) != IFSelect_RetDone) return 8;

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
