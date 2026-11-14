// Independent original weighted supports and topology; never consumes Go output.
#include <Geom_BSplineSurface.hxx>
#include <BRepBuilderAPI_MakeFace.hxx>
#include <BRepBuilderAPI_MakeWire.hxx>
#include <BRepBuilderAPI_MakeSolid.hxx>
#include <BRepBuilderAPI_Sewing.hxx>
#include <BRepAlgoAPI_Cut.hxx>
#include <BRep_Tool.hxx>
#include <BRepCheck_Analyzer.hxx>
#include <BRepGProp.hxx>
#include <GProp_GProps.hxx>
#include <STEPControl_Writer.hxx>
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

static TopoDS_Solid loft(bool inner) {
 const double xy[9][2]={{1,0},{1,1},{0,1},{-1,1},{-1,0},{-1,-1},{0,-1},{1,-1},{1,0}};
 const double wu[9]={1,.5,1,.75,1,.75,1,.75,1};
 const double outerZ[3]={0,1,3},innerZ[3]={0,2,3},outerR[3]={3,1,2},innerR[3]={.5,.75,.5};
 const auto z=inner?innerZ:outerZ;const auto r=inner?innerR:outerR;
 const double cuts[5]={-2,-1,1,4,8};
 BRepBuilderAPI_Sewing sewing(1e-10);
 for(int band=0;band<2;++band) {
  NCollection_Array2<gp_Pnt> poles(1,9,1,2);NCollection_Array2<double> weights(1,9,1,2);
  for(int u=0;u<9;++u)for(int v=0;v<2;++v){poles.SetValue(u+1,v+1,gp_Pnt(.25+r[band+v]*xy[u][0],-.5+r[band+v]*xy[u][1],z[band+v]));weights.SetValue(u+1,v+1,wu[u]);}
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
#include <TDocStd_Document.hxx>
#include <TDataStd_Name.hxx>
#include <XCAFDoc_DocumentTool.hxx>
#include <XCAFDoc_ShapeTool.hxx>
#include <NCollection_Sequence.hxx>
int main(int argc,char**argv){try{
 if(argc!=3)return 2;
 occ::handle<TDocStd_Document> doc=new TDocStd_Document(TCollection_ExtendedString("BinXCAF"));
 auto shapes=XCAFDoc_DocumentTool::ShapeTool(doc->Main());
 if(std::string(argv[1])=="write"||std::string(argv[1])=="write-bore"){
 TopoDS_Shape local=loft(false);if(std::string(argv[1])=="write-bore"){BRepAlgoAPI_Cut cut(local,loft(true));cut.Build();if(!cut.IsDone())return 7;local=cut.Shape();}
 auto part=shapes->AddShape(local,false);auto root=shapes->NewShape();
 TDataStd_Name::Set(part,TCollection_ExtendedString("Original weighted station product"));TDataStd_Name::Set(root,TCollection_ExtendedString("Repeated curved AP242 product"));
 gp_Trsf a;a.SetTranslation(gp_Vec(10,0,0));auto u=shapes->AddComponent(root,part,TopLoc_Location(a));TDataStd_Name::Set(u,TCollection_ExtendedString("Translated weighted use"));
 gp_Trsf b;b.SetValues(0,0,1,0,1,0,0,20,0,1,0,0);u=shapes->AddComponent(root,part,TopLoc_Location(b));TDataStd_Name::Set(u,TCollection_ExtendedString("Cyclic weighted use"));shapes->UpdateAssemblies();
 STEPCAFControl_Writer writer;DESTEP_Parameters params;params.WriteSchema=DESTEP_Parameters::WriteMode_StepSchema_AP242DIS;
 if(!writer.Transfer(doc,params)||writer.Write(argv[2])!=IFSelect_RetDone)return 3;return 0;
 }
 if(std::string(argv[1])!="read")return 2;
 STEPCAFControl_Reader reader;if(reader.ReadFile(argv[2])!=IFSelect_RetDone || !reader.Transfer(doc))return 4;
 NCollection_Sequence<TDF_Label> roots;shapes->GetFreeShapes(roots);if(roots.Length()!=1)return 5;
 auto shape=shapes->GetShape(roots.Value(1));if(!BRepCheck_Analyzer(shape).IsValid())return 6;
 GProp_GProps v,a;BRepGProp::VolumeProperties(shape,v,1e-12);BRepGProp::SurfaceProperties(shape,a,1e-12);auto center=v.CentreOfMass();int solids=0,faces=0;for(TopExp_Explorer ex(shape,TopAbs_SOLID);ex.More();ex.Next())solids++;for(TopExp_Explorer ex(shape,TopAbs_FACE);ex.More();ex.Next())faces++;
 std::cout<<std::setprecision(17)<<"{\"volume\":"<<v.Mass()<<",\"area\":"<<a.Mass()<<",\"centroid\":["<<center.X()<<","<<center.Y()<<","<<center.Z()<<"],\"solids\":"<<solids<<",\"faces\":"<<faces<<",\"occurrences\":[";
 NCollection_Sequence<TDF_Label> uses;shapes->GetComponents(roots.Value(1),uses,false);TDF_Label original;for(int i=1;i<=uses.Length();i++){TDF_Label part;if(!shapes->GetReferredShape(uses.Value(i),part))return 7;if(i==1)original=part;else if(part!=original)return 8;auto pose=shapes->GetShape(uses.Value(i)).Location().Transformation();if(i>1)std::cout<<",";std::cout<<"[";for(int r=1;r<=3;r++)for(int c=1;c<=4;c++){if(r!=1||c!=1)std::cout<<",";std::cout<<pose.Value(r,c);}std::cout<<"]";}
 std::cout<<"],\"shared\":true}\n";return 0;
 }catch(const std::exception&e){std::cerr<<e.what()<<"\n";return 1;}}
