#include <IGESControl_Writer.hxx>
#include <IGESControl_Reader.hxx>
#include <IGESGeom_ConicArc.hxx>
#include <IGESGeom_CurveOnSurface.hxx>
#include <IGESGeom_TrimmedSurface.hxx>
#include <NCollection_HArray1.hxx>
#include <IGESGeom_Point.hxx>
#include <IGESBasic_SubfigureDef.hxx>

#include <IGESGeom_Direction.hxx>
#include <IGESSolid_PlaneSurface.hxx>
#include <BRepCheck_Analyzer.hxx>
#include <BRepGProp.hxx>
#include <GProp_GProps.hxx>
#include <TopExp_Explorer.hxx>
#include <TopoDS_Shape.hxx>
#include <iostream>
#include <iomanip>
int main(int argc,char**argv){if(argc!=3)return 2;try{if(std::string(argv[1])=="write"){
 occ::handle<IGESGeom_Point> p=new IGESGeom_Point;p->Init(gp_XYZ(0,0,3),nullptr);occ::handle<IGESGeom_Direction> z=new IGESGeom_Direction,x=new IGESGeom_Direction;z->Init(gp_XYZ(0,0,1));x->Init(gp_XYZ(1,0,0));occ::handle<IGESSolid_PlaneSurface> plane=new IGESSolid_PlaneSurface;plane->Init(p,z,x);
 occ::handle<IGESGeom_ConicArc> uv=new IGESGeom_ConicArc,model=new IGESGeom_ConicArc;uv->Init(4,0,25,0,0,-100,0,gp_XY(5,0),gp_XY(5,0));model->Init(4,0,25,0,0,-100,3,gp_XY(5,0),gp_XY(5,0));uv->OwnCorrect();model->OwnCorrect();
 occ::handle<IGESGeom_CurveOnSurface> cos=new IGESGeom_CurveOnSurface;cos->Init(0,plane,uv,model,3);occ::handle<IGESGeom_TrimmedSurface> face=new IGESGeom_TrimmedSurface;occ::handle<IGESGeom_ConicArc> iu=new IGESGeom_ConicArc,im=new IGESGeom_ConicArc;iu->Init(1,0,4,0,0,-4,0,gp_XY(2,0),gp_XY(2,0));im->Init(1,0,4,0,0,-4,3,gp_XY(2,0),gp_XY(2,0));iu->OwnCorrect();im->OwnCorrect();iu->InitStatus(0,1,0,0);im->InitStatus(0,1,0,0);occ::handle<IGESGeom_CurveOnSurface> inner=new IGESGeom_CurveOnSurface;inner->Init(0,plane,iu,im,3);inner->InitStatus(0,1,0,0);occ::handle<NCollection_HArray1<occ::handle<IGESGeom_CurveOnSurface>>> holes=new NCollection_HArray1<occ::handle<IGESGeom_CurveOnSurface>>(1,1);holes->SetValue(1,inner);face->Init(plane,1,cos,holes);
 for(auto q:{occ::handle<IGESData_IGESEntity>(p),occ::handle<IGESData_IGESEntity>(z),occ::handle<IGESData_IGESEntity>(x),occ::handle<IGESData_IGESEntity>(plane),occ::handle<IGESData_IGESEntity>(uv),occ::handle<IGESData_IGESEntity>(model),occ::handle<IGESData_IGESEntity>(cos)})q->InitStatus(0,1,0,0);
 IGESControl_Writer w("MM",0);if(!w.AddEntity(face))return 3;return w.Write(argv[2])?0:4;}
 IGESControl_Reader r;if(r.ReadFile(argv[2])!=IFSelect_RetDone||!r.TransferRoots())return 5;auto s=r.OneShape();if(s.IsNull()||!BRepCheck_Analyzer(s).IsValid())return 6;int n=0;for(TopExp_Explorer e(s,TopAbs_FACE);e.More();e.Next())n++;GProp_GProps props;BRepGProp::SurfaceProperties(s,props);std::cout<<std::setprecision(17)<<"{\"Faces\":"<<n<<",\"Area\":"<<props.Mass()<<",\"CenterZ\":"<<props.CentreOfMass().Z()<<"}\n";return 0;
}catch(const Standard_Failure&e){std::cerr<<e.what();return 9;}}
