// Independent OCCT type-144 face backed by a periodic type-128 spline.
#include <BRepAdaptor_Surface.hxx>
#include <BRepBuilderAPI_MakeFace.hxx>
#include <BRepCheck_Analyzer.hxx>
#include <BRepGProp.hxx>
#include <BRepTools.hxx>
#include <BRepTools_WireExplorer.hxx>
#include <BRep_Tool.hxx>
#include <Geom2d_Curve.hxx>
#include <GProp_GProps.hxx>
#include <Geom_BSplineSurface.hxx>
#include <IGESControl_Reader.hxx>
#include <IGESControl_Writer.hxx>
#include <IGESGeom_BSplineCurve.hxx>
#include <IGESGeom_BSplineSurface.hxx>
#include <IGESGeom_CompositeCurve.hxx>
#include <IGESGeom_CurveOnSurface.hxx>
#include <IGESGeom_Line.hxx>
#include <IGESGeom_TrimmedSurface.hxx>
#include <Standard_Failure.hxx>
#include <TopExp.hxx>
#include <TopoDS.hxx>
#include <TopoDS_Wire.hxx>

#include <array>
#include <iomanip>
#include <iostream>
#include <string_view>

static occ::handle<Geom_BSplineSurface> periodicSupport()
{
    const std::array<gp_XY,5> xy{gp_XY(6,0),gp_XY(5,4),gp_XY(1,7),gp_XY(-4,3),gp_XY(-2,-4)};
    NCollection_Array2<gp_Pnt> poles(1,5,1,2);
    for (int row = 0; row < 5; ++row) {
        poles(row+1,1) = gp_Pnt(xy[row].X(),xy[row].Y(),0);
        poles(row+1,2) = gp_Pnt(xy[row].X(),xy[row].Y(),4);
    }
    NCollection_Array1<double> ku(1,6), kv(1,2);
    NCollection_Array1<int> mu(1,6), mv(1,2);
    for (int i = 1; i <= 6; ++i) {
        ku(i) = i-1;
        mu(i) = 1;
    }
    kv(1) = 0; kv(2) = 4;
    mv.Init(2);
    return new Geom_BSplineSurface(poles,ku,kv,mu,mv,2,1,true,false);
}

static TopoDS_Face original(bool crossesSeam)
{
    auto surface = periodicSupport();
    if (!surface->IsUPeriodic()) return {};
    const double u = crossesSeam ? 4.75 : 1.25;
    return BRepBuilderAPI_MakeFace(surface,u,u+.5,1.,3.,1e-9).Face();
}

