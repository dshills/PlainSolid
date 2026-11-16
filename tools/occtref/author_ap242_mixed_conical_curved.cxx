#include <BRepAdaptor_Curve.hxx>
#include <BRepAlgoAPI_Cut.hxx>
#include <BRepCheck_Analyzer.hxx>
#include <BRepFilletAPI_MakeChamfer.hxx>
#include <BRepFilletAPI_MakeFillet.hxx>
#include <BRepGProp.hxx>
#include <BRepPrimAPI_MakeCone.hxx>
#include <BRepPrimAPI_MakeCylinder.hxx>
#include <GProp_GProps.hxx>
#include <Quantity_Color.hxx>
#include <NCollection_IndexedMap.hxx>
#include <NCollection_Sequence.hxx>
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

#include <StepGeom_Circle.hxx>
#include <STEPCAFControl_Writer.hxx>
#include <STEPCAFControl_Reader.hxx>
#include <TDataStd_Name.hxx>
#include <TDocStd_Document.hxx>
#include <TopExp.hxx>
#include <TopExp_Explorer.hxx>
#include <TopTools_ShapeMapHasher.hxx>
#include <TopoDS.hxx>
#include <XCAFDoc_DocumentTool.hxx>
#include <XCAFDoc_ColorTool.hxx>
#include <XCAFDoc_GraphNode.hxx>
#include <XCAFDoc_ShapeTool.hxx>
#include <algorithm>
#include <array>
#include <cmath>
#include <functional>
#include <iomanip>
#include <iostream>
#include <string_view>
#include <vector>

using ShapeMap = NCollection_IndexedMap<TopoDS_Shape, TopTools_ShapeMapHasher>;

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
        const auto entity = model->Value(i);
        const auto circle = occ::down_cast<StepGeom_Circle>(entity);
        const auto torus = occ::down_cast<StepGeom_ToroidalSurface>(entity);
        occ::handle<StepGeom_Axis2Placement3d> frame;
        if (!circle.IsNull()) frame = circle->Position().Axis2Placement3d();
        if (!torus.IsNull()) frame = torus->Position();
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
    if (cones != 7 || lowerToruses != 1) return false;
    // Keep the original product roots and only their currently reachable entities.
    model->ClearEntities();
    auto iterator = roots;
    for (iterator.Start(); iterator.More(); iterator.Next()) model->AddWithRefs(iterator.Value());
    return model->NbEntities() > 0;
}



static TopoDS_Shape rounded() {
    const auto stock = BRepPrimAPI_MakeCone(gp_Ax2(gp_Pnt(0,2,0), gp::DZ(), gp::DX()), 8, 16, 6).Shape();
    BRepFilletAPI_MakeFillet fillets(stock);
    ShapeMap edges;
    TopExp::MapShapes(stock, TopAbs_EDGE, edges);
    int selected = 0;
    for (int i = 1; i <= edges.Extent(); ++i) {
        const auto e = TopoDS::Edge(edges(i));
        const BRepAdaptor_Curve curve(e);
        if (curve.GetType() != GeomAbs_Circle) continue;
        const auto z = curve.Circle().Location().Z();
        if (std::abs(z) < 1e-12) { fillets.Add(15./16, e); ++selected; }
        if (std::abs(z-6) < 1e-12) { fillets.Add(15./8, e); ++selected; }
    }
    fillets.Build();
    if (selected != 2 || !fillets.IsDone()) return {};
    const auto cut = BRepPrimAPI_MakeCylinder(gp_Ax2(gp_Pnt(1.5,4,-1), gp::DZ(), gp::DX()), 1, 8).Shape();
    return BRepAlgoAPI_Cut(fillets.Shape(), cut).Shape();
}

