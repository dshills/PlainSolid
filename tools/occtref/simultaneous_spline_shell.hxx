// Independently authored OCCT surfaces for a determinant-one warped
// simultaneous edge crossing. No native Go geometry is read.
static TopoDS_Shape simultaneousWarpedShell(bool skew){
 BRepBuilderAPI_Sewing sewing(1e-9);
 auto point=[&](const std::array<double,3>& p){
   if(!skew)return gp_Pnt(p[0],p[1],p[2]);
   return gp_Pnt(.875+.25*(p[1]+p[2]),.875+.25*(p[0]+p[2]),.5+.25*(p[0]+p[1]));
 };
 for(int fixed=0;fixed<3;++fixed)for(int side=0;side<2;++side){
   const int u=(fixed+1)%3,v=(fixed+2)%3;
   std::array<double,3> q{};q[fixed]=side;
   const auto p=point(q);q[u]=1;const auto U=gp_Vec(p,point(q));q[u]=0;q[v]=1;const auto V=gp_Vec(p,point(q));
   const double d=p.X()-p.Y(),a=U.X()-U.Y(),b=V.X()-V.Y();
   NCollection_Array2<gp_Pnt> poles(1,3,1,3);
   for(int i=0;i<3;++i)for(int j=0;j<3;++j){
     const double s=i*.5,t=j*.5;
     const double wz=(d*d+2*d*(a*s+b*t)+2*a*b*s*t+(i==2?a*a:0)+(j==2?b*b:0))/64.;
     poles.SetValue(i+1,j+1,gp_Pnt(p.X()+U.X()*s+V.X()*t,p.Y()+U.Y()*s+V.Y()*t,p.Z()+U.Z()*s+V.Z()*t+wz));
   }
   NCollection_Array1<double> k(1,2);k.SetValue(1,0);k.SetValue(2,1);NCollection_Array1<int> m(1,2);m.Init(3);
   occ::handle<Geom_BSplineSurface> pSurf=new Geom_BSplineSurface(poles,k,k,m,m,2,2);
   auto f=BRepBuilderAPI_MakeFace(pSurf,1e-9).Face();if(side==0)f.Reverse();sewing.Add(f);
 }
 sewing.Perform();auto sewn=sewing.SewedShape();
 TopExp_Explorer e(sewn,TopAbs_SHELL);if(!e.More())throw std::runtime_error("missing simultaneous reference shell");
 auto solid=BRepBuilderAPI_MakeSolid(TopoDS::Shell(e.Current())).Solid();
 e.Next();if(e.More()||!BRepCheck_Analyzer(solid).IsValid())throw std::runtime_error("invalid simultaneous reference shell");
 return solid;
}

static void simultaneousSplineBooleans(std::ostream& out){
 auto a=simultaneousWarpedShell(false),b=simultaneousWarpedShell(true);
 splineShellRecord(out,"simultaneous-union",BRepAlgoAPI_Fuse(a,b).Shape());
 splineShellRecord(out,"simultaneous-common",BRepAlgoAPI_Common(a,b).Shape());
 splineShellRecord(out,"simultaneous-cut",BRepAlgoAPI_Cut(a,b).Shape());
}
