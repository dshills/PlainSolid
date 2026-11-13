// Optional independent OCCT oracle; not linked into the Go library.
// Build with build_extrusion_bridge.sh. No shared oracle dispatch is modified.
#include <STEPControl_Reader.hxx>
#include <STEPControl_Writer.hxx>
#include <StepData_StepModel.hxx>
#include <StepGeom_Surface.hxx>
#include <GeomToStep_MakeSurface.hxx>
#include <Geom_BSplineCurve.hxx>
#include <Geom_SurfaceOfLinearExtrusion.hxx>
#include <Geom_RectangularTrimmedSurface.hxx>
#include <NCollection_Array1.hxx>
#include <BRepCheck_Analyzer.hxx>
#include <BRepBuilderAPI_MakeFace.hxx>
#include <BRep_Tool.hxx>
#include <BRepTools.hxx>
#include <TopoDS.hxx>
#include <cmath>
#include <BRepGProp.hxx>
#include <GProp_GProps.hxx>
#include <TopExp_Explorer.hxx>
#include <TopoDS_Shape.hxx>
#include <Standard_Failure.hxx>
#include <iostream>
#include <iomanip>
#include <string>
int main(int argc,char**argv){
 if(argc!=3)return 2;
 try{
  if(std::string(argv[1])=="write"||std::string(argv[1])=="write-negative"){
   bool negative=std::string(argv[1])=="write-negative";
   NCollection_Array1<gp_Pnt> p(1,3);p(1)=gp_Pnt(2,0,0);p(2)=gp_Pnt(4,0,1);p(3)=gp_Pnt(3,0,3);
   NCollection_Array1<double> w(1,3);w(1)=1;w(2)=.5;w(3)=1;
   NCollection_Array1<double> k(1,2);k(1)=0;k(2)=1;NCollection_Array1<int> m(1,2);m(1)=m(2)=3;
   occ::handle<Geom_BSplineCurve> curve=new Geom_BSplineCurve(p,w,k,m,2);
   occ::handle<Geom_SurfaceOfLinearExtrusion> support=new Geom_SurfaceOfLinearExtrusion(curve,gp_Dir(0,negative?-3:3,negative?-4:4));
   occ::handle<Geom_RectangularTrimmedSurface> patch=new Geom_RectangularTrimmedSurface(support,0.,1.,0.,5.);
   GeomToStep_MakeSurface convert(patch);
   STEPControl_Writer writer;
   if(writer.Transfer(BRepBuilderAPI_MakeFace(patch,1e-9).Face(),STEPControl_AsIs)!=IFSelect_RetDone)return 7;
   writer.Model()->AddWithRefs(convert.Value());
   return writer.Write(argv[2])==IFSelect_RetDone?0:3;
  }
  STEPControl_Reader reader;if(reader.ReadFile(argv[2])!=IFSelect_RetDone||reader.TransferRoots()==0)return 4;
  TopoDS_Shape shape=reader.OneShape();if(shape.IsNull()||!BRepCheck_Analyzer(shape).IsValid())return 5;
  int faces=0;for(TopExp_Explorer e(shape,TopAbs_FACE);e.More();e.Next())++faces;
  GProp_GProps mass;BRepGProp::SurfaceProperties(shape,mass,1e-12);
  if(faces!=1)return 8;
  TopExp_Explorer explorer(shape,TopAbs_FACE);auto face=TopoDS::Face(explorer.Current());
  auto surface=BRep_Tool::Surface(face);auto basis=surface;
  if(auto trim=occ::down_cast<Geom_RectangularTrimmedSurface>(basis);!trim.IsNull())basis=trim->BasisSurface();
  if(!basis->IsKind(STANDARD_TYPE(Geom_SurfaceOfLinearExtrusion)))return 9;
  double u0,u1,v0,v1;BRepTools::UVBounds(face,u0,u1,v0,v1);
  auto integrate=[&](int n){double total=0,h=(u1-u0)/n;for(int i=0;i<=n;++i){gp_Pnt p;gp_Vec du,dv;surface->D1(u0+i*h,(v0+v1)/2,p,du,dv);double q=du.Crossed(dv).Magnitude();total+=(i==0||i==n?1:i%2?4:2)*q;}return total*h*(v1-v0)/3;};
  double coarse=integrate(8192),fine=integrate(16384);if(std::abs(fine-coarse)>1e-10)return 10;
  gp_Pnt point;gp_Vec du,dv;surface->D1(.5,1.25,point,du,dv);
  std::cout<<std::setprecision(17)<<"{\"Faces\":"<<faces<<",\"Area\":"<<fine<<",\"Point\":["<<point.X()<<","<<point.Y()<<","<<point.Z()<<"],\"Du\":["<<du.X()<<","<<du.Y()<<","<<du.Z()<<"],\"Dv\":["<<dv.X()<<","<<dv.Y()<<","<<dv.Z()<<"]}\n";return 0;

 }catch(const Standard_Failure&e){std::cerr<<e.what()<<"\n";return 6;}
}
