// Independently convert the analytic radial polynomial into a Bezier roof and
// intersect its closed stock with a box. No PlainSolid recipe is read.
#include <BRepAlgoAPI_Common.hxx>
#include <BRepBuilderAPI_MakeFace.hxx>
#include <BRepBuilderAPI_MakeSolid.hxx>
#include <BRepBuilderAPI_Sewing.hxx>
#include <BRepCheck_Analyzer.hxx>
#include <BRepGProp.hxx>
#include <BRepPrimAPI_MakeBox.hxx>
#include <Geom_BezierSurface.hxx>
#include <GProp_GProps.hxx>
#include <NCollection_Array2.hxx>
#include <NCollection_IndexedMap.hxx>
#include <Standard_Failure.hxx>
#include <TopExp.hxx>
#include <TopExp_Explorer.hxx>
#include <TopTools_ShapeMapHasher.hxx>
#include <TopoDS.hxx>
#include <TopoDS_Solid.hxx>
#include <array>
#include <cmath>
#include <functional>
#include <iomanip>
#include <iostream>
#include <stdexcept>

namespace {
using Grid = std::array<std::array<long double, 9>, 9>;
Grid multiply(const Grid& a, const Grid& b) {
  Grid r{};
  for (int i=0; i<9; ++i) for (int j=0; j<9; ++j) if (a[i][j] != 0)
    for (int k=0; k+i<9; ++k) for (int l=0; l+j<9; ++l)
      r[i+k][j+l] += a[i][j]*b[k][l];
  return r;
}
int choose(int n, int k) {
  int r=1;
  for (int i=1; i<=k; ++i) r=r*(n+1-i)/i;
  return r;
}
Grid roofPoles() {
  Grid power{};
  power[0][0]=-1225.L/256;
  for (int i : {5,15,25}) {
    Grid factor{};
    factor[0][0]=.5L-i*i/4096.L;
    factor[1][0]=factor[0][1]=-1;
    factor[2][0]=factor[0][2]=1;
    power=multiply(power,factor);
  }
  Grid poles{};
  for (int i=0; i<9; ++i) for (int j=0; j<9; ++j)
    for (int k=0; k<=i; ++k) for (int l=0; l<=j; ++l)
      poles[i][j] += power[k][l]*choose(i,k)*choose(j,l)/choose(8,k)/choose(8,l);
  return poles;
}
TopoDS_Solid radialStock() {
  const auto roof=roofPoles();
  constexpr double floor=-4;
  BRepBuilderAPI_Sewing sewing(1e-10);
  auto face=[&](int u,int v,bool reverse,const std::function<gp_Pnt(int,int)>& f) {
    NCollection_Array2<gp_Pnt> controls(1,u,1,v);
    for (int i=0;i<u;++i) for (int j=0;j<v;++j) controls.SetValue(i+1,j+1,f(i,j));
    occ::handle<Geom_BezierSurface> s=new Geom_BezierSurface(controls);
    auto q=BRepBuilderAPI_MakeFace(s,1e-10).Face();
    if (reverse) q.Reverse();
    sewing.Add(q);
  };
  face(2,2,true,[&](int i,int j){return gp_Pnt(i,j,floor);});
  face(9,9,false,[&](int i,int j){return gp_Pnt(i/8.,j/8.,double(roof[i][j]));});
  for (int k : {0,8}) {
    face(9,2,k==0,[&](int i,int j){return gp_Pnt(k/8.,i/8.,j ? double(roof[k][i]) : floor);});
    face(9,2,k==8,[&](int i,int j){return gp_Pnt(i/8.,k/8.,j ? double(roof[i][k]) : floor);});
  }
  sewing.Perform();
  TopExp_Explorer shells(sewing.SewedShape(),TopAbs_SHELL);
  if (!shells.More()) throw std::runtime_error("missing independent radial shell");
  auto b=BRepBuilderAPI_MakeSolid(TopoDS::Shell(shells.Current())).Solid();
  shells.Next();
  if (shells.More() || !BRepCheck_Analyzer(b).IsValid()) throw std::runtime_error("invalid independent radial stock");
  return b;
}
}
int main() {
  try {
    const auto box=BRepPrimAPI_MakeBox(gp_Pnt(-.125,-.125,0),gp_Pnt(1.125,1.125,1)).Shape();
    BRepAlgoAPI_Common boolean(box,radialStock());
    const auto common=boolean.Shape();
    if (!boolean.IsDone() || common.IsNull() || !BRepCheck_Analyzer(common).IsValid()) throw std::runtime_error("invalid radial common");
    using Map=NCollection_IndexedMap<TopoDS_Shape,TopTools_ShapeMapHasher>;
    Map faces,solids;
    TopExp::MapShapes(common,TopAbs_FACE,faces);TopExp::MapShapes(common,TopAbs_SOLID,solids);
    GProp_GProps total;
    BRepGProp::VolumeProperties(common,total,1e-12);
    std::cout<<std::setprecision(17)<<"{\"volume\":"<<total.Mass()<<",\"faces\":"<<faces.Extent()<<",\"solids\":"<<solids.Extent()<<",\"components\":[";
    for (int i=1;i<=solids.Extent();++i) {
      Map fs; TopExp::MapShapes(solids(i),TopAbs_FACE,fs);
      int holes=0;
      for (int j=1;j<=fs.Extent();++j) {
        int wires=0; for (TopExp_Explorer w(fs(j),TopAbs_WIRE);w.More();w.Next()) ++wires;
        holes+=wires-1;
      }
      GProp_GProps m; BRepGProp::VolumeProperties(solids(i),m,1e-12);
      if(i>1) std::cout<<',';
      std::cout<<"{\"faces\":"<<fs.Extent()<<",\"holes\":"<<holes<<",\"volume\":"<<m.Mass()<<'}';
    }
    std::cout<<"]}\n";
    return 0;
  } catch(const Standard_Failure& e) { std::cerr<<e.what()<<'\n'; }
    catch(const std::exception& e) { std::cerr<<e.what()<<'\n'; }
  return 1;
}
