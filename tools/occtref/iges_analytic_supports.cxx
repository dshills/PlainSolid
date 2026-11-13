// Independent IGES mixed analytic entity author/evaluator. No BRep transfer claim.
#include <IGESControl_Writer.hxx>
#include <IGESControl_Reader.hxx>
#include <IGESData_IGESModel.hxx>
#include <IGESSolid_PlaneSurface.hxx>
#include <IGESSolid_CylindricalSurface.hxx>
#include <IGESSolid_SphericalSurface.hxx>
#include <IGESSolid_ToroidalSurface.hxx>
#include <IGESGeom_Point.hxx>
#include <IGESBasic_SubfigureDef.hxx>
#include <IGESGeom_Direction.hxx>
#include <IGESToBRep_BasicSurface.hxx>
#include <Geom_Surface.hxx>
#include <Standard_Failure.hxx>
#include <iostream>
#include <iomanip>
#include <string>
int main(int argc,char**argv){if(argc!=3)return 2;try{
 std::string mode=argv[1];if(mode=="write"||mode=="write-negative"){
  IGESControl_Writer writer("MM",0);gp_XYZ centers[]={gp_XYZ(2,3,4),gp_XYZ(0,1,2),gp_XYZ(-2,1,4),gp_XYZ(3,4,5)};
  for(int i=0;i<4;++i){occ::handle<IGESGeom_Point> p=new IGESGeom_Point;p->Init(centers[i],nullptr);occ::handle<IGESGeom_Direction> axis=new IGESGeom_Direction,ref=new IGESGeom_Direction;axis->Init(gp_XYZ(mode=="write"?1:-1,0,0));ref->Init(gp_XYZ(0,0,-1));occ::handle<IGESData_IGESEntity> root;
   if(i==0){occ::handle<IGESSolid_ToroidalSurface> s=new IGESSolid_ToroidalSurface;s->Init(p,axis,4.,1.25,ref);root=s;}else if(i==1){occ::handle<IGESSolid_PlaneSurface> s=new IGESSolid_PlaneSurface;s->Init(p,axis,ref);root=s;}else if(i==2){occ::handle<IGESSolid_SphericalSurface> s=new IGESSolid_SphericalSurface;s->Init(p,2.5,axis,ref);root=s;}else{occ::handle<IGESSolid_CylindricalSurface> s=new IGESSolid_CylindricalSurface;s->Init(p,axis,1.5,ref);root=s;}if(!writer.AddEntity(root))return 3;
  }return writer.Write(argv[2])?0:4;
 }
 IGESControl_Reader r;if(r.ReadFile(argv[2])!=IFSelect_RetDone)return 5;auto m=r.IGESModel();IGESToBRep_BasicSurface transfer;transfer.SetModel(m);int count=0;std::cout<<std::setprecision(17)<<"{\"Samples\":[";for(int i=1;i<=m->NbEntities();++i){auto e=m->Entity(i);int type=e->TypeNumber();if(type!=190&&type!=192&&type!=196&&type!=198)continue;auto g=transfer.TransferBasicSurface(e);if(g.IsNull())return 6;gp_Pnt p;gp_Vec du,dv;g->D1(.375,-1.25,p,du,dv);if(count++)std::cout<<",";std::cout<<"{\"Point\":["<<p.X()<<","<<p.Y()<<","<<p.Z()<<"],\"Du\":["<<du.X()<<","<<du.Y()<<","<<du.Z()<<"],\"Dv\":["<<dv.X()<<","<<dv.Y()<<","<<dv.Z()<<"]}";}std::cout<<"]}\n";return count==4?0:7;
}catch(const Standard_Failure&e){std::cerr<<e.what();return 8;}}
