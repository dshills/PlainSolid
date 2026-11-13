// Optional independent analytic support oracle; no shape transfer is asserted.
#include <STEPControl_Reader.hxx>
#include <STEPControl_Writer.hxx>
#include <XSControl_WorkSession.hxx>
#include <StepData_StepModel.hxx>
#include <StepGeom_SphericalSurface.hxx>
#include <GeomToStep_MakeSphericalSurface.hxx>
#include <Geom_SphericalSurface.hxx>
#include <StepToGeom.hxx>
#include <StepGeom_GeometricRepresentationContextAndGlobalUnitAssignedContext.hxx>
#include <StepBasic_SiUnitAndLengthUnit.hxx>
#include <StepBasic_SiUnitAndPlaneAngleUnit.hxx>
#include <StepBasic_SiUnitAndSolidAngleUnit.hxx>
#include <StepShape_GeometricSet.hxx>
#include <StepShape_GeometricSetSelect.hxx>
#include <StepRepr_Representation.hxx>
#include <StepRepr_RepresentationItem.hxx>
#include <NCollection_HArray1.hxx>
#include <TCollection_HAsciiString.hxx>
#include <Standard_Failure.hxx>
#include <iostream>
#include <iomanip>
#include <string>
int main(int argc,char**argv){if(argc!=3)return 2;try{
 std::string mode=argv[1];if(mode=="write"||mode=="write-negative"){
  auto name=[](const char*s){return occ::handle<TCollection_HAsciiString>(new TCollection_HAsciiString(s));};
  occ::handle<Geom_SphericalSurface> g=new Geom_SphericalSurface(gp_Ax3(gp_Pnt(2,3,4),gp_Dir(mode=="write"?1:-1,0,0),gp_Dir(0,0,-1)),2.5);
  GeomToStep_MakeSphericalSurface convert(g);auto s=convert.Value();s->SetName(name("SPHERE"));
  occ::handle<StepBasic_SiUnitAndLengthUnit> length=new StepBasic_SiUnitAndLengthUnit;length->Init(true,StepBasic_spMilli,StepBasic_sunMetre);
  occ::handle<StepBasic_SiUnitAndPlaneAngleUnit> angle=new StepBasic_SiUnitAndPlaneAngleUnit;angle->Init(false,StepBasic_spMilli,StepBasic_sunRadian);
  occ::handle<StepBasic_SiUnitAndSolidAngleUnit> solid=new StepBasic_SiUnitAndSolidAngleUnit;solid->Init(false,StepBasic_spMilli,StepBasic_sunSteradian);
  occ::handle<NCollection_HArray1<occ::handle<StepBasic_NamedUnit>>> units=new NCollection_HArray1<occ::handle<StepBasic_NamedUnit>>(1,3);units->SetValue(1,length);units->SetValue(2,angle);units->SetValue(3,solid);
  occ::handle<StepGeom_GeometricRepresentationContextAndGlobalUnitAssignedContext> context=new StepGeom_GeometricRepresentationContextAndGlobalUnitAssignedContext;context->Init(name(""),name(""),3,units);
  occ::handle<NCollection_HArray1<StepShape_GeometricSetSelect>> elements=new NCollection_HArray1<StepShape_GeometricSetSelect>(1,1);StepShape_GeometricSetSelect select;select.SetValue(s);elements->SetValue(1,select);
  occ::handle<StepShape_GeometricSet> set=new StepShape_GeometricSet;set->Init(name(""),elements);
  occ::handle<NCollection_HArray1<occ::handle<StepRepr_RepresentationItem>>> items=new NCollection_HArray1<occ::handle<StepRepr_RepresentationItem>>(1,1);items->SetValue(1,set);
  occ::handle<StepRepr_Representation> rep=new StepRepr_Representation;rep->Init(name(""),items,context);
  STEPControl_Writer w;w.Model()->AddWithRefs(rep);w.WS()->ComputeGraph(true);return w.Write(argv[2])==IFSelect_RetDone?0:3;
 }
 STEPControl_Reader r;if(r.ReadFile(argv[2])!=IFSelect_RetDone)return 4;auto m=r.StepModel();occ::handle<StepGeom_SphericalSurface> s;for(int i=1;i<=m->NbEntities();++i){auto x=occ::down_cast<StepGeom_SphericalSurface>(m->Entity(i));if(!x.IsNull()){if(!s.IsNull())return 5;s=x;}}if(s.IsNull())return 6;auto g=StepToGeom::MakeSphericalSurface(s);if(g.IsNull())return 7;
 gp_Pnt p;gp_Vec du,dv;g->D1(.375,-1.25,p,du,dv);std::cout<<std::setprecision(17)<<"{\"Point\":["<<p.X()<<","<<p.Y()<<","<<p.Z()<<"],\"Du\":["<<du.X()<<","<<du.Y()<<","<<du.Z()<<"],\"Dv\":["<<dv.X()<<","<<dv.Y()<<","<<dv.Z()<<"]}\n";return 0;
}catch(const Standard_Failure&e){std::cerr<<e.what();return 8;}}
