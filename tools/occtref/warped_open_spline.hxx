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
static void warpedCornerSplineBooleans(std::ostream& out){
 auto stock=warpedOpenPrism(0,0,0);
 BRepBuilderAPI_MakePolygon rim;
 for(auto xy:std::array<std::array<double,2>,4>{{{-1,-1},{2,-1},{2,2},{-1,2}}})
   rim.Add(gp_Pnt(xy[0],xy[1],(xy[0]+xy[1])/4));
 rim.Close();
 auto top=BRepBuilderAPI_MakeFace(rim.Wire()).Face();
 auto tool=BRepPrimAPI_MakePrism(top,gp_Vec(0,0,-2)).Shape();
 if(!BRepCheck_Analyzer(tool).IsValid())throw std::runtime_error("invalid independent corner cutter");
 splineShellRecord(out,"warped-corner-common",BRepAlgoAPI_Common(stock,tool).Shape());
 splineShellRecord(out,"warped-corner-cut",BRepAlgoAPI_Cut(stock,tool).Shape());
}
