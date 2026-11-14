// Independent OCCT regularized Booleans on analytically authored bodies.
#include <Geom_BezierCurve.hxx>
#include <NCollection_Array1.hxx>

static TopoDS_Shape quadraticGraphPrism(double c, double xSquare) {
  NCollection_Array1<gp_Pnt> poles(1, 3);
  poles.SetValue(1, gp_Pnt(0, 0, c));
  poles.SetValue(2, gp_Pnt(.5, 0, c));
  poles.SetValue(3, gp_Pnt(1, 0, c+xSquare));
  auto top = new Geom_BezierCurve(poles);
  BRepBuilderAPI_MakeWire wire;
  wire.Add(BRepBuilderAPI_MakeEdge(gp_Pnt(0,0,0), gp_Pnt(1,0,0)).Edge());
  wire.Add(BRepBuilderAPI_MakeEdge(gp_Pnt(1,0,0), gp_Pnt(1,0,c+xSquare)).Edge());
  auto edge = BRepBuilderAPI_MakeEdge(top).Edge();
  edge.Reverse();
  wire.Add(edge);
  wire.Add(BRepBuilderAPI_MakeEdge(gp_Pnt(0,0,c), gp_Pnt(0,0,0)).Edge());
  auto face = BRepBuilderAPI_MakeFace(wire.Wire(), true).Face();
  auto out = BRepPrimAPI_MakePrism(face, gp_Vec(0,1,0)).Shape();
  GProp_GProps volume;
  BRepGProp::VolumeProperties(out, volume);
  if (volume.Mass() < 0) out.Reverse();
  return out;
}

static void graphBooleanRecords(std::ostream& out) {
  auto a = quadraticGraphPrism(.75, 1);
  auto b = BRepPrimAPI_MakeBox(1,1,1).Shape();
  record(out, "graph-union", BRepAlgoAPI_Fuse(a,b).Shape());
  record(out, "graph-intersection", BRepAlgoAPI_Common(a,b).Shape());
  record(out, "graph-difference", BRepAlgoAPI_Cut(a,b).Shape());
}

static TopoDS_Shape saddleGraphPrism(double factor) {
  BRepBuilderAPI_Sewing sewing(1e-9);
  for (int fixed=0;fixed<3;++fixed) for (int side=0;side<2;++side) {
    const int u=(fixed+1)%3, v=(fixed+2)%3;
    NCollection_Array2<gp_Pnt> poles(1,2,1,2);
    for (int i=0;i<2;++i) for (int j=0;j<2;++j) {
      double p[3]{};p[fixed]=side;p[u]=i;p[v]=j;
      const auto z=p[2]*(2.+.5*p[0]*p[1]+factor*(p[0]-.5)*(p[1]-.5));
      poles.SetValue(i+1,j+1,gp_Pnt(p[0],p[1],z));
    }
    NCollection_Array1<double> k(1,2);k.SetValue(1,0);k.SetValue(2,1);NCollection_Array1<int> m(1,2);m.Init(2);
    occ::handle<Geom_BSplineSurface> s=new Geom_BSplineSurface(poles,k,k,m,m,1,1);
    auto face=BRepBuilderAPI_MakeFace(s,1e-9).Face();if(side==0)face.Reverse();sewing.Add(face);
  }
  sewing.Perform();auto shape=sewing.SewedShape();TopExp_Explorer e(shape,TopAbs_SHELL);
  if(!e.More())throw std::runtime_error("missing saddle graph shell");
  auto s=BRepBuilderAPI_MakeSolid(TopoDS::Shell(e.Current())).Solid();e.Next();
  if(e.More()||!BRepCheck_Analyzer(s).IsValid())throw std::runtime_error("invalid saddle graph shell");
  return s;
}

static void saddleGraphBooleanRecords(std::ostream& out) {
  auto a=saddleGraphPrism(1.),b=saddleGraphPrism(0.);
  splineShellRecord(out,"saddle-union",BRepAlgoAPI_Fuse(a,b).Shape());
  splineShellRecord(out,"saddle-common",BRepAlgoAPI_Common(a,b).Shape());
}

