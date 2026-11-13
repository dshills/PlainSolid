// Standalone optional fixture generator; intentionally independent of the
// native analytic seam construction. Link against the pinned OCCT STEP build.
#include <BRepPrimAPI_MakeSphere.hxx>
#include <BRepPrimAPI_MakeCylinder.hxx>
#include <BRepAlgoAPI_Cut.hxx>
#include <BRepCheck_Analyzer.hxx>
#include <STEPControl_Writer.hxx>
#include <gp_Ax2.hxx>

int main(int argc, char** argv) {
  if (argc != 2) return 1;
  const auto sphere = BRepPrimAPI_MakeSphere(5).Shape();
  const auto tool = BRepPrimAPI_MakeCylinder(
      gp_Ax2(gp_Pnt(1, 0, -.75), gp_Dir(0, 0, 1)), 2, 7).Shape();
  const auto shape = BRepAlgoAPI_Cut(sphere, tool).Shape();
  if (!BRepCheck_Analyzer(shape).IsValid()) return 2;
  STEPControl_Writer writer;
  if (writer.Transfer(shape, STEPControl_AsIs) != IFSelect_RetDone) return 3;
  return writer.Write(argv[1]) == IFSelect_RetDone ? 0 : 4;
}
