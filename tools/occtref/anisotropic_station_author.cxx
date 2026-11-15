// Independent anisotropic weighted original supports and topology; no Go geometry input.
#include <Geom_BSplineSurface.hxx>
#include <BRepBuilderAPI_MakeFace.hxx>
#include <BRepBuilderAPI_MakeWire.hxx>
#include <BRepBuilderAPI_MakeSolid.hxx>
#include <BRepBuilderAPI_Sewing.hxx>

#include <BRep_Tool.hxx>
#include <BRepAlgoAPI_Cut.hxx>
#include <BRepCheck_Analyzer.hxx>
#include <BRepGProp.hxx>
#include <GProp_GProps.hxx>
#include <STEPControl_Writer.hxx>
#include <STEPControl_Reader.hxx>
#include <TopExp_Explorer.hxx>
#include <TopoDS.hxx>
#include <TopoDS_Solid.hxx>
#include <TopoDS_Shell.hxx>
#include <gp_Pln.hxx>
#include <fstream>
#include <iomanip>
#include <stdexcept>
#include <iostream>
#include <string>

static TopoDS_Solid loft(bool inner=false) {
 const double xy[9][2]={{1,0},{1,1},{0,1},{-1,1},{-1,0},{-1,-1},{0,-1},{1,-1},{1,0}};
 const double wu[9]={1,.5,1,.75,1,.75,1,.75,1};
 const double oz[3]={0,1,3},ox[3]={3,1,2},oy[3]={1,2,1},iz[3]={0,2,3},ix[3]={.5,.25,.375},iy[3]={.25,.5,.25};
 const double *z=inner?iz:oz,*rx=inner?ix:ox,*ry=inner?iy:oy;
 const double cuts[5]={-2,-1,1,4,8};
 BRepBuilderAPI_Sewing sewing(1e-10);
 for(int band=0;band<2;++band) {
  NCollection_Array2<gp_Pnt> poles(1,9,1,2);NCollection_Array2<double> weights(1,9,1,2);
  for(int u=0;u<9;++u)for(int v=0;v<2;++v){poles.SetValue(u+1,v+1,gp_Pnt(.25+rx[band+v]*xy[u][0],-.5+ry[band+v]*xy[u][1],z[band+v]));weights.SetValue(u+1,v+1,wu[u]);}
  NCollection_Array1<double> uk(1,5),vk(1,2);NCollection_Array1<int> um(1,5),vm(1,2);
  for(int i=1;i<=5;++i){uk.SetValue(i,cuts[i-1]);um.SetValue(i,i==1||i==5?3:2);}vk.SetValue(1,0);vk.SetValue(2,1);vm.SetValue(1,2);vm.SetValue(2,2);
  occ::handle<Geom_BSplineSurface> surface=new Geom_BSplineSurface(poles,weights,uk,vk,um,vm,2,1);
  auto side=BRepBuilderAPI_MakeFace(surface,1e-10).Face();sewing.Add(side);
  for(TopExp_Explorer ex(side,TopAbs_EDGE);ex.More();ex.Next()) {
   auto edge=TopoDS::Edge(ex.Current());double lo,hi;auto curve=BRep_Tool::Curve(edge,lo,hi);if(curve.IsNull())continue;
   auto a=curve->Value(lo),b=curve->Value(hi),mid=curve->Value((lo+hi)/2);
   if(a.Z()!=b.Z()||a.Z()!=mid.Z()||(a.Z()!=0&&a.Z()!=3))continue;
   edge.Orientation(TopAbs_FORWARD);
   auto cap=BRepBuilderAPI_MakeFace(gp_Pln(gp_Pnt(0,0,a.Z()),gp_Dir(0,0,1)),BRepBuilderAPI_MakeWire(edge).Wire(),true).Face();
   if(a.Z()==0)cap.Reverse();sewing.Add(cap);
  }
 }
 sewing.Perform();auto result=BRepBuilderAPI_MakeSolid(TopoDS::Shell(sewing.SewedShape())).Solid();
 if(!BRepCheck_Analyzer(result).IsValid())throw std::runtime_error("invalid original weighted loft");return result;
}
#include <STEPCAFControl_Writer.hxx>
#include <STEPCAFControl_Reader.hxx>
#include <DESTEP_Parameters.hxx>
#include <TDocStd_Document.hxx>
#include <TDataStd_Name.hxx>
#include <XCAFDoc_DocumentTool.hxx>
#include <XCAFDoc_ShapeTool.hxx>
#include <NCollection_Sequence.hxx>
int main(int argc,char**argv) {
 try {
  if(argc!=4)return 2;
  const std::string mode=argv[3];
  const bool assembly=mode=="filled-assembly"||mode=="filled-assembly-negative"||mode=="bore-assembly"||mode=="read-assembly"||mode=="filled-nested-assembly"||mode=="bore-nested-assembly"||mode=="read-nested-assembly";
  const bool reading=mode=="read"||mode=="read-assembly"||mode=="read-nested-assembly";
  const bool nested=mode.find("nested")!=std::string::npos;
  if(mode!="filled"&&mode!="bore"&&!assembly&&!reading)return 2;
  TopoDS_Shape shape;
  if(!reading){shape=loft();if(mode=="bore"||mode=="bore-assembly"||mode=="bore-nested-assembly")shape=BRepAlgoAPI_Cut(shape,loft(true)).Shape();}
  else if(!assembly){STEPControl_Reader reader;if(reader.ReadFile(argv[1])!=IFSelect_RetDone||reader.TransferRoots()!=1)return 4;shape=reader.OneShape();}
  if(assembly){
   occ::handle<TDocStd_Document> doc=new TDocStd_Document(TCollection_ExtendedString("BinXCAF"));
   auto shapes=XCAFDoc_DocumentTool::ShapeTool(doc->Main());
   if(!reading){
    auto part=shapes->AddShape(shape,false),root=shapes->NewShape();
    TDataStd_Name::Set(part,TCollection_ExtendedString("Original anisotropic product"));TDataStd_Name::Set(root,TCollection_ExtendedString("Repeated anisotropic assembly"));
    auto target=part;
    if(nested){target=shapes->NewShape();TDataStd_Name::Set(target,TCollection_ExtendedString("Shared anisotropic subgroup"));gp_Trsf inner;inner.SetTranslation(gp_Vec(1,2,0));shapes->AddComponent(target,part,TopLoc_Location(inner));}
    shapes->AddComponent(root,target,TopLoc_Location());gp_Trsf pose;pose.SetValues(0,0,1,10,1,0,0,-2,0,1,0,3);if(mode=="filled-assembly-negative")pose.SetValues(-1,0,0,10,0,1,0,-2,0,0,-1,3);shapes->AddComponent(root,target,TopLoc_Location(pose));shapes->UpdateAssemblies();
    STEPCAFControl_Writer writer;DESTEP_Parameters params;params.WriteSchema=DESTEP_Parameters::WriteMode_StepSchema_AP242DIS;
    if(!writer.Transfer(doc,params)||writer.Write(argv[1])!=IFSelect_RetDone)return 5;
   }else{STEPCAFControl_Reader reader;if(reader.ReadFile(argv[1])!=IFSelect_RetDone||!reader.Transfer(doc))return 4;}
   NCollection_Sequence<TDF_Label> roots,uses;shapes->GetFreeShapes(roots);if(roots.Length()!=1)return 6;
   shapes->GetComponents(roots.Value(1),uses,false);if(uses.Length()!=2)return 6;TDF_Label first;
   for(int i=1;i<=2;i++){TDF_Label part;if(!shapes->GetReferredShape(uses.Value(i),part))return 6;if(i==1)first=part;else if(first!=part)return 6;}
   if(nested){NCollection_Sequence<TDF_Label> inner;shapes->GetComponents(first,inner,false);if(inner.Length()!=1)return 6;TDF_Label leaf;if(!shapes->GetReferredShape(inner.Value(1),leaf)||leaf==first)return 6;}
   shape=shapes->GetShape(roots.Value(1));
  }
  if(shape.IsNull()||!BRepCheck_Analyzer(shape).IsValid())throw std::runtime_error("invalid independent weighted result");
  int solids=0,shells=0;for(TopExp_Explorer e(shape,TopAbs_SOLID);e.More();e.Next())++solids;for(TopExp_Explorer e(shape,TopAbs_SHELL);e.More();e.Next())++shells;
  if(solids!=(assembly?2:1)||shells!=solids)throw std::runtime_error("unexpected component count");
  if(!reading&&!assembly){STEPControl_Writer writer;if(writer.Transfer(shape,STEPControl_AsIs)!=IFSelect_RetDone||writer.Write(argv[1])!=IFSelect_RetDone)throw std::runtime_error("STEP export failed");}
  GProp_GProps volume,area;BRepGProp::VolumeProperties(shape,volume,1e-12);BRepGProp::SurfaceProperties(shape,area,1e-12);
  auto c=volume.CentreOfMass();std::ofstream out(argv[2]);out<<std::setprecision(17)<<"{\"Volume\":"<<volume.Mass()<<",\"Area\":"<<area.Mass()<<",\"Centroid\":["<<c.X()<<","<<c.Y()<<","<<c.Z()<<"],\"Solids\":"<<solids<<",\"Shells\":"<<shells<<",\"Shared\":"<<(assembly?"true":"false")<<"}\n";
  return out?0:3;
 }catch(const std::exception&e){std::cerr<<e.what()<<"\n";return 1;}
}
