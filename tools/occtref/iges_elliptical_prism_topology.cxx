// Independent IGES154 solid source construction and conventional material transfer.
#include <IGESControl_Writer.hxx>
#include <IGESControl_Reader.hxx>
#include <IGESSolid_Cylinder.hxx>
#include <BRepCheck_Analyzer.hxx>
#include <BRepGProp.hxx>
#include <GProp_GProps.hxx>
#include <TopExp_Explorer.hxx>
#include <BRepAdaptor_Surface.hxx>
#include <TopoDS.hxx>
#include <TopoDS_Face.hxx>
#include <Standard_Failure.hxx>
#include <iostream>
#include <iomanip>
#include <string>
int main(int argc,char**argv){if(argc!=3)return 2;try{
 IGESControl_Reader r;if(r.ReadFile(argv[2])!=IFSelect_RetDone)return 5;r.TransferRoots();auto s=r.OneShape();
 if(s.IsNull()){std::cerr<<"IGES154: no shape transferred\n";return 6;}if(!BRepCheck_Analyzer(s).IsValid())return 7;
 int edges=0,faces=0,solids=0;double area=0,volume=0;gp_XYZ first(0,0,0);
 for(TopExp_Explorer e(s,TopAbs_EDGE);e.More();e.Next())++edges;
 for(TopExp_Explorer e(s,TopAbs_SOLID);e.More();e.Next())++solids;
 for(TopExp_Explorer e(s,TopAbs_FACE);e.More();e.Next()){
  ++faces;auto face=TopoDS::Face(e.Current());BRepAdaptor_Surface sf(face);
  double sense=face.Orientation()==TopAbs_REVERSED?-1:1;
  if(sf.GetType()==GeomAbs_Plane){
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
 std::cout<<std::setprecision(17)<<"{\"Area\":"<<area<<",\"Volume\":"<<volume<<",\"LibraryVolume\":"<<library.Mass()<<",\"Faces\":"<<faces<<",\"Solids\":"<<solids<<",\"EdgeUses\":"<<edges<<",\"Center\":["<<c.X()<<","<<c.Y()<<","<<c.Z()<<"]}\n";
 return 0;
}catch(const Standard_Failure&e){std::cerr<<e.what();return 9;}}
