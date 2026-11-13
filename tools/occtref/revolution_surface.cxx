// Independent optional OCCT swept-surface oracle; not part of the Go library.
#include <STEPControl_Reader.hxx>
#include <STEPControl_Writer.hxx>
#include <StepData_StepModel.hxx>
#include <StepGeom_Surface.hxx>
#include <StepGeom_RectangularTrimmedSurface.hxx>
#include <TCollection_HAsciiString.hxx>
#include <GeomToStep_MakeSurface.hxx>
#include <Geom_BSplineCurve.hxx>
#include <Geom_SurfaceOfRevolution.hxx>
#include <Geom_RectangularTrimmedSurface.hxx>
#include <NCollection_Array1.hxx>
#include <BRepCheck_Analyzer.hxx>
#include <BRepBuilderAPI_MakeFace.hxx>
#include <BRep_Tool.hxx>
#include <BRepTools.hxx>
#include <TopoDS.hxx>
#include <TopExp_Explorer.hxx>
#include <Standard_Failure.hxx>
#include <gp_Ax1.hxx>
#include <cmath>
#include <iomanip>
#include <iostream>
#include <string>
int main(int argc,char**argv){
 if(argc!=3)return 2;
 try{
  std::string mode=argv[1];
  if(mode=="write"||mode=="write-curved"||mode=="write-oblique"||mode=="write-rational"){
   bool curved=mode!="write";int n=curved?3:2;NCollection_Array1<gp_Pnt> poles(1,n);poles(1)=gp_Pnt(2,0,0);poles(n)=gp_Pnt(3,0,3);if(curved)poles(2)=gp_Pnt(4,0,1);
   if(mode=="write-oblique") for(int i=1;i<=n;++i) poles(i)=gp_Pnt(3*poles(i).X(),4*poles(i).X(),poles(i).Z());
   NCollection_Array1<double> knots(1,2);knots(1)=0;knots(2)=1;NCollection_Array1<int> mults(1,2);mults(1)=n;mults(2)=n;
   occ::handle<Geom_BSplineCurve> profile=new Geom_BSplineCurve(poles,knots,mults,n-1);
   if(mode=="write-rational"){NCollection_Array1<double> weights(1,3);weights(1)=1;weights(2)=.5;weights(3)=1;profile=new Geom_BSplineCurve(poles,weights,knots,mults,n-1);}
   occ::handle<Geom_SurfaceOfRevolution> support=new Geom_SurfaceOfRevolution(profile,gp_Ax1(gp_Pnt(0,0,0),gp_Dir(0,0,1)));
   occ::handle<Geom_RectangularTrimmedSurface> patch=new Geom_RectangularTrimmedSurface(support,.25,1.25,0.,1.);
   STEPControl_Writer writer;if(writer.Transfer(BRepBuilderAPI_MakeFace(patch,1e-9).Face(),STEPControl_AsIs)!=IFSelect_RetDone)return 3;
   // The OCCT rectangular-surface converter hardcodes degree U values for
   // revolutions. Author the typed bounded record explicitly in the model's
   // radian context, using the original native bounds without conversion.
   GeomToStep_MakeSurface convert(support);
   occ::handle<StepGeom_RectangularTrimmedSurface> bounded=new StepGeom_RectangularTrimmedSurface;
   bounded->Init(new TCollection_HAsciiString("radian revolution"),convert.Value(),.25,1.25,0.,1.,true,true);
   writer.Model()->AddWithRefs(bounded);return writer.Write(argv[2])==IFSelect_RetDone?0:4;
  }
  STEPControl_Reader reader;if(reader.ReadFile(argv[2])!=IFSelect_RetDone||reader.TransferRoots()==0)return 5;auto shape=reader.OneShape();if(shape.IsNull()||!BRepCheck_Analyzer(shape).IsValid())return 6;
  int faces=0;for(TopExp_Explorer x(shape,TopAbs_FACE);x.More();x.Next())++faces;if(faces!=1)return 7;
  TopExp_Explorer x(shape,TopAbs_FACE);auto face=TopoDS::Face(x.Current());auto surface=BRep_Tool::Surface(face);auto basis=surface;
  if(auto trim=occ::down_cast<Geom_RectangularTrimmedSurface>(basis);!trim.IsNull())basis=trim->BasisSurface();if(!basis->IsKind(STANDARD_TYPE(Geom_SurfaceOfRevolution)))return 8;
  double u0,u1,v0,v1;BRepTools::UVBounds(face,u0,u1,v0,v1);
  auto integrate=[&](int n){double total=0,h=(v1-v0)/n;for(int i=0;i<=n;++i){gp_Pnt p;gp_Vec du,dv;surface->D1((u0+u1)/2,v0+i*h,p,du,dv);total+=(i==0||i==n?1:i%2?4:2)*du.Crossed(dv).Magnitude();}return total*h*(u1-u0)/3;};
  double coarse=integrate(8192),fine=integrate(16384);if(std::abs(coarse-fine)>1e-10)return 9;
  gp_Pnt p;gp_Vec du,dv;surface->D1(.5,.5,p,du,dv);
  std::cout<<std::setprecision(17)<<"{\"Faces\":"<<faces<<",\"Area\":"<<fine<<",\"Point\":["<<p.X()<<","<<p.Y()<<","<<p.Z()<<"],\"Du\":["<<du.X()<<","<<du.Y()<<","<<du.Z()<<"],\"Dv\":["<<dv.X()<<","<<dv.Y()<<","<<dv.Z()<<"]}\n";return 0;
 }catch(const Standard_Failure&e){std::cerr<<e.what()<<"\n";return 10;}
}
