// Independent analytic rounded triangle; no PlainSolid geometry input.
#include <BRepBuilderAPI_MakeEdge.hxx>
#include <BRepBuilderAPI_MakeWire.hxx>
#include <BRepBuilderAPI_MakeFace.hxx>
#include <BRepBuilderAPI_Transform.hxx>
#include <BRepPrimAPI_MakePrism.hxx>
#include <BRepCheck_Analyzer.hxx>
#include <BRepGProp.hxx>
#include <GProp_GProps.hxx>
#include <STEPControl_Reader.hxx>
#include <STEPCAFControl_Reader.hxx>
#include <STEPCAFControl_Writer.hxx>
#include <TDocStd_Document.hxx>
#include <TDataStd_Name.hxx>
#include <XCAFDoc_DocumentTool.hxx>
#include <XCAFDoc_ShapeTool.hxx>
#include <NCollection_Sequence.hxx>
#include <STEPControl_Writer.hxx>
#include <DESTEP_Parameters.hxx>
#include <TopExp.hxx>
#include <NCollection_IndexedMap.hxx>
#include <TopTools_ShapeMapHasher.hxx>
#include <gp_Circ.hxx>
#include <gp_Ax2.hxx>
#include <iostream>
#include <iomanip>
#include <string>
int main(int argc,char**argv){try{
 if(argc!=3)return 2;TopoDS_Shape shape;
 if(std::string(argv[1])=="write"||std::string(argv[1])=="write-negative"||std::string(argv[1])=="write-model-only"||std::string(argv[1])=="write-model-only-posed"||std::string(argv[1])=="write-assembly"||std::string(argv[1])=="write-nested-assembly"){
  const double r=.625;gp_Pnt p[6]={{0,-r,0},{4,-r,0},{4.375,.5,0},{.375,3.5,0},{-r,3,0},{-r,0,0}};gp_Pnt c[3]={{4,0,0},{0,3,0},{0,0,0}};
  BRepBuilderAPI_MakeWire wire;for(int i=0;i<6;i++){if(i%2==0)wire.Add(BRepBuilderAPI_MakeEdge(p[i],p[(i+1)%6]));else wire.Add(BRepBuilderAPI_MakeEdge(gp_Circ(gp_Ax2(c[i/2],gp_Dir(0,0,1)),r),p[i],p[(i+1)%6]));}
  auto face=BRepBuilderAPI_MakeFace(wire.Wire()).Face();bool negative=std::string(argv[1])=="write-negative"||std::string(argv[1])=="write-model-only-posed";shape=BRepPrimAPI_MakePrism(face,gp_Vec(0,0,negative?-2:2)).Shape();
  if(negative){gp_Trsf pose;pose.SetValues(0,0,1,1,1,0,0,-2,0,1,0,3);shape=BRepBuilderAPI_Transform(shape,pose,true).Shape();}
  if(std::string(argv[1])=="write-assembly"||std::string(argv[1])=="write-nested-assembly"){
   occ::handle<TDocStd_Document> doc=new TDocStd_Document(TCollection_ExtendedString("BinXCAF"));auto shapes=XCAFDoc_DocumentTool::ShapeTool(doc->Main());
   auto part=shapes->AddShape(shape,false),root=shapes->NewShape();TDataStd_Name::Set(part,TCollection_ExtendedString("Original rounded triangle"));TDataStd_Name::Set(root,TCollection_ExtendedString("Shared rounded products"));
   auto target=part;
   if(std::string(argv[1])=="write-nested-assembly"){target=shapes->NewShape();TDataStd_Name::Set(target,TCollection_ExtendedString("Reusable rounded subgroup"));gp_Trsf inner;inner.SetTranslation(gp_Vec(1,2,0));shapes->AddComponent(target,part,TopLoc_Location(inner));}
   shapes->AddComponent(root,target,TopLoc_Location());gp_Trsf pose;pose.SetValues(0,0,1,10,1,0,0,-2,0,1,0,3);shapes->AddComponent(root,target,TopLoc_Location(pose));shapes->UpdateAssemblies();
   STEPCAFControl_Writer writer;DESTEP_Parameters params;params.WriteSurfaceCurMode=false;
   if(!writer.Transfer(doc,params)||writer.Write(argv[2])!=IFSelect_RetDone)return 7;return 0;
  }
  if(!BRepCheck_Analyzer(shape).IsValid())return 3;STEPControl_Writer writer;DESTEP_Parameters params;params.WriteSurfaceCurMode=std::string(argv[1]).find("model-only")==std::string::npos;if(writer.Transfer(shape,STEPControl_AsIs,params)!=IFSelect_RetDone||writer.Write(argv[2])!=IFSelect_RetDone)return 4;
 }else if(std::string(argv[1])=="read-assembly"||std::string(argv[1])=="read-nested-assembly"){
  occ::handle<TDocStd_Document> doc=new TDocStd_Document(TCollection_ExtendedString("BinXCAF"));STEPCAFControl_Reader reader;if(reader.ReadFile(argv[2])!=IFSelect_RetDone||!reader.Transfer(doc))return 5;
  auto shapes=XCAFDoc_DocumentTool::ShapeTool(doc->Main());NCollection_Sequence<TDF_Label> roots,uses;shapes->GetFreeShapes(roots);if(roots.Length()!=1)return 7;
  shapes->GetComponents(roots.Value(1),uses,false);if(uses.Length()!=2)return 7;TDF_Label original;
  for(int i=1;i<=2;i++){TDF_Label part;if(!shapes->GetReferredShape(uses.Value(i),part))return 7;if(i==1)original=part;else if(part!=original)return 7;}
  if(std::string(argv[1])=="read-nested-assembly"){NCollection_Sequence<TDF_Label> inner;shapes->GetComponents(original,inner,false);if(inner.Length()!=1)return 7;TDF_Label leaf;if(!shapes->GetReferredShape(inner.Value(1),leaf)||leaf==original)return 7;}
  shape=shapes->GetShape(roots.Value(1));
 }else if(std::string(argv[1])=="read"){STEPControl_Reader reader;if(reader.ReadFile(argv[2])!=IFSelect_RetDone||reader.TransferRoots()!=1)return 5;shape=reader.OneShape();}else return 2;
 if(shape.IsNull()||!BRepCheck_Analyzer(shape).IsValid())return 6;
 GProp_GProps v,a;BRepGProp::VolumeProperties(shape,v,1e-12);BRepGProp::SurfaceProperties(shape,a,1e-12);NCollection_IndexedMap<TopoDS_Shape,TopTools_ShapeMapHasher> faces,edges,vertices,solids;TopExp::MapShapes(shape,TopAbs_FACE,faces);TopExp::MapShapes(shape,TopAbs_EDGE,edges);TopExp::MapShapes(shape,TopAbs_VERTEX,vertices);TopExp::MapShapes(shape,TopAbs_SOLID,solids);auto c=v.CentreOfMass();
 std::cout<<std::setprecision(17)<<"{\"volume\":"<<v.Mass()<<",\"area\":"<<a.Mass()<<",\"centroid\":["<<c.X()<<","<<c.Y()<<","<<c.Z()<<"],\"solids\":"<<solids.Extent()<<",\"faces\":"<<faces.Extent()<<",\"edges\":"<<edges.Extent()<<",\"vertices\":"<<vertices.Extent()<<"}\n";return 0;
 }catch(...){return 9;}}