static TopoDS_Shape chamfered() {
    const auto outer = BRepPrimAPI_MakeCone(gp_Ax2(gp_Pnt(0,0,2), gp::DZ()), 5, 7, 8).Shape();
    const auto inner = BRepPrimAPI_MakeCone(gp_Ax2(gp_Pnt(1.5,2,2), gp::DZ()), 2, 3, 8).Shape();
    const auto tube = BRepAlgoAPI_Cut(outer, inner).Shape();
    if (tube.IsNull() || !BRepCheck_Analyzer(tube).IsValid()) return {};
    BRepFilletAPI_MakeChamfer chamfer(tube);
    const std::array<std::array<double,2>,2> r{{{.25,.5},{.1875,.375}}}, z{{{2,1},{1,2}}};
    std::array<std::array<bool,2>,2> seen{};
    for (TopExp_Explorer f(tube, TopAbs_FACE); f.More(); f.Next()) {
        const auto cap = TopoDS::Face(f.Current());
        if (BRepAdaptor_Surface(cap).GetType() != GeomAbs_Plane) continue;
        for (TopExp_Explorer e(cap, TopAbs_EDGE); e.More(); e.Next()) {
            const auto edge = TopoDS::Edge(e.Current());
            const BRepAdaptor_Curve curve(edge);
            if (curve.GetType() != GeomAbs_Circle) continue;
            const auto o = curve.Circle().Location();
            int side = o.X() > 1, end = o.Z() > 6;
            if (seen[side][end]) return {};
            seen[side][end] = true;
            chamfer.Add(r[side][end], z[side][end] * std::hypot(1., side ? .125 : .25), edge, cap);
        }
    }
    for (auto side : seen) for (bool end : side) if (!end) return {};
    chamfer.Build();
    return chamfer.IsDone() ? chamfer.Shape() : TopoDS_Shape{};
}

static void describeUse(const TopoDS_Shape& shape, const TopoDS_Shape& local, const gp_Trsf& pose,
                        bool shared, const std::vector<int>& path = {}, int definition = 0) {
    GProp_GProps mass, area;
    BRepGProp::VolumeProperties(shape, mass, 1e-10, true);
    BRepGProp::SurfaceProperties(shape, area, 1e-10);
    ShapeMap solids, faces, edges, vertices;
    TopExp::MapShapes(shape, TopAbs_SOLID, solids);
    TopExp::MapShapes(shape, TopAbs_FACE, faces);
    TopExp::MapShapes(shape, TopAbs_EDGE, edges);
    TopExp::MapShapes(shape, TopAbs_VERTEX, vertices);
    const auto c = mass.CentreOfMass();
    std::cout << '{';
    if (!path.empty()) {
        std::cout << "\"path\":[";
        for (std::size_t i = 0; i < path.size(); ++i) {
            if (i != 0) std::cout << ',';
            std::cout << path[i];
        }
        std::cout << "],";
    }
    if (definition != 0) std::cout << "\"definition\":" << definition << ',';
    std::cout << "\"sharedWithFirst\":" << (shared ? "true" : "false")
              << ",\"valid\":" << (BRepCheck_Analyzer(local).IsValid() ? "true" : "false")
              << ",\"volume\":" << mass.Mass() << ",\"area\":" << area.Mass()
              << ",\"centroid\":[" << c.X() << ',' << c.Y() << ',' << c.Z()
              << "],\"topology\":[" << solids.Extent() << ',' << faces.Extent() << ',' << edges.Extent() << ',' << vertices.Extent()
              << "],\"pose\":[";
    for (int row = 1; row <= 3; ++row) for (int col = 1; col <= 4; ++col) {
        if (row != 1 || col != 1) std::cout << ',';
        std::cout << pose.Value(row,col);
    }
    std::cout << "]}";
}

static void describe(const occ::handle<XCAFDoc_ShapeTool>& shapes, const TDF_Label& root) {
    NCollection_Sequence<TDF_Label> uses;
    shapes->GetComponents(root, uses, false);
    std::cout << std::setprecision(17) << "{\"occurrences\":[";
    TDF_Label first;
    for (int i = 1; i <= uses.Length(); ++i) {
        TDF_Label part;
        const auto label = uses.Value(i);
        if (!shapes->GetReferredShape(label, part)) return;
        if (i == 1) first = part;
        if (i > 1) std::cout << ',';
        const auto shape = shapes->GetShape(label);
        describeUse(shape, shapes->GetShape(part), shape.Location().Transformation(), part == first);
    }
    std::cout << "]}\n";
}

