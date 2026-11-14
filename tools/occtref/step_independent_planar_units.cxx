// Independent OCCT selected-product material transfer and direct surface quadrature.
#include <STEPControl_Writer.hxx>
#include <BRepPrimAPI_MakeBox.hxx>
#include <STEPControl_Reader.hxx>
#include <StepData_StepModel.hxx>
#include <XSControl_WorkSession.hxx>
#include <StepShape_EdgeCurve.hxx>
#include <StepGeom_SurfaceCurve.hxx>
#include <BRepBuilderAPI_MakeEdge.hxx>
#include <BRepBuilderAPI_MakeWire.hxx>
#include <BRepBuilderAPI_MakeFace.hxx>
#include <BRepPrimAPI_MakePrism.hxx>
#include <BRepCheck_Analyzer.hxx>
#include <BRepGProp.hxx>
#include <BRepAdaptor_Curve.hxx>
#include <BRepAdaptor_Surface.hxx>
#include <GCPnts_AbscissaPoint.hxx>
#include <GProp_GProps.hxx>
#include <gp_Elips.hxx>
#include <gp_Pln.hxx>
#include <TopExp_Explorer.hxx>
#include <TopoDS.hxx>
#include <TopoDS_Wire.hxx>
#include <TopoDS_Edge.hxx>
#include <TopoDS_Face.hxx>
#include <iostream>
#include <iomanip>
#include <string>
#include <BRepPrimAPI_MakeSphere.hxx>
#include <Interface_Static.hxx>
#include <cmath>
#include <gp_Sphere.hxx>
#include <gp_Torus.hxx>
int main(int argc,char**argv){
 if(argc!=3)return 2;
 if(std::string(argv[1])=="write"){ STEPControl_Writer writer;Interface_Static::SetCVal("write.step.unit","M");writer.Transfer(BRepPrimAPI_MakeBox(gp_Pnt(-10000,-4000,-2000),2000,3000,4000).Shape(),STEPControl_AsIs);return writer.Write(argv[2])==IFSelect_RetDone?0:3;}
 if(std::string(argv[1])!="read")return 2;
 STEPControl_Reader r;
 if(r.ReadFile(argv[2])!=IFSelect_RetDone||r.TransferRoots()<1)return 5;
 auto s=r.OneShape();if(!BRepCheck_Analyzer(s).IsValid())return 6;

 int edges=0,faces=0,solids=0;double area=0,volume=0;gp_XYZ first(0,0,0);
 for(TopExp_Explorer e(s,TopAbs_EDGE);e.More();e.Next())++edges;
 for(TopExp_Explorer e(s,TopAbs_SOLID);e.More();e.Next())++solids;
 for(TopExp_Explorer e(s,TopAbs_FACE);e.More();e.Next()){
  ++faces;auto face=TopoDS::Face(e.Current());BRepAdaptor_Surface sf(face);
  double sense=face.Orientation()==TopAbs_REVERSED?-1:1;
  if(sf.GetType()==GeomAbs_Torus){
   const auto torus=sf.Torus();const double pi=std::acos(-1.0),R=torus.MajorRadius(),r=torus.MinorRadius();
   if(std::abs(sf.LastUParameter()-sf.FirstUParameter()-2*pi)>1e-12 || std::abs(sf.LastVParameter()-sf.FirstVParameter()-2*pi)>1e-12)return 8;
   double a=4*pi*pi*R*r;area+=a;first+=torus.Location().XYZ()*a;volume+=sense*2*pi*pi*R*r*r;
  }else if(sf.GetType()==GeomAbs_Sphere){
   const auto sphere=sf.Sphere();const double radius=sphere.Radius(),pi=std::acos(-1.0);
   if(std::abs(sf.LastUParameter()-sf.FirstUParameter()-2*pi)>1e-12 || std::abs(sf.FirstVParameter()+pi/2)>1e-12 || std::abs(sf.LastVParameter()-pi/2)>1e-12)return 7;
   double a=4*pi*radius*radius;area+=a;first+=sphere.Location().XYZ()*a;volume+=sense*a*radius/3;
  }else if(sf.GetType()==GeomAbs_Plane){
   GProp_GProps g;BRepGProp::SurfaceProperties(face,g);double a=g.Mass();auto c=g.CentreOfMass();
   gp_Pnt p;gp_Vec du,dv;sf.D1(0,0,p,du,dv);auto normal=du.Crossed(dv);normal.Normalize();normal*=sense;
   area+=a;first+=c.XYZ()*a;volume+=a*c.XYZ().Dot(normal.XYZ())/3;
  }else{
   const int n=32768;double lo=sf.FirstUParameter(),hi=sf.LastUParameter(),vl=sf.FirstVParameter(),vh=sf.LastVParameter(),h=(hi-lo)/n;
   for(int i=0;i<=n;i++){gp_Pnt p;gp_Vec du,dv;sf.D1(lo+i*h,(vl+vh)/2,p,du,dv);auto cross=du.Crossed(dv);double weight=(i==0||i==n)?1:(i%2?4:2);double scale=weight*h*(vh-vl)/3;double a=scale*cross.Magnitude();area+=a;first+=p.XYZ()*a;volume+=sense*scale*p.XYZ().Dot(cross.XYZ())/3;}
  }
 }
 GProp_GProps library;BRepGProp::VolumeProperties(s,library);
 gp_Pnt c(first/area);
 std::cout<<std::setprecision(17)<<"{\"Area\":"<<area<<",\"Volume\":"<<volume<<",\"LibraryVolume\":"<<library.Mass()<<",\"Faces\":"<<faces<<",\"Solids\":"<<solids<<",\"EdgeUses\":"<<edges<<",\"SurfaceCenter\":["<<c.X()<<","<<c.Y()<<","<<c.Z()<<"]}\n";
}
