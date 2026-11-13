// Independent OCCT entity author and analytic support evaluator (no face claim).
#include <IGESControl_Writer.hxx>
#include <IGESControl_Reader.hxx>
#include <IGESData_IGESModel.hxx>
#include <IGESSolid_CylindricalSurface.hxx>
#include <IGESGeom_Point.hxx>
#include <IGESBasic_SubfigureDef.hxx>
#include <IGESGeom_Direction.hxx>
#include <IGESToBRep_BasicSurface.hxx>
#include <Geom_CylindricalSurface.hxx>
#include <Standard_Failure.hxx>
#include <iostream>
#include <iomanip>
#include <string>
int main(int argc,char**argv){if(argc!=3)return 2;try{
 std::string mode=argv[1];if(mode=="write"||mode=="write-negative"){
  occ::handle<IGESGeom_Point> p=new IGESGeom_Point;p->Init(gp_XYZ(2,3,4),nullptr);
  occ::handle<IGESGeom_Direction> axis=new IGESGeom_Direction,ref=new IGESGeom_Direction;
  axis->Init(gp_XYZ(mode=="write"?1:-1,0,0));ref->Init(gp_XYZ(0,0,-1));
  occ::handle<IGESSolid_CylindricalSurface> s=new IGESSolid_CylindricalSurface;s->Init(p,axis,2.5,ref);
  IGESControl_Writer w("MM",0);if(!w.AddEntity(s))return 3;return w.Write(argv[2])?0:4;
 }
 IGESControl_Reader r;if(r.ReadFile(argv[2])!=IFSelect_RetDone)return 5;
 auto m=r.IGESModel();occ::handle<IGESSolid_CylindricalSurface> s;
 for(int i=1;i<=m->NbEntities();++i){auto x=occ::down_cast<IGESSolid_CylindricalSurface>(m->Entity(i));if(!x.IsNull()){if(!s.IsNull())return 6;s=x;}}
 if(s.IsNull()||!s->IsParametrised())return 7;
 IGESToBRep_BasicSurface transfer;transfer.SetModel(m);auto g=transfer.TransferRigthCylindricalSurface(s);if(g.IsNull())return 8;
 gp_Pnt p;gp_Vec du,dv;g->D1(.375,-1.25,p,du,dv);
 std::cout<<std::setprecision(17)<<"{\"Point\":["<<p.X()<<","<<p.Y()<<","<<p.Z()<<"],\"Du\":["<<du.X()<<","<<du.Y()<<","<<du.Z()<<"],\"Dv\":["<<dv.X()<<","<<dv.Y()<<","<<dv.Z()<<"]}\n";return 0;
}catch(const Standard_Failure&e){std::cerr<<e.what();return 9;}}
