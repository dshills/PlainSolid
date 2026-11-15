// Independently authored complete common-base stocks. No PlainSolid import,
// tessellation or section curve: OCCT intersects the two original surfaces.
// A-B=(105/128)(v-f(u))(v-g(u)), B=3+uv/64, base=0.
// f=1/4+3u/8-3u^2/8+u^3/8; g=5/8+u^4/8.
// The node modes use g=f+(1-2u)(1+u^3)/64 for Common, Fuse and Cut.
// The two-node modes use g=f+(4u-1)(4u-3)(1+u^2)/32.
// The tangent diagnostic uses g=f+(2u-1)^2(1+u^2)/64.
// The common denominator multiplier makes every degree-eight model pole dyadic.
#include <BRepAlgoAPI_Common.hxx>
#include <BRepAlgoAPI_Cut.hxx>
#include <BRepAlgoAPI_Fuse.hxx>
#include <BRepBuilderAPI_MakeFace.hxx>
#include <BRepBuilderAPI_MakeSolid.hxx>
#include <BRepBuilderAPI_Sewing.hxx>
#include <BRepCheck_Analyzer.hxx>
#include <BRepGProp.hxx>
#include <GProp_GProps.hxx>
#include <Geom_BezierSurface.hxx>
#include <NCollection_Array2.hxx>
#include <TopExp_Explorer.hxx>
#include <TopoDS.hxx>
#include <TopoDS_Face.hxx>
#include <TopoDS_Solid.hxx>
#include <functional>
#include <iomanip>
#include <iostream>
#include <stdexcept>
#include <string>
#include <vector>

static double powerValue(const double* coefficients, double u) {
  double value = 0;
  for (int i = 4; i >= 0; --i) value = value*u + coefficients[i];
  return value;
}

