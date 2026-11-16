// Independent OCCT type-144 face backed by a periodic type-128 spline.
#include <BRepAdaptor_Surface.hxx>
#include <BRepBuilderAPI_MakeFace.hxx>
#include <BRepCheck_Analyzer.hxx>
#include <BRepGProp.hxx>
#include <BRepTools.hxx>
#include <GProp_GProps.hxx>
#include <Geom_BSplineSurface.hxx>
#include <IGESControl_Reader.hxx>
#include <IGESControl_Writer.hxx>
#include <Standard_Failure.hxx>
#include <TopExp.hxx>
#include <TopoDS.hxx>

#include <array>
#include <iomanip>
#include <iostream>
#include <string_view>

static TopoDS_Face original(bool crossesSeam)
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
    occ::handle<Geom_BSplineSurface> surface = new Geom_BSplineSurface(poles,ku,kv,mu,mv,2,1,true,false);
    if (!surface->IsUPeriodic()) return {};
    const double u = crossesSeam ? 4.75 : 1.25;
    return BRepBuilderAPI_MakeFace(surface,u,u+.5,1.,3.,1e-9).Face();
}

static bool report(const TopoDS_Shape& shape)
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
    std::cout << "]}\n";
    return valid;
}

int main(int argc,char** argv)
{
    if (argc != 3) return 2;
    try {
        const std::string_view mode(argv[1]);
        if (mode == "write" || mode == "write-seam") {
            const TopoDS_Face face = original(mode == "write-seam");
            if (face.IsNull() || !report(face)) return 3;
            IGESControl_Writer writer("MM",0);
            if (!writer.AddShape(face) || !writer.Write(argv[2])) return 4;
            return 0;
        }
        if (mode != "read") return 2;
        IGESControl_Reader reader;
        if (reader.ReadFile(argv[2]) != IFSelect_RetDone || reader.TransferRoots() != 1 || reader.NbShapes() != 1) return 5;
        return report(reader.OneShape()) ? 0 : 6;
    } catch (const Standard_Failure& e) {
        std::cerr << e.what() << "\n";
        return 7;
    }
}
