// Optional independently authored direct IGES composite wire oracle.
#include <IGESControl_Writer.hxx>
#include <IGESControl_Reader.hxx>
#include <IGESGeom_CompositeCurve.hxx>
#include <IGESGeom_Line.hxx>
#include <IGESGeom_CircularArc.hxx>
#include <NCollection_HArray1.hxx>
#include <BRepCheck_Analyzer.hxx>
#include <BRepGProp.hxx>
#include <GProp_GProps.hxx>
#include <TopExp_Explorer.hxx>
#include <Standard_Failure.hxx>
#include <gp_XY.hxx>
#include <gp_XYZ.hxx>
#include <iostream>
#include <iomanip>
#include <string>
int main(int argc,char**argv){if(argc!=3)return 2;try{
 if(std::string(argv[1])=="write"){
  occ::handle<IGESGeom_Line> line=new IGESGeom_Line;line->Init(gp_XYZ(0,0,0),gp_XYZ(1,0,0));
  occ::handle<IGESGeom_CircularArc> arc=new IGESGeom_CircularArc;arc->Init(0,gp_XY(1,1),gp_XY(1,0),gp_XY(2,1));
  occ::handle<NCollection_HArray1<occ::handle<IGESData_IGESEntity>>> leaves=new NCollection_HArray1<occ::handle<IGESData_IGESEntity>>(1,2);leaves->SetValue(1,line);leaves->SetValue(2,arc);
  occ::handle<IGESGeom_CompositeCurve> root=new IGESGeom_CompositeCurve;root->Init(leaves);IGESControl_Writer writer("MM",0);if(!writer.AddEntity(root))return 3;return writer.Write(argv[2])?0:4;
 }
 IGESControl_Reader reader;if(reader.ReadFile(argv[2])!=IFSelect_RetDone||reader.TransferRoots()==0)return 5;auto shape=reader.OneShape();if(shape.IsNull()||!BRepCheck_Analyzer(shape).IsValid())return 6;
 int edges=0,wires=0;for(TopExp_Explorer x(shape,TopAbs_EDGE);x.More();x.Next())++edges;for(TopExp_Explorer x(shape,TopAbs_WIRE);x.More();x.Next())++wires;
 GProp_GProps p;BRepGProp::LinearProperties(shape,p);std::cout<<std::setprecision(17)<<"{\"Edges\":"<<edges<<",\"Wires\":"<<wires<<",\"Length\":"<<p.Mass()<<"}\n";return 0;
}catch(const Standard_Failure&e){std::cerr<<e.what();return 7;}}