static bool describeNested(const occ::handle<XCAFDoc_ShapeTool>& shapes, const TDF_Label& root) {
    NCollection_Sequence<TDF_Label> outerUses;
    shapes->GetComponents(root, outerUses, false);
    if (outerUses.Length() != 2) return false;
    TDF_Label group, repeated;
    if (!shapes->GetReferredShape(outerUses.Value(1), group) ||
        !shapes->GetReferredShape(outerUses.Value(2), repeated) || group != repeated) return false;
    NCollection_Sequence<TDF_Label> innerUses;
    shapes->GetComponents(group, innerUses, false);
    if (innerUses.Length() != 3) return false;

    TDF_Label first;
    std::cout << std::setprecision(17) << "{\"groupsShared\":true,\"occurrences\":[";
    for (int i = 1; i <= outerUses.Length(); ++i) {
        const auto parent = shapes->GetShape(outerUses.Value(i)).Location();
        for (int j = 1; j <= innerUses.Length(); ++j) {
            TDF_Label product;
            const auto label = innerUses.Value(j);
            if (!shapes->GetReferredShape(label, product)) return false;
            if (i == 1 && j == 1) first = product;
            if (i != 1 || j != 1) std::cout << ',';
            const auto local = shapes->GetShape(product);
            const auto world = parent * shapes->GetShape(label).Location();
            describeUse(local.Moved(world), local, world.Transformation(), product == first, {i, j});
        }
    }
    std::cout << "]}\n";
    return true;
}

// Independently inspect the transferred XCAF graph and expanded worlds.
// Shared assembly definitions are counted by distinct parent definitions.
struct DAGAssembly {
    TDF_Label definition;
    std::vector<TDF_Label> parents;
};

struct DAGOccurrence {
    TopoDS_Shape local;
    TopLoc_Location world;
    std::vector<int> path;
    int definition;
};

struct DAGDescription {
    std::vector<DAGAssembly> assemblies;
    std::vector<TDF_Label> materials;
    std::vector<DAGOccurrence> occurrences;
};

static bool collectDAG(const occ::handle<XCAFDoc_ShapeTool>& shapes, const TDF_Label& definition,
                       const TopLoc_Location& world, std::vector<int>& path, DAGDescription& report) {
    NCollection_Sequence<TDF_Label> uses;
    if (!shapes->GetComponents(definition, uses, false) || uses.IsEmpty() || path.size() >= 8) return false;
    for (int i = 1; i <= uses.Length(); ++i) {
        const auto use = uses.Value(i);
        TDF_Label child;
        if (!shapes->GetReferredShape(use, child)) return false;
        auto location = world * shapes->GetShape(use).Location();
        path.push_back(i);
        if (shapes->IsAssembly(child)) {
            auto found = std::find_if(report.assemblies.begin(), report.assemblies.end(),
                                      [&](const auto& a) { return a.definition == child; });
            if (found == report.assemblies.end()) {
                report.assemblies.push_back({child, {definition}});
            } else if (std::find(found->parents.begin(), found->parents.end(), definition) == found->parents.end()) {
                found->parents.push_back(definition);
            }
            if (!collectDAG(shapes, child, location, path, report)) return false;
        } else {
            if (report.occurrences.size() >= 256) return false;
            auto found = std::find(report.materials.begin(), report.materials.end(), child);
            const auto index = int(found - report.materials.begin()) + 1;
            if (found == report.materials.end()) report.materials.push_back(child);
            report.occurrences.push_back({shapes->GetShape(child), location, path, index});
        }
        path.pop_back();
    }
    return true;
}