// Independently authored common-base graphs. The two curved roofs coincide
// on the entire left half; the first rises linearly above the second at the
// interior knot. The remaining faces close each original material shell.
static TopoDS_Shape partialGraphPrism(bool raised) {
  BRepBuilderAPI_Sewing sewing(1e-9);
  for (int fixed=0;fixed<3;++fixed) for (int side=0;side<2;++side) {
    const int u=(fixed+1)%3,v=(fixed+2)%3;
    const int nU=raised&&u==0?3:2, nV=raised&&v==0?3:2;
    NCollection_Array2<gp_Pnt> poles(1,nU,1,nV);
    for(int i=0;i<nU;++i)for(int j=0;j<nV;++j){
      double p[3]{};p[fixed]=side;p[u]=double(i)/(nU-1);p[v]=double(j)/(nV-1);
      const auto z=p[2]*(2.+p[0]*p[1]+(raised?std::max(p[0]-.5,0.):0.));
      poles.SetValue(i+1,j+1,gp_Pnt(p[0],p[1],z));
    }
    NCollection_Array1<double> ku(1,nU),kv(1,nV);
    NCollection_Array1<int> mu(1,nU),mv(1,nV);
    for(int i=1;i<=nU;++i){ku.SetValue(i,double(i-1)/(nU-1));mu.SetValue(i,i==1||i==nU?2:1);}
    for(int j=1;j<=nV;++j){kv.SetValue(j,double(j-1)/(nV-1));mv.SetValue(j,j==1||j==nV?2:1);}
    occ::handle<Geom_BSplineSurface> s=new Geom_BSplineSurface(poles,ku,kv,mu,mv,1,1);
    auto face=BRepBuilderAPI_MakeFace(s,1e-9).Face();if(side==0)face.Reverse();sewing.Add(face);
  }
  sewing.Perform();auto shape=sewing.SewedShape();TopExp_Explorer e(shape,TopAbs_SHELL);
  if(!e.More())throw std::runtime_error("missing partial graph shell");
  auto s=BRepBuilderAPI_MakeSolid(TopoDS::Shell(e.Current())).Solid();e.Next();
  if(e.More()||!BRepCheck_Analyzer(s).IsValid())throw std::runtime_error("invalid partial graph shell");
  return s;
}

// Independent explicit boundary of the regularized positive-thickness
// region. The zero-thickness x=.5 side collapses to the common graph edge.
static TopoDS_Shape partialGraphRegularizedDifference() {
  BRepBuilderAPI_Sewing sewing(1e-9);
  for(int fixed=0;fixed<3;++fixed)for(int side=0;side<2;++side){
    if(fixed==0&&side==0)continue;
    const int u=(fixed+1)%3,v=(fixed+2)%3;
    NCollection_Array2<gp_Pnt> poles(1,2,1,2);
    for(int i=0;i<2;++i)for(int j=0;j<2;++j){
      double p[3]{};p[fixed]=side;p[u]=i;p[v]=j;
      const double x=.5+.5*p[0],y=p[1],z=2.+x*y+p[2]*(x-.5);
      poles.SetValue(i+1,j+1,gp_Pnt(x,y,z));
    }
    NCollection_Array1<double> k(1,2);k.SetValue(1,0);k.SetValue(2,1);
    NCollection_Array1<int> m(1,2);m.Init(2);
    occ::handle<Geom_BSplineSurface> surface=new Geom_BSplineSurface(poles,k,k,m,m,1,1);
    auto face=BRepBuilderAPI_MakeFace(surface,1e-9).Face();if(side==0)face.Reverse();sewing.Add(face);
  }
  sewing.Perform();auto shape=sewing.SewedShape();TopExp_Explorer e(shape,TopAbs_SHELL);
  if(!e.More())throw std::runtime_error("missing regularized difference shell");
  auto s=BRepBuilderAPI_MakeSolid(TopoDS::Shell(e.Current())).Solid();e.Next();
  if(e.More()||!BRepCheck_Analyzer(s).IsValid())throw std::runtime_error("invalid regularized difference shell");
  return s;
}

