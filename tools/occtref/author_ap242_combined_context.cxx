// Independent XCAF source author and style/geometry transfer observer.
#include <STEPCAFControl_Writer.hxx>
#include <STEPCAFControl_Reader.hxx>
#include <TDocStd_Document.hxx>
#include <TDataStd_Name.hxx>
#include <TDF_ChildIterator.hxx>
#include <NCollection_Sequence.hxx>
#include <XCAFDoc_DocumentTool.hxx>
#include <XCAFDoc_ShapeTool.hxx>
#include <XCAFDoc_ColorTool.hxx>
#include <XCAFDoc_GraphNode.hxx>
#include <Quantity_Color.hxx>
#include <BRepPrimAPI_MakeBox.hxx>
#include <BRepGProp.hxx>
#include <BRepCheck_Analyzer.hxx>
#include <GProp_GProps.hxx>
#include <iostream>
#include <iomanip>
#include <string>
#include <functional>
int main(int argc,char**argv) {
 if(argc!=3) return 2;
 occ::handle<TDocStd_Document> d=new TDocStd_Document(TCollection_ExtendedString("BinXCAF"));
 auto shapes=XCAFDoc_DocumentTool::ShapeTool(d->Main());
 auto colors=XCAFDoc_DocumentTool::ColorTool(d->Main());
 if(std::string(argv[1])=="read") {
  STEPCAFControl_Reader reader; reader.SetSHUOMode(true);
  if(reader.ReadFile(argv[2])!=IFSelect_RetDone || !reader.Transfer(d)) return 3;
  NCollection_Sequence<TDF_Label> roots; shapes->GetFreeShapes(roots);
  double volume=0,area=0;
  for(int i=1;i<=roots.Length();++i) {
   GProp_GProps v,a; auto s=shapes->GetShape(roots.Value(i));
   if(!BRepCheck_Analyzer(s).IsValid())return 7;
   BRepGProp::VolumeProperties(s,v); BRepGProp::SurfaceProperties(s,a);
   volume+=v.Mass(); area+=a.Mass();
  }
  std::cout<<std::setprecision(17)<<"{\"volume\":"<<volume<<",\"area\":"<<area<<",\"colors\":[";
  bool comma=false;
  for(TDF_ChildIterator it(d->Main(),true);it.More();it.Next()) {
   Quantity_Color c;
   if(colors->GetColor(it.Value(),XCAFDoc_ColorSurf,c)) {
    if(comma) std::cout<<","; comma=true;
    double red,green,blue; c.Values(red,green,blue,Quantity_TOC_sRGB);
    std::cout<<"["<<red<<","<<green<<","<<blue<<"]";
   }
  }
  std::cout<<"],\"occurrences\":[";
  comma=false;
  NCollection_Sequence<TDF_Label> path;
  std::function<bool(TDF_Label,gp_Trsf,int)> visit;
  visit=[&](TDF_Label parent,gp_Trsf parentPose,int depth) {
   if(depth>8)return false;
   NCollection_Sequence<TDF_Label> uses; shapes->GetComponents(parent,uses,false);
   for(int j=1;j<=uses.Length();++j) {
    auto use=uses.Value(j);TDF_Label product;if(!shapes->GetReferredShape(use,product))return false;
    auto shape=shapes->GetShape(use);gp_Trsf world=parentPose*shape.Location().Transformation();
    NCollection_Sequence<TDF_Label> children;shapes->GetComponents(product,children,false);
    if(children.Length()) {path.Append(use);if(!visit(product,world,depth+1))return false;path.Remove(path.Length());continue;}
    Quantity_Color c;path.Append(use);occ::handle<XCAFDoc_GraphNode> node;
    bool component=shapes->FindSHUO(path,node) && colors->GetColor(node->Label(),XCAFDoc_ColorSurf,c);path.Remove(path.Length());
    if(!component)component=colors->GetColor(use,XCAFDoc_ColorSurf,c);
    if(!component&&!colors->GetColor(product,XCAFDoc_ColorSurf,c))return false;
    auto t=world.TranslationPart();double red,green,blue;c.Values(red,green,blue,Quantity_TOC_sRGB);
    if(comma)std::cout<<",";comma=true;
    std::cout<<"{\"translation\":["<<t.X()<<","<<t.Y()<<","<<t.Z()<<"],\"component\":"<<(component?"true":"false")<<",\"depth\":"<<depth+1<<",\"color\":["<<red<<","<<green<<","<<blue<<"],\"rotation\":[";
    for(int row=1;row<=3;row++)for(int col=1;col<=3;col++){if(row!=1||col!=1)std::cout<<",";std::cout<<world.Value(row,col);}
    std::cout<<"]}";
   }
   return true;
  };
  for(int i=1;i<=roots.Length();++i)if(!visit(roots.Value(i),gp_Trsf(),0))return 6;
  std::cout<<"]}\n"; return 0;
 }
 if(std::string(argv[1])!="write") return 2;
 auto part=shapes->AddShape(BRepPrimAPI_MakeBox(2,3,4).Shape(),false);
 auto other=shapes->AddShape(BRepPrimAPI_MakeBox(1,2,3).Shape(),false);
 auto group=shapes->NewShape();
 auto root=shapes->NewShape();
 TDataStd_Name::Set(part,TCollection_ExtendedString("Exact planar part"));
 TDataStd_Name::Set(other,TCollection_ExtendedString("Small independent part"));
 TDataStd_Name::Set(root,TCollection_ExtendedString("AP242 nested shared occurrence styles"));
 gp_Trsf a; a.SetTranslation(gp_Vec(10,0,0));
 auto first=shapes->AddComponent(group,part,TopLoc_Location(a));
 TDataStd_Name::Set(first,TCollection_ExtendedString("Translated use"));
 gp_Trsf b; b.SetValues(0,0,1,0,1,0,0,20,0,1,0,0);
 auto second=shapes->AddComponent(group,part,TopLoc_Location(b));
 TDataStd_Name::Set(second,TCollection_ExtendedString("Cyclic use"));
 gp_Trsf c; c.SetTranslation(gp_Vec(-10,-5,7));
 auto third=shapes->AddComponent(group,other,TopLoc_Location(c));
 TDataStd_Name::Set(third,TCollection_ExtendedString("Independent use"));
 colors->SetColor(part,Quantity_Color(1,0,0,Quantity_TOC_RGB),XCAFDoc_ColorSurf);
 colors->SetColor(other,Quantity_Color(0,1,0,Quantity_TOC_RGB),XCAFDoc_ColorSurf);

 colors->SetColor(first,Quantity_Color(0,1,1,Quantity_TOC_RGB),XCAFDoc_ColorSurf);
 TDataStd_Name::Set(group,TCollection_ExtendedString("Shared styled group"));
 gp_Trsf outer1;outer1.SetTranslation(gp_Vec(100,0,0));
 auto rootFirst=shapes->AddComponent(root,group,TopLoc_Location(outer1));
 gp_Trsf outer2;outer2.SetValues(0,0,1,0,1,0,0,100,0,1,0,0);
 shapes->AddComponent(root,group,TopLoc_Location(outer2));
 NCollection_Sequence<TDF_Label> path;path.Append(rootFirst);path.Append(second);occ::handle<XCAFDoc_GraphNode> shuo;
 if(!shapes->SetSHUO(path,shuo))return 8;
 colors->SetColor(shuo->Label(),Quantity_Color(0,0,1,Quantity_TOC_RGB),XCAFDoc_ColorSurf);
 shapes->UpdateAssemblies();
 STEPCAFControl_Writer writer; DESTEP_Parameters params;
 params.WriteSchema=DESTEP_Parameters::WriteMode_StepSchema_AP242DIS;
 if(!writer.Transfer(d,params)||writer.Write(argv[2])!=IFSelect_RetDone) return 4;
}
