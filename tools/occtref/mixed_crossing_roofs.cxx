// Independently build the two graph stocks used in the mixed native
// intersection/difference/union qualification and measure the OCCT results.
#include <BRepAlgoAPI_Common.hxx>
#include <BRepAlgoAPI_Cut.hxx>
#include <BRepAlgoAPI_Fuse.hxx>
#include <BRepBuilderAPI_MakeFace.hxx>
#include <BRepBuilderAPI_MakeSolid.hxx>
#include <BRepBuilderAPI_Sewing.hxx>
#include <BRepBuilderAPI_Transform.hxx>
#include <BRepCheck_Analyzer.hxx>
#include <BRepClass3d_SolidClassifier.hxx>
#include <BRepGProp.hxx>
#include <Geom_BezierSurface.hxx>
#include <GProp.hxx>
#include <GProp_GProps.hxx>
#include <IntCurvesFace_ShapeIntersector.hxx>
#include <NCollection_Array2.hxx>
#include <NCollection_IndexedMap.hxx>
#include <TopExp.hxx>
#include <TopExp_Explorer.hxx>
#include <TopoDS.hxx>
#include <TopoDS_Face.hxx>
#include <TopoDS_Solid.hxx>
#include <TopTools_ShapeMapHasher.hxx>
#include <Standard_Failure.hxx>
#include <gp_Mat.hxx>
#include <gp_Lin.hxx>
#include <gp_Trsf.hxx>

#include <algorithm>
#include <cmath>
#include <functional>
#include <iomanip>
#include <iostream>
#include <stdexcept>
#include <string>
#include <vector>

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
  gp_Mat shift;
  GProp::HOperator(c,gp_Pnt(0,0,0),mass.Mass(),shift);
  const auto inertia=mass.MatrixOfInertia()+shift;
  std::cout << "{\"name\":\"" << name << "\",\"volume\":" << mass.Mass()
    << ",\"area\":" << area.Mass() << ",\"centroid\":[" << c.X() << "," << c.Y() << "," << c.Z()
    << "],\"inertia\":[";
  for (int i=1; i<=3; ++i) {
    if (i>1) std::cout << ',';
    std::cout << '[' << inertia.Value(i,1) << ',' << inertia.Value(i,2) << ',' << inertia.Value(i,3) << ']';
  }
  std::cout << "],\"faces\":" << faces.Extent() << ",\"solids\":" << solids.Extent()
    << ",\"euler\":" << vertices.Extent()-edges.Extent()+faces.Extent()-holes << "}\n";
}

void reportVerticalWall(const char* name, const TopoDS_Shape& shape) {
  // The first four samples cover the tool's X-min wall, through the
  // stock roof at Z=0. The next three use the perpendicular side pair.
  const gp_Pnt points[] = {{0,.5,-.75},{0,.5,-.25},{0,.5,.0625},{0,.5,.25},
                           {0,0,-.75},{0,0,-.25},{0,0,.0625}};
  std::cout << "{\"name\":\"" << name << "\",\"locations\":[";
  for (unsigned i=0; i<sizeof(points)/sizeof(*points); ++i) {
    // Probe open sectors around the wall; direct classifiers on computed
    // OCCT trims can return the adjacent state within their own tolerance.
    int inside=0;
    for (double dx : {-1e-4,1e-4})
      for (double dy : {-1e-4,1e-4}) {
        const gp_Pnt probe(points[i].X()+dx,points[i].Y()+dy,points[i].Z());
        bool contained=false;
        for (TopExp_Explorer it(shape,TopAbs_SOLID); it.More(); it.Next()) {
          BRepClass3d_SolidClassifier classifier(it.Current(),probe,1e-8);
          const auto state=classifier.State();
          if (state!=TopAbs_IN && state!=TopAbs_OUT)
            throw std::runtime_error("non-open wall probe");
          contained=contained || state==TopAbs_IN;
        }
        inside+=contained;
      }
    const int location=inside==0 ? 0 : inside==4 ? 2 : 1;
    std::cout << (i ? "," : "") << location;
  }
  std::cout << "]}\n";
}

void reportObliqueLine(const char* name, const TopoDS_Shape& shape) {
  IntCurvesFace_ShapeIntersector intersector;
  intersector.Load(shape,1e-9);
  const gp_Vec direction(1,1.0/4,0);
  const double scale=direction.Magnitude();
  intersector.Perform(gp_Lin(gp_Pnt(0,1.0/4,0),gp_Dir(direction)),scale/16,scale*15/16);
  if (!intersector.IsDone()) throw std::runtime_error("incomplete oblique line query");
  std::vector<double> stations;
  for (int i=1;i<=intersector.NbPnt();++i) stations.push_back(intersector.WParameter(i)/scale);
  std::sort(stations.begin(),stations.end());
  stations.erase(std::unique(stations.begin(),stations.end(),[](double a,double b) {
    return std::abs(a-b)<1e-8;
  }),stations.end());
  std::cout << "{\"name\":\"" << name << "\",\"stations\":[";
  for (unsigned i=0;i<stations.size();++i) std::cout << (i ? "," : "") << stations[i];
  std::cout << "]}\n";
}

} // namespace

int main(int argc, char** argv) {
  try {
    std::cout << std::setprecision(17);
    const auto a=graphStock(false), b=graphStock(true);
    if (argc==2 && std::string(argv[1])=="vertical-wall") {
      reportVerticalWall("intersection", BRepAlgoAPI_Common(a,b).Shape());
      reportVerticalWall("union", BRepAlgoAPI_Fuse(a,b).Shape());
      reportVerticalWall("difference", BRepAlgoAPI_Cut(a,b).Shape());
      return 0;
    }
    if (argc==2 && std::string(argv[1])=="oblique-line") {
      reportObliqueLine("intersection", BRepAlgoAPI_Common(a,b).Shape());
      reportObliqueLine("union", BRepAlgoAPI_Fuse(a,b).Shape());
      reportObliqueLine("difference", BRepAlgoAPI_Cut(a,b).Shape());
      return 0;
    }
    if (argc==2 && std::string(argv[1])=="placed-mass") {
      gp_Trsf pose;
      pose.SetValues(.6,0,.8,7.0/3,0,1,0,2.0/7,-.8,0,.6,-1.0/3);
      report("intersection",BRepBuilderAPI_Transform(BRepAlgoAPI_Common(a,b).Shape(),pose,true).Shape());
      report("union",BRepBuilderAPI_Transform(BRepAlgoAPI_Fuse(a,b).Shape(),pose,true).Shape());
      report("difference",BRepBuilderAPI_Transform(BRepAlgoAPI_Cut(a,b).Shape(),pose,true).Shape());
      return 0;
    }
    if (argc!=1) throw std::runtime_error("unknown probe");
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
