// Independent OCCT prism-tool feature constructions.
static void featureRecords(std::ostream& out){
 for(int concave=0;concave<2;++concave)for(int pocket=0;pocket<2;++pocket){
  std::vector<gp_Pnt> points=concave?std::vector<gp_Pnt>{{1,1,4},{3,1,4},{3,2,4},{2,2,4},{2,3,4},{1,3,4}}:std::vector<gp_Pnt>{{1,1,4},{3,1,4},{3,3,4},{1,3,4}};
  BRepBuilderAPI_MakeWire wire;
  for(size_t i=0;i<points.size();++i)wire.Add(BRepBuilderAPI_MakeEdge(points[i],points[(i+1)%points.size()]));
  auto face=BRepBuilderAPI_MakeFace(wire.Wire()).Face();
  auto tool=BRepPrimAPI_MakePrism(face,gp_Vec(concave?.25:0,concave?.125:0,pocket?-1:1)).Shape();
  auto base=BRepPrimAPI_MakeBox(4,4,4).Shape();
  TopoDS_Shape result=pocket?BRepAlgoAPI_Cut(base,tool).Shape():BRepAlgoAPI_Fuse(base,tool).Shape();
  const char* name=concave?(pocket?"oblique-pocket":"oblique-protrusion"):(pocket?"pocket":"protrusion");
  record(out,name,result);
 }
 for(int oblique=0;oblique<2;++oblique){
 std::vector<gp_Pnt> p={{1,1,4},{3,1,4},{3,3,4},{1,3,4}};BRepBuilderAPI_MakeWire w;
 for(size_t i=0;i<p.size();++i)w.Add(BRepBuilderAPI_MakeEdge(p[i],p[(i+1)%p.size()]));
 auto tool=BRepPrimAPI_MakePrism(BRepBuilderAPI_MakeFace(w.Wire()).Face(),gp_Vec(oblique?.5:0,0,-4)).Shape();
 auto result=BRepAlgoAPI_Cut(BRepPrimAPI_MakeBox(4,4,4).Shape(),tool).Shape();
 record(out,oblique?"oblique-through":"through",result);
 }
 for(int tray=0;tray<2;++tray){auto base=BRepPrimAPI_MakeBox(4,4,4).Shape();auto cavity=BRepPrimAPI_MakeBox(gp_Pnt(.5,.5,.5),3,3,tray?3.5:3).Shape();record(out,tray?"hollow-tray":"hollow-cavity",BRepAlgoAPI_Cut(base,cavity).Shape());}
 for(int mode=0;mode<3;++mode){
 auto wire=[](double lo,double hi,bool reverse){std::vector<gp_Pnt> p={{lo,lo,4},{hi,lo,4},{hi,hi,4},{lo,hi,4}};if(reverse)std::reverse(p.begin(),p.end());BRepBuilderAPI_MakeWire w;for(size_t i=0;i<4;++i)w.Add(BRepBuilderAPI_MakeEdge(p[i],p[(i+1)%4]));return w.Wire();};
 BRepBuilderAPI_MakeFace f(wire(.5,3.5,false));f.Add(wire(1.5,2.5,true));
 auto tool=BRepPrimAPI_MakePrism(f.Face(),gp_Vec(0,0,mode==0?1:(mode==1?-1:-4))).Shape();auto base=BRepPrimAPI_MakeBox(4,4,4).Shape();TopoDS_Shape result=mode==0?BRepAlgoAPI_Fuse(base,tool).Shape():BRepAlgoAPI_Cut(base,tool).Shape();record(out,mode==0?"annular-protrusion":(mode==1?"annular-blind":"annular-through"),result);
 }
}