static bool describeDAG(const occ::handle<XCAFDoc_ShapeTool>& shapes, const TDF_Label& root) {
    DAGDescription report;
    report.assemblies.push_back({root, {}});
    std::vector<int> path;
    if (!collectDAG(shapes, root, {}, path, report)) return false;
    const auto shared = std::count_if(report.assemblies.begin(), report.assemblies.end(),
                                     [](const auto& a) { return a.parents.size() > 1; });
    std::cout << std::setprecision(17) << "{\"sharedAssemblies\":" << shared
              << ",\"assemblyDefinitions\":" << report.assemblies.size()
              << ",\"materialDefinitions\":" << report.materials.size() << ",\"occurrences\":[";
    for (std::size_t i = 0; i < report.occurrences.size(); ++i) {
        if (i != 0) std::cout << ',';
        const auto& use = report.occurrences[i];
        describeUse(use.local.Moved(use.world), use.local, use.world.Transformation(),
                    use.definition == 1, use.path, use.definition);
    }
    std::cout << "]}\n";
    return true;
}

static void describeRGB(const Quantity_Color& color) {
    double r, g, b;
    color.Values(r, g, b, Quantity_TOC_sRGB);
    std::cout << '[' << r << ',' << g << ',' << b << ']';
}

static bool describeDAGAppearance(const occ::handle<XCAFDoc_ShapeTool>& shapes,
                                  const occ::handle<XCAFDoc_ColorTool>& colors, const TDF_Label& root) {
    NCollection_Sequence<TDF_Label> labels;
    std::vector<int> ordinals;
    bool comma = false;
    std::cout << std::setprecision(17) << "{\"styledPaths\":[";
    std::function<bool(const TDF_Label&)> visit = [&](const TDF_Label& definition) {
        if (ordinals.size() >= 8) return false;
        NCollection_Sequence<TDF_Label> uses;
        if (!shapes->GetComponents(definition, uses, false) || uses.IsEmpty()) return false;
        for (int i = 1; i <= uses.Length(); ++i) {
            TDF_Label child;
            const auto use = uses.Value(i);
            if (!shapes->GetReferredShape(use, child)) return false;
            labels.Append(use);
            ordinals.push_back(i);
            if (shapes->IsAssembly(child)) {
                if (!visit(child)) return false;
            } else {
                Quantity_Color body, override;
                if (!colors->GetColor(child, XCAFDoc_ColorSurf, body)) return false;
                occ::handle<XCAFDoc_GraphNode> shuo;
                const bool overridden = shapes->FindSHUO(labels, shuo) &&
                    colors->GetColor(shuo->Label(), XCAFDoc_ColorSurf, override);
                if (comma) std::cout << ',';
                comma = true;
                std::cout << "{\"path\":[";
                for (std::size_t j = 0; j < ordinals.size(); ++j) {
                    if (j) std::cout << ',';
                    std::cout << ordinals[j];
                }
                std::cout << "],\"product\":";
                describeRGB(body);
                std::cout << ",\"override\":";
                if (overridden) describeRGB(override); else std::cout << "null";
                std::cout << ",\"faces\":[";
                bool faceComma = false;
                NCollection_Sequence<TDF_Label> faces;
                shapes->GetSubShapes(child, faces);
                for (int j = 1; j <= faces.Length(); ++j) {
                    Quantity_Color c;
                    const auto face = shapes->GetShape(faces.Value(j));
                    if (face.ShapeType() != TopAbs_FACE || !colors->GetColor(faces.Value(j), XCAFDoc_ColorSurf, c)) continue;
                    const BRepAdaptor_Surface surface(TopoDS::Face(face));
                    if (surface.GetType() != GeomAbs_Plane) return false;
                    if (faceComma) std::cout << ',';
                    faceComma = true;
                    std::cout << "{\"planeZ\":" << surface.Plane().Location().Z() << ",\"color\":";
                    describeRGB(c);
                    std::cout << '}';
                }
                std::cout << "]}";
            }
            labels.Remove(labels.Length());
            ordinals.pop_back();
        }
        return true;
    };
    if (!visit(root)) return false;
    std::cout << "]}\n";
    return true;
}