static void partialGraphBooleanRecords(std::ostream& out) {
  auto a=partialGraphPrism(true),b=partialGraphPrism(false);
  splineShellRecord(out,"partial-union",BRepAlgoAPI_Fuse(a,b).Shape());
  splineShellRecord(out,"partial-common",BRepAlgoAPI_Common(a,b).Shape());
  splineShellRecord(out,"partial-raw-cut",BRepAlgoAPI_Cut(a,b).Shape());
  splineShellRecord(out,"partial-difference",partialGraphRegularizedDifference());
}

// Independent analytical paraboloid roof plus a shared rectangular base.
// OCCT constructs and intersects the closed B-reps itself.
static TopoDS_Shape ellipticalGraphPrism(bool curved) {
  BRepBuilderAPI_Sewing sewing(1e-9);
  const double sq[3]{.25,-.25,.25};
  for(int fixed=0;fixed<3;++fixed)for(int side=0;side<2;++side){
    const int u=(fixed+1)%3,v=(fixed+2)%3;
    const int du=u==2?1:2,dv=v==2?1:2;
    NCollection_Array2<gp_Pnt> poles(1,du+1,1,dv+1);
    for(int i=0;i<=du;++i)for(int j=0;j<=dv;++j){
      int p[3]{};p[fixed]=side*(fixed==2?1:2);p[u]=i;p[v]=j;
      const double z=p[2]*(24.+(curved?1.-16.*sq[p[0]]-64.*sq[p[1]]:0.));
      poles.SetValue(i+1,j+1,gp_Pnt(.5*p[0],.5*p[1],z));
    }
    NCollection_Array1<double> k(1,2);k.SetValue(1,0);k.SetValue(2,1);
    NCollection_Array1<int> mu(1,2),mv(1,2);mu.Init(du+1);mv.Init(dv+1);
    occ::handle<Geom_BSplineSurface> s=new Geom_BSplineSurface(poles,k,k,mu,mv,du,dv);
    auto face=BRepBuilderAPI_MakeFace(s,1e-9).Face();if(side==0)face.Reverse();sewing.Add(face);
  }
  sewing.Perform();auto shape=sewing.SewedShape();TopExp_Explorer e(shape,TopAbs_SHELL);
  if(!e.More())throw std::runtime_error("missing ellipse graph shell");
  auto s=BRepBuilderAPI_MakeSolid(TopoDS::Shell(e.Current())).Solid();e.Next();
  if(e.More()||!BRepCheck_Analyzer(s).IsValid())throw std::runtime_error("invalid ellipse graph shell");
  return s;
}

static void ellipticalGraphRecord(std::ostream& out,const char* name,const TopoDS_Shape& shape) {
  if(!BRepCheck_Analyzer(shape).IsValid())throw std::runtime_error("invalid elliptical graph result");
  GProp_GProps volume,area;
  BRepGProp::VolumeProperties(shape,volume,1e-12);
  for(TopExp_Explorer f(shape,TopAbs_FACE);f.More();f.Next()) {
    GProp_GProps local;BRepGProp::SurfaceProperties(f.Current(),local,1e-12);area.Add(local);
  }
  const auto c=volume.CentreOfMass();
  out<<std::setprecision(17)<<"{\"Name\":\""<<name<<"\",\"Volume\":"<<volume.Mass()<<",\"Area\":"<<area.Mass()<<",\"Centroid\":["<<c.X()<<','<<c.Y()<<','<<c.Z()<<"]}\n";
}
static void ellipticalGraphBooleanRecords(std::ostream& out) {
  auto a=ellipticalGraphPrism(true),b=ellipticalGraphPrism(false);
  ellipticalGraphRecord(out,"ellipse-union",BRepAlgoAPI_Fuse(a,b).Shape());
  ellipticalGraphRecord(out,"ellipse-common",BRepAlgoAPI_Common(a,b).Shape());
  ellipticalGraphRecord(out,"ellipse-difference",BRepAlgoAPI_Cut(a,b).Shape());
}

