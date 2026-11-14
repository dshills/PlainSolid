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
#include <Quantity_Color.hxx>
#include <BRepPrimAPI_MakeBox.hxx>
#include <BRepGProp.hxx>
#include <GProp_GProps.hxx>
#include <iostream>
#include <iomanip>
#include <string>
int main(int argc,char**argv) {
 if(argc!=3) return 2;
 occ::handle<TDocStd_Document> d=new TDocStd_Document(TCollection_ExtendedString("BinXCAF"));
 auto shapes=XCAFDoc_DocumentTool::ShapeTool(d->Main());
 auto colors=XCAFDoc_DocumentTool::ColorTool(d->Main());
 if(std::string(argv[1])=="read") {
  STEPCAFControl_Reader reader;
  if(reader.ReadFile(argv[2])!=IFSelect_RetDone || !reader.Transfer(d)) return 3;
  NCollection_Sequence<TDF_Label> roots; shapes->GetFreeShapes(roots);
  double volume=0,area=0;
  for(int i=1;i<=roots.Length();++i) {
   GProp_GProps v,a; auto s=shapes->GetShape(roots.Value(i));
   BRepGProp::VolumeProperties(s,v); BRepGProp::SurfaceProperties(s,a);
   volume+=v.Mass(); area+=a.Mass();
  }
  std::cout<<std::setprecision(17)<<"{\"volume\":"<<volume<<",\"area\":"<<area<<",\"colors\":[";
  bool comma=false;
  for(TDF_ChildIterator it(d->Main(),true);it.More();it.Next()) {
   Quantity_Color c;
   if(colors->GetColor(it.Value(),XCAFDoc_ColorSurf,c)) {
    if(comma) std::cout<<","; comma=true;
    std::cout<<"["<<c.Red()<<","<<c.Green()<<","<<c.Blue()<<"]";
   }
  }
  std::cout<<"]}\n"; return 0;
 }
 if(std::string(argv[1])!="write") return 2;
 auto part=shapes->AddShape(BRepPrimAPI_MakeBox(2,3,4).Shape(),false);
 auto other=shapes->AddShape(BRepPrimAPI_MakeBox(1,2,3).Shape(),false);
 auto root=shapes->NewShape();
 TDataStd_Name::Set(part,TCollection_ExtendedString("Exact planar part"));
 TDataStd_Name::Set(other,TCollection_ExtendedString("Small independent part"));
 TDataStd_Name::Set(root,TCollection_ExtendedString("AP242 palette assembly"));
 gp_Trsf a; a.SetTranslation(gp_Vec(10,0,0));
 auto first=shapes->AddComponent(root,part,TopLoc_Location(a));
 TDataStd_Name::Set(first,TCollection_ExtendedString("Translated use"));
 gp_Trsf b; b.SetValues(0,0,1,0,1,0,0,20,0,1,0,0);
 auto second=shapes->AddComponent(root,part,TopLoc_Location(b));
 TDataStd_Name::Set(second,TCollection_ExtendedString("Cyclic use"));
 gp_Trsf c; c.SetTranslation(gp_Vec(-10,-5,7));
 auto third=shapes->AddComponent(root,other,TopLoc_Location(c));
 TDataStd_Name::Set(third,TCollection_ExtendedString("Independent use"));
 colors->SetColor(part,Quantity_Color(1,0,0,Quantity_TOC_RGB),XCAFDoc_ColorSurf);
 colors->SetColor(other,Quantity_Color(0,0,1,Quantity_TOC_RGB),XCAFDoc_ColorSurf);
 shapes->UpdateAssemblies();
 STEPCAFControl_Writer writer; DESTEP_Parameters params;
 params.WriteSchema=DESTEP_Parameters::WriteMode_StepSchema_AP242DIS;
 if(!writer.Transfer(d,params)||writer.Write(argv[2])!=IFSelect_RetDone) return 4;
}
