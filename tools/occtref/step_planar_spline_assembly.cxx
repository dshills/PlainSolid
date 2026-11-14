// Independent XCAF construction and ordinary material transfer. No Go geometry.
#include <STEPCAFControl_Writer.hxx>
#include <STEPCAFControl_Reader.hxx>
#include <STEPControl_Reader.hxx>
#include <TDocStd_Document.hxx>
#include <TDataStd_Name.hxx>
#include <XCAFDoc_DocumentTool.hxx>
#include <XCAFDoc_ShapeTool.hxx>
#include <BRepPrimAPI_MakeBox.hxx>
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
static TopoDS_Shape warped(){
 const gp_Pnt p[]={gp_Pnt(0,0,0),gp_Pnt(2,0,0),gp_Pnt(2,2,0),gp_Pnt(0,2,0),gp_Pnt(0,0,1),gp_Pnt(2,0,1),gp_Pnt(2,2,2),gp_Pnt(0,2,1)};
 const int c[6][4]={{0,1,3,2},{4,7,5,6},{0,4,1,5},{3,2,7,6},{0,3,4,7},{1,5,2,6}};
 NCollection_Array1<double> k(1,2);k.SetValue(1,0);k.SetValue(2,1);NCollection_Array1<int> m(1,2);m.Init(2);BRepBuilderAPI_Sewing sew(1e-8);
 for(int f=0;f<6;f++){NCollection_Array2<gp_Pnt> poles(1,2,1,2);for(int u=0;u<2;u++)for(int v=0;v<2;v++)poles.SetValue(u+1,v+1,p[c[f][2*u+v]]);occ::handle<Geom_BSplineSurface>s=new Geom_BSplineSurface(poles,k,k,m,m,1,1);sew.Add(BRepBuilderAPI_MakeFace(s,1e-8).Face());}sew.Perform();return BRepBuilderAPI_MakeSolid(TopoDS::Shell(sew.SewedShape())).Solid();
}
int main(int argc,char**argv){if(argc!=3)return 2;try{
 std::string mode=argv[1];if(mode=="write"){
 auto s=warped();if(!BRepCheck_Analyzer(s).IsValid())return 3;
 occ::handle<TDocStd_Document>d=new TDocStd_Document(TCollection_ExtendedString("BinXCAF"));auto t=XCAFDoc_DocumentTool::ShapeTool(d->Main());auto curve=t->AddShape(s,false);auto box=t->AddShape(BRepPrimAPI_MakeBox(1,2,3).Shape(),false);auto root=t->NewShape();
 TDataStd_Name::Set(curve,TCollection_ExtendedString("Warped material"));TDataStd_Name::Set(box,TCollection_ExtendedString("Planar material"));TDataStd_Name::Set(root,TCollection_ExtendedString("Mixed assembly"));
 gp_Trsf a;a.SetTranslation(gp_Vec(10,0,0));auto left=t->AddComponent(root,curve,TopLoc_Location(a));TDataStd_Name::Set(left,TCollection_ExtendedString("Curved left"));
 gp_Trsf b;b.SetValues(0,-1,0,0,1,0,0,20,0,0,1,0);auto right=t->AddComponent(root,curve,TopLoc_Location(b));TDataStd_Name::Set(right,TCollection_ExtendedString("Curved right"));
 gp_Trsf c;c.SetTranslation(gp_Vec(-10,0,0));auto stock=t->AddComponent(root,box,TopLoc_Location(c));TDataStd_Name::Set(stock,TCollection_ExtendedString("Planar instance"));
 t->UpdateAssemblies();STEPCAFControl_Writer w;if(!w.Transfer(d)||w.Write(argv[2])!=IFSelect_RetDone)return 4;return 0;}
 if(mode!="read")return 2;
 STEPControl_Reader r;if(r.ReadFile(argv[2])!=IFSelect_RetDone)return 5;r.TransferRoots();auto s=r.OneShape();if(s.IsNull()||!BRepCheck_Analyzer(s).IsValid())return 6;
 occ::handle<TDocStd_Document>d=new TDocStd_Document(TCollection_ExtendedString("BinXCAF"));STEPCAFControl_Reader xr;if(xr.ReadFile(argv[2])!=IFSelect_RetDone||!xr.Transfer(d))return 7;auto t=XCAFDoc_DocumentTool::ShapeTool(d->Main());NCollection_Sequence<TDF_Label> roots;t->GetFreeShapes(roots);if(roots.Length()!=1)return 8;NCollection_Sequence<TDF_Label> children;t->GetComponents(roots.Value(1),children);std::set<int> definitions;for(int i=1;i<=children.Length();i++){TDF_Label p;if(!t->GetReferredShape(children.Value(i),p))return 8;definitions.insert(p.Tag());}
 GProp_GProps v,a;BRepGProp::VolumeProperties(s,v,1e-10);BRepGProp::SurfaceProperties(s,a,1e-10);int faces=0,solids=0;for(TopExp_Explorer x(s,TopAbs_FACE);x.More();x.Next())faces++;for(TopExp_Explorer x(s,TopAbs_SOLID);x.More();x.Next())solids++;
 std::cout<<std::setprecision(17)<<"{\"Faces\":"<<faces<<",\"Solids\":"<<solids<<",\"Definitions\":"<<definitions.size()<<",\"Occurrences\":"<<children.Length()<<",\"Volume\":"<<v.Mass()<<",\"Area\":"<<a.Mass()<<",\"Center\":["<<v.CentreOfMass().X()<<","<<v.CentreOfMass().Y()<<","<<v.CentreOfMass().Z()<<"]}\n";return 0;
 }catch(const Standard_Failure&e){std::cerr<<e.what();return 9;}}
