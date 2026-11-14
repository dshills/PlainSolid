#include <BRepAlgoAPI_Cut.hxx>
#include <BRepBuilderAPI_MakeVertex.hxx>
#include <BRepCheck_Analyzer.hxx>
#include <BRepExtrema_DistShapeShape.hxx>
#include <BRepPrimAPI_MakeCone.hxx>
#include <BRepPrimAPI_MakeCylinder.hxx>
#include <TopExp_Explorer.hxx>
#include <TopoDS_Compound.hxx>
#include <BRep_Builder.hxx>
#include <gp_Ax2.hxx>
#include <iomanip>
#include <iostream>

int main() {
  const gp_Dir vertical(0, 0, 1);
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
