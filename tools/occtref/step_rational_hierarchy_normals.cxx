// Independent finite-face normals of transferred STEP. Tolerance is oracle-only.
#include <STEPControl_Reader.hxx>
#include <BRep_Tool.hxx>
#include <BRepClass_FaceClassifier.hxx>
#include <GeomAPI_ProjectPointOnSurf.hxx>
#include <GeomLProp_SLProps.hxx>
#include <GeomAdaptor_Surface.hxx>
#include <TopExp_Explorer.hxx>
#include <TopoDS.hxx>
#include <TopLoc_Location.hxx>
#include <gp_Sphere.hxx>
#include <iostream>
#include <iomanip>
#include <cstdlib>
int main(int argc,char**argv){
 if(argc!=5)return 2;STEPControl_Reader reader;if(reader.ReadFile(argv[1])!=IFSelect_RetDone||reader.TransferRoots()<1)return 3;
 gp_Pnt world(std::strtod(argv[2],nullptr),std::strtod(argv[3],nullptr),std::strtod(argv[4],nullptr));
 std::cout<<std::setprecision(17)<<"[";bool first=true;
 for(TopExp_Explorer ex(reader.OneShape(),TopAbs_FACE);ex.More();ex.Next()){
  auto face=TopoDS::Face(ex.Current());TopLoc_Location location;auto surface=BRep_Tool::Surface(face,location);auto point=world.Transformed(location.Transformation().Inverted());
  GeomAPI_ProjectPointOnSurf projection(point,surface);if(projection.NbPoints()==0||projection.LowerDistance()>1e-7)continue;
  double u,v;projection.LowerDistanceParameters(u,v);bool contained=false;
  // The projection can select U=0 for a distinct face ending at U=2*pi.
  // Test periodic chart representatives independently, retaining each face once.
  for(int i=-1;i<=1&&!contained;i++)for(int j=-1;j<=1&&!contained;j++){
   if((i&&!surface->IsUPeriodic())||(j&&!surface->IsVPeriodic()))continue;
   double a=u+(i?i*surface->UPeriod():0),b=v+(j?j*surface->VPeriod():0);
   BRepClass_FaceClassifier classifier(face,gp_Pnt2d(a,b),1e-7);
   if(classifier.State()==TopAbs_IN||classifier.State()==TopAbs_ON){u=a;v=b;contained=true;}
  }
  if(!contained)continue;
  GeomLProp_SLProps props(surface,u,v,1,1e-10);gp_Vec normal;
  if(props.IsNormalDefined())normal=gp_Vec(props.Normal());else{GeomAdaptor_Surface adapter(surface);if(adapter.GetType()!=GeomAbs_Sphere)return 4;normal=gp_Vec(adapter.Sphere().Location(),point);}
  normal.Transform(location.Transformation());if(face.Orientation()==TopAbs_REVERSED)normal.Reverse();normal.Normalize();if(!first)std::cout<<",";first=false;std::cout<<"["<<normal.X()<<","<<normal.Y()<<","<<normal.Z()<<"]";
 }
 std::cout<<"]\n";
}
