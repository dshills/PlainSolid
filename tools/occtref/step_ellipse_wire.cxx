// Independent geometric construction, not a PlainSolid-authored STEP string.
#include <STEPControl_Writer.hxx>
#include <STEPControl_Reader.hxx>
#include <StepData_StepModel.hxx>
#include <XSControl_WorkSession.hxx>
#include <StepShape_GeometricCurveSet.hxx>
#include <StepGeom_TrimmedCurve.hxx>
#include <BRepBuilderAPI_MakeEdge.hxx>
#include <BRepBuilderAPI_MakeWire.hxx>
#include <BRepCheck_Analyzer.hxx>
#include <BRepGProp.hxx>
#include <BRepAdaptor_Curve.hxx>
#include <GCPnts_AbscissaPoint.hxx>
#include <TopoDS.hxx>
#include <GProp_GProps.hxx>
#include <gp_Elips.hxx>
#include <TopExp_Explorer.hxx>
#include <TopoDS_Wire.hxx>
#include <TopoDS_Edge.hxx>

#include <iostream>
#include <iomanip>
#include <string>
int main(int argc,char**argv){if(argc!=3)return 2;if(std::string(argv[1])=="write"||std::string(argv[1])=="write-natural"){gp_Elips e(gp_Ax2(gp_Pnt(1,2,3),gp_Dir(0,1,0),gp_Dir(0,0,-1)),4,2);auto edge=BRepBuilderAPI_MakeEdge(e).Edge();auto wire=BRepBuilderAPI_MakeWire(edge).Wire();STEPControl_Writer w;if(w.Transfer(wire,STEPControl_AsIs)!=IFSelect_RetDone)return 3;if(std::string(argv[1])=="write-natural"){auto m=w.Model();for(int i=1;i<=m->NbEntities();++i){auto set=occ::down_cast<StepShape_GeometricCurveSet>(m->Entity(i));if(set.IsNull())continue;auto trim=occ::down_cast<StepGeom_TrimmedCurve>(set->ElementsValue(1).Value());if(trim.IsNull())return 7;auto item=set->ElementsValue(1);item.SetValue(trim->BasisCurve());set->Elements()->SetValue(1,item);}STEPControl_Writer natural;for(int i=1;i<=m->NbEntities();++i){auto e=m->Entity(i);std::string type=e->DynamicType()->Name();if(type=="StepShape_ShapeDefinitionRepresentation"||type=="StepBasic_ApplicationProtocolDefinition"||type=="StepBasic_ProductRelatedProductCategory")natural.Model()->AddWithRefs(e);}natural.WS()->ComputeGraph(true);return natural.Write(argv[2])==IFSelect_RetDone?0:8;}return w.Write(argv[2])==IFSelect_RetDone?0:4;}STEPControl_Reader r;if(r.ReadFile(argv[2])!=IFSelect_RetDone||r.TransferRoots()!=1)return 5;auto s=r.OneShape();if(!BRepCheck_Analyzer(s).IsValid())return 6;int n=0;double length=0;gp_Pnt sample;gp_Vec derivative;for(TopExp_Explorer e(s,TopAbs_EDGE);e.More();e.Next()){++n;BRepAdaptor_Curve c(TopoDS::Edge(e.Current()));length+=GCPnts_AbscissaPoint::Length(c,c.FirstParameter(),c.LastParameter(),1e-11);c.D1(.375,sample,derivative);}GProp_GProps p;BRepGProp::LinearProperties(s,p);auto c=p.CentreOfMass();std::cout<<std::setprecision(17)<<"{\"Edges\":"<<n<<",\"Length\":"<<length<<",\"Center\":["<<c.X()<<","<<c.Y()<<","<<c.Z()<<"],\"Point\":["<<sample.X()<<","<<sample.Y()<<","<<sample.Z()<<"],\"Derivative\":["<<derivative.X()<<","<<derivative.Y()<<","<<derivative.Z()<<"]}\n";}