int main(int argc, char** argv) {
    if (argc != 3) return 1;
    occ::handle<TDocStd_Document> doc = new TDocStd_Document(TCollection_ExtendedString("BinXCAF"));
    auto shapes = XCAFDoc_DocumentTool::ShapeTool(doc->Main());
    const std::string_view mode(argv[1]);
    const bool nest = mode == "write-nested" || mode == "write-nested-exact" || mode == "read-nested";
    const bool appearance = mode == "write-dag-appearance" || mode == "write-dag-rounded-appearance" ||
                            mode == "write-dag-rounded-frame-appearance" || mode == "read-dag-appearance";
    const bool roundedFrame = mode == "write-dag-rounded-frame" || mode == "write-dag-rounded-frame-appearance";
    const bool dag = mode == "write-dag" || mode == "write-dag-exact" || mode == "read-dag" || appearance || roundedFrame;
    auto colors = XCAFDoc_DocumentTool::ColorTool(doc->Main());
    const bool exact = mode == "write-exact" || mode == "write-nested-exact" || mode == "write-dag-exact" ||
                       mode == "write-dag-appearance" || mode == "write-dag-rounded-frame";
    if (mode == "write" || mode == "write-exact" || mode == "write-nested" || mode == "write-nested-exact" ||
        mode == "write-dag" || mode == "write-dag-exact" || mode == "write-dag-appearance" ||
        mode == "write-dag-rounded-appearance" || roundedFrame) {
        const auto fillet = rounded(), chamfer = chamfered();
        if (fillet.IsNull() || chamfer.IsNull() || !BRepCheck_Analyzer(fillet).IsValid() || !BRepCheck_Analyzer(chamfer).IsValid()) return 2;
        auto root = shapes->NewShape(), f = shapes->AddShape(fillet, false), c = shapes->AddShape(chamfer, false);
        TDataStd_Name::Set(root,TCollection_ExtendedString("Mixed original rounded and chamfered AP242"));
        TDataStd_Name::Set(f,TCollection_ExtendedString("Conical rounded through-bore"));
        TDataStd_Name::Set(c,TCollection_ExtendedString("Chamfered eccentric annular stock"));
        gp_Trsf a, b, d;
        a.SetTranslation(gp_Vec(30,2,4));
        if (roundedFrame) {
            b.SetValues(-15./19,-6./19,10./19,-30, 10./19,-15./19,6./19,24, 6./19,10./19,15./19,3);
        } else {
            b.SetValues(0,0,1,-30, 1,0,0,24, 0,1,0,3);
        }
        d.SetValues(.6,-.8,0,0, .8,.6,0,-30, 0,0,1,8);
        const auto group = (nest || dag) ? shapes->NewShape() : root;
        shapes->AddComponent(group, f, TopLoc_Location(a));
        const auto secondRollingUse = shapes->AddComponent(group, f, TopLoc_Location(b));
        shapes->AddComponent(group, c, TopLoc_Location(d));
        if (nest) {
            TDataStd_Name::Set(root,TCollection_ExtendedString("Repeated mixed conical AP242 subassembly"));
            TDataStd_Name::Set(group,TCollection_ExtendedString("Original mixed conical group"));
            gp_Trsf outerA, outerB;
            outerA.SetValues(.6,-.8,0,100, .8,.6,0,20, 0,0,1,12);
            outerB.SetValues(1,0,0,-100, 0,.6,-.8,-20, 0,.8,.6,30);
            shapes->AddComponent(root, group, TopLoc_Location(outerA));
            shapes->AddComponent(root, group, TopLoc_Location(outerB));
        }
        if (dag) {
            TDataStd_Name::Set(root, TCollection_ExtendedString("Mixed conical DAG root"));
            TDataStd_Name::Set(group, TCollection_ExtendedString("Shared conical core"));
            auto branchA = shapes->NewShape(), branchB = shapes->NewShape();
            TDataStd_Name::Set(branchA, TCollection_ExtendedString("First conical parent"));
            TDataStd_Name::Set(branchB, TCollection_ExtendedString("Second conical parent"));
            gp_Trsf first, second, direct, middleA, middleB;
            first.SetValues(.6,-.8,0,100, .8,.6,0,20, 0,0,1,12);
            second.SetValues(1,0,0,-100, 0,.6,-.8,-20, 0,.8,.6,30);
            direct.SetTranslation(gp_Vec(0,150,40));
            middleA.SetValues(0,0,1,12, 1,0,0,4, 0,1,0,-6);
            middleB.SetValues(.6,0,.8,-4, 0,1,0,7, -.8,0,.6,9);
            if (appearance) {
                first.SetTranslation(gp_Vec(100, 20, 12));
                middleA.SetTranslation(gp_Vec(12, 4, -6));
            }
            const auto firstMiddleUse = shapes->AddComponent(branchA, group, TopLoc_Location(middleA));
            shapes->AddComponent(branchB, group, TopLoc_Location(middleB));
            const auto firstOuterUse = shapes->AddComponent(root, branchA, TopLoc_Location(first));
            shapes->AddComponent(root, branchB, TopLoc_Location(second));
            shapes->AddComponent(root, c, TopLoc_Location(direct));
            if (appearance) {
                colors->SetColor(f, Quantity_Color(1, 0, 0, Quantity_TOC_sRGB), XCAFDoc_ColorSurf);
                colors->SetColor(c, Quantity_Color(0, 1, 0, Quantity_TOC_sRGB), XCAFDoc_ColorSurf);
                int coloredFaces = 0;
                for (TopExp_Explorer faces(chamfer, TopAbs_FACE); faces.More(); faces.Next()) {
                    const auto face = TopoDS::Face(faces.Current());
                    const BRepAdaptor_Surface surface(face);
                    if (surface.GetType() != GeomAbs_Plane || surface.Plane().Location().Z() != 2.) continue;
                    const auto label = shapes->AddSubShape(c, face);
                    if (label.IsNull()) return 8;
                    colors->SetColor(label, Quantity_Color(1, 1, 0, Quantity_TOC_sRGB), XCAFDoc_ColorSurf);
                    ++coloredFaces;
                }
                NCollection_Sequence<TDF_Label> path;
                path.Append(firstOuterUse);
                path.Append(firstMiddleUse);
                path.Append(secondRollingUse);
                occ::handle<XCAFDoc_GraphNode> shuo;
                if (coloredFaces != 1 || !shapes->SetSHUO(path, shuo)) return 8;
                colors->SetColor(shuo->Label(), Quantity_Color(0, 0, 1, Quantity_TOC_sRGB), XCAFDoc_ColorSurf);
            }

        }
        shapes->UpdateAssemblies();
        STEPCAFControl_Writer writer;
        DESTEP_Parameters params;
        params.WriteSchema = DESTEP_Parameters::WriteMode_StepSchema_AP242DIS;
        params.WriteSurfaceCurMode = false;
        if (!writer.Transfer(doc,params) ||
            (exact && !selectExactCarriers(writer.ChangeWriter().Model())) ||
            writer.Write(argv[2]) != IFSelect_RetDone) return 3;
        if (appearance) return describeDAGAppearance(shapes, colors, root) ? 0 : 8;
        if (dag) return describeDAG(shapes, root) ? 0 : 7;
        if (nest) return describeNested(shapes, root) ? 0 : 7;
        describe(shapes,root);
        return 0;
    }
    if (mode != "read" && mode != "read-nested" && mode != "read-dag" && mode != "read-dag-appearance") return 4;
    STEPCAFControl_Reader reader;
    if (appearance) reader.SetSHUOMode(true);
    if (reader.ReadFile(argv[2]) != IFSelect_RetDone || !reader.Transfer(doc)) return 5;
    NCollection_Sequence<TDF_Label> roots;
    shapes->GetFreeShapes(roots);
    if (roots.Length() != 1) return 6;
    if (appearance) return describeDAGAppearance(shapes, colors, roots.Value(1)) ? 0 : 8;
    if (dag) return describeDAG(shapes, roots.Value(1)) ? 0 : 7;
    if (nest) return describeNested(shapes, roots.Value(1)) ? 0 : 7;
    describe(shapes,roots.Value(1));
    return 0;
}