// Author the seam graph directly in the pinned IGES writer. The BRep
// converter's write-seam control truncates both model arcs in this case.
static bool writeExactSeam(const char* path)
{
    const auto native = periodicSupport();
    auto ku = new NCollection_HArray1<double>(-2,7);
    auto kv = new NCollection_HArray1<double>(-1,2);
    auto weights = new NCollection_HArray2<double>(0,6,0,1);
    auto poles = new NCollection_HArray2<gp_XYZ>(0,6,0,1);
    for (int i = -2; i <= 7; ++i) ku->SetValue(i, i);
    for (int i = -1; i <= 2; ++i) kv->SetValue(i, i <= 0 ? 0. : 4.);
    for (int i = 0; i < 7; ++i) {
        for (int j = 0; j < 2; ++j) {
            poles->SetValue(i,j,native->Pole(i%5+1,j+1).XYZ());
            weights->SetValue(i,j,1.);
        }
    }
    occ::handle<IGESGeom_BSplineSurface> support = new IGESGeom_BSplineSurface;
    support->Init(6,1,2,1,true,false,true,true,false,ku,kv,weights,poles,0.,5.,0.,4.);
    support->InitStatus(0,1,0,0);

    using MemberArray = NCollection_HArray1<occ::handle<IGESData_IGESEntity>>;
    occ::handle<MemberArray> uvMembers = new MemberArray(1,6), modelMembers = new MemberArray(1,6);
    // One continuous lift across U=5. Each horizontal half has an
    // independent parameter line and matching complete model spline.
    const std::array<gp_XY,6> corners{gp_XY(4.75,3),gp_XY(4.75,1),gp_XY(5,1),
                                     gp_XY(5.25,1),gp_XY(5.25,3),gp_XY(5,3)};
    for (int i = 0; i < 6; ++i) {
        const auto a = corners[i], b = corners[(i+1)%6];
        occ::handle<IGESGeom_Line> uv = new IGESGeom_Line;
        uv->Init(gp_XYZ(a.X(),a.Y(),0),gp_XYZ(b.X(),b.Y(),0));
        uv->InitStatus(0,1,0,0);
        uvMembers->SetValue(i+1,uv);

        const int degree = a.X() == b.X() ? 1 : 2;
        const double first = 2.*i, last = first+1.;
        auto knots = new NCollection_HArray1<double>(-degree,degree+1);
        auto w = new NCollection_HArray1<double>(0,degree);
        auto p = new NCollection_HArray1<gp_XYZ>(0,degree);
        for (int k = -degree; k <= degree+1; ++k) knots->SetValue(k,k <= 0 ? first : last);
        for (int k = 0; k <= degree; ++k) w->SetValue(k,1.);
        const gp_XYZ start = native->Value(a.X(),a.Y()).XYZ(), end = native->Value(b.X(),b.Y()).XYZ();
        p->SetValue(0,start);
        p->SetValue(degree,end);
        if (degree == 2) {
            // Recover the middle Bézier control from the exact midpoint
            // of the independently constructed quadratic isoparametric span.
            const gp_XYZ mid = native->Value((a.X()+b.X())/2.,a.Y()).XYZ();
            p->SetValue(1,mid*2. - (start+end)*.5);
        }
        occ::handle<IGESGeom_BSplineCurve> model = new IGESGeom_BSplineCurve;
        model->Init(degree,degree,false,false,true,false,knots,w,p,first,last,gp_XYZ());
        model->InitStatus(0,1,0,0);
        modelMembers->SetValue(i+1,model);
    }
    occ::handle<IGESGeom_CompositeCurve> uv = new IGESGeom_CompositeCurve, model = new IGESGeom_CompositeCurve;
    uv->Init(uvMembers);
    model->Init(modelMembers);
    uv->InitStatus(0,1,0,0);
    model->InitStatus(0,1,0,0);
    occ::handle<IGESGeom_CurveOnSurface> outer = new IGESGeom_CurveOnSurface;
    outer->Init(0,support,uv,model,3);
    outer->InitStatus(0,1,5,0);
    occ::handle<IGESGeom_TrimmedSurface> face = new IGESGeom_TrimmedSurface;
    face->Init(support,1,outer,nullptr);
    IGESControl_Writer writer("MM",0);
    return writer.AddEntity(face) && writer.Write(path);
}

