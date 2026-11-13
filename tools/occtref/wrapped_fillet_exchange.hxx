#ifdef PLAINSOLID_OCCT_STEP
static int writeWrappedRimFilletSTEP(const char* path){
 const double xy[9][2]={{1,0},{1,1},{0,1},{-1,1},{-1,0},{-1,-1},{0,-1},{1,-1},{1,0}},wu[9]={1,1,2,2,4,2,2,1,1};
 BRepBuilderAPI_Sewing sewing(1e-8);
 for(int band=0;band<3;++band){
  const int nv=band==1?2:3;NCollection_Array2<gp_Pnt> p(1,9,1,nv);NCollection_Array2<double>w(1,9,1,nv);
  double radius[3],z[3],wv[3],v0=0,v1=1;
  if(band==0){radius[0]=2.5;radius[1]=3;radius[2]=3;z[0]=0;z[1]=0;z[2]=.5;wv[0]=2;wv[1]=1;wv[2]=1;v0=3;v1=4;}
  else if(band==1){radius[0]=radius[1]=3;z[0]=.5;z[1]=4;wv[0]=wv[1]=1;}
  else{radius[0]=3;radius[1]=3;radius[2]=2;z[0]=4;z[1]=5;z[2]=5;wv[0]=1;wv[1]=1;wv[2]=2;}
  for(int u=0;u<9;++u)for(int v=0;v<nv;++v){p.SetValue(u+1,v+1,gp_Pnt(radius[v]*xy[u][0],radius[v]*xy[u][1],z[v]));w.SetValue(u+1,v+1,wu[u]*wv[v]);}
  NCollection_Array1<double> uk(1,5),vk(1,2);NCollection_Array1<int> um(1,5),vm(1,2);for(int i=1;i<=5;++i){uk.SetValue(i,i-1);um.SetValue(i,i==1||i==5?3:2);}vk.SetValue(1,v0);vk.SetValue(2,v1);vm.SetValue(1,nv);vm.SetValue(2,nv);
  occ::handle<Geom_BSplineSurface> support=new Geom_BSplineSurface(p,w,uk,vk,um,vm,2,nv-1);const auto side=BRepBuilderAPI_MakeFace(support,1e-8).Face();sewing.Add(side);
  for(TopExp_Explorer ex(side,TopAbs_EDGE);ex.More();ex.Next()){auto edge=TopoDS::Edge(ex.Current());double lo,hi;auto curve=BRep_Tool::Curve(edge,lo,hi);if(curve.IsNull())continue;auto a=curve->Value(lo),b=curve->Value(hi),mid=curve->Value((lo+hi)/2);if(a.Z()!=b.Z()||a.Z()!=mid.Z()||(a.Z()!=0&&a.Z()!=5))continue;edge.Orientation(TopAbs_FORWARD);auto cap=BRepBuilderAPI_MakeFace(gp_Pln(gp_Pnt(0,0,a.Z()),gp_Dir(0,0,1)),BRepBuilderAPI_MakeWire(edge).Wire(),true).Face();if(a.Z()==0)cap.Reverse();sewing.Add(cap);}
 }
 sewing.Perform();auto shape=BRepBuilderAPI_MakeSolid(TopoDS::Shell(sewing.SewedShape())).Solid();if(!BRepCheck_Analyzer(shape).IsValid())throw std::runtime_error("invalid independent wrapped fillet");STEPControl_Writer writer;if(writer.Transfer(shape,STEPControl_AsIs)!=IFSelect_RetDone||writer.Write(path)!=IFSelect_RetDone)throw std::runtime_error("wrapped fillet STEP export failed");return 0;
}
#endif
