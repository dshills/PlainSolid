// Independent exact quadratic roof prism; no native mesh or topology consumed.
static TopoDS_Shape repeatedSplineRoofPrism(){
 BRepBuilderAPI_Sewing sewing(1e-9);
 auto patch=[&](int degreeU,const std::vector<gp_Pnt>& p,bool reversed){
 NCollection_Array2<gp_Pnt> poles(1,degreeU+1,1,2);for(int i=1;i<=degreeU+1;++i)for(int j=1;j<=2;++j)poles.SetValue(i,j,p[(i-1)*2+j-1]);
 NCollection_Array1<double> knots(1,2);knots.SetValue(1,0);knots.SetValue(2,1);NCollection_Array1<int> mu(1,2),mv(1,2);mu.Init(degreeU+1);mv.Init(2);
 occ::handle<Geom_BSplineSurface> surf=new Geom_BSplineSurface(poles,knots,knots,mu,mv,degreeU,1);
 TopoDS_Face face=BRepBuilderAPI_MakeFace(surf,1e-9).Face();if(reversed)face.Reverse();sewing.Add(face);
 };
 const double xs[]={-.5,.5,1.5},zs[]={15./16,-17./16,15./16};
 std::vector<gp_Pnt> floor,roof;
 for(int i=0;i<3;++i)for(double y:{-.5,1.5}){floor.emplace_back(xs[i],y,-45./16);roof.emplace_back(xs[i],y,zs[i]);}
 patch(2,floor,true);patch(2,roof,false);
 for(double x:{-.5,1.5}){std::vector<gp_Pnt> p;for(double y:{-.5,1.5})for(double z:{-45./16,15./16})p.emplace_back(x,y,z);patch(1,p,x<0);}
 for(double y:{-.5,1.5}){std::vector<gp_Pnt> p;for(int i=0;i<3;++i){p.emplace_back(xs[i],y,-45./16);p.emplace_back(xs[i],y,zs[i]);}patch(2,p,y>0);}
 sewing.Perform();TopExp_Explorer shells(sewing.SewedShape(),TopAbs_SHELL);if(!shells.More())throw std::runtime_error("missing repeated spline shell");
 auto solid=BRepBuilderAPI_MakeSolid(TopoDS::Shell(shells.Current())).Solid();shells.Next();if(shells.More()||!BRepCheck_Analyzer(solid).IsValid())throw std::runtime_error("invalid repeated spline shell");return solid;
}
static void repeatedSplineShellBooleans(std::ostream& out){auto a=BRepPrimAPI_MakeBox(1,1,1).Shape(),b=repeatedSplineRoofPrism();splineShellRecord(out,"repeated-b",b);splineShellRecord(out,"repeated-union",BRepAlgoAPI_Fuse(a,b).Shape());splineShellRecord(out,"repeated-common",BRepAlgoAPI_Common(a,b).Shape());splineShellRecord(out,"repeated-cut",BRepAlgoAPI_Cut(a,b).Shape());}
