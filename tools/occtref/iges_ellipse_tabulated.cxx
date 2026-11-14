#include <IGESControl_Writer.hxx>
#include <IGESControl_Reader.hxx>
#include <IGESGeom_ConicArc.hxx>
#include <IGESGeom_CompositeCurve.hxx>
#include <IGESGeom_TabulatedCylinder.hxx>
#include <IGESGeom_TransformationMatrix.hxx>
#include <NCollection_HArray1.hxx>
#include <NCollection_HArray2.hxx>
#include <BRepCheck_Analyzer.hxx>
#include <BRepGProp.hxx>
#include <GProp_GProps.hxx>
#include <TopExp_Explorer.hxx>
#include <TopoDS_Shape.hxx>
#include <iostream>
#include <iomanip>
int main(int argc,char**argv){if(argc!=3)return 2;try{if(std::string(argv[1])=="write"){
 auto a=new NCollection_HArray2<double>(1,3,1,4);double v[3][4]={{0,0,1,1},{1,0,0,2},{0,1,0,3}};for(int i=1;i<=3;i++)for(int j=1;j<=4;j++)a->SetValue(i,j,v[i-1][j-1]);occ::handle<IGESGeom_TransformationMatrix> tr=new IGESGeom_TransformationMatrix;tr->Init(a);
 occ::handle<NCollection_HArray1<occ::handle<IGESData_IGESEntity>>> members=new NCollection_HArray1<occ::handle<IGESData_IGESEntity>>(1,4);gp_XY p[4]={{5,0},{0,2},{-5,0},{0,-2}};for(int i=0;i<4;i++){occ::handle<IGESGeom_ConicArc> c=new IGESGeom_ConicArc;c->Init(4,0,25,0,0,-100,0,p[i],p[(i+1)%4]);c->OwnCorrect();c->InitTransf(tr);c->InitStatus(0,1,0,0);members->SetValue(i+1,c);}occ::handle<IGESGeom_CompositeCurve> c=new IGESGeom_CompositeCurve;c->Init(members);c->InitStatus(0,1,0,0);occ::handle<IGESGeom_TabulatedCylinder> surf=new IGESGeom_TabulatedCylinder;surf->Init(c,gp_XYZ(4,7,3));IGESControl_Writer w("MM",0);if(!w.AddEntity(surf))return 3;return w.Write(argv[2])?0:4;}
 IGESControl_Reader r;if(r.ReadFile(argv[2])!=IFSelect_RetDone||!r.TransferRoots())return 5;auto s=r.OneShape();if(s.IsNull()||!BRepCheck_Analyzer(s).IsValid())return 6;int n=0;for(TopExp_Explorer e(s,TopAbs_FACE);e.More();e.Next())n++;GProp_GProps props;BRepGProp::SurfaceProperties(s,props);std::cout<<std::setprecision(17)<<"{\"Faces\":"<<n<<",\"Area\":"<<props.Mass()<<"}\n";return 0;
}catch(const Standard_Failure&e){std::cerr<<e.what();return 9;}}
