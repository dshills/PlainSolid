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
#include <BRepBuilderAPI_NurbsConvert.hxx>
#include <BRepBuilderAPI_GTransform.hxx>
#include <gp_GTrsf.hxx>
#include <gp_Mat.hxx>
#include <BRepGProp.hxx>
#include <BRep_Builder.hxx>
#include <BRep_Tool.hxx>
#include <BRepCheck_Analyzer.hxx>
#include <Geom_BSplineSurface.hxx>
#include <Geom2d_BSplineCurve.hxx>
#include <TopExp_Explorer.hxx>
#include <TopoDS.hxx>
#include <vector>
#include <stdexcept>
#include <GProp_GProps.hxx>
#include <iostream>
#include <iomanip>
#include <string>
// Reparameterize independently in OCCT, before STEP is written. Both shape
// dimensions are dyadic so original and new UV coefficients stay exact.
static TopoDS_Shape normalizedCharts(double x,double y,double z,bool shifted,bool mixed,bool oblique,bool quarter) {
 auto shape=BRepBuilderAPI_NurbsConvert(BRepPrimAPI_MakeBox(x,y,z).Shape()).Shape();
 BRep_Builder builder;
 for(TopExp_Explorer fs(shape,TopAbs_FACE);fs.More();fs.Next()) {
  auto face=TopoDS::Face(fs.Current());
  auto surface=occ::down_cast<Geom_BSplineSurface>(BRep_Tool::Surface(face));
  if(surface.IsNull()||surface->NbUKnots()!=2||surface->NbVKnots()!=2) throw std::runtime_error("support");
  double u0=surface->UKnot(1),u1=surface->UKnot(2),v0=surface->VKnot(1),v1=surface->VKnot(2);
  struct Arc {TopoDS_Edge edge;occ::handle<Geom2d_BSplineCurve> curve;double first,last;};
  std::vector<Arc> arcs;
  for(TopExp_Explorer es(face,TopAbs_EDGE);es.More();es.Next()) {
   auto edge=TopoDS::Edge(es.Current());double first,last;
   auto pc=occ::down_cast<Geom2d_BSplineCurve>(BRep_Tool::CurveOnSurface(edge,face,first,last));
   if(pc.IsNull()) throw std::runtime_error("pcurve");
   pc=occ::down_cast<Geom2d_BSplineCurve>(pc->Copy());
   for(int i=1;i<=pc->NbPoles();++i) {auto p=pc->Pole(i);pc->SetPole(i,gp_Pnt2d(3*(p.X()-u0)/(u1-u0)+(shifted?-3:0),6*(p.Y()-v0)/(v1-v0)+(shifted?6:0)));}
   pc->IncreaseDegree(mixed?4:3);
   pc->InsertKnot(first+(last-first)*(quarter?.25:.5),1,0);
   arcs.push_back({edge,pc,first,last});
  }
  NCollection_Array1<double> knots(1,2);knots(1)=shifted?-3:0;knots(2)=shifted?0:3;
  surface->SetUKnots(knots);knots(1)=shifted?6:0;knots(2)=shifted?12:6;surface->SetVKnots(knots);
  surface->IncreaseDegree(2,1);
  for(int j=1;j<=2;++j){auto a=surface->Pole(1,j),b=surface->Pole(3,j);surface->SetPole(2,j,gp_Pnt((3*a.X()+b.X())/4,(3*a.Y()+b.Y())/4,(3*a.Z()+b.Z())/4));for(int i=1;i<=3;++i)surface->SetWeight(i,j,i);}
  for(auto& a:arcs) {builder.UpdateEdge(a.edge,a.curve,face,BRep_Tool::Tolerance(a.edge));builder.Range(a.edge,face,a.first,a.last);}
 }
 if(!BRepCheck_Analyzer(shape).IsValid()) throw std::runtime_error("invalid normalized original");
 gp_GTrsf rotation(oblique?gp_Mat(.6,-.8,0,.8,.6,0,0,0,1):gp_Mat(0,0,1,1,0,0,0,1,0),gp_XYZ(0,0,0));
 auto placed=BRepBuilderAPI_GTransform(shape,rotation,true).Shape();
 if(!BRepCheck_Analyzer(placed).IsValid())throw std::runtime_error("invalid oblique original");
 return placed;
}
int main(int argc,char**argv) {
 if(argc!=3) return 2;
 occ::handle<TDocStd_Document> d=new TDocStd_Document(TCollection_ExtendedString("BinXCAF"));
 auto shapes=XCAFDoc_DocumentTool::ShapeTool(d->Main());
 auto colors=XCAFDoc_DocumentTool::ColorTool(d->Main());
 if(std::string(argv[1])=="read") {
  STEPCAFControl_Reader reader;
  if(reader.ReadFile(argv[2])!=IFSelect_RetDone || !reader.Transfer(d)) return 3;
  NCollection_Sequence<TDF_Label> roots; shapes->GetFreeShapes(roots);
  double volume=0,area=0,mx=0,my=0,mz=0;int solids=0,faces=0;
  for(int i=1;i<=roots.Length();++i) {
   GProp_GProps v,a; auto s=shapes->GetShape(roots.Value(i));
   if(!BRepCheck_Analyzer(s).IsValid()) return 5;
   for(TopExp_Explorer ex(s,TopAbs_SOLID);ex.More();ex.Next())++solids;
   for(TopExp_Explorer ex(s,TopAbs_FACE);ex.More();ex.Next())++faces;
   BRepGProp::VolumeProperties(s,v); BRepGProp::SurfaceProperties(s,a);
   volume+=v.Mass(); area+=a.Mass();
   auto c=v.CentreOfMass();mx+=v.Mass()*c.X();my+=v.Mass()*c.Y();mz+=v.Mass()*c.Z();
  }
  std::cout<<std::setprecision(17)<<"{\"volume\":"<<volume<<",\"area\":"<<area<<",\"solids\":"<<solids<<",\"faces\":"<<faces<<",\"centroid\":["<<mx/volume<<","<<my/volume<<","<<mz/volume<<"],\"colors\":[";
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
 if(std::string(argv[1])!="write"&&std::string(argv[1])!="write-shifted"&&std::string(argv[1])!="write-mixed"&&std::string(argv[1])!="write-oblique"&&std::string(argv[1])!="write-quarter") return 2;
 bool quarter=std::string(argv[1])=="write-quarter";bool shifted=std::string(argv[1])=="write-shifted";bool mixed=std::string(argv[1])=="write-mixed";bool oblique=std::string(argv[1])=="write-oblique";
 auto part=shapes->AddShape(normalizedCharts(15,30,9,shifted,mixed,oblique,quarter),false);
 auto other=shapes->AddShape(normalizedCharts(30,15,6,shifted,mixed,oblique,quarter),false);
 auto root=shapes->NewShape();
 TDataStd_Name::Set(part,TCollection_ExtendedString("Exact planar part"));
 TDataStd_Name::Set(other,TCollection_ExtendedString("Small independent part"));
 TDataStd_Name::Set(root,TCollection_ExtendedString("AP242 elevated NURBS assembly"));
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