static void tangentSplineShellRecords(std::ostream& out, bool quartic = false, bool separable = false) {
  BRepBuilderAPI_Sewing sewing(1e-9);
  const double sq[3]{.25,-.25,.25};
  const double q2[5]{3,0,-1,0,3}, q4[5]{1,-1,1,-1,1};
  const int degree[3]{quartic || separable ? 4 : 2, quartic ? 4 : 2, 1};
  for (int fixed = 0; fixed < 3; ++fixed) for (int side = 0; side < 2; ++side) {
    const int u = (fixed + 1) % 3, v = (fixed + 2) % 3;
    const int du = degree[u], dv = degree[v];
    NCollection_Array2<gp_Pnt> poles(1,du+1,1,dv+1);
    for (int i = 0; i <= du; ++i) for (int j = 0; j <= dv; ++j) {
      int p[3]{};
      p[fixed] = side * degree[fixed];
      p[u] = i;
      p[v] = j;
      const double z = p[2] == 0 ? -1 : separable ? -q4[p[0]]/16 - sq[p[1]] : quartic ?
        -(9*(q4[p[0]]+q4[p[1]])+2*q2[p[0]]*q2[p[1]])/256 :
        -sq[p[0]] - sq[p[1]];
      poles.SetValue(i+1,j+1,gp_Pnt(double(p[0])/degree[0],double(p[1])/degree[1],z));
    }
    NCollection_Array1<double> k(1,2);
    k.SetValue(1,0); k.SetValue(2,1);
    NCollection_Array1<int> mu(1,2),mv(1,2);
    mu.Init(du+1); mv.Init(dv+1);
    occ::handle<Geom_BSplineSurface> support = new Geom_BSplineSurface(poles,k,k,mu,mv,du,dv);
    if (separable && u == 0) support->InsertUKnot(.5, 4, 0);
    if (separable && v == 0) support->InsertVKnot(.5, 4, 0);
    auto face = BRepBuilderAPI_MakeFace(support,1e-9).Face();
    if (side == 0) face.Reverse();
    sewing.Add(face);
  }
  sewing.Perform();
  TopExp_Explorer ex(sewing.SewedShape(),TopAbs_SHELL);
  if (!ex.More()) throw std::runtime_error("missing tangent roof");
  auto tool = BRepBuilderAPI_MakeSolid(TopoDS::Shell(ex.Current())).Solid();
  ex.Next();
  if (ex.More() || !BRepCheck_Analyzer(tool).IsValid()) throw std::runtime_error("invalid tangent roof");
  const auto stock = BRepPrimAPI_MakeBox(1,1,1).Shape();
  auto common = BRepAlgoAPI_Common(stock,tool);
  if (!common.IsDone() || TopExp_Explorer(common.Shape(),TopAbs_SOLID).More())
    throw std::runtime_error("isolated tangency produced common material");
  record(out,"isolated-tangent-stock-cut",BRepAlgoAPI_Cut(stock,tool).Shape(),1e-11);
}

