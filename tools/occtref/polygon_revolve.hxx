// Independent polygon-meridian partial revolution, using OCCT's native sweep.
#include <BRepBuilderAPI_MakePolygon.hxx>
#include <BRepPrimAPI_MakeRevol.hxx>
static int polygonRevolveReference(const char* path,bool axis=false) {
 const double concave[6][2]={{1,0},{3,0},{3,1},{2,1},{2,2},{1,2}};
 const double axisConcave[6][2]={{0,0},{2,0},{2,1},{1,1},{1,2},{0,2}};
 const double apexEnd[4][2]={{0,0},{1,0},{2,2},{0,3}};
 const double apex[4][2]={{0,0},{2,1},{1,3},{0,3}};
 const double tapered[6][2]={{1,0},{3,0},{2.5,1},{3,2},{1.5,2},{1,1}};
 std::ofstream out(path);
 for(int kind=0;kind<(axis?3:2);++kind)for(double angle:{M_PI/2,-M_PI/3,3*M_PI/2}) {
  BRepBuilderAPI_MakePolygon wire;for(int i=0;i<(axis&&kind?4:6);++i){const double* p=axis?(kind==2?apexEnd[i]:(kind?apex[i]:axisConcave[i])):(kind?tapered[i]:concave[i]);wire.Add(gp_Pnt(2+p[1],3,4+p[0]));}wire.Close();
  auto face=BRepBuilderAPI_MakeFace(wire.Wire()).Face();
  auto solid=BRepPrimAPI_MakeRevol(face,gp_Ax1(gp_Pnt(2,3,4),gp_Dir(1,0,0)),angle,true).Shape();
  if(!BRepCheck_Analyzer(solid).IsValid())throw std::runtime_error("invalid independent polygon revolve");
  GProp_GProps volume,area;BRepGProp::VolumeProperties(solid,volume,1e-12);BRepGProp::SurfaceProperties(solid,area,1e-12);
  const auto c=volume.CentreOfMass();
  out<<std::setprecision(17)<<"{\"Area\":"<<area.Mass()<<",\"Volume\":"<<volume.Mass()<<",\"Centroid\":["<<c.X()<<','<<c.Y()<<','<<c.Z()<<"]}\n";
 }
 return 0;
}

static int circularHoleExtrusionReference(const char* path,int mode=0) {
 const double p[5][2]={{0,0},{6,0},{7,3},{4,5},{0,4}};
 std::ofstream out(path);
 for(double height:{3.,-3.}) {
  BRepBuilderAPI_MakePolygon wire;for(const auto& v:p)wire.Add(gp_Pnt(2,3-v[1],4+v[0]));wire.Close();
  auto face=BRepBuilderAPI_MakeFace(wire.Wire()).Face();TopoDS_Shape shape=BRepPrimAPI_MakePrism(face,gp_Vec(height,0,0)).Shape();
  const double holes[2][3]={{2,2,.5},{5,2,.75}};
  for(int i=0;i<2;++i){const auto& h=holes[i];const bool blind=mode==2||(mode==1&&i==1);const double depth=blind?(i==0?.75:1.25):std::abs(height);auto tool=BRepPrimAPI_MakeCylinder(gp_Ax2(gp_Pnt(2+(blind?0:std::min(0.,height)),3-h[1],4+h[0]),gp_Dir(blind?std::copysign(1.,height):1,0,0),gp_Dir(0,0,1)),h[2],depth).Shape();shape=BRepAlgoAPI_Cut(shape,tool).Shape();}
  if(!BRepCheck_Analyzer(shape).IsValid())throw std::runtime_error("invalid circular-hole prism reference");
  GProp_GProps v,a;BRepGProp::VolumeProperties(shape,v,1e-12);BRepGProp::SurfaceProperties(shape,a,1e-12);auto c=v.CentreOfMass();out<<std::setprecision(17)<<"{\"Area\":"<<a.Mass()<<",\"Volume\":"<<v.Mass()<<",\"Centroid\":["<<c.X()<<','<<c.Y()<<','<<c.Z()<<"]}\n";
 }
 return 0;
}

