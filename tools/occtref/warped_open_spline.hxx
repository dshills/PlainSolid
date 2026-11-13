// Independent construction: vertically sweep a bilinear sheet with OCCT's
// native prism operator. No six-face Go topology or exchange file is consumed.
static TopoDS_Shape warpedOpenPrism(double x0,double y0,double z0){
 NCollection_Array2<gp_Pnt> poles(1,2,1,2);
 for(int i=1;i<=2;++i)for(int j=1;j<=2;++j){double x=x0+i-1,y=y0+j-1;poles.SetValue(i,j,gp_Pnt(x,y,z0+x*y/8));}
 NCollection_Array1<double> knots(1,2);knots.SetValue(1,0);knots.SetValue(2,1);
 NCollection_Array1<int> mult(1,2);mult.Init(2);
 occ::handle<Geom_BSplineSurface> support=new Geom_BSplineSurface(poles,knots,knots,mult,mult,1,1);
 auto face=BRepBuilderAPI_MakeFace(support,1e-10).Face();
 auto solid=BRepPrimAPI_MakePrism(face,gp_Vec(0,0,1)).Shape();
 if(!BRepCheck_Analyzer(solid).IsValid())throw std::runtime_error("invalid independently swept warped prism");
 return solid;
}
static void warpedOpenSplineBooleans(std::ostream& out){
 auto a=warpedOpenPrism(0,0,0),b=warpedOpenPrism(.5,.5,-.5);
 splineShellRecord(out,"warped-open-union",BRepAlgoAPI_Fuse(a,b).Shape());
 splineShellRecord(out,"warped-open-common",BRepAlgoAPI_Common(a,b).Shape());
 splineShellRecord(out,"warped-open-cut",BRepAlgoAPI_Cut(a,b).Shape());
}
static void warpedCornerSplineBooleans(std::ostream& out, const std::string& kind, double slope, double offset){
 auto stock=warpedOpenPrism(0,0,0);
 BRepBuilderAPI_MakePolygon rim;
 for(auto xy:std::array<std::array<double,2>,4>{{{-1,-1},{2,-1},{2,2},{-1,2}}})
   rim.Add(gp_Pnt(xy[0],xy[1],slope*(xy[0]+xy[1])+offset));
 rim.Close();
 auto top=BRepBuilderAPI_MakeFace(rim.Wire()).Face();
 auto tool=BRepPrimAPI_MakePrism(top,gp_Vec(0,0,-4)).Shape();
 if(!BRepCheck_Analyzer(tool).IsValid())throw std::runtime_error("invalid independent corner cutter");
 splineShellRecord(out,("warped-"+kind+"-common").c_str(),BRepAlgoAPI_Common(stock,tool).Shape());
 splineShellRecord(out,("warped-"+kind+"-cut").c_str(),BRepAlgoAPI_Cut(stock,tool).Shape());
}
static void warpedSplineSlab(std::ostream& out){
 auto stock=warpedOpenPrism(0,0,0);
 BRepBuilderAPI_MakePolygon rim;
 for(auto xy:std::array<std::array<double,2>,4>{{{-1,-1},{2,-1},{2,2},{-1,2}}})
   rim.Add(gp_Pnt(xy[0],xy[1],(xy[0]+xy[1])/4+1./8));
 rim.Close();
 auto tool=BRepPrimAPI_MakePrism(BRepBuilderAPI_MakeFace(rim.Wire()).Face(),gp_Vec(0,0,1./4)).Shape();
 if(!BRepCheck_Analyzer(tool).IsValid())throw std::runtime_error("invalid independent slab cutter");
 splineShellRecord(out,"warped-parallel-slab",BRepAlgoAPI_Common(stock,tool).Shape());
}
static void warpedSplineTwoPlanes(std::ostream& out,bool crossing){
 auto stock=warpedOpenPrism(0,0,0);
 auto halfspace=[&](double slope,double offset,double height){
  BRepBuilderAPI_MakePolygon rim;
  for(auto xy:std::array<std::array<double,2>,4>{{{-1,-1},{2,-1},{2,2},{-1,2}}})
    rim.Add(gp_Pnt(xy[0],xy[1],slope*(xy[0]+xy[1])+offset));
  rim.Close();
  auto tool=BRepPrimAPI_MakePrism(BRepBuilderAPI_MakeFace(rim.Wire()).Face(),gp_Vec(0,0,height)).Shape();
  if(!BRepCheck_Analyzer(tool).IsValid())throw std::runtime_error("invalid independent two-plane cutter");
  return tool;
 };
 auto lower=crossing?halfspace(1./4,3./8,2):halfspace(1./8,1./8,2);
 auto upper=crossing?halfspace(0,3./4,-2):halfspace(1./4,1./2,-2);
 auto first=BRepAlgoAPI_Common(stock,lower).Shape();
 splineShellRecord(out,crossing?"warped-crossing-planes":"warped-separated-planes",BRepAlgoAPI_Common(first,upper).Shape());
}
// OCCT's ordinary volume integrator does not split a C0 span for first
// moments. Use the independent span-aware Gauss–Kronrod integrator.
static void creasedGraphRecord(std::ostream& out,const char* name,const TopoDS_Shape& s){
 if(!BRepCheck_Analyzer(s).IsValid())throw std::runtime_error("invalid independent creased Boolean");
 GProp_GProps mass,area;
 if(BRepGProp::VolumePropertiesGK(s,mass,1e-11,true,true,true)<0)throw std::runtime_error("OCCT span-aware volume failed");
 BRepGProp::SurfaceProperties(s,area,1e-10);
 const auto p=mass.CentreOfMass();
 int shells=0;for(TopExp_Explorer it(s,TopAbs_SHELL);it.More();it.Next())++shells;
 out<<std::setprecision(17)<<"{\"Name\":\""<<name<<"\",\"Volume\":"<<mass.Mass()<<",\"Area\":"<<area.Mass()<<",\"Centroid\":["<<p.X()<<','<<p.Y()<<','<<p.Z()<<"],\"Shells\":"<<shells<<"}\n";
}
static void quadraticGraphPlaneBooleans(std::ostream& out,bool crease=false){
 NCollection_Array2<gp_Pnt> poles(1,crease?5:3,1,2);
 const double z[]={1./64,1./128,1./32,9./128,9./64};
 for(int i=1;i<=(crease?5:3);++i)for(int j=1;j<=2;++j)poles.SetValue(i,j,gp_Pnt((i-1)/(crease?4.:2.),j-1,(crease?z[i-1]:(i==3?1./8:0))+(j-1)/4.));
 NCollection_Array1<double> knots(1,2);knots.SetValue(1,0);knots.SetValue(2,1);
 NCollection_Array1<int> xmult(1,2),ymult(1,2);xmult.Init(3);ymult.Init(2);
 NCollection_Array1<double> creaseKnots(1,3);creaseKnots.SetValue(1,0);creaseKnots.SetValue(2,.5);creaseKnots.SetValue(3,1);
 NCollection_Array1<int> creaseMult(1,3);creaseMult.Init(3);creaseMult.SetValue(2,2);
 occ::handle<Geom_BSplineSurface> support=new Geom_BSplineSurface(poles,crease?creaseKnots:knots,knots,crease?creaseMult:xmult,ymult,2,1);
 auto stock=BRepPrimAPI_MakePrism(BRepBuilderAPI_MakeFace(support,1e-10).Face(),gp_Vec(0,0,1)).Shape();
 auto tool=BRepPrimAPI_MakeBox(gp_Pnt(-1,-1,-2),gp_Pnt(2,2,3./16)).Shape();
 if(!BRepCheck_Analyzer(stock).IsValid()||!BRepCheck_Analyzer(tool).IsValid())throw std::runtime_error("invalid independent quadratic stock or cutter");
 auto common=BRepAlgoAPI_Common(stock,tool).Shape(), cut=BRepAlgoAPI_Cut(stock,tool).Shape();
 if(!crease){splineShellRecord(out,"quadratic-plane-common",common);splineShellRecord(out,"quadratic-plane-cut",cut);return;}
 creasedGraphRecord(out,"piecewise-quadratic-plane-common",common);
 creasedGraphRecord(out,"piecewise-quadratic-plane-cut",cut);
}
static TopoDS_Shape obliquePolynomialCutter(){
 BRepBuilderAPI_MakePolygon rim;
 for(auto xy:std::array<std::array<double,2>,4>{{{-1,-1},{2,-1},{2,2},{-1,2}}})
  rim.Add(gp_Pnt(xy[0],xy[1],3./16+xy[0]/32));
 rim.Close();
 return BRepPrimAPI_MakePrism(BRepBuilderAPI_MakeFace(rim.Wire()).Face(),gp_Vec(0,0,-3)).Shape();
}
static void polynomialGraphPlaneBooleans(std::ostream& out,bool oblique=false){
 NCollection_Array2<gp_Pnt> poles(1,5,1,2);
 const double z[]={0,0,0,1./32,1./8};
 for(int i=1;i<=5;++i)for(int j=1;j<=2;++j)poles.SetValue(i,j,gp_Pnt((i-1)/4.,j-1,z[i-1]+(j-1)/4.));
 NCollection_Array1<double> knots(1,2);knots.SetValue(1,0);knots.SetValue(2,1);
 NCollection_Array1<int> xmult(1,2),ymult(1,2);xmult.Init(5);ymult.Init(2);
 occ::handle<Geom_BSplineSurface> support=new Geom_BSplineSurface(poles,knots,knots,xmult,ymult,4,1);
 auto stock=BRepPrimAPI_MakePrism(BRepBuilderAPI_MakeFace(support,1e-10).Face(),gp_Vec(0,0,1)).Shape();
 auto tool=oblique?obliquePolynomialCutter():BRepPrimAPI_MakeBox(gp_Pnt(-1,-1,-2),gp_Pnt(3,3,3./16)).Shape();
 if(!BRepCheck_Analyzer(stock).IsValid()||!BRepCheck_Analyzer(tool).IsValid())throw std::runtime_error("invalid independent polynomial stock or cutter");
 splineShellRecord(out,oblique?"oblique-polynomial-plane-common":"polynomial-plane-common",BRepAlgoAPI_Common(stock,tool).Shape());
 splineShellRecord(out,oblique?"oblique-polynomial-plane-cut":"polynomial-plane-cut",BRepAlgoAPI_Cut(stock,tool).Shape());
}
static void piecewisePolynomialGraphPlaneBooleans(std::ostream& out,bool oblique=false){
 // Native OCCT construction of x³/8+y/4+|x-.5|/32, swept vertically.
 NCollection_Array2<gp_Pnt> poles(1,9,1,2);
 const double z[]={1./64,3./256,1./128,1./128,1./64,1./32,7./128,23./256,9./64};
 for(int i=1;i<=9;++i)for(int j=1;j<=2;++j)poles.SetValue(i,j,gp_Pnt((i-1)/8.,j-1,z[i-1]+(j-1)/4.));
 NCollection_Array1<double> xk(1,3),yk(1,2);xk.SetValue(1,0);xk.SetValue(2,.5);xk.SetValue(3,1);yk.SetValue(1,0);yk.SetValue(2,1);
 NCollection_Array1<int> xm(1,3),ym(1,2);xm.Init(5);xm.SetValue(2,4);ym.Init(2);
 occ::handle<Geom_BSplineSurface> support=new Geom_BSplineSurface(poles,xk,yk,xm,ym,4,1);
 auto stock=BRepPrimAPI_MakePrism(BRepBuilderAPI_MakeFace(support,1e-10).Face(),gp_Vec(0,0,1)).Shape();
 auto tool=oblique?obliquePolynomialCutter():BRepPrimAPI_MakeBox(gp_Pnt(-1,-1,-2),gp_Pnt(2,2,3./16)).Shape();
 if(!BRepCheck_Analyzer(stock).IsValid()||!BRepCheck_Analyzer(tool).IsValid())throw std::runtime_error("invalid independent piecewise polynomial stock or cutter");
 creasedGraphRecord(out,oblique?"oblique-piecewise-polynomial-plane-common":"piecewise-polynomial-plane-common",BRepAlgoAPI_Common(stock,tool).Shape());
 creasedGraphRecord(out,oblique?"oblique-piecewise-polynomial-plane-cut":"piecewise-polynomial-plane-cut",BRepAlgoAPI_Cut(stock,tool).Shape());
}
static void rationalGraphPlaneBooleans(std::ostream& out,bool crease=false){
 // Polynomial support Z=X/16+X²/32+(1+X)Y/4. The plane section is
 // rational because solving for Y introduces the nonconstant 1+X.
 NCollection_Array2<gp_Pnt> poles(1,crease?5:3,1,2);
 const double z[]={0,1./32,3./32};
 const double cz[]={1./128,5./256,5./128,19./256,13./128};
 for(int i=1;i<=(crease?5:3);++i)for(int j=1;j<=2;++j){
  const double x=(i-1)/(crease?4.:2.),y=j-1;
  poles.SetValue(i,j,gp_Pnt(x,y,(crease?cz[i-1]:z[i-1])+(1+x)*y/4.));
 }
 NCollection_Array1<double> knots(1,2);knots.SetValue(1,0);knots.SetValue(2,1);
 NCollection_Array1<int> xm(1,2),ym(1,2);xm.Init(3);ym.Init(2);
 NCollection_Array1<double> ck(1,3);ck.SetValue(1,0);ck.SetValue(2,.5);ck.SetValue(3,1);
 NCollection_Array1<int> cm(1,3);cm.Init(3);cm.SetValue(2,2);
 occ::handle<Geom_BSplineSurface> support=new Geom_BSplineSurface(poles,crease?ck:knots,knots,crease?cm:xm,ym,2,1);
 auto stock=BRepPrimAPI_MakePrism(BRepBuilderAPI_MakeFace(support,1e-10).Face(),gp_Vec(0,0,1)).Shape();
 auto tool=BRepPrimAPI_MakeBox(gp_Pnt(-1,-1,-2),gp_Pnt(2,2,crease?23./128:3./16)).Shape();
 if(!BRepCheck_Analyzer(stock).IsValid()||!BRepCheck_Analyzer(tool).IsValid())throw std::runtime_error("invalid independent rational graph stock or cutter");
 auto common=BRepAlgoAPI_Common(stock,tool).Shape(), cut=BRepAlgoAPI_Cut(stock,tool).Shape();
 if(crease){
  // The unsplit reference still verifies valid topology and external area.
  // OCCT's integration on this specific rational section crossing a crease
  // depends on its integration mode. Integrate separate OCCT half-solids and
  // sum their first moments; artificial station faces are not counted in area.
  auto piecewiseRecord=[&](const char* name,const TopoDS_Shape& result){
   if(!BRepCheck_Analyzer(result).IsValid())throw std::runtime_error("invalid creased rational Boolean");
   const auto a=BRepPrimAPI_MakeBox(gp_Pnt(-1,-1,-2),gp_Pnt(.5,2,3)).Shape();
   const auto b=BRepPrimAPI_MakeBox(gp_Pnt(.5,-1,-2),gp_Pnt(2,2,3)).Shape();
   GProp_GProps mass,area;
   for(const auto& half:{a,b}){
    const auto part=BRepAlgoAPI_Common(result,half).Shape();
    if(!BRepCheck_Analyzer(part).IsValid())throw std::runtime_error("invalid rational Boolean integration slab");
    GProp_GProps props;BRepGProp::VolumeProperties(part,props,1e-11,true);
    mass.Add(props);
   }
   BRepGProp::SurfaceProperties(result,area,1e-10);
   const auto p=mass.CentreOfMass();
   int shells=0;for(TopExp_Explorer it(result,TopAbs_SHELL);it.More();it.Next())++shells;
   out<<std::setprecision(17)<<"{\"Name\":\""<<name<<"\",\"Volume\":"<<mass.Mass()<<",\"Area\":"<<area.Mass()<<",\"Centroid\":["<<p.X()<<','<<p.Y()<<','<<p.Z()<<"],\"Shells\":"<<shells<<"}\n";
  };
  piecewiseRecord("piecewise-rational-graph-plane-common",common);
  piecewiseRecord("piecewise-rational-graph-plane-cut",cut);
 }else{
  splineShellRecord(out,"rational-graph-plane-common",common);
  splineShellRecord(out,"rational-graph-plane-cut",cut);
 }
}
static void rationalBezierGraphPlaneBooleans(std::ostream& out){
 NCollection_Array1<double> knots(1,2);knots.SetValue(1,0);knots.SetValue(2,1);
 NCollection_Array1<int> xm(1,2),ym(1,2);xm.Init(5);ym.Init(2);
 auto tool=BRepPrimAPI_MakeBox(gp_Pnt(-1,-1,-2),gp_Pnt(2,2,3./16)).Shape();
 for(int degree:{3,4}){
  NCollection_Array2<gp_Pnt> poles(1,5,1,2);
  for(int i=1;i<=5;++i)for(int j=1;j<=2;++j){
   const double x=(i-1)/4.,y=j-1,z=i==5?1./16:(degree==3&&i==4?1./64:0.);
   poles.SetValue(i,j,gp_Pnt(x,y,z+(1+x)*y/4.));
  }
  occ::handle<Geom_BSplineSurface> sheet=new Geom_BSplineSurface(poles,knots,knots,xm,ym,4,1);
  const auto stock=BRepPrimAPI_MakePrism(BRepBuilderAPI_MakeFace(sheet,1e-10).Face(),gp_Vec(0,0,1)).Shape();
  if(!BRepCheck_Analyzer(stock).IsValid())throw std::runtime_error("invalid independent high degree graph stock");
  const auto prefix="rational-bezier-"+std::to_string(degree);
  splineShellRecord(out,(prefix+"-common").c_str(),BRepAlgoAPI_Common(stock,tool).Shape());
  splineShellRecord(out,(prefix+"-cut").c_str(),BRepAlgoAPI_Cut(stock,tool).Shape());
 }
 // Author the separate quadratic-denominator fixture in degree two. Its
 // Y derivative is (1+3X²)/4; the test kernel uses a degree-elevated chart.
 NCollection_Array2<gp_Pnt> poles(1,3,1,2);
 NCollection_Array1<int> qm(1,2);qm.Init(3);
 for(int i=1;i<=3;++i)for(int j=1;j<=2;++j){
  const double x=(i-1)/2.,y=j-1;
  poles.SetValue(i,j,gp_Pnt(x,y,(i==3?1.:1./4)*y));
 }
 occ::handle<Geom_BSplineSurface> sheet=new Geom_BSplineSurface(poles,knots,knots,qm,ym,2,1);
 const auto stock=BRepPrimAPI_MakePrism(BRepBuilderAPI_MakeFace(sheet,1e-10).Face(),gp_Vec(0,0,1)).Shape();
 if(!BRepCheck_Analyzer(stock).IsValid())throw std::runtime_error("invalid independent quadratic denominator stock");
 splineShellRecord(out,"rational-bezier-denominator-common",BRepAlgoAPI_Common(stock,tool).Shape());
 splineShellRecord(out,"rational-bezier-denominator-cut",BRepAlgoAPI_Cut(stock,tool).Shape());
 NCollection_Array2<double> weights(1,3,1,2);
 const double x[]={0,2,3},w[]={1,.75,.5};
 for(int i=1;i<=3;++i)for(int j=1;j<=2;++j){
  const double y=j-1;
  poles.SetValue(i,j,gp_Pnt(x[i-1],y,y/4.+(i==3?1./16:0.)));
  weights.SetValue(i,j,w[i-1]);
 }
 occ::handle<Geom_BSplineSurface> weighted=new Geom_BSplineSurface(poles,weights,knots,knots,qm,ym,2,1);
 const auto stockWeighted=BRepPrimAPI_MakePrism(BRepBuilderAPI_MakeFace(weighted,1e-10).Face(),gp_Vec(0,0,1)).Shape();
 const auto toolWeighted=BRepPrimAPI_MakeBox(gp_Pnt(-1,-1,-2),gp_Pnt(7,3,3./16)).Shape();
 if(!BRepCheck_Analyzer(stockWeighted).IsValid())throw std::runtime_error("invalid independently swept rational stock");
 splineShellRecord(out,"rational-bezier-weighted-common",BRepAlgoAPI_Common(stockWeighted,toolWeighted).Shape());
 splineShellRecord(out,"rational-bezier-weighted-cut",BRepAlgoAPI_Cut(stockWeighted,toolWeighted).Shape());
}
