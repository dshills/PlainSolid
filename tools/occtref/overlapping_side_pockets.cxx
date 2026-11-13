// Independent OCCT reference for two overlapping parallel blind cuts.
#include <BRepPrimAPI_MakeBox.hxx>
#include <BRepPrimAPI_MakeCylinder.hxx>
#include <BRepAlgoAPI_Fuse.hxx>
#include <BRepAlgoAPI_Cut.hxx>
#include <ShapeUpgrade_UnifySameDomain.hxx>
#include <BRepCheck_Analyzer.hxx>
#include <BRepGProp.hxx>
#include <BRepAdaptor_Surface.hxx>
#include <GProp_GProps.hxx>
#include <TopExp_Explorer.hxx>
#include <TopoDS.hxx>
#include <STEPControl_Writer.hxx>
#include <iostream>
#include <iomanip>
#include <string>
int main(int argc,char**argv){if(argc!=3)return 2;std::string mode=argv[1];if(mode!="side"&&mode!="interior")return 2;
 double y=mode=="side"?4:0,dy=mode=="side"?6:8;
 auto stock=BRepPrimAPI_MakeBox(gp_Pnt(0,y,0),10,dy,6).Shape();
 auto a=BRepPrimAPI_MakeCylinder(gp_Ax2(gp_Pnt(3.5,4,3),gp_Dir(0,0,1)),2.5,4).Shape();
 auto b=BRepPrimAPI_MakeCylinder(gp_Ax2(gp_Pnt(6.5,4,3),gp_Dir(0,0,1)),2.5,4).Shape();
 BRepAlgoAPI_Fuse fuse(a,b);if(!fuse.IsDone())return 3;BRepAlgoAPI_Cut cut(stock,fuse.Shape());if(!cut.IsDone())return 4;
 ShapeUpgrade_UnifySameDomain unify(cut.Shape(),true,true,false);unify.Build();auto shape=unify.Shape();if(!BRepCheck_Analyzer(shape).IsValid())return 5;
 GProp_GProps volume,area;BRepGProp::VolumeProperties(shape,volume);BRepGProp::SurfaceProperties(shape,area);int faces=0,planes=0,cylinders=0;for(TopExp_Explorer it(shape,TopAbs_FACE);it.More();it.Next()){++faces;BRepAdaptor_Surface s(TopoDS::Face(it.Current()));if(s.GetType()==GeomAbs_Plane)++planes;if(s.GetType()==GeomAbs_Cylinder)++cylinders;}
 STEPControl_Writer writer;if(writer.Transfer(shape,STEPControl_AsIs)!=IFSelect_RetDone||writer.Write(argv[2])!=IFSelect_RetDone)return 6;
 auto c=volume.CentreOfMass();std::cout<<std::setprecision(17)<<"{\"Faces\":"<<faces<<",\"Planes\":"<<planes<<",\"Cylinders\":"<<cylinders<<",\"Volume\":"<<volume.Mass()<<",\"Area\":"<<area.Mass()<<",\"Center\":["<<c.X()<<","<<c.Y()<<","<<c.Z()<<"]}\n";
}