static bool report(const TopoDS_Shape& shape, bool boundary = false)
{
    NCollection_IndexedMap<TopoDS_Shape, TopTools_ShapeMapHasher> faces, edges, wires, vertices;
    TopExp::MapShapes(shape,TopAbs_FACE,faces);
    TopExp::MapShapes(shape,TopAbs_EDGE,edges);
    TopExp::MapShapes(shape,TopAbs_WIRE,wires);
    TopExp::MapShapes(shape,TopAbs_VERTEX,vertices);
    if (faces.Extent() != 1) return false;
    const auto face = TopoDS::Face(faces(1));
    double u0,u1,v0,v1;
    BRepTools::UVBounds(face,u0,u1,v0,v1);
    const BRepAdaptor_Surface surface(face);
    GProp_GProps area, length;
    BRepGProp::SurfaceProperties(face,area);
    BRepGProp::LinearProperties(face,length);
    const bool valid = BRepCheck_Analyzer(shape).IsValid();
    std::cout << std::setprecision(17) << "{\"valid\":" << (valid?"true":"false")
              << ",\"topology\":[" << faces.Extent() << "," << wires.Extent() << "," << edges.Extent() << "," << vertices.Extent()
              << "],\"area\":" << area.Mass() << ",\"perimeter\":" << length.Mass()
              << ",\"periodicU\":" << (surface.IsUPeriodic()?"true":"false")
              << ",\"bounds\":[" << u0 << "," << u1 << "," << v0 << "," << v1 << "],\"samples\":[";
    const std::array<std::array<double,2>,5> uv{{{0,0},{1,0},{1,1},{0,1},{.5,.5}}};
    for (std::size_t i = 0; i < uv.size(); ++i) {
        if (i) std::cout << ",";
        const double u = u0+(u1-u0)*uv[i][0], v = v0+(v1-v0)*uv[i][1];
        gp_Pnt p;
        gp_Vec du,dv;
        surface.D1(u,v,p,du,dv);
        std::cout << "{\"uv\":[" << u << "," << v << "],\"point\":[" << p.X() << "," << p.Y() << "," << p.Z()
                  << "],\"du\":[" << du.X() << "," << du.Y() << "," << du.Z() << "],\"dv\":["
                  << dv.X() << "," << dv.Y() << "," << dv.Z() << "]}";
    }
    std::cout << "]";
    if (boundary) {
        std::cout << ",\"edges\":[";
        std::size_t count = 0;
        for (BRepTools_WireExplorer traversal(BRepTools::OuterWire(face),face); traversal.More(); traversal.Next()) {
            const auto edge = traversal.Current();
            double p0,p1,c0,c1;
            const auto uv = BRep_Tool::CurveOnSurface(edge,face,p0,p1);
            const auto model = BRep_Tool::Curve(edge,c0,c1);
            if (uv.IsNull() || model.IsNull()) return false;
            const bool reverse = edge.Orientation() == TopAbs_REVERSED;
            const auto a = uv->Value(reverse?p1:p0), b = uv->Value(reverse?p0:p1);
            const auto x = model->Value(reverse?c1:c0), y = model->Value(reverse?c0:c1);
            const auto p = uv->Value((p0+p1)/2.);
            const auto z = model->Value((c0+c1)/2.);
            if (count++) std::cout << ",";
            std::cout << "{\"uvStart\":[" << a.X() << "," << a.Y() << "],\"uvEnd\":[" << b.X() << "," << b.Y()
                      << "],\"modelStart\":[" << x.X() << "," << x.Y() << "," << x.Z()
                      << "],\"modelEnd\":[" << y.X() << "," << y.Y() << "," << y.Z()
                      << "],\"uvMid\":[" << p.X() << "," << p.Y()
                      << "],\"modelMid\":[" << z.X() << "," << z.Y() << "," << z.Z()
                      << "],\"uvRange\":[" << p0 << "," << p1 << "],\"modelRange\":[" << c0 << "," << c1 << "]}";
        }
        std::cout << "]";
    }
    std::cout << "}\n";
    return valid;
}

int main(int argc,char** argv)
{
    if (argc != 3) return 2;
    try {
        const std::string_view mode(argv[1]);
        if (mode == "write-exact-seam") {
            return writeExactSeam(argv[2]) ? 0 : 8;
        }
        if (mode == "write" || mode == "write-seam") {
            const TopoDS_Face face = original(mode == "write-seam");
            if (face.IsNull() || !report(face)) return 3;
            IGESControl_Writer writer("MM",0);
            if (!writer.AddShape(face) || !writer.Write(argv[2])) return 4;
            return 0;
        }
        if (mode != "read" && mode != "read-edges") return 2;
        IGESControl_Reader reader;
        if (reader.ReadFile(argv[2]) != IFSelect_RetDone || reader.TransferRoots() != 1 || reader.NbShapes() != 1) return 5;
        return report(reader.OneShape(),mode == "read-edges") ? 0 : 6;
    } catch (const Standard_Failure& e) {
        std::cerr << e.what() << "\n";
        return 7;
    }
}
