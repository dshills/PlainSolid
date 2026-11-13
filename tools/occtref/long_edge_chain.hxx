// Independent analytic profile construction followed by native OCCT features.
static int writeLongEdgeChainReference(const char* path,bool features) {
 const double pi=std::acos(-1.0);BRepBuilderAPI_MakeWire wire;
 auto p=[](double y,double z){return gp_Pnt(0,y,z);};
 auto line=[&](double y0,double z0,double y1,double z1){wire.Add(BRepBuilderAPI_MakeEdge(p(y0,z0),p(y1,z1)).Edge());};
 line(0,0,10,0);line(10,0,10,9);line(10,9,9,10);line(9,10,1,10);
 wire.Add(BRepBuilderAPI_MakeEdge(gp_Circ(gp_Ax2(p(1,9),gp_Dir(1,0,0),gp_Dir(0,1,0)),1),pi/2,pi).Edge());line(0,9,0,0);
 TopoDS_Shape shape=BRepPrimAPI_MakePrism(BRepBuilderAPI_MakeFace(wire.Wire()).Face(),gp_Vec(20,0,0)).Shape();
 if(features){const auto bore=BRepPrimAPI_MakeCylinder(gp_Ax2(gp_Pnt(5,5,0),gp_Dir(0,0,1)),1,10).Shape();const auto pocket=BRepPrimAPI_MakeBox(gp_Pnt(10,5,0),3,3,10).Shape();const auto boss=BRepPrimAPI_MakeBox(gp_Pnt(15,5,10),2,2,3).Shape();shape=BRepAlgoAPI_Cut(BRepAlgoAPI_Cut(shape,bore).Shape(),pocket).Shape();shape=BRepAlgoAPI_Fuse(shape,boss).Shape();}
 if(!BRepCheck_Analyzer(shape).IsValid())throw std::runtime_error("invalid independent long edge chain");
 STEPControl_Writer writer;if(writer.Transfer(shape,STEPControl_AsIs)!=IFSelect_RetDone||writer.Write(path)!=IFSelect_RetDone)throw std::runtime_error("long edge chain STEP export");return 0;
}
