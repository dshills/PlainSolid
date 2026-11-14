// Independent pinned OCCT analytic STEP readback and original Boolean comparison.
#include <STEPControl_Reader.hxx>
#include <BRepCheck_Analyzer.hxx>
#include <BRepAdaptor_Surface.hxx>
#include <BRepGProp.hxx>
#include <BRep_Tool.hxx>
#include <BRepPrimAPI_MakeTorus.hxx>
#include <BRepPrimAPI_MakeCylinder.hxx>
#include <BRepAlgoAPI_Common.hxx>
#include <BRepAlgoAPI_Cut.hxx>
#include <BRepAlgoAPI_Fuse.hxx>
#include <Geom_Curve.hxx>
#include <Geom2d_Curve.hxx>
#include <GProp_GProps.hxx>
#include <TopExp_Explorer.hxx>
#include <TopoDS.hxx>
#include <Standard_Failure.hxx>
#include <fstream>
#include <iomanip>
#include <iostream>
#include <cmath>
#include <stdexcept>
int main(int argc,char**argv){try{
 if(argc!=9&&argc!=10)throw std::runtime_error("usage: torus_cylinder_step OUT STEP R r c lower upper op [pose]");
 double R=std::stod(argv[3]),r=std::stod(argv[4]),c=std::stod(argv[5]),lower=std::stod(argv[6]),upper=std::stod(argv[7]);int op=std::stoi(argv[8]);
 int pose=argc==10?std::stoi(argv[9]):0;
 gp_Ax2 frame(gp_Pnt(0,0,0),gp_Dir(0,0,1),gp_Dir(1,0,0));
 if(pose==1)frame=gp_Ax2(gp_Pnt(2,-3,4),gp_Dir(-1,0,0),gp_Dir(0,0,1));
 else if(pose==2)frame=gp_Ax2(gp_Pnt(.1,-.4,.375),gp_Dir(.8,0,.6),gp_Dir(.6,0,-.8));
 else if(pose!=0)throw std::runtime_error("pose");
 STEPControl_Reader reader;if(reader.ReadFile(argv[2])!=IFSelect_RetDone||reader.TransferRoots()!=1)throw std::runtime_error("readback roots");auto shape=reader.OneShape();if(shape.IsNull()||!BRepCheck_Analyzer(shape).IsValid())throw std::runtime_error("invalid readback");
 int solids=0,shells=0,faces=0,types[3]={0,0,0};for(TopExp_Explorer x(shape,TopAbs_SOLID);x.More();x.Next())solids++;for(TopExp_Explorer x(shape,TopAbs_SHELL);x.More();x.Next())shells++;if(solids!=1||shells!=1)throw std::runtime_error("one closed solid required");
 double residual=0;
 for(TopExp_Explorer it(shape,TopAbs_FACE);it.More();it.Next()){
 auto face=TopoDS::Face(it.Current());BRepAdaptor_Surface s(face);faces++;
 if(s.GetType()==GeomAbs_Torus){types[0]++;if(std::abs(s.Torus().MajorRadius()-R)>1e-12||std::abs(s.Torus().MinorRadius()-r)>1e-12)throw std::runtime_error("torus source");}
 else if(s.GetType()==GeomAbs_Cylinder){types[1]++;if(std::abs(s.Cylinder().Radius()-c)>1e-12)throw std::runtime_error("wall source");}
 else if(s.GetType()==GeomAbs_Plane){types[2]++;double z=gp_Vec(frame.Location(),s.Plane().Location()).Dot(gp_Vec(frame.Direction()));if(std::min(std::abs(z-lower),std::abs(z-upper))>1e-12)throw std::runtime_error("cap source");}
 else throw std::runtime_error("nonanalytic support");
 for(TopExp_Explorer j(face,TopAbs_EDGE);j.More();j.Next()){
 auto edge=TopoDS::Edge(j.Current());double a,b,pa,pb;auto curve=BRep_Tool::Curve(edge,a,b);auto pc=BRep_Tool::CurveOnSurface(edge,face,pa,pb);if(curve.IsNull()||pc.IsNull())throw std::runtime_error("missing curve");
 for(int k=0;k<=8;k++){double t=a+(b-a)*k/8.;gp_Pnt2d uv=pc->Value(t);residual=std::max(residual,curve->Value(t).Distance(s.Value(uv.X(),uv.Y())));}
 }
 }
 if(types[0]!=1||types[1]!=(op==3?2:1)||types[2]!=(op==3?2:0)||faces!=(op==3?5:2)||residual>1e-7)throw std::runtime_error("support/edge validation");
 auto torus=BRepPrimAPI_MakeTorus(frame,R,r).Shape();auto cylinderFrame=frame;cylinderFrame.SetLocation(frame.Location().Translated(gp_Vec(frame.Direction())*lower));auto cylinder=BRepPrimAPI_MakeCylinder(cylinderFrame,c,upper-lower).Shape();TopoDS_Shape expected;
 if(op==1)expected=BRepAlgoAPI_Common(torus,cylinder).Shape();else if(op==2)expected=BRepAlgoAPI_Cut(torus,cylinder).Shape();else if(op==3)expected=BRepAlgoAPI_Fuse(torus,cylinder).Shape();else throw std::runtime_error("op");
 if(!BRepCheck_Analyzer(expected).IsValid())throw std::runtime_error("invalid original Boolean");
 GProp_GProps v,a,rv,ra;BRepGProp::VolumeProperties(shape,v,1e-10,true);BRepGProp::SurfaceProperties(shape,a,1e-10);BRepGProp::VolumeProperties(expected,rv,1e-10,true);BRepGProp::SurfaceProperties(expected,ra,1e-10);
 std::ofstream out(argv[1]);out.exceptions(std::ios::failbit|std::ios::badbit);out<<std::setprecision(17)<<"{\"Volume\":"<<v.Mass()<<",\"Area\":"<<a.Mass()<<",\"CentroidZ\":"<<v.CentreOfMass().Z()<<",\"ReferenceVolume\":"<<rv.Mass()<<",\"ReferenceArea\":"<<ra.Mass()<<",\"ReferenceCentroidZ\":"<<rv.CentreOfMass().Z()<<",\"Centroid\":["<<v.CentreOfMass().X()<<','<<v.CentreOfMass().Y()<<','<<v.CentreOfMass().Z()<<"],\"ReferenceCentroid\":["<<rv.CentreOfMass().X()<<','<<rv.CentreOfMass().Y()<<','<<rv.CentreOfMass().Z()<<"],\"EdgeResidual\":"<<residual<<",\"Solids\":"<<solids<<",\"Shells\":"<<shells<<",\"Faces\":"<<faces<<"}\n";
 return 0;
}catch(const Standard_Failure&e){std::cerr<<e.what()<<'\n';return 1;}catch(const std::exception&e){std::cerr<<e.what()<<'\n';return 1;}}
