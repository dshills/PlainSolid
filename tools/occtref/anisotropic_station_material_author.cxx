// Independently authored original NURBS geometry. Never reads PlainSolid output.
#include <Geom_BSplineSurface.hxx>
#include <BRepBuilderAPI_MakeFace.hxx>
#include <BRepBuilderAPI_MakeWire.hxx>
#include <BRepBuilderAPI_MakeSolid.hxx>
#include <BRepBuilderAPI_MakeVertex.hxx>
#include <BRepBuilderAPI_Sewing.hxx>
#include <BRepAlgoAPI_Cut.hxx>
#include <BRepExtrema_DistShapeShape.hxx>
#include <BRep_Tool.hxx>
#include <BRepCheck_Analyzer.hxx>
#include <BRepGProp.hxx>
#include <GProp_GProps.hxx>
#include <STEPControl_Writer.hxx>
#include <TopExp.hxx>
#include <TopExp_Explorer.hxx>
#include <NCollection_IndexedMap.hxx>
#include <TopTools_ShapeMapHasher.hxx>
#include <TopoDS.hxx>
#include <TopoDS_Solid.hxx>
#include <TopoDS_Shell.hxx>
#include <Standard_Version.hxx>
#include <math.hxx>
#include <math_Vector.hxx>
#include <cmath>
#include <gp_Pln.hxx>
#include <fstream>
#include <iomanip>
#include <stdexcept>
#include <iostream>
#include <string>
#include <vector>

using Surface = occ::handle<Geom_BSplineSurface>;
static TopoDS_Solid loft(bool inner,std::vector<Surface>& originals) {
 const double xy[9][2]={{1,0},{1,1},{0,1},{-1,1},{-1,0},{-1,-1},{0,-1},{1,-1},{1,0}};
 const double wu[9]={1,.5,1,.75,1,.75,1,.75,1};
 const double outer[3][3]={{-4,3,1},{-3,1,2},{-1,2,1}};
 const double hole[3][3]={{-4,.5,.25},{-2,.25,.5},{-1,.375,.25}};
 const auto stations=inner?hole:outer;
 const double cuts[5]={-2,-1,1,4,8};
 BRepBuilderAPI_Sewing sewing(1e-10);
 for(int band=0;band<2;++band) {
  NCollection_Array2<gp_Pnt> poles(1,9,1,2);NCollection_Array2<double> weights(1,9,1,2);
  for(int u=0;u<9;++u)for(int v=0;v<2;++v){const auto row=stations[band+v];poles.SetValue(u+1,v+1,gp_Pnt(.25+row[1]*xy[u][0],-.5+row[2]*xy[u][1],row[0]));weights.SetValue(u+1,v+1,wu[u]);}
  NCollection_Array1<double> uk(1,5),vk(1,2);NCollection_Array1<int> um(1,5),vm(1,2);
  for(int i=1;i<=5;++i){uk.SetValue(i,cuts[i-1]);um.SetValue(i,i==1||i==5?3:2);}
  vk.SetValue(1,2);vk.SetValue(2,5);vm.SetValue(1,2);vm.SetValue(2,2);
  Surface surface=new Geom_BSplineSurface(poles,weights,uk,vk,um,vm,2,1);
  originals.push_back(surface);
  auto side=BRepBuilderAPI_MakeFace(surface,1e-10).Face();sewing.Add(side);
  for(TopExp_Explorer ex(side,TopAbs_EDGE);ex.More();ex.Next()) {
   auto edge=TopoDS::Edge(ex.Current());double lo,hi;auto curve=BRep_Tool::Curve(edge,lo,hi);if(curve.IsNull())continue;
   auto a=curve->Value(lo),b=curve->Value(hi),mid=curve->Value((lo+hi)/2);
   if(a.Z()!=b.Z()||a.Z()!=mid.Z()||(a.Z()!=-4&&a.Z()!=-1))continue;
   edge.Orientation(TopAbs_FORWARD);
   auto cap=BRepBuilderAPI_MakeFace(gp_Pln(gp_Pnt(0,0,a.Z()),gp_Dir(0,0,1)),BRepBuilderAPI_MakeWire(edge).Wire(),true).Face();
   if(a.Z()==-4)cap.Reverse();sewing.Add(cap);
  }
 }
 sewing.Perform();auto result=BRepBuilderAPI_MakeSolid(TopoDS::Shell(sewing.SewedShape())).Solid();
 if(!BRepCheck_Analyzer(result).IsValid())throw std::runtime_error("invalid independently authored anisotropic loft");
 return result;
}
// Independent OCCT D1 quadrature with original knot partitions. Two orders
// must agree; this is numerical reference evidence, not a rigorous enclosure.
static double integrateSurface(const Surface& surface,double u0,double u1,double v0,double v1,int order){
 math_Vector points(1,order),weights(1,order);math::GaussPoints(order,points);math::GaussWeights(order,weights);
 double sum=0;for(int i=1;i<=order;++i)for(int j=1;j<=order;++j){double u=(u0+u1+(u1-u0)*points(i))/2,v=(v0+v1+(v1-v0)*points(j))/2;gp_Pnt p;gp_Vec du,dv;surface->D1(u,v,p,du,dv);sum+=weights(i)*weights(j)*du.Crossed(dv).Magnitude();}
 return sum*(u1-u0)*(v1-v0)/4;
}
// Same planar cap image, integrated through an independently authored exact
// rational strip; it does not replace the authored planar face.
static double capArea(const std::vector<Surface> originals[2],int cap,int order) {
 const auto outer=originals[0][cap];const int row=cap+1;
 NCollection_Array2<gp_Pnt> poles(1,9,1,2);NCollection_Array2<double> weights(1,9,1,2);
 for(int u=1;u<=9;++u) {
  gp_Pnt a(.25,-.5,cap?-1:-4);
  if(!originals[1].empty())a=originals[1][cap]->Pole(u,row);
  poles.SetValue(u,1,a);poles.SetValue(u,2,outer->Pole(u,row));
  weights.SetValue(u,1,outer->Weight(u,row));weights.SetValue(u,2,outer->Weight(u,row));
 }
 const double cuts[5]={-2,-1,1,4,8};
 NCollection_Array1<double> uk(1,5),vk(1,2);NCollection_Array1<int> um(1,5),vm(1,2);
 for(int i=1;i<=5;++i){uk.SetValue(i,cuts[i-1]);um.SetValue(i,i==1||i==5?3:2);}
 vk.SetValue(1,0);vk.SetValue(2,1);vm.SetValue(1,2);vm.SetValue(2,2);
 Surface surface=new Geom_BSplineSurface(poles,weights,uk,vk,um,vm,2,1);
 double value=0;for(int i=0;i<4;++i)value+=integrateSurface(surface,cuts[i],cuts[i+1],0.,1.,order);
 return value;
}

