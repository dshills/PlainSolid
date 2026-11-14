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
#include <STEPControl_Reader.hxx>
#include <iostream>
#include <iomanip>
#include <string>
int main(int argc,char**argv){if(argc!=3)return 2;std::string mode=argv[1];if(mode!="unequal-center-side"&&mode!="unequal-center-through"&&mode!="offset-side"&&mode!="offset-through"&&mode!="side"&&mode!="interior"&&mode!="read"&&mode!="side-through"&&mode!="unequal-side"&&mode!="unequal-through"&&mode!="general-side"&&mode!="general-through"&&mode!="exact-wall-side"&&mode!="exact-wall-through")return 2;
 TopoDS_Shape shape; if(mode=="read"){STEPControl_Reader reader;if(reader.ReadFile(argv[2])!=IFSelect_RetDone||reader.TransferRoots()!=1)return 7;shape=reader.OneShape();}else{
 bool unequalCenter=mode=="unequal-center-side"||mode=="unequal-center-through"; bool offset=mode=="offset-side"||mode=="offset-through"; bool exactWall=mode=="exact-wall-side"||mode=="exact-wall-through",general=mode=="general-side"||mode=="general-through", unequal=exactWall||general||mode=="unequal-side"||mode=="unequal-through",through=mode=="unequal-center-through"||mode=="offset-through"||mode=="side-through"||mode=="unequal-through"||mode=="general-through"||mode=="exact-wall-through";
 double y=unequalCenter?0:offset?0:unequal?0:(mode!="interior"?4:0),dy=(unequalCenter||offset)?10:general?12:unequal?22:(mode!="interior"?6:8);
 auto stock=BRepPrimAPI_MakeBox(gp_Pnt(0,y,0),unequalCenter?60:offset?40:(general||exactWall)?50:unequal?60:10,dy,6).Shape();
 auto a=BRepPrimAPI_MakeCylinder(gp_Ax2(gp_Pnt(unequalCenter?7:offset?12.5:unequal?20:3.5,unequalCenter?-3:offset?-7.5:unequal?0:4,through?0:3),gp_Dir(0,0,1)),unequalCenter?5:offset?65./8:general?5:unequal?13:2.5,through?7:4).Shape();
 auto b=BRepPrimAPI_MakeCylinder(gp_Ax2(gp_Pnt(unequalCenter?30:offset?22:exactWall?24:general?24.5:unequal?34:6.5,unequalCenter?-20:offset?-7.5:unequal?0:4,through?0:3),gp_Dir(0,0,1)),unequalCenter?29:offset?87./8:general?8.5:unequal?15:2.5,through?7:4).Shape();
 BRepAlgoAPI_Fuse fuse(a,b);if(!fuse.IsDone())return 3;BRepAlgoAPI_Cut cut(stock,fuse.Shape());if(!cut.IsDone())return 4;
 ShapeUpgrade_UnifySameDomain unify(cut.Shape(),true,true,false);unify.Build();shape=unify.Shape();}if(!BRepCheck_Analyzer(shape).IsValid())return 5;
 GProp_GProps volume,area;BRepGProp::VolumeProperties(shape,volume);BRepGProp::SurfaceProperties(shape,area);int faces=0,planes=0,cylinders=0;for(TopExp_Explorer it(shape,TopAbs_FACE);it.More();it.Next()){++faces;BRepAdaptor_Surface s(TopoDS::Face(it.Current()));if(s.GetType()==GeomAbs_Plane)++planes;if(s.GetType()==GeomAbs_Cylinder)++cylinders;}
 if(mode!="read"){STEPControl_Writer writer;if(writer.Transfer(shape,STEPControl_AsIs)!=IFSelect_RetDone||writer.Write(argv[2])!=IFSelect_RetDone)return 6;}
 auto c=volume.CentreOfMass();std::cout<<std::setprecision(17)<<"{\"Faces\":"<<faces<<",\"Planes\":"<<planes<<",\"Cylinders\":"<<cylinders<<",\"Volume\":"<<volume.Mass()<<",\"Area\":"<<area.Mass()<<",\"Center\":["<<c.X()<<","<<c.Y()<<","<<c.Z()<<"]}\n";
}