#ifdef PLAINSOLID_OCCT_STEP
static int writeCircularPocketReference(const char* path, bool allThrough=false) {
 const double p[5][2]={{0,0},{6,0},{7,3},{4,5},{0,4}};
 BRepBuilderAPI_MakePolygon wire;for(const auto& v:p)wire.Add(gp_Pnt(2,3-v[1],4+v[0]));wire.Close();
 auto face=BRepBuilderAPI_MakeFace(wire.Wire()).Face();TopoDS_Shape shape=BRepPrimAPI_MakePrism(face,gp_Vec(3,0,0)).Shape();
 auto through=BRepPrimAPI_MakeCylinder(gp_Ax2(gp_Pnt(2,1,6),gp_Dir(1,0,0),gp_Dir(0,0,1)),.5,3).Shape();
 auto blind=BRepPrimAPI_MakeCylinder(gp_Ax2(gp_Pnt(2,1,9),gp_Dir(1,0,0),gp_Dir(0,0,1)),.75,allThrough?3:1.25).Shape();
 shape=BRepAlgoAPI_Cut(BRepAlgoAPI_Cut(shape,through).Shape(),blind).Shape();
 if(!BRepCheck_Analyzer(shape).IsValid())throw std::runtime_error("invalid independent pocket STEP source");
 STEPControl_Writer writer;if(writer.Transfer(shape,STEPControl_AsIs)!=IFSelect_RetDone||writer.Write(path)!=IFSelect_RetDone)throw std::runtime_error("independent pocket STEP export");return 0;
}

static int writeDiskPocketReference(const char* path) {
 const gp_Dir axis(-1,0,0), x(0,0,1);
 TopoDS_Shape stock=BRepPrimAPI_MakeCylinder(gp_Ax2(gp_Pnt(2,7,-2),axis,x),4,3).Shape();
 TopoDS_Shape through=BRepPrimAPI_MakeCylinder(gp_Ax2(gp_Pnt(2,7,-4),axis,x),.75,3).Shape();
 TopoDS_Shape blind=BRepPrimAPI_MakeCylinder(gp_Ax2(gp_Pnt(2,7,0),axis,x),1,1.25).Shape();
 TopoDS_Shape shape=BRepAlgoAPI_Cut(BRepAlgoAPI_Cut(stock,through).Shape(),blind).Shape();
 if(!BRepCheck_Analyzer(shape).IsValid())throw std::runtime_error("invalid independent disk pocket source");
 STEPControl_Writer writer;if(writer.Transfer(shape,STEPControl_AsIs)!=IFSelect_RetDone||writer.Write(path)!=IFSelect_RetDone)throw std::runtime_error("independent disk pocket STEP export");return 0;
}

static int writeRoundedStockReference(const char* path,bool pockets=false) {
 const double pi=std::acos(-1.0);
 const gp_Pnt origin(2,-3,4); const gp_Dir x(0,1,0),axis(-1,0,0);
 auto pt=[&](double u,double v){return gp_Pnt(2,-3+u,4-v);};
 BRepBuilderAPI_MakeWire wire;
 auto line=[&](double u0,double v0,double u1,double v1){wire.Add(BRepBuilderAPI_MakeEdge(pt(u0,v0),pt(u1,v1)).Edge());};
 auto arc=[&](double u,double v,double r,double a,double b){gp_Circ circle(gp_Ax2(pt(u,v),axis,x),r);wire.Add(BRepBuilderAPI_MakeEdge(circle,a,b).Edge());};
 line(.5,0,7,0);arc(7,1,1,-pi/2,0);line(8,1,8,5);line(8,5,1.5,5);arc(1.5,3.5,1.5,pi/2,pi);line(0,3.5,0,.5);arc(.5,.5,.5,pi,3*pi/2);
 TopoDS_Shape shape=BRepPrimAPI_MakePrism(BRepBuilderAPI_MakeFace(wire.Wire()).Face(),gp_Vec(-3,0,0)).Shape();
 if(pockets){ auto through=BRepPrimAPI_MakeCylinder(gp_Ax2(pt(2,2),axis,x),.5,3).Shape(); auto blind=BRepPrimAPI_MakeCylinder(gp_Ax2(pt(5,3),axis,x),.75,1).Shape();shape=BRepAlgoAPI_Cut(BRepAlgoAPI_Cut(shape,through).Shape(),blind).Shape(); }
 if(!BRepCheck_Analyzer(shape).IsValid())throw std::runtime_error("invalid independent rounded stock source");
 STEPControl_Writer writer;if(writer.Transfer(shape,STEPControl_AsIs)!=IFSelect_RetDone||writer.Write(path)!=IFSelect_RetDone)throw std::runtime_error("independent rounded stock STEP export");return 0;
}

#endif // PLAINSOLID_OCCT_STEP
