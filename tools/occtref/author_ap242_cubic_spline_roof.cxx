// Independent OCCT-authored two-axis cubic polynomial roof solid.
#include <STEPControl_Writer.hxx>
#include <STEPControl_Reader.hxx>
#include <Interface_Static.hxx>
#include <Geom_BSplineSurface.hxx>
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
 if(std::string(argv[1])=="write"){
 BRepBuilderAPI_Sewing sew(1e-9);
 for(int f=0;f<6;++f){int nu=(f==1||f==2||f==5)?4:2,nv=(f==1||f==3||f==4)?4:2;NCollection_Array2<gp_Pnt> p(1,nu,1,nv);
 for(int i=0;i<nu;++i)for(int j=0;j<nv;++j){double x=0,y=0,z=0,u=(double)i/(nu-1),v=(double)j/(nv-1),cu=i==nu-1?1:0,cv=j==nv-1?1:0;
 switch(f){case 0:x=v;y=u;break;case 1:x=u;y=v;z=2+cu+cv;break;case 2:x=u;z=v*(2+cu);break;case 3:x=v;y=1;z=u*(3+cv);break;case 4:y=v;z=u*(2+cv);break;case 5:x=1;y=u;z=v*(3+cu);break;}
 p.SetValue(i+1,j+1,gp_Pnt(6*x,6*y,z));}
 NCollection_Array1<double> k(1,2);k(1)=0;k(2)=1;NCollection_Array1<int> mu(1,2),mv(1,2);mu(1)=mu(2)=nu;mv(1)=mv(2)=nv;
 occ::handle<Geom_BSplineSurface> s=new Geom_BSplineSurface(p,k,k,mu,mv,nu-1,nv-1,false,false);sew.Add(BRepBuilderAPI_MakeFace(s,1e-9).Face());}
 sew.Perform();shape=BRepBuilderAPI_MakeSolid(TopoDS::Shell(sew.SewedShape())).Solid();BRep_Builder builder;
 for(TopExp_Explorer fs(shape,TopAbs_FACE);fs.More();fs.Next()){auto face=TopoDS::Face(fs.Current());for(TopExp_Explorer es(face,TopAbs_EDGE);es.More();es.Next()){auto edge=TopoDS::Edge(es.Current());double first,last;auto pc=BRep_Tool::CurveOnSurface(edge,face,first,last);auto spline=Geom2dConvert::CurveToBSplineCurve(new Geom2d_TrimmedCurve(pc,first,last));builder.UpdateEdge(edge,spline,face,BRep_Tool::Tolerance(edge));builder.Range(edge,face,first,last);}}
 if(!BRepCheck_Analyzer(shape).IsValid())return 3;
 STEPControl_Writer w;Interface_Static::SetCVal("write.step.schema","AP242DIS");if(w.Transfer(shape,STEPControl_AsIs)!=IFSelect_RetDone||w.Write(argv[2])!=IFSelect_RetDone)return 4;
 }else if(std::string(argv[1])=="read"){STEPControl_Reader r;if(r.ReadFile(argv[2])!=IFSelect_RetDone||r.TransferRoots()==0)return 5;shape=r.OneShape();}else return 2;
 if(!BRepCheck_Analyzer(shape).IsValid())return 6;GProp_GProps v,a;BRepGProp::VolumeProperties(shape,v);BRepGProp::SurfaceProperties(shape,a);int faces=0,solids=0;for(TopExp_Explorer e(shape,TopAbs_FACE);e.More();e.Next())++faces;for(TopExp_Explorer e(shape,TopAbs_SOLID);e.More();e.Next())++solids;
 std::cout<<std::setprecision(17)<<"{\"Volume\":"<<v.Mass()<<",\"Area\":"<<a.Mass()<<",\"Faces\":"<<faces<<",\"Solids\":"<<solids<<",\"Centroid\":["<<v.CentreOfMass().X()<<","<<v.CentreOfMass().Y()<<","<<v.CentreOfMass().Z()<<"]}\n";
}
