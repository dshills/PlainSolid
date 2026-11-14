// Standalone pinned-OCCT oracle. No PlainSolid geometry or faceting is read.
// Build with the same include/lib and libraries as build.sh (geometry mode).
#include <BRepPrimAPI_MakeCone.hxx>
#include <BRepPrimAPI_MakeCylinder.hxx>
#include <BRepFilletAPI_MakeFillet.hxx>
#include <BRepAlgoAPI_Cut.hxx>
#include <BRepAdaptor_Curve.hxx>
#include <BRepCheck_Analyzer.hxx>
#include <BRepGProp.hxx>
#include <GProp_GProps.hxx>
#include <IntCurvesFace_ShapeIntersector.hxx>
#include <TopExp.hxx>
#include <TopExp_Explorer.hxx>
#include <NCollection_IndexedMap.hxx>
#include <TopTools_ShapeMapHasher.hxx>
using ShapeMap = NCollection_IndexedMap<TopoDS_Shape, TopTools_ShapeMapHasher>;
#include <TopoDS.hxx>
#include <gp_Lin.hxx>
#include <gp_Ax2.hxx>
#include <gp_Circ.hxx>
#include <Standard_Failure.hxx>
#include <fstream>
#include <iomanip>
#include <iostream>
#include <algorithm>
#include <vector>
#include <cmath>
#include <stdexcept>
static void record(std::ostream& out,const char* name,const TopoDS_Shape& shape){
 if(!BRepCheck_Analyzer(shape).IsValid())throw std::runtime_error("invalid independent OCCT shape");
 GProp_GProps v,a;BRepGProp::VolumeProperties(shape,v,1e-10);BRepGProp::SurfaceProperties(shape,a,1e-10);auto c=v.CentreOfMass();
 ShapeMap vertices,edges,faces;TopExp::MapShapes(shape,TopAbs_VERTEX,vertices);TopExp::MapShapes(shape,TopAbs_EDGE,edges);TopExp::MapShapes(shape,TopAbs_FACE,faces);
 int faceEuler=0;for(int i=1;i<=faces.Extent();++i){int wires=0;for(TopExp_Explorer w(faces(i),TopAbs_WIRE);w.More();w.Next())++wires;faceEuler+=2-wires;}
 int solids=0,shells=0;for(TopExp_Explorer x(shape,TopAbs_SOLID);x.More();x.Next())++solids;for(TopExp_Explorer x(shape,TopAbs_SHELL);x.More();x.Next())++shells;
 out<<std::setprecision(17)<<"{\"Name\":\""<<name<<"\",\"Volume\":"<<v.Mass()<<",\"Area\":"<<a.Mass()<<",\"Centroid\":["<<c.X()<<','<<c.Y()<<','<<c.Z()<<"],\"Shells\":"<<shells<<",\"Solids\":"<<solids<<",\"Euler\":"<<vertices.Extent()-edges.Extent()+faceEuler<<"}\n";
}
static void line(std::ostream& out,const char* name,const TopoDS_Shape& shape,gp_Pnt origin,gp_Vec direction,double lo,double hi){
 const double scale=direction.Magnitude();IntCurvesFace_ShapeIntersector query;query.Load(shape,1e-9);query.Perform(gp_Lin(origin,gp_Dir(direction)),lo*scale,hi*scale);if(!query.IsDone())throw std::runtime_error("OCCT line intersection failed");
 std::vector<double> values;for(int i=1;i<=query.NbPnt();++i)values.push_back(query.WParameter(i)/scale);std::sort(values.begin(),values.end());values.erase(std::unique(values.begin(),values.end(),[](double a,double b){return std::abs(a-b)<1e-8;}),values.end());
 out<<std::setprecision(17)<<"{\"Name\":\""<<name<<"\",\"Parameters\":[";for(size_t i=0;i<values.size();++i){if(i)out<<',';out<<values[i];}out<<"]}\n";
}
int main(int argc,char**argv){try{
 if(argc!=2)throw std::runtime_error("usage: conical_mixed_fillets OUTPUT_JSONL");std::ofstream out(argv[1]);out.exceptions(std::ios::failbit|std::ios::badbit);
 auto cone=BRepPrimAPI_MakeCone(8,16,6).Shape();BRepFilletAPI_MakeFillet fillets(cone);ShapeMap edges;TopExp::MapShapes(cone,TopAbs_EDGE,edges);int selected=0;
 for(int i=1;i<=edges.Extent();++i){auto edge=TopoDS::Edge(edges(i));BRepAdaptor_Curve curve(edge);if(curve.GetType()!=GeomAbs_Circle)continue;const double z=curve.Circle().Location().Z();if(std::abs(z)<1e-12){fillets.Add(15./16,edge);++selected;}else if(std::abs(z-6)<1e-12){fillets.Add(15./8,edge);++selected;}}
 if(selected!=2)throw std::runtime_error("missing independent conical rims");fillets.Build();if(!fillets.IsDone())throw std::runtime_error("OCCT conical rolling fillets failed");auto stock=fillets.Shape();record(out,"stock",stock);
 auto tool=[](double x,double z,double height){return BRepPrimAPI_MakeCylinder(gp_Ax2(gp_Pnt(x,0,z),gp_Dir(0,0,1)),.5,height).Shape();};
 auto body=BRepAlgoAPI_Cut(stock,tool(-2,-1,8)).Shape();body=BRepAlgoAPI_Cut(body,tool(2,0,1)).Shape();body=BRepAlgoAPI_Cut(body,tool(2,2,4)).Shape();record(out,"mixed",body);
 line(out,"oblique",body,gp_Pnt(837./104,0,15./208),gp_Vec(1,1,1),-1,8);
 line(out,"mixed-zones",body,gp_Pnt(-4,0,.5),gp_Vec(1,0,.25),-3,20);
 line(out,"mixed-reverse",body,gp_Pnt(-4,0,.5),gp_Vec(-1,0,-.25),-20,3);
 return 0;
 }catch(const Standard_Failure&e){std::cerr<<e.GetMessageString()<<'\n';return 1;}catch(const std::exception&e){std::cerr<<e.what()<<'\n';return 1;}}