static void multipleKnotTangentSplineShellRecords(std::ostream& out, bool splitStock = false) {
  // A bilinear roof constructed directly in OCCT from exact dyadic
  // coordinates has two isolated contacts at its own knot vertices.
  const std::vector<double> stations[3]{{0,.25,.5,.75,1},{0,.5,1},{0,1}};
  const double xGap[5]{.125,0,.125,0,.125}, yGap[3]{.125,0,.125};
  BRepBuilderAPI_Sewing sewing(1e-9);
  for (int axis=0;axis<3;++axis) for (int side=0;side<2;++side) {
    const int u=(axis+1)%3,v=(axis+2)%3;
    const int nu=int(stations[u].size()),nv=int(stations[v].size());
    NCollection_Array2<gp_Pnt> poles(1,nu,1,nv);
    for (int i=0;i<nu;++i) for (int j=0;j<nv;++j) {
      int p[3]{};p[u]=i;p[v]=j;p[axis]=side*int(stations[axis].size()-1);
      const double z=p[2] ? -xGap[p[0]]-yGap[p[1]] : -1;
      poles.SetValue(i+1,j+1,gp_Pnt(stations[0][p[0]],stations[1][p[1]],z));
    }
    NCollection_Array1<double> ku(1,nu),kv(1,nv);
    NCollection_Array1<int> mu(1,nu),mv(1,nv);
    for(int i=1;i<=nu;++i){ku.SetValue(i,stations[u][i-1]);mu.SetValue(i,i==1||i==nu?2:1);}
    for(int j=1;j<=nv;++j){kv.SetValue(j,stations[v][j-1]);mv.SetValue(j,j==1||j==nv?2:1);}
    occ::handle<Geom_BSplineSurface> support=new Geom_BSplineSurface(poles,ku,kv,mu,mv,1,1);
    auto f=BRepBuilderAPI_MakeFace(support,1e-9).Face();if(side==0)f.Reverse();sewing.Add(f);
  }
  sewing.Perform();
  TopExp_Explorer ex(sewing.SewedShape(),TopAbs_SHELL);
  if(!ex.More())throw std::runtime_error("missing two-contact tool shell");
  auto tool=BRepBuilderAPI_MakeSolid(TopoDS::Shell(ex.Current())).Solid();
  ex.Next();
  if(ex.More()||!BRepCheck_Analyzer(tool).IsValid())throw std::runtime_error("invalid two-contact tool shell");
  TopoDS_Shape stock;
  if (!splitStock) {
    stock=BRepPrimAPI_MakeBox(1,1,1).Shape();
  } else {
    BRepBuilderAPI_Sewing s(1e-9);
    for (int axis=0;axis<3;++axis) for (int side=0;side<2;++side) {
      const int u=(axis+1)%3,v=(axis+2)%3;
      for(int span=0;span<(axis==0?1:2);++span) {
        NCollection_Array2<gp_Pnt> poles(1,2,1,2);
        for(int i=0;i<2;++i)for(int j=0;j<2;++j) {
          double p[3]{};p[axis]=side;p[u]=i;p[v]=j;
          if(axis!=0)p[0]=(span+p[0])*.5;
          poles.SetValue(i+1,j+1,gp_Pnt(p[0],p[1],p[2]));
        }
        NCollection_Array1<double> k(1,2);k.SetValue(1,0);k.SetValue(2,1);
        NCollection_Array1<int> m(1,2);m.Init(2);
        occ::handle<Geom_BSplineSurface> q=new Geom_BSplineSurface(poles,k,k,m,m,1,1);
        auto f=BRepBuilderAPI_MakeFace(q,1e-9).Face();if(side==0)f.Reverse();s.Add(f);
      }
    }
    s.Perform();TopExp_Explorer sh(s.SewedShape(),TopAbs_SHELL);
    if(!sh.More())throw std::runtime_error("missing two-contact stock shell");
    stock=BRepBuilderAPI_MakeSolid(TopoDS::Shell(sh.Current())).Solid();sh.Next();
    if(sh.More() || !BRepCheck_Analyzer(stock).IsValid())throw std::runtime_error("invalid two-contact stock shell");
  }
  auto common=BRepAlgoAPI_Common(stock,tool);
  if(!common.IsDone()||TopExp_Explorer(common.Shape(),TopAbs_SOLID).More())
    throw std::runtime_error("two isolated contacts produced common material");
  record(out,"isolated-tangent-stock-cut",BRepAlgoAPI_Cut(stock,tool).Shape(),1e-11);
}
