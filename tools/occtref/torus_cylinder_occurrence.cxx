#include <BRepPrimAPI_MakeTorus.hxx>
#include <BRepPrimAPI_MakeCylinder.hxx>
#include <BRepAlgoAPI_Common.hxx>
#include <BRepAlgoAPI_Cut.hxx>
#include <BRepAlgoAPI_Fuse.hxx>
#include <BRepCheck_Analyzer.hxx>
#include <BRepGProp.hxx>
#include <GProp_GProps.hxx>
#include <BRepAdaptor_Surface.hxx>
#include <BRepExtrema_DistShapeShape.hxx>
#include <BRepBuilderAPI_MakeVertex.hxx>
#include <BRep_Builder.hxx>
#include <TopoDS_Compound.hxx>
#include <IntCurvesFace_ShapeIntersector.hxx>
#include <BRepClass3d_SolidClassifier.hxx>
#include <TopoDS.hxx>
#include <TopExp_Explorer.hxx>
#include <gp_Lin.hxx>
#include <Standard_Failure.hxx>
#include <fstream>
#include <iomanip>
#include <iostream>
#include <algorithm>
#include <cmath>
#include <stdexcept>
#include <vector>
static gp_Pnt point(double x,double y,double z){return gp_Pnt(3*x/13-12*y/13+4*z/13+1./7,36*x/65+5*y/13+48*z/65-2./9,-4*x/5+3*z/5+3./11);}
static gp_Dir dir(double x,double y,double z){return gp_Dir(3*x/13-12*y/13+4*z/13,36*x/65+5*y/13+48*z/65,-4*x/5+3*z/5);}
static void writeMass(std::ostream& out,const GProp_GProps& mass){auto c=mass.CentreOfMass();auto m=mass.MatrixOfInertia();out<<",\"Volume\":"<<mass.Mass()<<",\"Center\":["<<c.X()<<','<<c.Y()<<','<<c.Z()<<"],\"Inertia\":[";for(int i=1;i<=3;i++)for(int j=1;j<=3;j++){if(i!=1||j!=1)out<<',';out<<m.Value(i,j);}out<<']';}
int main(int argc,char**argv){try{if(argc!=2)throw std::runtime_error("output path");std::ofstream out(argv[1]);out.exceptions(std::ios::failbit|std::ios::badbit);out<<std::setprecision(17);
 auto a=BRepPrimAPI_MakeTorus(gp_Ax2(point(0,0,0),dir(0,0,1),dir(1,0,0)),5,2).Shape();auto b=BRepPrimAPI_MakeCylinder(gp_Ax2(point(0,0,-3),dir(0,0,1),dir(1,0,0)),6,7).Shape();TopoDS_Shape bodies[3]={BRepAlgoAPI_Common(a,b).Shape(),BRepAlgoAPI_Cut(a,b).Shape(),BRepAlgoAPI_Fuse(a,b).Shape()};
 for(int i=0;i<3;i++){if(!BRepCheck_Analyzer(bodies[i]).IsValid())throw std::runtime_error("invalid");gp_Pnt foot;gp_Vec normal;bool found=false;for(TopExp_Explorer f(bodies[i],TopAbs_FACE);f.More();f.Next()){auto face=TopoDS::Face(f.Current());BRepAdaptor_Surface surface(face);if(surface.GetType()!=GeomAbs_Torus)continue;gp_Vec du,dv;surface.D1(0,i==0?std::acos(-1.):0,foot,du,dv);normal=du.Crossed(dv);if(face.Orientation()==TopAbs_REVERSED)normal.Reverse();normal.Normalize();found=true;break;}if(!found||foot.Distance(point(i==0?3:7,0,0))>1e-7)throw std::runtime_error("normal source point");
 BRep_Builder builder;TopoDS_Compound boundary;builder.MakeCompound(boundary);for(TopExp_Explorer face(bodies[i],TopAbs_FACE);face.More();face.Next())builder.Add(boundary,face.Current());BRepExtrema_DistShapeShape distance(BRepBuilderAPI_MakeVertex(point(4,1,1.5)).Shape(),boundary,1e-10);if(!distance.IsDone())throw std::runtime_error("world distance");
 IntCurvesFace_ShapeIntersector q;q.Load(bodies[i],1e-10);double speed=std::sqrt(1.01);q.Perform(gp_Lin(point(0,0,0),dir(1,0,.1)),-8*speed,8*speed);if(!q.IsDone())throw std::runtime_error("section");std::vector<double> roots;for(int j=1;j<=q.NbPnt();j++)roots.push_back(q.WParameter(j)/speed);std::sort(roots.begin(),roots.end());roots.erase(std::unique(roots.begin(),roots.end(),[](double x,double y){return std::abs(x-y)<1e-8;}),roots.end());if(roots.size()!=(i==2?2u:4u))throw std::runtime_error("roots");for(size_t j=0;j<roots.size();j+=2){double t=(roots[j]+roots[j+1])/2;BRepClass3d_SolidClassifier c(bodies[i],point(t,0,t/10),1e-10);if(c.State()!=TopAbs_IN)throw std::runtime_error("material midpoint");}
 GProp_GProps mass;BRepGProp::VolumeProperties(bodies[i],mass);out<<"{\"Operation\":"<<i;writeMass(out,mass);out<<",\"SquaredDistance\":"<<distance.Value()*distance.Value()<<",\"Normal\":["<<normal.X()<<','<<normal.Y()<<','<<normal.Z()<<"],\"Point\":["<<foot.X()<<','<<foot.Y()<<','<<foot.Z()<<"],\"Parameters\":[";for(size_t j=0;j<roots.size();j++){if(j)out<<',';out<<roots[j];}out<<"]}\n";
 }return 0;}catch(const Standard_Failure&e){std::cerr<<e.what()<<'\n';return 1;}catch(const std::exception&e){std::cerr<<e.what()<<'\n';return 1;}}
