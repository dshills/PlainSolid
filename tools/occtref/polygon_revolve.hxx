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
