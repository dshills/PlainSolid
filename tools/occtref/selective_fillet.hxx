#include <BRepFilletAPI_MakeFillet.hxx>
#include <BRep_Tool.hxx>
// Independently apply OCCT's actual 3D edge fillet builder to a box.
static void selectiveFilletRecord(std::ostream& out){
 auto box=BRepPrimAPI_MakeBox(4,3,2).Shape();BRepFilletAPI_MakeFillet fillet(box);int selected=0;
 NCollection_IndexedMap<TopoDS_Shape,TopTools_ShapeMapHasher> edges;TopExp::MapShapes(box,TopAbs_EDGE,edges);
 for(int i=1;i<=edges.Extent();++i){
  auto edge=TopoDS::Edge(edges(i));TopoDS_Vertex va,vb;TopExp::Vertices(edge,va,vb);if(va.IsNull()||vb.IsNull())continue;
  const auto a=BRep_Tool::Pnt(va),b=BRep_Tool::Pnt(vb);
  if(std::abs(a.X()-b.X())>1e-12||std::abs(a.Y()-b.Y())>1e-12||std::abs(std::abs(a.Z()-b.Z())-2)>1e-12)continue;
  if(std::abs(a.X())<1e-12&&std::abs(a.Y())<1e-12){fillet.Add(.25,edge);++selected;}
  if(std::abs(a.X()-4)<1e-12&&std::abs(a.Y()-3)<1e-12){fillet.Add(.5,edge);++selected;}
 }
 if(selected!=2)throw std::runtime_error("selective fillet edge selection failed");fillet.Build();if(!fillet.IsDone())throw std::runtime_error("OCCT unequal fillet failed");
 twoContourRecord(out,"selective-fillet",fillet.Shape());
}

static void circularRimFilletRecord(std::ostream& out){
 auto stock=BRepPrimAPI_MakeCylinder(3,5).Shape();
 BRepFilletAPI_MakeFillet fillet(stock);
 int selected=0;
 NCollection_IndexedMap<TopoDS_Shape,TopTools_ShapeMapHasher> edges;TopExp::MapShapes(stock,TopAbs_EDGE,edges);
 for(int i=1;i<=edges.Extent();++i){
  auto edge=TopoDS::Edge(edges(i));TopoDS_Vertex a,b;TopExp::Vertices(edge,a,b);
  if(a.IsNull()||b.IsNull()||!a.IsSame(b))continue;
  const double z=BRep_Tool::Pnt(a).Z();
  if(std::abs(z)<1e-12){fillet.Add(.5,edge);++selected;}
  else if(std::abs(z-5)<1e-12){fillet.Add(1.,edge);++selected;}
 }
 if(selected!=2)throw std::runtime_error("circular rim fillet edge selection failed");
 fillet.Build();
 if(!fillet.IsDone())throw std::runtime_error("OCCT circular rim fillet failed");
 splineShellRecord(out,"circular-rim-fillet",fillet.Shape());
}
