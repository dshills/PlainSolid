// Independent original rational common-speed model/pcurve roof solid.
#include <STEPControl_Writer.hxx>
#include <STEPControl_Reader.hxx>
#include <Interface_Static.hxx>
#include <Geom_BSplineSurface.hxx>
#include <Geom_BSplineCurve.hxx>
#include <Geom_TrimmedCurve.hxx>
#include <GeomConvert.hxx>
#include <NCollection_IndexedMap.hxx>
#include <TopTools_ShapeMapHasher.hxx>
#include <TopExp.hxx>
#include <Geom2d_TrimmedCurve.hxx>
#include <Geom2dConvert.hxx>
#include <BRep_Tool.hxx>
#include <BRep_Builder.hxx>
#include <NCollection_Array1.hxx>
#include <NCollection_Array2.hxx>
#include <BRepBuilderAPI_MakeFace.hxx>
#include <BRepBuilderAPI_Sewing.hxx>
#include <BRepBuilderAPI_MakeSolid.hxx>
#include <BRepCheck_Analyzer.hxx>
#include <BRepGProp.hxx>
#include <GProp_GProps.hxx>
#include <TopoDS.hxx>
#include <TopoDS_Solid.hxx>

#include <TopExp_Explorer.hxx>
#include <iostream>
#include <iomanip>
#include <string>
int main(int argc,char**argv){if(argc!=3)return 2;TopoDS_Shape shape;
 if(std::string(argv[1])=="write"||std::string(argv[1])=="write-half"){double ratio=std::string(argv[1])=="write"?2:0.5;
 BRepBuilderAPI_Sewing sew(1e-9);
 for(int f=0;f<6;++f){int nu=(f==1||f==2)?3:2,nv=f==3?3:2;NCollection_Array2<gp_Pnt> p(1,nu,1,nv);
 for(int i=0;i<nu;++i)for(int j=0;j<nv;++j){double x=0,y=0,h=0,r=0;
 switch(f){case 0:x=j;y=i;break;case 1:x=i/2.;y=j;h=1;r=i==2?1:0;break;case 2:x=i/2.;h=j;r=i==2?j:0;break;case 3:x=j/2.;y=1;h=i;r=j==2?i:0;break;case 4:y=j;h=i;break;case 5:x=1;y=i;h=j;r=j;break;}
 p.SetValue(i+1,j+1,gp_Pnt(4*x,3*y,2*h+r));}
 NCollection_Array1<double> k(1,2);k(1)=0;k(2)=1;NCollection_Array1<int> mu(1,2),mv(1,2);mu(1)=mu(2)=nu;mv(1)=mv(2)=nv;
 occ::handle<Geom_BSplineSurface> s=new Geom_BSplineSurface(p,k,k,mu,mv,nu-1,nv-1,false,false);sew.Add(BRepBuilderAPI_MakeFace(s,1e-9).Face());}
 sew.Perform();shape=BRepBuilderAPI_MakeSolid(TopoDS::Shell(sew.SewedShape())).Solid();BRep_Builder builder;
 NCollection_IndexedMap<TopoDS_Shape,TopTools_ShapeMapHasher> edges;TopExp::MapShapes(shape,TopAbs_EDGE,edges);
 for(int ei=1;ei<=edges.Extent();++ei){auto edge=TopoDS::Edge(edges(ei));double first,last;auto c=BRep_Tool::Curve(edge,first,last);auto s=GeomConvert::CurveToBSplineCurve(new Geom_TrimmedCurve(c,first,last));if(first!=0||last!=1||s->NbKnots()!=2||s->NbPoles()!=s->Degree()+1)return 7;double w=1;for(int i=1;i<=s->NbPoles();++i,w*=ratio)s->SetWeight(i,w);builder.UpdateEdge(edge,s,BRep_Tool::Tolerance(edge));}

 for(TopExp_Explorer fs(shape,TopAbs_FACE);fs.More();fs.Next()){auto face=TopoDS::Face(fs.Current());for(TopExp_Explorer es(face,TopAbs_EDGE);es.More();es.Next()){auto edge=TopoDS::Edge(es.Current());double first,last;auto pc=BRep_Tool::CurveOnSurface(edge,face,first,last);auto spline=Geom2dConvert::CurveToBSplineCurve(new Geom2d_TrimmedCurve(pc,first,last));if(spline->Degree()!=1||first!=0||last!=1)return 8;spline->SetWeight(2,ratio);builder.UpdateEdge(edge,spline,face,BRep_Tool::Tolerance(edge));builder.Range(edge,face,first,last);}}
 if(!BRepCheck_Analyzer(shape).IsValid())return 3;
 STEPControl_Writer w;Interface_Static::SetCVal("write.step.schema","AP242DIS");if(w.Transfer(shape,STEPControl_AsIs)!=IFSelect_RetDone||w.Write(argv[2])!=IFSelect_RetDone)return 4;
 }else if(std::string(argv[1])=="read"){STEPControl_Reader r;if(r.ReadFile(argv[2])!=IFSelect_RetDone||r.TransferRoots()==0)return 5;shape=r.OneShape();}else return 2;
 if(!BRepCheck_Analyzer(shape).IsValid())return 6;GProp_GProps v,a;BRepGProp::VolumeProperties(shape,v,1e-12);BRepGProp::SurfaceProperties(shape,a,1e-12);int faces=0,solids=0;for(TopExp_Explorer e(shape,TopAbs_FACE);e.More();e.Next())++faces;for(TopExp_Explorer e(shape,TopAbs_SOLID);e.More();e.Next())++solids;
 std::cout<<std::setprecision(17)<<"{\"Volume\":"<<v.Mass()<<",\"Area\":"<<a.Mass()<<",\"Faces\":"<<faces<<",\"Solids\":"<<solids<<",\"Centroid\":["<<v.CentreOfMass().X()<<","<<v.CentreOfMass().Y()<<","<<v.CentreOfMass().Z()<<"]}\n";
}