static TopoDS_Solid stock(bool raised, const double* g, int axis, double lo = 0, double hi = 1) {
  const double v[]={0,.5,1}, f[]={.25,.375,-.375,.125,0};
  double x[9],sum[9]={},prod[9]={};
  for(int i=0;i<9;++i)x[i]=i/8.0;
  for(int i=0;i<5;++i){sum[i]=f[i]+g[i];for(int j=0;j<5;++j)prod[i+j]+=f[i]*g[j];}
  auto binomial=[](int n,int k){double r=1;for(int j=1;j<=k;++j)r=r*(n-j+1)/j;return r;};
  auto bern=[&](const double* p,int i){double r=0;for(int j=0;j<=i;++j)r+=p[j]*binomial(i,j)/binomial(8,j);return r;};
  auto roof=[&](int i,int j){return 3+x[i]*v[j]/64+(raised?(105.0/128)*(bern(prod,i)-v[j]*bern(sum,i)+(j==2?1:0)):0);};
  BRepBuilderAPI_Sewing sewing(1e-10);
  auto face=[&](int nu,int nv,bool reverse,const std::function<gp_Pnt(int,int)>& point, bool clip = true){
    NCollection_Array2<gp_Pnt> poles(1,nu,1,nv);
    for (int i = 0; i < nu; ++i) for (int j = 0; j < nv; ++j) {
      auto p = point(i, j);
      if (axis == 1) p.SetCoord(p.Y(), p.X(), p.Z());
      poles.SetValue(i+1, j+1, p);
    }
    occ::handle<Geom_BezierSurface> support=new Geom_BezierSurface(poles);
    if (clip && (lo != 0 || hi != 1)) support->Segment(lo, hi, 0, 1);
    auto f = BRepBuilderAPI_MakeFace(support, 1e-10).Face();
    if (reverse != (axis == 1)) f.Reverse();
    sewing.Add(f);
  };
  face(2,2,true,[&](int i,int j){return gp_Pnt(i,j,0);});
  face(9,3,false,[&](int i,int j){return gp_Pnt(x[i],v[j],roof(i,j));});
  for (double u : {lo, hi}) {
    const double fv = powerValue(f, u), gv = powerValue(g, u);
    face(3, 2, u == lo, [&](int i, int j) {
      const double gap = fv*gv - v[i]*(fv+gv) + (i == 2 ? 1 : 0);
      const double z = 3 + u*v[i]/64 + (raised ? (105.0/128)*gap : 0);
      return gp_Pnt(u, v[i], j ? z : 0);
    }, false);
  }
  for(int k:{0,2}) face(9,2,k==2,[&](int i,int j){return gp_Pnt(x[i],v[k],j?roof(i,k):0);});
  sewing.Perform();TopExp_Explorer shells(sewing.SewedShape(),TopAbs_SHELL);
  if(!shells.More())throw std::runtime_error("missing source shell");
  auto s=BRepBuilderAPI_MakeSolid(TopoDS::Shell(shells.Current())).Solid();shells.Next();
  if(shells.More()||!BRepCheck_Analyzer(s).IsValid())throw std::runtime_error("invalid source");
  GProp_GProps m;BRepGProp::VolumeProperties(s,m);if(m.Mass()<=0)throw std::runtime_error("wrong source orientation");
  return s;
}
int main(int argc, char** argv){
  const std::string mode = argc == 2 ? argv[1] : "";
  const bool twoNodeCut = mode == "node-pair-cut";
  const bool cut = mode == "node-cut" || twoNodeCut;
  const bool twoNodes = mode == "node-pair" || twoNodeCut;
  const bool oneNode = mode == "node-cut" || mode == "node";
  const bool tangent = mode == "tangent";
  if (argc != 1 && !oneNode && !twoNodes && !tangent) return 2;
  const double disjoint[] = {.625, 0, 0, 0, .125};
  const double crossing[] = {17.0/64, 11.0/32, -.375, 9.0/64, -1.0/32};
  const double twiceCrossing[] = {11.0/32, -1.0/8, 7.0/32, -3.0/8, .5};
  const double tangentBranch[] = {17.0/64, 5.0/16, -19.0/64, 1.0/16, 1.0/16};
  const auto* g = tangent ? tangentBranch : (oneNode ? crossing : (twoNodes ? twiceCrossing : disjoint));
  const int firstOperation = cut ? 2 : 0;
  const int operationEnd = mode == "node" || mode == "node-pair" ? 2 : 3;
  std::cout << std::setprecision(17) << "[";
  bool first = true;
  auto report = [&](const TopoDS_Shape& shape, int axis, int order, int op, int slab) {
    GProp_GProps mass;
    BRepGProp::VolumeProperties(shape, mass, 1e-10);
    int solids = 0;
    std::vector<double> partVolumes;
    const bool measureParts = cut || (tangent && op == 2);
    for (TopExp_Explorer it(shape, TopAbs_SOLID); it.More(); it.Next()) {
      ++solids;
      if (measureParts) {
        GProp_GProps part;
        BRepGProp::VolumeProperties(it.Current(), part, 1e-10);
        partVolumes.push_back(part.Mass());
      }
    }
    if (!first) std::cout << ",";
    first = false;
    std::cout << "{\"Order\":" << order << ",\"Operation\":" << op
      << ",\"Valid\":" << (BRepCheck_Analyzer(shape).IsValid() ? "true" : "false")
      << ",\"Solids\":" << solids << ",\"Volume\":" << mass.Mass();
    if (twoNodes || tangent) std::cout << ",\"Axis\":" << axis;
    if (slab >= 0) std::cout << ",\"FreeSlab\":" << slab;
    if (measureParts) {
      std::cout << ",\"PartVolumes\":[";
      for (std::size_t i = 0; i < partVolumes.size(); ++i) {
        if (i > 0) std::cout << ",";
        std::cout << partVolumes[i];
      }
      std::cout << "]";
    }
    std::cout << "}";
  };
  for (int axis = 0; axis < (twoNodes || tangent ? 2 : 1); ++axis) {
    auto a = stock(true, g, axis), b = stock(false, g, axis);
    for(int order=0;order<2;++order){auto x=order?b:a,y=order?a:b;
      for(int op=firstOperation;op<operationEnd;++op){TopoDS_Shape s;
        if(op==0){BRepAlgoAPI_Common f(x,y);if(!f.IsDone())return 2;s=f.Shape();}
        else if(op==1){BRepAlgoAPI_Fuse f(x,y);if(!f.IsDone())return 2;s=f.Shape();}
        else{BRepAlgoAPI_Cut f(x,y);if(!f.IsDone())return 2;s=f.Shape();}
        report(s, axis, order, op, -1);
      }
    }
    if (twoNodeCut) {
      // Reverse Cut has three exact free slabs. Independently clipping the
      // original stocks at the analytical stations measures each component
      // without using the whole-shape oracle's point-contact grouping.
      const double stations[] = {0, .25, .75, 1};
      for (int slab = 0; slab < 3; ++slab) {
        auto x = stock(false, g, axis, stations[slab], stations[slab+1]);
        auto y = stock(true, g, axis, stations[slab], stations[slab+1]);
        BRepAlgoAPI_Cut f(x, y);
        if (!f.IsDone()) return 2;
        report(f.Shape(), axis, 1, 2, slab);
      }
    }
  }
  std::cout<<"]\n";
}
