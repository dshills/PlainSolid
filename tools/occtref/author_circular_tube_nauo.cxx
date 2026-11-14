#include <STEPCAFControl_Writer.hxx>
#include <Interface_Static.hxx>
#include <cstring>
#include <TDocStd_Document.hxx>
#include <TDataStd_Name.hxx>
#include <XCAFDoc_DocumentTool.hxx>
#include <XCAFDoc_ShapeTool.hxx>
#include <BRepPrimAPI_MakeBox.hxx>
#include <BRepPrimAPI_MakeCylinder.hxx>
#include <BRepAlgoAPI_Cut.hxx>
int main(int argc,char**argv){if(argc<2||argc>4)return 2;if(argc==4&&std::strcmp(argv[3],"scaled"))return 2;double k=argc==4?10.:1.;occ::handle<TDocStd_Document>d=new TDocStd_Document(TCollection_ExtendedString("BinXCAF"));auto t=XCAFDoc_DocumentTool::ShapeTool(d->Main());auto part=t->AddShape(BRepPrimAPI_MakeBox(2,3,4).Shape(),false);auto root=t->NewShape();TDataStd_Name::Set(part,TCollection_ExtendedString("Exact planar part"));TDataStd_Name::Set(root,TCollection_ExtendedString("Mixed circular NAUO assembly"));gp_Trsf a;a.SetTranslation(gp_Vec(10,0,0));auto first=t->AddComponent(root,part,TopLoc_Location(a));TDataStd_Name::Set(first,TCollection_ExtendedString("Translated use"));gp_Trsf b;b.SetValues(0,0,1,0,1,0,0,20,0,1,0,0);auto second=t->AddComponent(root,part,TopLoc_Location(b));TDataStd_Name::Set(second,TCollection_ExtendedString("Cyclic use"));auto other=t->AddShape(BRepAlgoAPI_Cut(BRepPrimAPI_MakeCylinder(3*k,5*k).Shape(),BRepPrimAPI_MakeCylinder(k,5*k).Shape()).Shape(),false);TDataStd_Name::Set(other,TCollection_ExtendedString("Circular tube part"));gp_Trsf c;c.SetTranslation(gp_Vec(-10,-5,7));auto third=t->AddComponent(root,other,TopLoc_Location(c));TDataStd_Name::Set(third,TCollection_ExtendedString("Independent use"));t->UpdateAssemblies();STEPCAFControl_Writer w;if(argc>=3&&!Interface_Static::SetCVal("write.step.unit",argv[2]))return 4;if(!w.Transfer(d)||w.Write(argv[1])!=IFSelect_RetDone)return 3;}
