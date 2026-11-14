// Independent original OCCT face construction and source/output measurement.
#include <STEPControl_Writer.hxx>
#include <STEPControl_Reader.hxx>
#include <BRepBuilderAPI_MakeEdge.hxx>
#include <BRepBuilderAPI_MakeWire.hxx>
#include <BRepBuilderAPI_MakeFace.hxx>
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
int main(int argc,char**argv){
 if(argc!=3)return 2;
 if(std::string(argv[1])=="write"){
  gp_Elips e(gp_Ax2(gp_Pnt(1,2,3),gp_Dir(0,1,0),gp_Dir(0,0,-1)),4,2);
  auto edge=BRepBuilderAPI_MakeEdge(e).Edge();
  auto wire=BRepBuilderAPI_MakeWire(edge).Wire();
  gp_Elips inner(e.Position(),2,1);
  auto hole=BRepBuilderAPI_MakeWire(BRepBuilderAPI_MakeEdge(inner).Edge()).Wire();hole.Reverse();
  BRepBuilderAPI_MakeFace builder(gp_Pln(e.Position()),wire,true);builder.Add(hole);auto face=builder.Face();
  STEPControl_Writer w;
  if(w.Transfer(face,STEPControl_AsIs)!=IFSelect_RetDone)return 3;
  return w.Write(argv[2])==IFSelect_RetDone?0:4;
 }
 STEPControl_Reader r;
 if(r.ReadFile(argv[2])!=IFSelect_RetDone||r.TransferRoots()!=1)return 5;
 auto s=r.OneShape();if(!BRepCheck_Analyzer(s).IsValid())return 6;
 int n=0;double length=0;gp_Pnt sample;gp_Vec derivative;
 for(TopExp_Explorer e(s,TopAbs_EDGE);e.More();e.Next()){
  ++n;BRepAdaptor_Curve c(TopoDS::Edge(e.Current()));
  length+=GCPnts_AbscissaPoint::Length(c,c.FirstParameter(),c.LastParameter(),1e-11);
  c.D1(.375,sample,derivative);
 }
 int faces=0;gp_Vec normal;for(TopExp_Explorer f(s,TopAbs_FACE);f.More();f.Next()){++faces;auto face=TopoDS::Face(f.Current());BRepAdaptor_Surface surface(face);gp_Pnt p;gp_Vec du,dv;surface.D1(0,0,p,du,dv);normal=du.Crossed(dv);normal.Normalize();if(face.Orientation()==TopAbs_REVERSED)normal.Reverse();}
 GProp_GProps p;BRepGProp::SurfaceProperties(s,p);auto c=p.CentreOfMass();
 std::cout<<std::setprecision(17)<<"{\"Area\":"<<p.Mass()<<",\"Edges\":"<<n
  <<",\"Faces\":"<<faces<<",\"Normal\":["<<normal.X()<<","<<normal.Y()<<","<<normal.Z()<<"],\"Length\":"<<length<<",\"Center\":["<<c.X()<<","<<c.Y()<<","<<c.Z()
  <<"],\"Point\":["<<sample.X()<<","<<sample.Y()<<","<<sample.Z()
  <<"],\"Derivative\":["<<derivative.X()<<","<<derivative.Y()<<","<<derivative.Z()<<"]}\n";
}
