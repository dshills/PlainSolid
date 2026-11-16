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
#include <StepGeom_ToroidalSurface.hxx>
#include <StepShape_AdvancedFace.hxx>
#include <StepShape_EdgeCurve.hxx>
#include <StepShape_EdgeLoop.hxx>
#include <StepShape_FaceBound.hxx>
#include <StepShape_OrientedEdge.hxx>
#include <StepShape_VertexPoint.hxx>

#include <TopExp.hxx>
#include <TopTools_ShapeMapHasher.hxx>
#include <TopoDS.hxx>
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

// The independent fillet and bore are already transferred here. Choose
// exact representations for the cone and model seam, and cardinal gauges
// for the source circles and toroids. Every STEP vertex is kept unchanged.
static bool selectExactCarriers(const occ::handle<StepData_StepModel>& model)
{
    const auto roots = Interface_Graph(model).RootEntities();
    for (int i = 1; i <= model->NbEntities(); ++i) {
        const auto frame = occ::down_cast<StepGeom_Axis2Placement3d>(model->Value(i));
        if (frame.IsNull()) continue;
        const auto x = frame->RefDirection();
        const auto z = frame->Axis();
        if (x.IsNull() || z.IsNull() || x->DirectionRatiosValue(1) != 1 ||
            std::abs(x->DirectionRatiosValue(2)) > 1e-14 || std::abs(x->DirectionRatiosValue(3)) > 1e-14) return false;
        auto reference = new NCollection_HArray1<double>(1, 3, 0.);
        reference->SetValue(1, 1.);
        x->SetDirectionRatios(reference);
        if (std::abs(z->DirectionRatiosValue(2)) == 1) {
            if (std::abs(z->DirectionRatiosValue(1)) > 1e-14 || z->DirectionRatiosValue(3) != 0) return false;
            auto meridian = new NCollection_HArray1<double>(1, 3, 0.);
            meridian->SetValue(2, z->DirectionRatiosValue(2));
            z->SetDirectionRatios(meridian);
        } else if (z->DirectionRatiosValue(1) != 0 || z->DirectionRatiosValue(2) != 0 ||
            std::abs(z->DirectionRatiosValue(3)) != 1) {
            return false;
        }
    }
    int cones = 0, lowerToruses = 0;
    for (int i = 1; i <= model->NbEntities(); ++i) {
        const auto face = occ::down_cast<StepShape_AdvancedFace>(model->Value(i));
        if (face.IsNull()) continue;
        const auto torus = occ::down_cast<StepGeom_ToroidalSurface>(face->FaceGeometry());
        if (!torus.IsNull() && torus->Position()->Axis()->DirectionRatiosValue(3) == -1.) {
            // The finite lower loop still follows the original model
            // traversal. Its bound must oppose the cone and cap uses.
            // The default exporter emits the reverse flag on this chart.
            if (face->NbBounds() != 1 || face->BoundsValue(1)->Orientation()) return false;
            face->BoundsValue(1)->SetOrientation(true);
            ++lowerToruses;
        }
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
            std::abs(p->CoordinatesValue(2) - placement->Location()->CoordinatesValue(2)) > 1e-14 ||
            std::abs(q->CoordinatesValue(2) - placement->Location()->CoordinatesValue(2)) > 1e-14) return false;
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
    if (cones != 1 || lowerToruses != 1) return false;
    // Keep the original product roots and only their currently reachable entities.
    model->ClearEntities();
    auto iterator = roots;
    for (iterator.Start(); iterator.More(); iterator.Next()) model->AddWithRefs(iterator.Value());
    return model->NbEntities() > 0;
}


int main(int argc, char** argv) {
    if (argc != 3) return 1;
    const gp_Ax2 stockAxis(gp_Pnt(0, 2, 0), gp::DZ(), gp::DX());
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
    const gp_Ax2 boreAxis(gp_Pnt(1.5, 4, -1), gp::DZ(), gp::DX());
    const auto cutter = BRepPrimAPI_MakeCylinder(boreAxis, 1, 8).Shape();
    const auto withBore = BRepAlgoAPI_Cut(body, cutter).Shape();
    if (body.IsNull() || withBore.IsNull() || !BRepCheck_Analyzer(body).IsValid() ||
        !BRepCheck_Analyzer(withBore).IsValid()) return 3;
    STEPControl_Writer writer;
    if (!Interface_Static::SetIVal("write.surfacecurve.mode",0) ||
        writer.Transfer(withBore, STEPControl_AsIs) != IFSelect_RetDone ||
        writer.Write(argv[2]) != IFSelect_RetDone || !selectExactCarriers(writer.Model()) ||
        writer.Write(argv[1]) != IFSelect_RetDone) return 4;
    GProp_GProps v, a;
    BRepGProp::VolumeProperties(withBore, v, 1e-10, true);
    BRepGProp::SurfaceProperties(withBore, a, 1e-10);
    std::cout << std::setprecision(17) << "{\"volume\":" << v.Mass() << ",\"area\":" << a.Mass() << "}\n";
}
