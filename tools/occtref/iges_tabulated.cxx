#include <IGESControl_Writer.hxx>
#include <IGESControl_Reader.hxx>
#include <IGESData_IGESModel.hxx>
#include <IGESGeom_TabulatedCylinder.hxx>
#include <IGESGeom_BSplineCurve.hxx>
#include <GeomToIGES_GeomCurve.hxx>
#include <Geom_BSplineCurve.hxx>
#include <Geom_SurfaceOfRevolution.hxx>
#include <Geom_RectangularTrimmedSurface.hxx>
#include <NCollection_Array1.hxx>
#include <BRepCheck_Analyzer.hxx>
#include <BRep_Tool.hxx>
#include <BRepTools.hxx>
#include <TopExp_Explorer.hxx>
#include <TopoDS.hxx>
#include <Standard_Failure.hxx>
#include <iostream>
#include <iomanip>
#include <cmath>
#include <string>
int main(int argc,char**argv){if(argc!=3)return 2;try{
 std::string mode=argv[1];if(mode=="write"||mode=="write-negative"){
  NCollection_Array1<gp_Pnt> poles(1,3);poles(1)=gp_Pnt(2,0,0);poles(2)=gp_Pnt(4,0,1);poles(3)=gp_Pnt(3,0,3);NCollection_Array1<double> weights(1,3);weights(1)=1;weights(2)=.5;weights(3)=1;NCollection_Array1<double> knots(1,2);knots(1)=0;knots(2)=1;NCollection_Array1<int> mults(1,2);mults(1)=mults(2)=3;
  occ::handle<Geom_BSplineCurve> profile=new Geom_BSplineCurve(poles,weights,knots,mults,2);GeomToIGES_GeomCurve converter;converter.SetUnit(1.);auto generatrix=converter.TransferCurve(profile,0,1);
  bool negative=mode=="write-negative";occ::handle<IGESGeom_TabulatedCylinder> root=new IGESGeom_TabulatedCylinder;root->Init(generatrix,gp_XYZ(2,negative?-3:3,negative?-4:4));IGESControl_Writer writer("MM",0);if(!writer.AddEntity(root))return 3;return writer.Write(argv[2])?0:4;
 }
 IGESControl_Reader reader;if(reader.ReadFile(argv[2])!=IFSelect_RetDone||reader.TransferRoots()==0)return 5;auto shape=reader.OneShape();if(shape.IsNull()||!BRepCheck_Analyzer(shape).IsValid())return 6;
 occ::handle<IGESGeom_TabulatedCylinder> root;auto model=reader.IGESModel();for(int i=1;i<=model->NbEntities();++i){auto e=occ::down_cast<IGESGeom_TabulatedCylinder>(model->Entity(i));if(!e.IsNull()){if(!root.IsNull())return 7;root=e;}}if(root.IsNull())return 8;
 int faces=0;for(TopExp_Explorer x(shape,TopAbs_FACE);x.More();x.Next())++faces;if(faces!=1)return 9;TopExp_Explorer x(shape,TopAbs_FACE);auto face=TopoDS::Face(x.Current());auto s=BRep_Tool::Surface(face);double u0,u1,v0,v1;BRepTools::UVBounds(face,u0,u1,v0,v1);
 auto integrate=[&](int n){double sum=0,h=(u1-u0)/n;for(int i=0;i<=n;++i){gp_Pnt p;gp_Vec du,dv;s->D1(u0+i*h,(v0+v1)/2,p,du,dv);sum+=(i==0||i==n?1:i%2?4:2)*du.Crossed(dv).Magnitude();}return sum*h*(v1-v0)/3;};double area=integrate(16384);if(std::abs(area-integrate(8192))>1e-10)return 10;
 double sample=.5;gp_Pnt p;gp_Vec du,dv;s->D1(sample,v0+.25*(v1-v0),p,du,dv);dv.Multiply(v1-v0);
 std::cout<<std::setprecision(17)<<"{\"Faces\":"<<faces<<",\"Area\":"<<area<<",\"SampleU\":"<<sample<<",\"Point\":["<<p.X()<<","<<p.Y()<<","<<p.Z()<<"],\"Du\":["<<du.X()<<","<<du.Y()<<","<<du.Z()<<"],\"Dv\":["<<dv.X()<<","<<dv.Y()<<","<<dv.Z()<<"]}\n";return 0;
}catch(const Standard_Failure&e){std::cerr<<e.what();return 11;}}
