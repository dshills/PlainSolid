#include <BRepAlgoAPI_Common.hxx>
#include <BRepAlgoAPI_Cut.hxx>
#include <BRepBuilderAPI_MakeFace.hxx>
#include <BRepCheck_Analyzer.hxx>
#include <BRepGProp.hxx>
#include <BRepPrimAPI_MakeBox.hxx>
#include <BRepPrimAPI_MakeHalfSpace.hxx>
#include <BRepPrimAPI_MakePrism.hxx>
#include <Geom_BSplineSurface.hxx>
#include <GProp_GProps.hxx>
#include <NCollection_Array1.hxx>
#include <NCollection_Array2.hxx>
#include <TopExp.hxx>
#include <NCollection_IndexedMap.hxx>
#include <TopTools_ShapeMapHasher.hxx>
#include <TopoDS_Face.hxx>
#include <TopoDS_Solid.hxx>
#include <gp_Pln.hxx>
#include <iomanip>
#include <iostream>
#include <string_view>

static TopoDS_Shape cubicStock() {
  // Sweep OCCT's independently authored surface z=x³/8+y/4 upward.
  NCollection_Array2<gp_Pnt> poles(1, 4, 1, 2);
  for (int i = 1; i <= 4; ++i)
    for (int j = 1; j <= 2; ++j) poles.SetValue(i, j, gp_Pnt((i-1)/3., j-1, (i==4 ? .125 : 0.)+(j-1)/4.));
  NCollection_Array1<double> knots(1, 2);
  knots.SetValue(1, 0);
  knots.SetValue(2, 1);
  NCollection_Array1<int> uMult(1, 2), vMult(1, 2);
  uMult.Init(4);
  vMult.Init(2);
  occ::handle<Geom_BSplineSurface> s = new Geom_BSplineSurface(poles, knots, knots, uMult, vMult, 3, 1);
  return BRepPrimAPI_MakePrism(BRepBuilderAPI_MakeFace(s, 1e-10).Face(), gp_Vec(0, 0, 1)).Shape();
}

static TopoDS_Shape twoStripStock() {
  NCollection_Array2<gp_Pnt> poles(1, 3, 1, 2);
  for (int i = 1; i <= 3; ++i)
    for (int j = 1; j <= 2; ++j) poles.SetValue(i, j, gp_Pnt((i-1)/2., j-1, (i==2 ? -1./32 : 1./32)+(j-1)/16.));
  NCollection_Array1<double> knots(1, 2);
  knots.SetValue(1, 0);
  knots.SetValue(2, 1);
  NCollection_Array1<int> uMult(1, 2), vMult(1, 2);
  uMult.Init(3);
  vMult.Init(2);
  occ::handle<Geom_BSplineSurface> s = new Geom_BSplineSurface(poles, knots, knots, uMult, vMult, 2, 1);
  return BRepPrimAPI_MakePrism(BRepBuilderAPI_MakeFace(s, 1e-10).Face(), gp_Vec(0, 0, 1)).Shape();
}

int main(int argc, char** argv) {
  if (argc > 2) return 3;
  const std::string_view mode = argc == 2 ? argv[1] : "";
  if (!mode.empty() && mode != "curved-band" && mode != "crossing-band" && mode != "sleeve" && mode != "two-strips") return 3;
  const bool crossing = mode == "crossing-band";
  const bool twoStrips = mode == "two-strips";
  const bool curved = crossing || mode == "curved-band" || twoStrips;
  const bool sleeve = mode == "sleeve";
  TopoDS_Shape stock = twoStrips ? twoStripStock() : curved ? cubicStock() : BRepPrimAPI_MakeBox(1, 1, 1).Shape();
  if (sleeve) {
    const auto body = BRepPrimAPI_MakeBox(3, 3, 1).Shape();
    const auto hole = BRepPrimAPI_MakeBox(gp_Pnt(1, 1, -1), gp_Pnt(2, 2, 2)).Shape();
    BRepAlgoAPI_Cut boring(body, hole);
    if (!boring.IsDone()) return 6;
    stock = boring.Shape();
  }
  if (!BRepCheck_Analyzer(stock).IsValid()) return 4;
  const gp_Pnt inside = twoStrips ? gp_Pnt(.5, .25, .046875) : sleeve ? gp_Pnt(2.5, .5, .5625) : crossing ? gp_Pnt(.75, .125, .1953125) :
                        curved ? gp_Pnt(.5, .125, .203125) : gp_Pnt(.875, .125, .6875);
  const gp_Pln wedgePlanes[] = {
    gp_Pln(gp_Pnt(0, 0, .375), gp_Dir(-.25, -.25, 1)),
    gp_Pln(gp_Pnt(0, 0, .75), gp_Dir(0, 0, 1)),
    gp_Pln(gp_Pnt(.75, 0, 0), gp_Dir(1, 0, 0))
  };
  const gp_Pln bandPlanes[] = {
    gp_Pln(gp_Pnt(0, 0, .1875), gp_Dir(0, 0, 1)),
    crossing ? gp_Pln(gp_Pnt(0, 0, .15625), gp_Dir(-1./16, 0, 1)) :
               gp_Pln(gp_Pnt(0, 0, .21875), gp_Dir(-1./128, 0, 1)),
  };
  const gp_Pln sleevePlanes[] = {
    gp_Pln(gp_Pnt(0, 0, .5), gp_Dir(0, 0, 1)),
    gp_Pln(gp_Pnt(0, 0, .3125), gp_Dir(-.125, 0, 1)),
  };
  const gp_Pln stripPlanes[] = {
    gp_Pln(gp_Pnt(0, 0, 5./128), gp_Dir(0, 0, 1)),
    gp_Pln(gp_Pnt(0, 0, 9./128), gp_Dir(0, 1./16, 1)),
  };
  for (int i = 0; i < (curved || sleeve ? 2 : 3); ++i) {
    const auto face = BRepBuilderAPI_MakeFace(twoStrips ? stripPlanes[i] : sleeve ? sleevePlanes[i] : curved ? bandPlanes[i] : wedgePlanes[i]).Face();
    const auto tool = BRepPrimAPI_MakeHalfSpace(face, inside).Solid();
    BRepAlgoAPI_Common cut(stock, tool);
    if (!cut.IsDone() || cut.Shape().IsNull() || !BRepCheck_Analyzer(cut.Shape()).IsValid()) return 1;
    stock = cut.Shape();
  }
  NCollection_IndexedMap<TopoDS_Shape, TopTools_ShapeMapHasher> solids, faces, edges, vertices;
  TopExp::MapShapes(stock, TopAbs_SOLID, solids);
  TopExp::MapShapes(stock, TopAbs_FACE, faces);
  TopExp::MapShapes(stock, TopAbs_EDGE, edges);
  TopExp::MapShapes(stock, TopAbs_VERTEX, vertices);
  if (solids.Extent() != 1) return 2;
  GProp_GProps m, a;
  if (curved) {
    if (BRepGProp::VolumePropertiesGK(stock, m, 1e-12, true, true, true) < 0.) return 5;
  } else {
    BRepGProp::VolumeProperties(stock, m, 1e-12);
  }
  BRepGProp::SurfaceProperties(stock, a, 1e-12);
  const gp_Pnt c = m.CentreOfMass();
  std::cout << std::setprecision(17) << '[' << m.Mass() << ',' << a.Mass()
            << ',' << c.X() << ',' << c.Y() << ',' << c.Z() << ',' << vertices.Extent()
            << ',' << edges.Extent() << ',' << faces.Extent() << "]\n";
}
