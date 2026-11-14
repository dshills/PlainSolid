// Reproduce why this pinned public XCAF authoring route cannot supply a
// genuinely occurrence-local face fixture. No synthetic STEP rewrite is used.
#include <TDocStd_Document.hxx>
#include <XCAFDoc_DocumentTool.hxx>
#include <XCAFDoc_ShapeTool.hxx>
#include <XCAFDoc_ColorTool.hxx>
#include <BRepPrimAPI_MakeBox.hxx>
#include <TopExp_Explorer.hxx>
#include <Quantity_Color.hxx>
#include <iostream>
int main(){
 occ::handle<TDocStd_Document> d=new TDocStd_Document(TCollection_ExtendedString("BinXCAF"));
 auto shapes=XCAFDoc_DocumentTool::ShapeTool(d->Main());
 auto colors=XCAFDoc_DocumentTool::ColorTool(d->Main());
 auto part=shapes->AddShape(BRepPrimAPI_MakeBox(2,3,4).Shape(),false);
 auto root=shapes->NewShape();gp_Trsf pose;pose.SetTranslation(gp_Vec(10,0,0));
 auto use=shapes->AddComponent(root,part,TopLoc_Location(pose));
 TopExp_Explorer faces(shapes->GetShape(use),TopAbs_FACE);
 auto face=faces.Current();auto label=shapes->AddSubShape(use,face);
 bool instance=colors->SetInstanceColor(face,XCAFDoc_ColorSurf,Quantity_Color(0,0,1,Quantity_TOC_RGB),true);
 std::cout<<"{\"componentFaceLabel\":"<<(!label.IsNull()?"true":"false")<<",\"instanceFaceColor\":"<<(instance?"true":"false")<<"}\n";
}
