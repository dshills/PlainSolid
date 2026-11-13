#pragma once
#ifdef PLAINSOLID_OCCT_STEP
static TopoDS_Solid wrappedStationBoreLoft(const double radii[3],bool single=false) {
 const double xy[9][2]={{1,0},{1,1},{0,1},{-1,1},{-1,0},{-1,-1},{0,-1},{1,-1},{1,0}};
 const double wu[9]={1,1,2,2,4,2,2,1,1},z[3]={0,2,5};
 BRepBuilderAPI_Sewing sewing(1e-8);
 for(int band=0;band<(single?1:2);++band){
  const int first=single?0:band,last=single?2:band+1;
  NCollection_Array2<gp_Pnt> poles(1,9,1,2);NCollection_Array2<double> weights(1,9,1,2);
  for(int u=0;u<9;++u)for(int v=0;v<2;++v){poles.SetValue(u+1,v+1,gp_Pnt(radii[v?last:first]*xy[u][0],radii[v?last:first]*xy[u][1],z[v?last:first]));weights.SetValue(u+1,v+1,wu[u]);}
  NCollection_Array1<double> uk(1,5),vk(1,2);NCollection_Array1<int> um(1,5),vm(1,2);
  for(int i=1;i<=5;++i){uk.SetValue(i,i-1);um.SetValue(i,i==1||i==5?3:2);}vk.SetValue(1,0);vk.SetValue(2,1);vm.SetValue(1,2);vm.SetValue(2,2);
  occ::handle<Geom_BSplineSurface> support=new Geom_BSplineSurface(poles,weights,uk,vk,um,vm,2,1);
  const auto side=BRepBuilderAPI_MakeFace(support,1e-8).Face();sewing.Add(side);
  for(TopExp_Explorer ex(side,TopAbs_EDGE);ex.More();ex.Next()){
   auto edge=TopoDS::Edge(ex.Current());double lo,hi;auto curve=BRep_Tool::Curve(edge,lo,hi);if(curve.IsNull())continue;
   auto a=curve->Value(lo),b=curve->Value(hi),mid=curve->Value((lo+hi)/2);
   if(a.Z()!=b.Z()||a.Z()!=mid.Z()||(a.Z()!=0&&a.Z()!=5))continue;
   edge.Orientation(TopAbs_FORWARD);auto cap=BRepBuilderAPI_MakeFace(gp_Pln(gp_Pnt(0,0,a.Z()),gp_Dir(0,0,1)),BRepBuilderAPI_MakeWire(edge).Wire(),true).Face();if(a.Z()==0)cap.Reverse();sewing.Add(cap);
  }
 }
 sewing.Perform();auto shape=BRepBuilderAPI_MakeSolid(TopoDS::Shell(sewing.SewedShape())).Solid();if(!BRepCheck_Analyzer(shape).IsValid())throw std::runtime_error("invalid independent station bore loft");return shape;
}
static int writeWrappedStationBoreSTEP(const char* path,bool single=false,bool bothSingle=false){
 const double outer[3]={3,4,bothSingle?4.0:3.0},inner[3]={1,1.5,single?1.5:1};
 auto shape=BRepAlgoAPI_Cut(wrappedStationBoreLoft(outer,bothSingle),wrappedStationBoreLoft(inner,single)).Shape();
 if(!BRepCheck_Analyzer(shape).IsValid())throw std::runtime_error("invalid independent station bore cut");
 STEPControl_Writer writer;if(writer.Transfer(shape,STEPControl_AsIs)!=IFSelect_RetDone||writer.Write(path)!=IFSelect_RetDone)throw std::runtime_error("station bore export failed");return 0;
}
#endif
