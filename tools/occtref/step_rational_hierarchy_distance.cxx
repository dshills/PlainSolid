// Independent original STEP boundary/material distances; tolerances are oracle-only.
#include <STEPControl_Reader.hxx>
#include <BRepExtrema_DistShapeShape.hxx>
#include <BRepBuilderAPI_MakeVertex.hxx>
#include <BRep_Builder.hxx>
#include <BRepClass3d_SolidClassifier.hxx>
#include <BRepGProp.hxx>
#include <GProp_GProps.hxx>
#include <TopExp_Explorer.hxx>
#include <TopoDS.hxx>
#include <TopoDS_Compound.hxx>
#include <iostream>
#include <iomanip>
#include <cstdlib>
#include <vector>
int main(int argc,char**argv){if(argc!=5)return 2;STEPControl_Reader r;if(r.ReadFile(argv[1])!=IFSelect_RetDone||r.TransferRoots()<1)return 3;
 gp_Pnt point(std::strtod(argv[2],nullptr),std::strtod(argv[3],nullptr),std::strtod(argv[4],nullptr));auto vertex=BRepBuilderAPI_MakeVertex(point).Vertex();bool first=true;std::cout<<std::setprecision(17)<<"[";
 for(TopExp_Explorer ex(r.OneShape(),TopAbs_SOLID);ex.More();ex.Next()){
  auto solid=TopoDS::Solid(ex.Current());TopoDS_Compound boundary;BRep_Builder builder;builder.MakeCompound(boundary);for(TopExp_Explorer f(solid,TopAbs_FACE);f.More();f.Next())builder.Add(boundary,f.Current());
  BRepExtrema_DistShapeShape distance(vertex,boundary,1e-10);if(!distance.IsDone())return 4;GProp_GProps props;BRepGProp::VolumeProperties(solid,props,1e-12,true,false);BRepClass3d_SolidClassifier classifier(solid,point,1e-8);double material=classifier.State()==TopAbs_IN?0:distance.Value();
  std::vector<gp_Pnt> points;for(int i=1;i<=distance.NbSolution();i++){auto p=distance.PointOnShape2(i);bool seen=false;for(auto q:points)if(p.Distance(q)<1e-7)seen=true;if(!seen)points.push_back(p);}
  if(!first)std::cout<<",";first=false;std::cout<<"{\"Volume\":"<<props.Mass()<<",\"BoundaryDistance\":"<<distance.Value()<<",\"MaterialDistance\":"<<material<<",\"Points\":[";bool fp=true;for(auto p:points){if(!fp)std::cout<<",";fp=false;std::cout<<"["<<p.X()<<","<<p.Y()<<","<<p.Z()<<"]";}std::cout<<"]}";
 }std::cout<<"]\n";
}
