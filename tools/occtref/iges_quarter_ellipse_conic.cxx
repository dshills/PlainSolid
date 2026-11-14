// Independent IGES104 conic entity and evaluator; no topology claim.
#include <IGESControl_Writer.hxx>
#include <IGESControl_Reader.hxx>
#include <IGESData_IGESModel.hxx>
#include <IGESGeom_ConicArc.hxx>
#include <IGESGeom_TransformationMatrix.hxx>
#include <IGESToBRep_BasicCurve.hxx>
#include <NCollection_HArray2.hxx>
#include <Geom_Curve.hxx>
#include <Standard_Failure.hxx>
#include <iostream>
#include <iomanip>
#include <string>
int main(int argc,char**argv){if(argc!=3)return 2;try{
 if(std::string(argv[1])=="write"||std::string(argv[1])=="write-reverse"){
  occ::handle<IGESGeom_ConicArc> c=new IGESGeom_ConicArc;c->Init(4,0,25,0,0,-100,0,std::string(argv[1])=="write-reverse"?gp_XY(0,-2):gp_XY(5,0),std::string(argv[1])=="write-reverse"?gp_XY(5,0):gp_XY(0,2));c->OwnCorrect();
  occ::handle<NCollection_HArray2<double>> a=new NCollection_HArray2<double>(1,3,1,4);double v[3][4]={{0,0,1,1},{1,0,0,2},{0,1,0,3}};if(std::string(argv[1])=="write-reverse"){v[0][2]=-1;v[2][1]=-1;}for(int i=1;i<=3;i++)for(int j=1;j<=4;j++)a->SetValue(i,j,v[i-1][j-1]);
  occ::handle<IGESGeom_TransformationMatrix> tr=new IGESGeom_TransformationMatrix;tr->Init(a);c->InitTransf(tr);
  IGESControl_Writer w("MM",0);if(!w.AddEntity(c))return 3;return w.Write(argv[2])?0:4;
 }
 IGESControl_Reader r;if(r.ReadFile(argv[2])!=IFSelect_RetDone)return 5;auto m=r.IGESModel();occ::handle<IGESGeom_ConicArc> c;
 for(int i=1;i<=m->NbEntities();i++){auto x=occ::down_cast<IGESGeom_ConicArc>(m->Entity(i));if(!x.IsNull()){if(!c.IsNull())return 6;c=x;}}if(c.IsNull())return 7;
 IGESToBRep_BasicCurve transfer;transfer.SetModel(m);auto g=transfer.TransferConicArc(c);if(g.IsNull())return 8;gp_Pnt p;gp_Vec d;g->D1(g->FirstParameter()+.375,p,d);auto s=g->Value(g->FirstParameter());auto e=g->Value(g->LastParameter());
 std::cout<<std::setprecision(17)<<"{\"First\":"<<g->FirstParameter()<<",\"Last\":"<<g->LastParameter()<<",\"Point\":["<<p.X()<<","<<p.Y()<<","<<p.Z()<<"],\"Derivative\":["<<d.X()<<","<<d.Y()<<","<<d.Z()<<"],\"Start\":["<<s.X()<<","<<s.Y()<<","<<s.Z()<<"],\"End\":["<<e.X()<<","<<e.Y()<<","<<e.Z()<<"]}\n";return 0;
}catch(const Standard_Failure&e){std::cerr<<e.what();return 9;}}
