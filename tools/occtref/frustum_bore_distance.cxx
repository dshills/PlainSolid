#include <BRepAlgoAPI_Cut.hxx>
#include <BRepBuilderAPI_MakeVertex.hxx>
#include <BRepCheck_Analyzer.hxx>
#include <BRepExtrema_DistShapeShape.hxx>
#include <BRepGProp.hxx>
#include <BRepPrimAPI_MakeCone.hxx>
#include <BRepPrimAPI_MakeCylinder.hxx>
#include <TopExp_Explorer.hxx>
#include <TopoDS_Compound.hxx>
#include <BRep_Builder.hxx>
#include <gp_Ax2.hxx>
#include <GProp_GProps.hxx>
#include <iomanip>
#include <iostream>
#include <string>

int main(int argc, char** argv) {
  const gp_Dir vertical(0, 0, 1);
  if(argc==2 && std::string(argv[1])=="mass") {
    std::cout<<std::setprecision(17)<<'[';
    for(int cavity=0;cavity<2;++cavity) {
      const double offset=cavity?.25:.5;
      auto stock=cavity ? BRepPrimAPI_MakeCone(gp_Ax2(gp_Pnt(0,0,0),vertical),2,5,4).Shape() :
          BRepPrimAPI_MakeCylinder(gp_Ax2(gp_Pnt(0,0,0),vertical),3,4).Shape();
      auto tool=BRepPrimAPI_MakeCylinder(gp_Ax2(gp_Pnt(offset,offset,cavity?.5:-1),vertical),1,cavity?3:6).Shape();
      auto b=BRepAlgoAPI_Cut(stock,tool);
      if(!b.IsDone() || !BRepCheck_Analyzer(b.Shape()).IsValid()) return 1;
      GProp_GProps m,a;
      BRepGProp::VolumeProperties(b.Shape(),m,1e-12);
      BRepGProp::SurfaceProperties(b.Shape(),a,1e-12);
      const auto p=m.CentreOfMass();
      const auto inertia=m.MatrixOfInertia();
      if(cavity) std::cout<<',';
      std::cout<<'['<<m.Mass()<<','<<a.Mass()<<','<<p.X()<<','<<p.Y()<<','<<p.Z();
      for(int i=1;i<=3;++i)for(int j=1;j<=3;++j) std::cout<<','<<inertia.Value(i,j);
      std::cout<<']';
    }
    std::cout<<"]\n";
    return 0;
  }
  auto stock = BRepPrimAPI_MakeCone(gp_Ax2(gp_Pnt(0, 0, 0), vertical), 3, 4, 4).Shape();
  const gp_Pnt probes[5]{gp_Pnt(.25,.25,5), gp_Pnt(0,0,5), gp_Pnt(2,0,5), gp_Pnt(.25,.25,.75), gp_Pnt(.25,.25,3.125)};
  double values[5]{};
  for (int cavity = 0; cavity < 2; ++cavity) {
    const double base = cavity ? .5 : -1;
    auto tool = BRepPrimAPI_MakeCylinder(gp_Ax2(gp_Pnt(.25,.25,base),vertical),1,cavity ? 3 : 6).Shape();
    auto cut = BRepAlgoAPI_Cut(stock,tool);
    if(!cut.IsDone() || !BRepCheck_Analyzer(cut.Shape()).IsValid()) return 1;
    // A compound of the native faces measures the material boundary even
    // when the probe lies inside the void of a closed cavity.
    TopoDS_Compound boundary;
    BRep_Builder builder;
    builder.MakeCompound(boundary);
    for(TopExp_Explorer f(cut.Shape(),TopAbs_FACE);f.More();f.Next()) builder.Add(boundary,f.Current());
    for(int i=cavity?3:0;i<(cavity?5:3);++i) {
      auto v=BRepBuilderAPI_MakeVertex(probes[i]).Vertex();
      BRepExtrema_DistShapeShape distance(v,boundary,1e-11);
      if(!distance.IsDone()) return 2;
      values[i]=distance.Value()*distance.Value();
    }
  }
  std::cout<<std::setprecision(17)<<'['<<values[0]<<','<<values[1]<<','<<values[2]<<','<<values[3]<<','<<values[4]<<"]\n";
}
