// Independent XCAF construction and ordinary material transfer. No Go geometry.
#include <STEPCAFControl_Writer.hxx>
#include <STEPCAFControl_Reader.hxx>
#include <STEPControl_Reader.hxx>
#include <TDocStd_Document.hxx>
#include <TDataStd_Name.hxx>
#include <XCAFDoc_DocumentTool.hxx>
#include <XCAFDoc_ShapeTool.hxx>
#include <BRepPrimAPI_MakeBox.hxx>
#include <BRepPrimAPI_MakeCylinder.hxx>
#include <BRepAlgoAPI_Cut.hxx>
#include <BRepBuilderAPI_MakeFace.hxx>
#include <BRepBuilderAPI_MakeSolid.hxx>
#include <BRepBuilderAPI_Sewing.hxx>
#include <BRepCheck_Analyzer.hxx>
#include <BRepGProp.hxx>
#include <GProp_GProps.hxx>
#include <Geom_BSplineSurface.hxx>
#include <NCollection_Array1.hxx>
#include <NCollection_Array2.hxx>
#include <TopoDS.hxx>
#include <TopExp_Explorer.hxx>
#include <NCollection_Sequence.hxx>
#include <Standard_Failure.hxx>
#include <iostream>
#include <iomanip>
#include <set>
#include <string>
int main(int argc,char**argv){if(argc!=3)return 2;try{
 std::string mode=argv[1];if(mode=="write"){

 auto stock=BRepPrimAPI_MakeCylinder(3,4).Shape();
 BRepAlgoAPI_Cut blind(stock,BRepPrimAPI_MakeCylinder(gp_Ax2(gp_Pnt(0,0,-1),gp_Dir(0,0,1)),1,3).Shape());blind.Build();
 BRepAlgoAPI_Cut through(stock,BRepPrimAPI_MakeCylinder(gp_Ax2(gp_Pnt(0,0,-1),gp_Dir(0,0,1)),1,6).Shape());through.Build();
 if(!blind.IsDone()||!through.IsDone()||!BRepCheck_Analyzer(blind.Shape()).IsValid()||!BRepCheck_Analyzer(through.Shape()).IsValid())return 3;
 occ::handle<TDocStd_Document>d=new TDocStd_Document(TCollection_ExtendedString("BinXCAF"));auto t=XCAFDoc_DocumentTool::ShapeTool(d->Main());auto a=t->AddShape(blind.Shape(),false);auto b=t->AddShape(through.Shape(),false);auto root=t->NewShape();
 TDataStd_Name::Set(a,TCollection_ExtendedString("Blind stock"));TDataStd_Name::Set(b,TCollection_ExtendedString("Through stock"));TDataStd_Name::Set(root,TCollection_ExtendedString("Bored assembly"));
 gp_Trsf x;x.SetTranslation(gp_Vec(10,0,0));t->AddComponent(root,a,TopLoc_Location(x));
 gp_Trsf y;y.SetValues(0,0,1,0,1,0,0,20,0,1,0,0);t->AddComponent(root,a,TopLoc_Location(y));
 gp_Trsf z;z.SetTranslation(gp_Vec(-10,0,0));t->AddComponent(root,b,TopLoc_Location(z));
 t->UpdateAssemblies();STEPCAFControl_Writer w;if(!w.Transfer(d)||w.Write(argv[2])!=IFSelect_RetDone)return 4;return 0;}
 if(mode!="read")return 2;
 STEPControl_Reader r;if(r.ReadFile(argv[2])!=IFSelect_RetDone)return 5;r.TransferRoots();auto s=r.OneShape();if(s.IsNull()||!BRepCheck_Analyzer(s).IsValid())return 6;
 occ::handle<TDocStd_Document>d=new TDocStd_Document(TCollection_ExtendedString("BinXCAF"));STEPCAFControl_Reader xr;if(xr.ReadFile(argv[2])!=IFSelect_RetDone||!xr.Transfer(d))return 7;auto t=XCAFDoc_DocumentTool::ShapeTool(d->Main());NCollection_Sequence<TDF_Label> roots;t->GetFreeShapes(roots);if(roots.Length()!=1)return 8;NCollection_Sequence<TDF_Label> children;t->GetComponents(roots.Value(1),children);std::set<int> definitions;for(int i=1;i<=children.Length();i++){TDF_Label p;if(!t->GetReferredShape(children.Value(i),p))return 8;definitions.insert(p.Tag());}
 GProp_GProps v,a;BRepGProp::VolumeProperties(s,v,1e-10);BRepGProp::SurfaceProperties(s,a,1e-10);int faces=0,solids=0;for(TopExp_Explorer x(s,TopAbs_FACE);x.More();x.Next())faces++;for(TopExp_Explorer x(s,TopAbs_SOLID);x.More();x.Next())solids++;
 std::cout<<std::setprecision(17)<<"{\"Faces\":"<<faces<<",\"Solids\":"<<solids<<",\"Definitions\":"<<definitions.size()<<",\"Occurrences\":"<<children.Length()<<",\"Volume\":"<<v.Mass()<<",\"Area\":"<<a.Mass()<<",\"Center\":["<<v.CentreOfMass().X()<<","<<v.CentreOfMass().Y()<<","<<v.CentreOfMass().Z()<<"]}\n";return 0;
 }catch(const Standard_Failure&e){std::cerr<<e.what();return 9;}}
