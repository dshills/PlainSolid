// Independent quadratic roof construction, not sampled Go geometry.
static TopoDS_Shape transverseSplinePrism(){
 BRepBuilderAPI_Sewing sew(1e-9);const double q[3]={9./64,-9./64,9./64},r2=25./256;
 auto patch=[&](int nu,int nv,std::function<gp_Pnt(int,int)> pole,bool reverse){NCollection_Array2<gp_Pnt> p(1,nu,1,nv);for(int i=0;i<nu;++i)for(int j=0;j<nv;++j)p.SetValue(i+1,j+1,pole(i,j));NCollection_Array1<double> knots(1,2);knots.SetValue(1,0);knots.SetValue(2,1);NCollection_Array1<int> um(1,2),vm(1,2);um.Init(nu);vm.Init(nv);occ::handle<Geom_BSplineSurface> surface=new Geom_BSplineSurface(p,knots,knots,um,vm,nu-1,nv-1);TopoDS_Face f=BRepBuilderAPI_MakeFace(surface,1e-9).Face();if(reverse)f.Reverse();sew.Add(f);};
 patch(2,2,[](int i,int j){return gp_Pnt(.125+.75*i,.125+.75*j,-.5);},true);
 patch(3,3,[&](int i,int j){return gp_Pnt(.125+.375*i,.125+.375*j,r2-q[i]-q[j]);},false);
 for(int side=0;side<2;++side){double x=.125+.75*side;patch(3,2,[&,x](int i,int j){return gp_Pnt(x,.125+.375*i,j?r2-9./64-q[i]:-.5);},side==0);}
 for(int side=0;side<2;++side){double y=.125+.75*side;patch(3,2,[&,y](int i,int j){return gp_Pnt(.125+.375*i,y,j?r2-9./64-q[i]:-.5);},side==1);}
 sew.Perform();TopExp_Explorer shells(sew.SewedShape(),TopAbs_SHELL);if(!shells.More())throw std::runtime_error("transverse spline shell missing");auto solid=BRepBuilderAPI_MakeSolid(TopoDS::Shell(shells.Current())).Solid();shells.Next();if(shells.More()||!BRepCheck_Analyzer(solid).IsValid())throw std::runtime_error("invalid transverse spline solid");return solid;
}
static void transverseSplineBooleans(std::ostream& out){auto a=BRepPrimAPI_MakeBox(1,1,1).Shape();auto b=transverseSplinePrism();splineShellRecord(out,"transverse-a",a);splineShellRecord(out,"transverse-b",b);splineShellRecord(out,"transverse-union",BRepAlgoAPI_Fuse(a,b).Shape());splineShellRecord(out,"transverse-common",BRepAlgoAPI_Common(a,b).Shape());splineShellRecord(out,"transverse-cut",BRepAlgoAPI_Cut(a,b).Shape());}
