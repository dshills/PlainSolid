// Independently build the two graph stocks used in the mixed native
// intersection/difference/union qualification and measure the OCCT results.
#include <BRepAlgoAPI_Common.hxx>
#include <BRepAlgoAPI_Cut.hxx>
#include <BRepAlgoAPI_Fuse.hxx>
#include <BRepBuilderAPI_MakeFace.hxx>
#include <BRepBuilderAPI_MakeSolid.hxx>
#include <BRepBuilderAPI_Sewing.hxx>
#include <BRepCheck_Analyzer.hxx>
#include <BRepGProp.hxx>
#include <Geom_BezierSurface.hxx>
#include <GProp_GProps.hxx>
#include <NCollection_Array2.hxx>
#include <NCollection_IndexedMap.hxx>
#include <TopExp.hxx>
#include <TopExp_Explorer.hxx>
#include <TopoDS.hxx>
#include <TopoDS_Face.hxx>
#include <TopoDS_Solid.hxx>
#include <TopTools_ShapeMapHasher.hxx>
#include <Standard_Failure.hxx>

#include <functional>
#include <iomanip>
#include <iostream>
#include <stdexcept>
#include <string>

namespace {

double roofControl(double x, double y, int i, int j, bool tool) {
  if (!tool) {
    return x * y / 64.0;
  }
  // The tool roof is xy/64 + (x-1/2)^2 - 1/16 + y(1-y)/64.
  // For a degree-two polynomial over [a,b], its middle Bernstein
  // coefficient is value(a) + (b-a)*derivative(a)/2.
  const double xQuadratic[] = {3.0 / 16, -5.0 / 16, 3.0 / 16};
  const double a = -1.0 / 8, b = 9.0 / 8;
  const double yQuadratic[] = {a*(1-a), a*(1-a)+(b-a)*(1-2*a)/2, b*(1-b)};
  return x * y / 64.0 + xQuadratic[i] + yQuadratic[j] / 64.0;
}

TopoDS_Solid graphStock(bool tool) {
  const double x0 = tool ? 0 : -1.0/8, x1 = tool ? 1 : 9.0/8;
  const double y0 = tool ? -1.0/8 : 0, y1 = tool ? 9.0/8 : 1;
  const double z0 = tool ? -0.5 : -1;
  const double x[] = {x0, (x0+x1)/2, x1};
  const double y[] = {y0, (y0+y1)/2, y1};
  const auto roof = [&](int i, int j) { return roofControl(x[i], y[j], i, j, tool); };

  BRepBuilderAPI_Sewing sewing(1e-10);
  auto face = [&](int u, int v, bool reverse, const std::function<gp_Pnt(int,int)>& point) {
    NCollection_Array2<gp_Pnt> controls(1, u, 1, v);
    for (int i=0; i<u; ++i)
      for (int j=0; j<v; ++j)
        controls.SetValue(i+1,j+1,point(i,j));
    occ::handle<Geom_BezierSurface> s = new Geom_BezierSurface(controls);
    auto f = BRepBuilderAPI_MakeFace(s, 1e-10).Face();
    if (reverse) f.Reverse();
    sewing.Add(f);
  };
  face(2,2,true,[&](int i,int j) { return gp_Pnt(x[2*i],y[2*j],z0); });
  face(3,3,false,[&](int i,int j) { return gp_Pnt(x[i],y[j],roof(i,j)); });
  for (int k : {0,2}) {
    face(3,2,k==0,[&](int i,int j) { return gp_Pnt(x[k],y[i],j ? roof(k,i) : z0); });
    face(3,2,k==2,[&](int i,int j) { return gp_Pnt(x[i],y[k],j ? roof(i,k) : z0); });
  }
  sewing.Perform();
  TopExp_Explorer shells(sewing.SewedShape(),TopAbs_SHELL);
  if (!shells.More()) throw std::runtime_error("missing graph stock shell");
  auto solid = BRepBuilderAPI_MakeSolid(TopoDS::Shell(shells.Current())).Solid();
  shells.Next();
  if (shells.More() || !BRepCheck_Analyzer(solid).IsValid())
    throw std::runtime_error("invalid graph stock");
  return solid;
}

void report(const std::string& name, const TopoDS_Shape& shape) {
  if (shape.IsNull() || !BRepCheck_Analyzer(shape).IsValid())
    throw std::runtime_error("invalid " + name + " result");
  using Map = NCollection_IndexedMap<TopoDS_Shape, TopTools_ShapeMapHasher>;
  Map vertices, edges, faces, solids;
  TopExp::MapShapes(shape,TopAbs_VERTEX,vertices);
  TopExp::MapShapes(shape,TopAbs_EDGE,edges);
  TopExp::MapShapes(shape,TopAbs_FACE,faces);
  TopExp::MapShapes(shape,TopAbs_SOLID,solids);
  int holes=0;
  for (int i=1; i<=faces.Extent(); ++i) {
    int wires=0;
    for (TopExp_Explorer wire(faces(i), TopAbs_WIRE); wire.More(); wire.Next()) ++wires;
    holes += wires-1;
  }
  GProp_GProps mass, area;
  BRepGProp::VolumeProperties(shape,mass);
  BRepGProp::SurfaceProperties(shape,area);
  const auto c=mass.CentreOfMass();
  std::cout << "{\"name\":\"" << name << "\",\"volume\":" << mass.Mass()
    << ",\"area\":" << area.Mass() << ",\"centroid\":[" << c.X() << "," << c.Y() << "," << c.Z()
    << "],\"faces\":" << faces.Extent() << ",\"solids\":" << solids.Extent()
    << ",\"euler\":" << vertices.Extent()-edges.Extent()+faces.Extent()-holes << "}\n";
}

} // namespace

int main() {
  try {
    std::cout << std::setprecision(17);
    const auto a=graphStock(false), b=graphStock(true);
    report("stock",a);
    report("tool",b);
    report("intersection", BRepAlgoAPI_Common(a,b).Shape());
    report("union", BRepAlgoAPI_Fuse(a,b).Shape());
    report("difference", BRepAlgoAPI_Cut(a,b).Shape());
    return 0;
  } catch (const Standard_Failure& e) {
    std::cerr << e.what() << '\n';
  } catch (const std::exception& e) {
    std::cerr << e.what() << '\n';
  }
  return 1;
}