int main(int argc,char**argv) {
 try {
  if(argc!=4)return 2;
  const std::string mode=argv[3];if(mode!="filled"&&mode!="bore")return 2;
  std::vector<Surface> original[2];TopoDS_Shape shape=loft(false,original[0]);
  if(mode=="bore") {auto inner=loft(true,original[1]);BRepAlgoAPI_Cut cut(shape,inner);cut.Build();if(!cut.IsDone())throw std::runtime_error("independent cut failed");shape=cut.Shape();}
  if(!BRepCheck_Analyzer(shape).IsValid())throw std::runtime_error("invalid independent material");
  int solids=0,shells=0,wires=0;
  for(TopExp_Explorer e(shape,TopAbs_SOLID);e.More();e.Next())++solids;
  for(TopExp_Explorer e(shape,TopAbs_SHELL);e.More();e.Next())++shells;
  NCollection_IndexedMap<TopoDS_Shape,TopTools_ShapeMapHasher> vertices,edges,faces;TopExp::MapShapes(shape,TopAbs_VERTEX,vertices);TopExp::MapShapes(shape,TopAbs_EDGE,edges);TopExp::MapShapes(shape,TopAbs_FACE,faces);
  for(int i=1;i<=faces.Extent();++i)for(TopExp_Explorer e(faces(i),TopAbs_WIRE);e.More();e.Next())++wires;
  const int holes=wires-faces.Extent(),euler=vertices.Extent()-edges.Extent()+faces.Extent()-holes;
  if(solids!=1||shells!=1||euler!=(mode=="bore"?0:2)||holes!=(mode=="bore"?2:0))throw std::runtime_error("unexpected independent topology");
  STEPControl_Writer writer;if(writer.Transfer(shape,STEPControl_AsIs)!=IFSelect_RetDone||writer.Write(argv[1])!=IFSelect_RetDone)throw std::runtime_error("STEP output failed");
  GProp_GProps volume,area;BRepGProp::VolumePropertiesGK(shape,volume,1e-12,true,true,true,true);BRepGProp::SurfaceProperties(shape,area,1e-12);auto c=volume.CentreOfMass();
  // Partition only the integral at original U knots; retain the authored
  // solid unchanged; retain the differing whole-face result diagnostically.
  double integrals[2]={0,0};const double cuts[5]={-2,-1,1,4,8};
  for(int pass=0;pass<2;++pass){int order=pass?48:32;for(const auto& family:original)for(const auto& surface:family)for(int i=0;i<4;++i)integrals[pass]+=integrateSurface(surface,cuts[i],cuts[i+1],2.,5.,order);integrals[pass]+=capArea(original,0,order)+capArea(original,1,order);}
  double spanArea=integrals[1],areaChange=std::abs(integrals[1]-integrals[0]);if(areaChange>1e-9)throw std::runtime_error("independent area quadrature did not converge");
  std::ofstream out(argv[2]);out<<std::setprecision(17)<<"{\"OCCTVersion\":\""<<OCC_VERSION_COMPLETE<<"\",\"Volume\":"<<volume.Mass()<<",\"Area\":"<<spanArea<<",\"AreaChange\":"<<areaChange<<",\"WholeFaceArea\":"<<area.Mass()<<",\"Centroid\":["<<c.X()<<","<<c.Y()<<","<<c.Z()<<"],\"Solids\":"<<solids<<",\"Shells\":"<<shells<<",\"Faces\":"<<faces.Extent()<<",\"Holes\":"<<holes<<",\"Euler\":"<<euler<<",\"Samples\":[";
  bool first=true;
  for(int side=0;side<2;++side)for(size_t band=0;band<original[side].size();++band)for(double u:{-1.75,0.,2.5,6.})for(double v:{2.,2.75,3.5,4.25,5.}) {
   auto point=original[side][band]->Value(u,v);
   BRepExtrema_DistShapeShape distance(BRepBuilderAPI_MakeVertex(point).Vertex(),shape);distance.Perform();
   if(!distance.IsDone()||distance.NbSolution()<1||distance.Value()>1e-8)throw std::runtime_error("original support sample absent from material boundary");
   if(!first)out<<",";first=false;
   out<<"{\"Side\":"<<side<<",\"Band\":"<<band<<",\"U\":"<<u<<",\"V\":"<<v<<",\"Point\":["<<point.X()<<","<<point.Y()<<","<<point.Z()<<"],\"BoundaryDistance\":"<<distance.Value()<<"}";
  }
  out<<"]}\n";return out?0:3;
 }catch(const std::exception&e){std::cerr<<e.what()<<"\n";return 1;}
}
