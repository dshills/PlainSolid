// Optional development oracle. This file is not part of the Go library.
#include <BRepAlgoAPI_Common.hxx>
#include <BRepAlgoAPI_Cut.hxx>
#include <BRepAlgoAPI_Fuse.hxx>
#include <BRepCheck_Analyzer.hxx>
#include <BRepBuilderAPI_Transform.hxx>
#include <BRepBuilderAPI_GTransform.hxx>
#include <BRepBuilderAPI_MakeFace.hxx>
#include <BRepBuilderAPI_MakeEdge.hxx>
#include <BRepBuilderAPI_MakeWire.hxx>
#include <BRepBuilderAPI_Sewing.hxx>
#include <BRepBuilderAPI_MakeSolid.hxx>
#include <TopoDS.hxx>
#include <gp_GTrsf.hxx>
#include <gp_Ax1.hxx>
#include <gp_Ax2.hxx>
#include <gp_Dir.hxx>
#include <gp_Trsf.hxx>
#include <BRepGProp.hxx>
#include <BRepPrimAPI_MakeBox.hxx>
#include <BRepPrimAPI_MakeCone.hxx>
#include <BRepPrimAPI_MakeCylinder.hxx>
#include <BRepPrimAPI_MakeSphere.hxx>
#include <BRepPrimAPI_MakeTorus.hxx>
#include <BRepPrimAPI_MakePrism.hxx>
#include <GProp_GProps.hxx>
#include <Standard_Failure.hxx>
#include <TopoDS_Shape.hxx>
#include <gp_Pnt.hxx>
#include <gp_Circ.hxx>
#ifdef PLAINSOLID_OCCT_STEP
#include <STEPControl_Reader.hxx>
#include <Geom_Parabola.hxx>
#include <Geom_Hyperbola.hxx>
#include <Interface_Static.hxx>
#include <BRepPrimAPI_MakeHalfSpace.hxx>
#include <gp_Pln.hxx>
#include <STEPControl_Writer.hxx>
#include <STEPCAFControl_Writer.hxx>
#include <STEPCAFControl_Reader.hxx>
#include <TDF_Tool.hxx>
#include <TCollection_AsciiString.hxx>
#include <TDocStd_Document.hxx>
#include <XCAFDoc_DocumentTool.hxx>
#include <XCAFDoc_ShapeTool.hxx>
#include <XCAFDoc_ColorTool.hxx>
#include <Quantity_Color.hxx>
#include <TDataStd_Name.hxx>
#include <TCollection_ExtendedString.hxx>
#include <TopLoc_Location.hxx>
#endif

#if defined(PLAINSOLID_OCCT_STEP) || defined(PLAINSOLID_OCCT_IGES)
#include <Geom_BSplineSurface.hxx>
#include <Geom2d_Line.hxx>
#include <BRepLib.hxx>
#include <gp_Pnt2d.hxx>
#include <gp_Dir2d.hxx>
#include <NCollection_Array2.hxx>
#include <TopoDS_Face.hxx>
#include <Geom_BSplineCurve.hxx>
#include <NCollection_Array1.hxx>
#include <TopoDS_Edge.hxx>
#include <TopExp_Explorer.hxx>
#endif

#ifdef PLAINSOLID_OCCT_IGES
#include <GeomToIGES_GeomSurface.hxx>
#include <GeomToIGES_GeomCurve.hxx>
#include <IGESGeom_CurveOnSurface.hxx>
#include <IGESGeom_TrimmedSurface.hxx>
#include <IGESGeom_TransformationMatrix.hxx>
#include <IGESData_IGESModel.hxx>
#include <NCollection_HArray2.hxx>
#include <NCollection_HArray1.hxx>
#include <IGESData_IGESEntity.hxx>
#include <IGESControl_Reader.hxx>
#include <IGESControl_Writer.hxx>
#include <gp_Dir.hxx>
#endif

#include <cmath>
#include <fstream>
#include <iomanip>
#include <iostream>
#include <stdexcept>
#include <string>
#include <set>
#include <functional>
#include <vector>
#include "iges_quadratic_trim.hxx"
#include "iges_curved_oval.hxx"
#include "wrapped_fillet_exchange.hxx"
#include "wrapped_station_bore_exchange.hxx"
#include "curved_oval_exchange.hxx"
#include "spline_lens_exchange.hxx"
#include "polygon_revolve.hxx"
#ifdef PLAINSOLID_OCCT_STEP
#include "long_edge_chain.hxx"
#endif
#ifdef PLAINSOLID_OCCT_IGES
#include "iges_spline_lens.hxx"
#endif
#include <array>

static void record(std::ostream& out, const char* name, const TopoDS_Shape& shape, double integrationError = 0) {
  if (shape.IsNull() || !BRepCheck_Analyzer(shape).IsValid())
    throw std::runtime_error(std::string("invalid reference shape: ") + name);
  GProp_GProps volume, area;
  if (integrationError > 0) {
    BRepGProp::VolumeProperties(shape, volume, integrationError, true);
    BRepGProp::SurfaceProperties(shape, area, integrationError);
  } else {
    BRepGProp::VolumeProperties(shape, volume);
    BRepGProp::SurfaceProperties(shape, area);
  }
  const gp_Pnt c = volume.CentreOfMass();
  out << std::setprecision(17) << "{\"Name\":\"" << name
      << "\",\"Area\":" << area.Mass() << ",\"Volume\":" << volume.Mass()
      << ",\"Centroid\":[" << c.X() << ',' << c.Y() << ',' << c.Z() << "]}\n";
}

static TopoDS_Shape roundRectanglePrism(double x, double y, double r, double h) {
  BRepBuilderAPI_MakeWire wire;
  const double pi = std::acos(-1.0);
  const double corners[4][2] = {{x-r,r},{x-r,y-r},{r,y-r},{r,r}};
  for (int i = 0; i < 4; ++i) {
    const double begin = (i-1)*pi/2, end = i*pi/2;
    const auto& c = corners[i];
    const gp_Circ circle(gp_Ax2(gp_Pnt(c[0],c[1],0),gp_Dir(0,0,1),gp_Dir(1,0,0)),r);
    wire.Add(BRepBuilderAPI_MakeEdge(circle,begin,end).Edge());
    const auto& next = corners[(i+1)%4];
    const gp_Pnt a(c[0]+r*std::cos(end),c[1]+r*std::sin(end),0);
    const gp_Pnt b(next[0]+r*std::cos(end),next[1]+r*std::sin(end),0);
    wire.Add(BRepBuilderAPI_MakeEdge(a,b).Edge());
  }
  return BRepPrimAPI_MakePrism(BRepBuilderAPI_MakeFace(wire.Wire()).Face(),gp_Vec(0,0,h)).Shape();
}

static TopoDS_Shape branch(const int (&cells)[4][3], const double (&m)[3][4]) {
  TopoDS_Shape result;
  for (const auto& cell : cells) {
    auto piece = BRepPrimAPI_MakeBox(gp_Pnt(cell[0], cell[1], cell[2]), 1, 1, 1).Shape();
    result = result.IsNull() ? piece : BRepAlgoAPI_Fuse(result, piece).Shape();
  }
  gp_GTrsf transform;
  for (int i = 0; i < 3; ++i) {
    for (int j = 0; j < 4; ++j) transform.SetValue(i + 1, j + 1, m[i][j]);
  }
  return BRepBuilderAPI_GTransform(result, transform, true).Shape();
}

#if defined(PLAINSOLID_OCCT_STEP) || defined(PLAINSOLID_OCCT_IGES)
static TopoDS_Edge quarterCircle() {
  NCollection_Array1<gp_Pnt> poles(1, 3);
  poles.SetValue(1, gp_Pnt(1, 0, 0));
  poles.SetValue(2, gp_Pnt(1, 1, 0));
  poles.SetValue(3, gp_Pnt(0, 1, 0));
  NCollection_Array1<double> weights(1, 3), knots(1, 2);
  NCollection_Array1<int> multiplicities(1, 2);
  weights.SetValue(1, 1);
  weights.SetValue(2, std::sqrt(0.5));
  weights.SetValue(3, 1);
  knots.SetValue(1, 0);
  knots.SetValue(2, 1);
  multiplicities.Init(3);
  return BRepBuilderAPI_MakeEdge(
      new Geom_BSplineCurve(poles, weights, knots, multiplicities, 2)).Edge();
}
static occ::handle<Geom_BSplineSurface> quarterCylinder() {
  NCollection_Array2<gp_Pnt> poles(1, 3, 1, 2);
  NCollection_Array2<double> weights(1, 3, 1, 2);
  for (int j = 1; j <= 2; ++j) {
    const double z = 2.0 * (j - 1);
    poles.SetValue(1, j, gp_Pnt(1, 0, z));
    poles.SetValue(2, j, gp_Pnt(1, 1, z));
    poles.SetValue(3, j, gp_Pnt(0, 1, z));
    weights.SetValue(1, j, 1);
    weights.SetValue(2, j, std::sqrt(0.5));
    weights.SetValue(3, j, 1);
  }
  NCollection_Array1<double> knots(1, 2);
  knots.SetValue(1, 0);
  knots.SetValue(2, 1);
  NCollection_Array1<int> um(1, 2), vm(1, 2);
  um.Init(3);
  vm.Init(2);
  return new Geom_BSplineSurface(poles, weights, knots, knots, um, vm, 2, 1);
}

#endif

#include "draft.hxx"
#include "feature.hxx"
#include "spline_shell_boolean.hxx"
#include "warped_open_spline.hxx"
#include "simultaneous_spline_shell.hxx"
#include "repeated_spline_shell.hxx"
#include "transverse_spline_shell.hxx"
#include "two_contour_spline_shell.hxx"
#include "selective_fillet.hxx"
#include "graph_boolean.hxx"

int main(int argc, char** argv) {
  try {
    if (argc < 2 || argc > 4)
      throw std::runtime_error("usage: occtref output [input.step|write-step|write-step-curves|step-curves input.step|write-step-surface|write-step-sphere|write-step-torus|write-step-cylinder|write-step-cone|write-step-frustum|step-surface input.step|write-iges|write-iges-surface|iges input.igs|iges-surface input.igs]");
    const std::string mode = argc >= 3 ? argv[2] : "";
    if(argc==3 && mode=="warped-open-spline"){std::ofstream out(argv[1]);out.exceptions(std::ios::failbit|std::ios::badbit);warpedOpenSplineBooleans(out);return 0;}
    if(argc==3 && mode=="finite-cylinder-sets") {
      std::ofstream out(argv[1]);out.exceptions(std::ios::failbit|std::ios::badbit);
      const auto stock=BRepPrimAPI_MakeCylinder(gp_Ax2(gp_Pnt(13,9,2),gp_Dir(-3,-4,0)),2,30).Shape();
      for(int wide=0;wide<2;++wide){
        const auto tool=BRepPrimAPI_MakeCylinder(gp_Ax2(gp_Pnt(-8,6,2),gp_Dir(4,-3,0)),wide?3:2,30).Shape();
        if(!wide)record(out,"finite-raw-union",BRepAlgoAPI_Fuse(stock,tool).Shape(),1e-11);
        const auto cut=BRepAlgoAPI_Cut(stock,tool).Shape();
        record(out,wide?"finite-raw-wide-cut":"finite-raw-cut",cut,1e-11);
        int count=0;for(TopExp_Explorer it(cut,TopAbs_SOLID);it.More();it.Next()){
          GProp_GProps properties;BRepGProp::VolumeProperties(it.Current(),properties,1e-11);
          const gp_Pnt c=properties.CentreOfMass();bool minus=-3*(c.X()-4)-4*(c.Y()+3)<0;
          record(out,wide?(minus?"finite-raw-wide-minus":"finite-raw-wide-plus"):(minus?"finite-raw-minus":"finite-raw-plus"),it.Current(),1e-11);++count;
        }
        if(count!=2)throw std::runtime_error("raw cylinder difference component count");
      }
      const auto separatedA=BRepPrimAPI_MakeCylinder(gp_Ax2(gp_Pnt(0,0,-2),gp_Dir(0,0,1)),2,4).Shape();
      for(int axial=0;axial<2;++axial){
        const auto separatedB=BRepPrimAPI_MakeCylinder(gp_Ax2(axial?gp_Pnt(0,0,11):gp_Pnt(4,0,3),gp_Dir(0,0,-1)),1,6).Shape();
        const auto combined=BRepAlgoAPI_Fuse(separatedA,separatedB).Shape();
        int count=0;for(TopExp_Explorer it(combined,TopAbs_SOLID);it.More();it.Next())++count;
        if(count!=2)throw std::runtime_error("separated finite union did not retain two solids");
        record(out,axial?"finite-separated-axial":"finite-separated-radial",combined,1e-11);
      }
      const auto unequalWide=BRepPrimAPI_MakeCylinder(gp_Ax2(gp_Pnt(1,2,7),gp_Dir(0,0,-1)),5,8).Shape();
      const auto unequalNarrow=BRepPrimAPI_MakeCylinder(gp_Ax2(gp_Pnt(1,-4,3),gp_Dir(0,1,0)),3,12).Shape();
      const auto unequalUnion=BRepAlgoAPI_Fuse(unequalWide,unequalNarrow).Shape();
      int unequalCount=0;for(TopExp_Explorer it(unequalUnion,TopAbs_SOLID);it.More();it.Next())++unequalCount;
      if(unequalCount!=1)throw std::runtime_error("unequal finite union not one solid");
      record(out,"finite-unequal-union",unequalUnion,1e-11);
      const auto unequalCut=BRepAlgoAPI_Cut(unequalWide,unequalNarrow).Shape();
      int cutSolids=0,cutShells=0;
      for(TopExp_Explorer it(unequalCut,TopAbs_SOLID);it.More();it.Next())++cutSolids;
      for(TopExp_Explorer it(unequalCut,TopAbs_SHELL);it.More();it.Next())++cutShells;
      if(cutSolids!=1||cutShells!=1)throw std::runtime_error("unequal finite cut not one solid with one shell");
      record(out,"finite-unequal-cut",unequalCut,1e-11);
      const auto coaxialTool=BRepPrimAPI_MakeCylinder(gp_Ax2(gp_Pnt(1,2,-4),gp_Dir(0,0,1)),3,15).Shape();
      const auto coaxialCut=BRepAlgoAPI_Cut(unequalWide,coaxialTool).Shape();
      int coaxialSolids=0,coaxialShells=0;
      for(TopExp_Explorer it(coaxialCut,TopAbs_SOLID);it.More();it.Next())++coaxialSolids;
      for(TopExp_Explorer it(coaxialCut,TopAbs_SHELL);it.More();it.Next())++coaxialShells;
      if(coaxialSolids!=1||coaxialShells!=1)throw std::runtime_error("coaxial finite cut not one solid with one shell");
      record(out,"finite-coaxial-cut",coaxialCut,1e-11);
      return 0;
    }
    if (argc == 3 && mode == "finite-cylinder-dispatch") {
      std::ofstream out(argv[1]); out.exceptions(std::ios::failbit | std::ios::badbit);
      // Independently authored finite cylinders: physical starts and lengths,
      // not the Go dispatcher's chart construction or intersection formulas.
      auto x = [](double lo, double hi) { return BRepPrimAPI_MakeCylinder(gp_Ax2(gp_Pnt(lo,0,0),gp_Dir(1,0,0)),5,hi-lo).Shape(); };
      auto z = [](double lo, double hi) { return BRepPrimAPI_MakeCylinder(gp_Ax2(gp_Pnt(0,0,lo),gp_Dir(0,0,1)),5,hi-lo).Shape(); };
      record(out,"finite-pair-cap",BRepAlgoAPI_Common(x(-6,2),z(-6,6)).Shape(),1e-11);
      record(out,"finite-pair-slab",BRepAlgoAPI_Common(x(-3,2),z(-6,6)).Shape(),1e-11);
      record(out,"finite-pair-quadrant",BRepAlgoAPI_Common(x(0,6),z(-6,0)).Shape(),1e-11);
      record(out,"finite-pair-dual-cap",BRepAlgoAPI_Common(x(-6,2),z(-6,3)).Shape(),1e-11);
      record(out,"finite-pair-dual-cap-tail",BRepAlgoAPI_Common(x(2,6),z(-6,-3)).Shape(),1e-11);
      record(out,"finite-pair-four-cap",BRepAlgoAPI_Common(x(-1,2),z(-3,4)).Shape(),1e-11);
      record(out,"finite-pair-four-cap-tail",BRepAlgoAPI_Common(x(1,2),z(-4,-3)).Shape(),1e-11);
      const auto unequalX = BRepPrimAPI_MakeCylinder(gp_Ax2(gp_Pnt(-2,0,0),gp_Dir(1,0,0)),3,6.5).Shape();
      const auto unequalZ = BRepPrimAPI_MakeCylinder(gp_Ax2(gp_Pnt(0,0,-2),gp_Dir(0,0,1)),5,3).Shape();
      record(out,"finite-pair-unequal-window",BRepAlgoAPI_Common(unequalX,unequalZ).Shape(),1e-11);
      const auto swappedX = BRepPrimAPI_MakeCylinder(gp_Ax2(gp_Pnt(.5,0,0),gp_Dir(1,0,0)),5,1.5).Shape();
      const auto swappedZ = BRepPrimAPI_MakeCylinder(gp_Ax2(gp_Pnt(0,0,-4.5),gp_Dir(0,0,1)),3,8).Shape();
      record(out,"finite-pair-unequal-swapped",BRepAlgoAPI_Common(swappedX,swappedZ).Shape(),1e-11);
      const auto obliqueParallelA = BRepPrimAPI_MakeCylinder(gp_Ax2(gp_Pnt(0,0,0),gp_Dir(3,4,0)),5,10).Shape();
      const auto obliqueParallelB = BRepPrimAPI_MakeCylinder(gp_Ax2(gp_Pnt(6,8,5),gp_Dir(-3,-4,0)),5,5).Shape();
      record(out,"finite-pair-oblique-parallel",BRepAlgoAPI_Common(obliqueParallelA,obliqueParallelB).Shape(),1e-11);
      const auto containedA = BRepPrimAPI_MakeCylinder(gp_Ax2(gp_Pnt(-2,3,4),gp_Dir(1,0,0)),2,8).Shape();
      const auto containedB = BRepPrimAPI_MakeCylinder(gp_Ax2(gp_Pnt(2,4,6),gp_Dir(0,0,-1)),5,4).Shape();
      record(out,"finite-pair-contained-skew",BRepAlgoAPI_Common(containedA,containedB).Shape(),1e-11);
      const auto signedA = BRepPrimAPI_MakeCylinder(gp_Ax2(gp_Pnt(1,2,-1),gp_Dir(0,0,-1)),5,5).Shape();
      const auto signedB = BRepPrimAPI_MakeCylinder(gp_Ax2(gp_Pnt(1,-4,-3),gp_Dir(0,1,0)),5,12).Shape();
      record(out,"finite-pair-signed-scaled",BRepAlgoAPI_Common(signedA,signedB).Shape(),1e-11);
      const auto pa = BRepPrimAPI_MakeCylinder(gp_Ax2(gp_Pnt(1,0,0),gp_Dir(0,1,0)),2,8).Shape();
      const auto pb = BRepPrimAPI_MakeCylinder(gp_Ax2(gp_Pnt(2,7,0),gp_Dir(0,-1,0)),2,3).Shape();
      record(out,"finite-pair-reverse-parallel",BRepAlgoAPI_Common(pa,pb).Shape(),1e-11);
      const auto distant = BRepPrimAPI_MakeCylinder(gp_Ax2(gp_Pnt(6,7,0),gp_Dir(0,-1,0)),2,3).Shape();
      const auto empty = BRepAlgoAPI_Common(pa,distant).Shape();
      if (TopExp_Explorer(empty,TopAbs_FACE).More() || TopExp_Explorer(empty,TopAbs_SOLID).More())
        throw std::runtime_error("disjoint finite cylinder common is not empty");
      return 0;
    }

    if(argc==3 && mode=="repeated-spline-shell"){std::ofstream out(argv[1]);out.exceptions(std::ios::failbit|std::ios::badbit);repeatedSplineShellBooleans(out);return 0;}
    if(argc==3 && mode=="multiple-open-spline-box"){std::ofstream out(argv[1]);out.exceptions(std::ios::failbit|std::ios::badbit);multipleOpenSplineBoxBooleans(out);return 0;}
    if(argc==3 && mode=="open-spline-box"){std::ofstream out(argv[1]);out.exceptions(std::ios::failbit|std::ios::badbit);openSplineBoxBooleans(out);return 0;}
    if(argc==3 && mode=="simultaneous-spline-shell"){std::ofstream out(argv[1]);out.exceptions(std::ios::failbit|std::ios::badbit);simultaneousSplineBooleans(out);return 0;}
    if(argc==3 && mode=="selective-fillet"){std::ofstream out(argv[1]);out.exceptions(std::ios::failbit|std::ios::badbit);selectiveFilletRecord(out);return 0;}
    if(argc==3 && mode=="circular-rim-fillet"){std::ofstream out(argv[1]);out.exceptions(std::ios::failbit|std::ios::badbit);circularRimFilletRecord(out);return 0;}
    if(argc==3 && mode=="mixed-spline-shell"){std::ofstream out(argv[1]);out.exceptions(std::ios::failbit|std::ios::badbit);mixedSplineShellBooleans(out);return 0;}
    if(argc==3 && mode=="two-contour-spline-shell"){std::ofstream out(argv[1]);out.exceptions(std::ios::failbit|std::ios::badbit);twoContourSplineBooleans(out);return 0;}
    if(argc==3 && mode=="transverse-spline-shell"){std::ofstream out(argv[1]);out.exceptions(std::ios::failbit|std::ios::badbit);transverseSplineBooleans(out);return 0;}
    if(argc==3 && mode=="spline-shell-boolean"){std::ofstream out(argv[1]);out.exceptions(std::ios::failbit|std::ios::badbit);splineShellBooleans(out);return 0;}
    if(argc==3 && mode=="feature"){std::ofstream out(argv[1]);out.exceptions(std::ios::failbit|std::ios::badbit);featureRecords(out);return 0;}
    if(argc==3 && mode=="graph-boolean"){std::ofstream out(argv[1]);out.exceptions(std::ios::failbit|std::ios::badbit);graphBooleanRecords(out);return 0;}
    if(argc==3 && mode=="saddle-graph-boolean"){std::ofstream out(argv[1]);out.exceptions(std::ios::failbit|std::ios::badbit);saddleGraphBooleanRecords(out);return 0;}
    if(argc==3 && mode=="partial-graph-boolean"){std::ofstream out(argv[1]);out.exceptions(std::ios::failbit|std::ios::badbit);partialGraphBooleanRecords(out);return 0;}
    if(argc==3 && mode=="elliptical-graph-boolean"){std::ofstream out(argv[1]);out.exceptions(std::ios::failbit|std::ios::badbit);ellipticalGraphBooleanRecords(out);return 0;}
    if(argc==3 && mode=="draft"){std::ofstream out(argv[1]);out.exceptions(std::ios::failbit|std::ios::badbit);draftRecords(out);return 0;}
    if ((argc == 3 && (mode == "write-step-color" || mode == "write-step-face-color")) || (argc == 4 && (mode == "step-color" || mode == "step-face-color"))) {
#ifdef PLAINSOLID_OCCT_STEP
      occ::handle<TDocStd_Document> doc = new TDocStd_Document(TCollection_ExtendedString("BinXCAF"));
      auto shapes = XCAFDoc_DocumentTool::ShapeTool(doc->Main());
      auto colors = XCAFDoc_DocumentTool::ColorTool(doc->Main());
      if (mode == "write-step-color" || mode == "write-step-face-color") {
        const auto label = shapes->AddShape(BRepPrimAPI_MakeBox(2,3,4).Shape(), false);
        TDataStd_Name::Set(label, TCollection_ExtendedString("Colored box"));
        if(mode == "write-step-face-color") {
          TopExp_Explorer ex(shapes->GetShape(label),TopAbs_FACE);
          const auto sub=shapes->AddSubShape(label,ex.Current());
          colors->SetColor(sub, Quantity_Color(0.8,0.1,0.3,Quantity_TOC_sRGB), XCAFDoc_ColorSurf);
        } else colors->SetColor(label, Quantity_Color(0.2,0.4,0.6,Quantity_TOC_sRGB), XCAFDoc_ColorSurf);
        STEPCAFControl_Writer writer;
        if (!writer.Transfer(doc) || writer.Write(argv[1]) != IFSelect_RetDone) throw std::runtime_error("color export failed");
      } else {
        STEPCAFControl_Reader reader;
        if (reader.ReadFile(argv[3]) != IFSelect_RetDone || !reader.Transfer(doc)) throw std::runtime_error("color import failed");
        NCollection_Sequence<TDF_Label> roots; shapes->GetFreeShapes(roots);
        std::ofstream out(argv[1]);if(!out)throw std::runtime_error("color output failed");
        out << std::setprecision(17) << "[";
        bool firstFaceColor=true;
        for(int i=1;i<=roots.Length();++i){
          const auto shape=shapes->GetShape(roots.Value(i));
          if(shape.IsNull()||!BRepCheck_Analyzer(shape).IsValid())throw std::runtime_error("invalid colored shape");
          if(mode == "step-face-color") {
            for(TopExp_Explorer ex(shape,TopAbs_FACE);ex.More();ex.Next()){
              Quantity_Color fc;if(!colors->GetColor(ex.Current(),XCAFDoc_ColorSurf,fc))continue;
              double red,green,blue;fc.Values(red,green,blue,Quantity_TOC_sRGB);
              if(!firstFaceColor)out<<',';firstFaceColor=false;out<<'['<<red<<','<<green<<','<<blue<<']';
            }
            continue;
          }
          Quantity_Color c;
          if(!colors->GetColor(roots.Value(i),XCAFDoc_ColorSurf,c))throw std::runtime_error("missing surface color");
          double red,green,blue;c.Values(red,green,blue,Quantity_TOC_sRGB);
          if(i>1)out<<',';out<<'['<<red<<','<<green<<','<<blue<<']';
        }
        out << "]\n";
      }
      return 0;
#else
      throw std::runtime_error("reference executable built without STEP support");
#endif
    }
    if (argc == 4 && (mode == "step-assembly" || mode == "step-colored-assembly")) {
#ifdef PLAINSOLID_OCCT_STEP
      occ::handle<TDocStd_Document> doc = new TDocStd_Document(TCollection_ExtendedString("BinXCAF"));
      STEPCAFControl_Reader reader;
      if (reader.ReadFile(argv[3]) != IFSelect_RetDone || !reader.Transfer(doc))
        throw std::runtime_error("OCCT assembly import failed");
      auto shapes = XCAFDoc_DocumentTool::ShapeTool(doc->Main());
      NCollection_Sequence<TDF_Label> roots; shapes->GetFreeShapes(roots);
      auto labelName = [](const TDF_Label& label) {
        occ::handle<TDataStd_Name> attr;
        return label.FindAttribute(TDataStd_Name::GetID(), attr)
            ? std::string(TCollection_AsciiString(attr->Get(), '?').ToCString()) : std::string();
      };
      auto key = [](const TDF_Label& label) { TCollection_AsciiString entry; TDF_Tool::Entry(label, entry); return std::string(entry.ToCString()); };
      std::set<std::string> seen;
      std::vector<std::string> names, occurrenceNames;
      std::vector<gp_Trsf> transforms;
      std::vector<Quantity_Color> bodyColors,faceColors;
      std::function<void(const TDF_Label&)> visit = [&](const TDF_Label& label) {
        if (!seen.insert(key(label)).second) return;
        names.push_back(labelName(label));
        if(mode=="step-colored-assembly"&&shapes->IsSimpleShape(label)){
          auto colors=XCAFDoc_DocumentTool::ColorTool(doc->Main());Quantity_Color color;
          if(colors->GetColor(label,XCAFDoc_ColorSurf,color))bodyColors.push_back(color);
          const auto localShape=shapes->GetShape(label);
          if(localShape.ShapeType()!=TopAbs_SOLID)for(TopExp_Explorer ex(localShape,TopAbs_SOLID);ex.More();ex.Next())if(colors->GetColor(ex.Current(),XCAFDoc_ColorSurf,color))bodyColors.push_back(color);
          for(TopExp_Explorer ex(shapes->GetShape(label),TopAbs_FACE);ex.More();ex.Next())if(colors->GetColor(ex.Current(),XCAFDoc_ColorSurf,color))faceColors.push_back(color);
        }
        NCollection_Sequence<TDF_Label> children;
        shapes->GetComponents(label, children, false);
        for (int i = 1; i <= children.Length(); ++i) {
          TDF_Label referred;
          if (!shapes->GetReferredShape(children.Value(i), referred)) throw std::runtime_error("missing referred product");
          occurrenceNames.push_back(labelName(children.Value(i)));
          transforms.push_back(shapes->GetLocation(children.Value(i)).Transformation());
          visit(referred);
        }
      };
      double assemblyVolume = 0, assemblyX = 0, assemblyY = 0, assemblyZ = 0;
      for (int i = 1; i <= roots.Length(); ++i) {
        auto shape = shapes->GetShape(roots.Value(i));
        if (shape.IsNull() || !BRepCheck_Analyzer(shape).IsValid()) throw std::runtime_error("invalid assembly root shape");
        GProp_GProps mass; BRepGProp::VolumeProperties(shape, mass, 1e-14);
        const double v = mass.Mass(); const gp_Pnt c = mass.CentreOfMass();
        assemblyVolume += v; assemblyX += v*c.X(); assemblyY += v*c.Y(); assemblyZ += v*c.Z();
        visit(roots.Value(i));
      }
      std::ofstream out(argv[1]); if (!out) throw std::runtime_error("cannot write assembly results");
      out << std::setprecision(17) << "{\"Roots\":" << roots.Length() << ",\"Volume\":" << assemblyVolume << ",\"Centroid\":[" << assemblyX/assemblyVolume << "," << assemblyY/assemblyVolume << "," << assemblyZ/assemblyVolume << "],\"Products\":[";
      for (size_t i=0;i<names.size();++i) { if(i)out<<',';out<<std::quoted(names[i]); }
      out << "],\"Occurrences\":[";
      for (size_t i=0;i<occurrenceNames.size();++i) {if(i)out<<',';out<<"{\"Name\":"<<std::quoted(occurrenceNames[i])<<",\"Transform\":[";
        for(int r=1;r<=3;++r){if(r>1)out<<',';out<<'[';for(int c=1;c<=4;++c){if(c>1)out<<',';out<<transforms[i].Value(r,c);}out<<']';}out<<"]}";}
      out << "],\"BodyColors\":[";
      for(size_t i=0;i<bodyColors.size();++i){double r,g,b;bodyColors[i].Values(r,g,b,Quantity_TOC_sRGB);if(i)out<<',';out<<'['<<r<<','<<g<<','<<b<<']';}
      out << "],\"FaceColors\":[";
      for(size_t i=0;i<faceColors.size();++i){double r,g,b;faceColors[i].Values(r,g,b,Quantity_TOC_sRGB);if(i)out<<',';out<<'['<<r<<','<<g<<','<<b<<']';}
      out << "]}\n";
      return 0;
#else
      throw std::runtime_error("reference executable built without STEP support");
#endif
    }
    if (argc == 3 && (mode == "write-step-assembly" || mode == "write-step-colored-assembly")) {
#ifdef PLAINSOLID_OCCT_STEP
      occ::handle<TDocStd_Document> doc = new TDocStd_Document(TCollection_ExtendedString("BinXCAF"));
      auto shapes = XCAFDoc_DocumentTool::ShapeTool(doc->Main());
      auto part = shapes->AddShape(BRepPrimAPI_MakeBox(1, 2, 3).Shape(), false);
      TDataStd_Name::Set(part, TCollection_ExtendedString("Bracket"));
      if(mode=="write-step-colored-assembly"){
        auto colors=XCAFDoc_DocumentTool::ColorTool(doc->Main());
        colors->SetColor(part,Quantity_Color(0.2,0.4,0.6,Quantity_TOC_sRGB),XCAFDoc_ColorSurf);
        TopExp_Explorer ex(shapes->GetShape(part),TopAbs_FACE);
        colors->SetColor(shapes->AddSubShape(part,ex.Current()),Quantity_Color(0.8,0.1,0.3,Quantity_TOC_sRGB),XCAFDoc_ColorSurf);
      }
      auto assembly = shapes->NewShape();
      TDataStd_Name::Set(assembly, TCollection_ExtendedString("Machine"));
      gp_Trsf a; a.SetTranslation(gp_Vec(10, 0, 0));
      auto left = shapes->AddComponent(assembly, part, TopLoc_Location(a));
      TDataStd_Name::Set(left, TCollection_ExtendedString("Left bracket"));
      gp_Trsf b; b.SetRotation(gp_Ax1(gp_Pnt(0, 0, 0), gp_Dir(0, 0, 1)), std::acos(-1.0)/2);
      b.SetTranslationPart(gp_Vec(0, 20, 0));
      auto right = shapes->AddComponent(assembly, part, TopLoc_Location(b));
      TDataStd_Name::Set(right, TCollection_ExtendedString("Right bracket"));
      shapes->UpdateAssemblies();
      STEPCAFControl_Writer writer;
      if (!writer.Transfer(doc) || writer.Write(argv[1]) != IFSelect_RetDone)
        throw std::runtime_error("OCCT assembly export failed");
      return 0;
#else
      throw std::runtime_error("reference executable built without STEP support");
#endif
    }
    if (argc == 3 && (mode == "write-step-parabola" || mode == "write-step-hyperbola")) {
#ifdef PLAINSOLID_OCCT_STEP
      const gp_Ax2 frame(gp_Pnt(1,2,3), gp_Dir(0,0,1));
      occ::handle<Geom_Curve> curve;
      if(mode == "write-step-parabola") curve = new Geom_Parabola(frame,2);
      else curve = new Geom_Hyperbola(frame,3,2);
      const auto edge=BRepBuilderAPI_MakeEdge(curve,-0.75,1.25).Edge();
      STEPControl_Writer writer;
      if(writer.Transfer(edge,STEPControl_GeometricCurveSet)!=IFSelect_RetDone||writer.Write(argv[1])!=IFSelect_RetDone)
        throw std::runtime_error("conic export failed");
      return 0;
#else
      throw std::runtime_error("reference executable built without STEP support");
#endif
    }
    if (argc == 3 && mode == "write-step-torus-elbow") {
#ifdef PLAINSOLID_OCCT_STEP
      const auto shape=BRepPrimAPI_MakeTorus(gp_Ax2(gp_Pnt(1,2,3),gp_Dir(0,0,1)),3,1,std::acos(0.0)).Shape();
      if(!BRepCheck_Analyzer(shape).IsValid())throw std::runtime_error("invalid torus elbow fixture");
      STEPControl_Writer writer;
      if(writer.Transfer(shape,STEPControl_AsIs)!=IFSelect_RetDone||writer.Write(argv[1])!=IFSelect_RetDone)throw std::runtime_error("torus elbow export failed");
      return 0;
#else
      throw std::runtime_error("reference executable built without STEP support");
#endif
    }
    if (argc == 3 && (mode == "write-step-hemisphere" || mode == "write-step-spherical-zone")) {
#ifdef PLAINSOLID_OCCT_STEP
      const auto shape=BRepPrimAPI_MakeSphere(gp_Ax2(gp_Pnt(1,2,3),gp_Dir(0,0,1)),2,mode == "write-step-spherical-zone" ? -std::acos(0.0)/3 : 0,mode == "write-step-spherical-zone" ? std::acos(0.0)/3 : std::acos(0.0)).Shape();
      if(!BRepCheck_Analyzer(shape).IsValid())throw std::runtime_error("invalid hemisphere fixture");
      STEPControl_Writer writer;
      if(writer.Transfer(shape,STEPControl_AsIs)!=IFSelect_RetDone||writer.Write(argv[1])!=IFSelect_RetDone)throw std::runtime_error("hemisphere export failed");
      return 0;
#else
      throw std::runtime_error("reference executable built without STEP support");
#endif
    }
    if (argc == 3 && mode == "write-step-cut-groove") {
#ifdef PLAINSOLID_OCCT_STEP
      const gp_Ax2 base(gp_Pnt(1,2,3),gp_Dir(0,0,1));
      const gp_Ax2 cut(gp_Pnt(1,2,4),gp_Dir(0,0,1));
      const auto stock=BRepPrimAPI_MakeCylinder(base,2,4).Shape();
      const auto ring=BRepAlgoAPI_Cut(BRepPrimAPI_MakeCylinder(cut,3,2).Shape(),BRepPrimAPI_MakeCylinder(cut,1,2).Shape()).Shape();
      const auto shape=BRepAlgoAPI_Cut(stock,ring).Shape();
      if(!BRepCheck_Analyzer(shape).IsValid())throw std::runtime_error("invalid cut groove fixture");
      STEPControl_Writer writer;
      if(writer.Transfer(shape,STEPControl_AsIs)!=IFSelect_RetDone||writer.Write(argv[1])!=IFSelect_RetDone)throw std::runtime_error("cut groove export failed");
      return 0;
#else
      throw std::runtime_error("reference executable built without STEP support");
#endif
    }
    if (argc == 3 && (mode == "write-step-collar" || mode == "write-step-groove")) {
#ifdef PLAINSOLID_OCCT_STEP
      const double endRadius=mode == "write-step-collar" ? 1 : 2;
      const double midRadius=mode == "write-step-collar" ? 2 : 1;
      const auto lower=BRepPrimAPI_MakeCylinder(gp_Ax2(gp_Pnt(1,2,3),gp_Dir(0,0,1)),endRadius,1).Shape();
      const auto mid=BRepPrimAPI_MakeCylinder(gp_Ax2(gp_Pnt(1,2,4),gp_Dir(0,0,1)),midRadius,2).Shape();
      const auto upper=BRepPrimAPI_MakeCylinder(gp_Ax2(gp_Pnt(1,2,6),gp_Dir(0,0,1)),endRadius,1).Shape();
      const auto shape=BRepAlgoAPI_Fuse(BRepAlgoAPI_Fuse(lower,mid).Shape(),upper).Shape();
      if(!BRepCheck_Analyzer(shape).IsValid())throw std::runtime_error("invalid axial shoulder fixture");
      STEPControl_Writer writer;
      if(writer.Transfer(shape,STEPControl_AsIs)!=IFSelect_RetDone||writer.Write(argv[1])!=IFSelect_RetDone)throw std::runtime_error("axial shoulder export failed");
      return 0;
#else
      throw std::runtime_error("reference executable built without STEP support");
#endif
    }
    if (argc == 3 && (mode == "write-step-stepped-cylinder" || mode == "write-step-axial-profile")) {
#ifdef PLAINSOLID_OCCT_STEP
      const auto lower=BRepPrimAPI_MakeCylinder(gp_Ax2(gp_Pnt(1,2,3),gp_Dir(0,0,1)),3,4).Shape();
      const auto upper=BRepPrimAPI_MakeCylinder(gp_Ax2(gp_Pnt(1,2,7),gp_Dir(0,0,1)),1,2).Shape();
      auto shape=BRepAlgoAPI_Fuse(lower,upper).Shape();
      if(mode == "write-step-axial-profile") {const auto top=BRepPrimAPI_MakeCylinder(gp_Ax2(gp_Pnt(1,2,9),gp_Dir(0,0,1)),0.5,1).Shape();shape=BRepAlgoAPI_Fuse(shape,top).Shape();}
      if(!BRepCheck_Analyzer(shape).IsValid())throw std::runtime_error("invalid stepped fixture");
      STEPControl_Writer writer;
      if(writer.Transfer(shape,STEPControl_AsIs)!=IFSelect_RetDone||writer.Write(argv[1])!=IFSelect_RetDone)throw std::runtime_error("stepped cylinder export failed");
      return 0;
#else
      throw std::runtime_error("reference executable built without STEP support");
#endif
    }
    if (argc == 3 && (mode == "write-step-spline-shell" || mode == "write-step-spline-shell-assembly" || mode == "write-step-colored-spline-assembly")) {
#ifdef PLAINSOLID_OCCT_STEP
      const gp_Pnt points[]={gp_Pnt(0,0,0),gp_Pnt(2,0,0),gp_Pnt(2,2,0),gp_Pnt(0,2,0),gp_Pnt(0,0,1),gp_Pnt(2,0,1),gp_Pnt(2,2,2),gp_Pnt(0,2,1)};
      const int corners[6][4]={{0,1,3,2},{4,7,5,6},{0,4,1,5},{3,2,7,6},{0,3,4,7},{1,5,2,6}};
      NCollection_Array1<double> knots(1,2);knots.SetValue(1,0);knots.SetValue(2,1);
      NCollection_Array1<int> mults(1,2);mults.SetValue(1,2);mults.SetValue(2,2);
      BRepBuilderAPI_Sewing sewing(1e-8);
      for(int f=0;f<6;++f){NCollection_Array2<gp_Pnt> poles(1,2,1,2);for(int u=0;u<2;++u)for(int v=0;v<2;++v)poles.SetValue(u+1,v+1,points[corners[f][2*u+v]]);occ::handle<Geom_BSplineSurface> support=new Geom_BSplineSurface(poles,knots,knots,mults,mults,1,1);sewing.Add(BRepBuilderAPI_MakeFace(support,1e-8).Face());}
      sewing.Perform();const auto shape=BRepBuilderAPI_MakeSolid(TopoDS::Shell(sewing.SewedShape())).Solid();
      if(!BRepCheck_Analyzer(shape).IsValid())throw std::runtime_error("invalid spline shell fixture");
      if(mode=="write-step-spline-shell-assembly"||mode=="write-step-colored-spline-assembly") {
        occ::handle<TDocStd_Document> doc=new TDocStd_Document(TCollection_ExtendedString("BinXCAF"));
        auto shapes=XCAFDoc_DocumentTool::ShapeTool(doc->Main());auto part=shapes->AddShape(shape,false);TDataStd_Name::Set(part,TCollection_ExtendedString("Warped part"));
        if(mode=="write-step-colored-spline-assembly") {auto colors=XCAFDoc_DocumentTool::ColorTool(doc->Main());colors->SetColor(part,Quantity_Color(.2,.4,.6,Quantity_TOC_sRGB),XCAFDoc_ColorSurf);TopExp_Explorer face(shape,TopAbs_FACE);colors->SetColor(shapes->AddSubShape(part,face.Current()),Quantity_Color(.8,.1,.3,Quantity_TOC_sRGB),XCAFDoc_ColorSurf);}
        auto assembly=shapes->NewShape();TDataStd_Name::Set(assembly,TCollection_ExtendedString("Spline assembly"));
        gp_Trsf a;a.SetTranslation(gp_Vec(10,0,0));auto left=shapes->AddComponent(assembly,part,TopLoc_Location(a));TDataStd_Name::Set(left,TCollection_ExtendedString("Left instance"));
        gp_Trsf b;b.SetRotation(gp_Ax1(gp_Pnt(0,0,0),gp_Dir(0,0,1)),std::acos(-1.0)/2);b.SetTranslationPart(gp_Vec(0,20,0));auto right=shapes->AddComponent(assembly,part,TopLoc_Location(b));TDataStd_Name::Set(right,TCollection_ExtendedString("Right instance"));
        shapes->UpdateAssemblies();STEPCAFControl_Writer writer;if(!writer.Transfer(doc)||writer.Write(argv[1])!=IFSelect_RetDone)throw std::runtime_error("spline shell assembly export failed");return 0;
      }
      STEPControl_Writer writer;if(writer.Transfer(shape,STEPControl_AsIs)!=IFSelect_RetDone||writer.Write(argv[1])!=IFSelect_RetDone)throw std::runtime_error("spline shell export failed");return 0;
#else
      throw std::runtime_error("reference executable built without STEP support");
#endif
    }
    if (argc == 3 && (mode == "write-iges-bounded-spline-face" || mode == "write-iges-placed-spline-face" || mode == "write-iges-holed-spline-face" || mode == "write-iges-multiholed-spline-face")) {
#ifdef PLAINSOLID_OCCT_IGES
      NCollection_Array2<gp_Pnt> poles(1,2,1,2);
      poles.SetValue(1,1,gp_Pnt(0,0,0));poles.SetValue(1,2,gp_Pnt(0,2,0));
      poles.SetValue(2,1,gp_Pnt(42,0,0));poles.SetValue(2,2,gp_Pnt(42,2,21));
      NCollection_Array2<double> weights(1,2,1,2);weights.SetValue(1,1,1);weights.SetValue(1,2,1);weights.SetValue(2,1,2);weights.SetValue(2,2,2);
      NCollection_Array1<double> knots(1,2);knots.SetValue(1,0);knots.SetValue(2,1);
      NCollection_Array1<int> mults(1,2);mults.SetValue(1,2);mults.SetValue(2,2);
      occ::handle<Geom_BSplineSurface> support=new Geom_BSplineSurface(poles,weights,knots,knots,mults,mults,1,1);
      auto shape=BRepBuilderAPI_MakeFace(support,.3125,.75,.25,.75,1e-8).Face();
      if(mode=="write-iges-holed-spline-face"||mode=="write-iges-multiholed-spline-face") {
        const auto outer=BRepBuilderAPI_MakeFace(support,0,1,0,1,1e-8).Face();
        BRepBuilderAPI_MakeFace builder(outer);
        const int holes=mode=="write-iges-multiholed-spline-face"?2:1;
        for(int i=0;i<holes;++i){const double lo=holes==1?.25:(i==0?.125:.625),hi=holes==1?.75:(i==0?.375:.875);
          const auto inner=BRepBuilderAPI_MakeFace(support,.3125,.75,lo,hi,1e-8).Face();TopExp_Explorer wires(inner,TopAbs_WIRE);
          if(!wires.More())throw std::runtime_error("missing inner wire");builder.Add(TopoDS::Wire(wires.Current().Reversed()));}
        shape=builder.Face();
      }
      if(!BRepCheck_Analyzer(shape).IsValid())throw std::runtime_error("invalid bounded spline face");
      IGESControl_Writer writer("MM");if(!writer.AddShape(shape))throw std::runtime_error("bounded spline face transfer failed");
      if(mode=="write-iges-placed-spline-face") {
        occ::handle<NCollection_HArray2<double>> values=new NCollection_HArray2<double>(1,3,1,4);
        const double a[3][4]={{0,-1,0,5},{1,0,0,7},{0,0,1,11}};
        for(int i=1;i<=3;++i)for(int j=1;j<=4;++j)values->SetValue(i,j,a[i-1][j-1]);
        occ::handle<IGESGeom_TransformationMatrix> placement=new IGESGeom_TransformationMatrix;placement->Init(values);
        const auto model=writer.Model();
        for(int i=1;i<=model->NbEntities();++i) {const auto e=model->Entity(i);if(e->TypeNumber()==128||e->TypeNumber()==126)e->InitTransf(placement);}
        writer.AddEntity(placement);
      }
      if(!writer.Write(argv[1]))throw std::runtime_error("bounded spline face export failed");
      return 0;
#else
      throw std::runtime_error("reference executable built without STEP support");
#endif
    }
    if (argc == 3 && (mode == "write-step-holed-spline-face" || mode == "write-step-multiholed-spline-face")) {
#ifdef PLAINSOLID_OCCT_STEP
      NCollection_Array2<gp_Pnt> poles(1,2,1,2);
      poles.SetValue(1,1,gp_Pnt(0,0,0));poles.SetValue(1,2,gp_Pnt(0,2,0));
      poles.SetValue(2,1,gp_Pnt(42,0,0));poles.SetValue(2,2,gp_Pnt(42,2,21));
      NCollection_Array2<double> weights(1,2,1,2);weights.SetValue(1,1,1);weights.SetValue(1,2,1);weights.SetValue(2,1,2);weights.SetValue(2,2,2);
      NCollection_Array1<double> knots(1,2);knots.SetValue(1,0);knots.SetValue(2,1);
      NCollection_Array1<int> mults(1,2);mults.SetValue(1,2);mults.SetValue(2,2);
      occ::handle<Geom_BSplineSurface> support=new Geom_BSplineSurface(poles,weights,knots,knots,mults,mults,1,1);
      const auto outer=BRepBuilderAPI_MakeFace(support,0,1,0,1,1e-8).Face();
      BRepBuilderAPI_MakeFace maker(outer);const int holes=mode=="write-step-multiholed-spline-face"?2:1;
      for(int i=0;i<holes;++i){const double lo=holes==1?.25:(i==0?.125:.625),hi=holes==1?.75:(i==0?.375:.875);
        const auto inner=BRepBuilderAPI_MakeFace(support,.3125,.75,lo,hi,1e-8).Face();TopExp_Explorer wires(inner,TopAbs_WIRE);if(!wires.More())throw std::runtime_error("missing hole wire");maker.Add(TopoDS::Wire(wires.Current().Reversed()));}
      const auto shape=maker.Face();
      if(!BRepCheck_Analyzer(shape).IsValid())throw std::runtime_error("invalid bounded spline face");
      STEPControl_Writer writer;if(writer.Transfer(shape,STEPControl_AsIs)!=IFSelect_RetDone||writer.Write(argv[1])!=IFSelect_RetDone)throw std::runtime_error("bounded spline face export failed");
      return 0;
#else
      throw std::runtime_error("reference executable built without STEP support");
#endif
    }
    if (argc == 3 && (mode == "write-step-wrapped-stepped-union" || mode == "write-step-wrapped-stepped-intersection" || mode == "write-step-wrapped-stepped-difference")) {
#ifdef PLAINSOLID_OCCT_STEP
      const auto a=BRepPrimAPI_MakeCylinder(gp_Ax2(gp_Pnt(0,0,0),gp_Dir(0,0,1)),2,4).Shape();
      const auto b=BRepPrimAPI_MakeCylinder(gp_Ax2(gp_Pnt(0,0,2),gp_Dir(0,0,1)),1,4).Shape();
      TopoDS_Shape shape;if(mode=="write-step-wrapped-stepped-union")shape=BRepAlgoAPI_Fuse(a,b).Shape();else if(mode=="write-step-wrapped-stepped-intersection")shape=BRepAlgoAPI_Common(a,b).Shape();else shape=BRepAlgoAPI_Cut(a,b).Shape();
      if(!BRepCheck_Analyzer(shape).IsValid())throw std::runtime_error("invalid coaxial analytic Boolean fixture");STEPControl_Writer writer;if(writer.Transfer(shape,STEPControl_AsIs)!=IFSelect_RetDone||writer.Write(argv[1])!=IFSelect_RetDone)throw std::runtime_error("coaxial analytic Boolean export failed");return 0;
#else
      throw std::runtime_error("reference executable built without STEP support");
#endif
    }
#ifdef PLAINSOLID_OCCT_STEP
    if(argc==3&&mode=="write-step-wrapped-rim-fillet")return writeWrappedRimFilletSTEP(argv[1]);
    if(argc==3&&mode=="write-step-wrapped-station-bore")return writeWrappedStationBoreSTEP(argv[1]);
    if(argc==3&&mode=="write-step-wrapped-station-bore-single")return writeWrappedStationBoreSTEP(argv[1],true);
    if(argc==3&&mode=="write-step-wrapped-annular-frustum")return writeWrappedStationBoreSTEP(argv[1],true,true);
    if(argc==3&&mode=="write-step-curved-oval-face")return writeCurvedOvalSTEP(argv[1]);
    if(argc==3&&mode=="write-step-rational-curved-oval-face")return writeCurvedOvalSTEP(argv[1],true);
    if(argc==3&&mode=="write-step-curved-oval-hole")return writeCurvedOvalSTEP(argv[1],false,true);
    if(argc==3&&mode=="write-step-curved-oval-shell")return writeCurvedOvalShellSTEP(argv[1]);
    if(argc==3&&mode=="write-step-spline-lens")return writeSplineLensSTEP(argv[1]);
    if(argc==3&&mode=="write-step-sheared-spline-lens")return writeSplineLensSTEP(argv[1],true);
    if(argc==3&&mode=="write-step-refined-spline-lens")return writeSplineLensSTEP(argv[1],false,true);
    if(argc==3&&mode=="write-step-partitioned-spline-lens-assembly")return writeSplineLensAssemblySTEP(argv[1],true);
    if(argc==3&&mode=="write-step-refined-curve-lens-assembly")return writeSplineLensAssemblySTEP(argv[1],false,true);
    if(argc==3&&mode=="write-step-rational-lens-assembly")return writeSplineLensAssemblySTEP(argv[1],false,false,true);
    if(argc==3&&mode=="write-step-concurrent-rational-lens")return writeConcurrentRationalLensSTEP(argv[1]);
    if(argc==3&&mode=="write-step-refined-curve-lens")return writeRefinedCurveLensSTEP(argv[1]);
    if(argc==3&&mode=="write-step-partitioned-spline-lens")return writePartitionedSplineLensSTEP(argv[1]);
    if(argc==3&&mode=="write-step-spline-lens-assembly")return writeSplineLensAssemblySTEP(argv[1]);
#endif
    if (argc == 3 && mode == "write-step-wrapped-station-loft") {
#ifdef PLAINSOLID_OCCT_STEP
      const double xy[9][2]={{1,0},{1,1},{0,1},{-1,1},{-1,0},{-1,-1},{0,-1},{1,-1},{1,0}},weightsU[9]={1,1,2,2,4,2,2,1,1},radii[3]={2,1,2},heights[3]={0,2,4};BRepBuilderAPI_Sewing sewing(1e-8);
      for(int band=0;band<2;++band){NCollection_Array2<gp_Pnt> poles(1,9,1,2);NCollection_Array2<double> weights(1,9,1,2);for(int u=0;u<9;++u)for(int v=0;v<2;++v){poles.SetValue(u+1,v+1,gp_Pnt(radii[band+v]*xy[u][0],radii[band+v]*xy[u][1],heights[band+v]));weights.SetValue(u+1,v+1,weightsU[u]);}NCollection_Array1<double> uk(1,5),vk(1,2);NCollection_Array1<int> um(1,5),vm(1,2);for(int i=1;i<=5;++i){uk.SetValue(i,i-1);um.SetValue(i,i==1||i==5?3:2);}vk.SetValue(1,0);vk.SetValue(2,1);vm.SetValue(1,2);vm.SetValue(2,2);occ::handle<Geom_BSplineSurface> support=new Geom_BSplineSurface(poles,weights,uk,vk,um,vm,2,1);const auto side=BRepBuilderAPI_MakeFace(support,1e-8).Face();sewing.Add(side);
        for(TopExp_Explorer ex(side,TopAbs_EDGE);ex.More();ex.Next()){auto edge=TopoDS::Edge(ex.Current());double lo,hi;auto curve=BRep_Tool::Curve(edge,lo,hi);if(curve.IsNull())continue;auto a=curve->Value(lo),b=curve->Value(hi),mid=curve->Value((lo+hi)/2);if(a.Z()!=b.Z()||a.Z()!=mid.Z()||(a.Z()!=0&&a.Z()!=4))continue;edge.Orientation(TopAbs_FORWARD);auto cap=BRepBuilderAPI_MakeFace(gp_Pln(gp_Pnt(0,0,a.Z()),gp_Dir(0,0,1)),BRepBuilderAPI_MakeWire(edge).Wire(),true).Face();if(a.Z()==0)cap.Reverse();sewing.Add(cap);}
      }
      sewing.Perform();const auto shape=BRepBuilderAPI_MakeSolid(TopoDS::Shell(sewing.SewedShape())).Solid();if(!BRepCheck_Analyzer(shape).IsValid())throw std::runtime_error("invalid wrapped station loft");STEPControl_Writer writer;if(writer.Transfer(shape,STEPControl_AsIs)!=IFSelect_RetDone||writer.Write(argv[1])!=IFSelect_RetDone)throw std::runtime_error("wrapped station loft export failed");return 0;
#else
      throw std::runtime_error("reference executable built without STEP support");
#endif
    }
    if (argc == 3 && mode == "write-step-wrapped-toroidal-void") {
#ifdef PLAINSOLID_OCCT_STEP
      auto toroid=[](double radius){const double xy[9][2]={{1,0},{1,1},{0,1},{-1,1},{-1,0},{-1,-1},{0,-1},{1,-1},{1,0}};const double w[9]={1,1,2,2,4,2,2,1,1};NCollection_Array2<gp_Pnt> poles(1,9,1,9);NCollection_Array2<double> weights(1,9,1,9);for(int u=0;u<9;++u)for(int v=0;v<9;++v){poles.SetValue(u+1,v+1,gp_Pnt(1+(3+radius*xy[v][0])*xy[u][0],2+(3+radius*xy[v][0])*xy[u][1],3+radius*xy[v][1]));weights.SetValue(u+1,v+1,w[u]*w[v]);}NCollection_Array1<double> knots(1,5);NCollection_Array1<int> mults(1,5);for(int i=1;i<=5;++i){knots.SetValue(i,i-1);mults.SetValue(i,i==1||i==5?3:2);}occ::handle<Geom_BSplineSurface> support=new Geom_BSplineSurface(poles,weights,knots,knots,mults,mults,2,2);auto face=BRepBuilderAPI_MakeFace(support,1e-8).Face();TopoDS_Shell shell;BRep_Builder builder;builder.MakeShell(shell);builder.Add(shell,face);return BRepBuilderAPI_MakeSolid(shell).Solid();};
      const auto shape=BRepAlgoAPI_Cut(toroid(1),toroid(.5)).Shape();if(!BRepCheck_Analyzer(shape).IsValid())throw std::runtime_error("invalid wrapped toroidal void");STEPControl_Writer writer;if(writer.Transfer(shape,STEPControl_AsIs)!=IFSelect_RetDone||writer.Write(argv[1])!=IFSelect_RetDone)throw std::runtime_error("wrapped toroidal void export failed");return 0;
#else
      throw std::runtime_error("reference executable built without STEP support");
#endif
    }
    if (argc == 3 && mode == "write-step-wrapped-toroid") {
#ifdef PLAINSOLID_OCCT_STEP
      const double xy[9][2]={{1,0},{1,1},{0,1},{-1,1},{-1,0},{-1,-1},{0,-1},{1,-1},{1,0}};
      NCollection_Array2<gp_Pnt> poles(1,9,1,9);NCollection_Array2<double> weights(1,9,1,9);
      for(int u=0;u<9;++u)for(int v=0;v<9;++v){poles.SetValue(u+1,v+1,gp_Pnt((3+xy[v][0])*xy[u][0],(3+xy[v][0])*xy[u][1],xy[v][1]));weights.SetValue(u+1,v+1,(u%2?.75:1)*(v%2?.75:1));}
      NCollection_Array1<double> knots(1,5);NCollection_Array1<int> mults(1,5);for(int i=1;i<=5;++i){knots.SetValue(i,i-1);mults.SetValue(i,i==1||i==5?3:2);}
      occ::handle<Geom_BSplineSurface> support=new Geom_BSplineSurface(poles,weights,knots,knots,mults,mults,2,2);
      auto face=BRepBuilderAPI_MakeFace(support,1e-8).Face();TopoDS_Shell shell;BRep_Builder builder;builder.MakeShell(shell);builder.Add(shell,face);auto shape=BRepBuilderAPI_MakeSolid(shell).Solid();
      if(!BRepCheck_Analyzer(shape).IsValid())throw std::runtime_error("invalid wrapped toroid");STEPControl_Writer writer;if(writer.Transfer(shape,STEPControl_AsIs)!=IFSelect_RetDone||writer.Write(argv[1])!=IFSelect_RetDone)throw std::runtime_error("wrapped toroid export failed");return 0;
#else
      throw std::runtime_error("reference executable built without STEP support");
#endif
    }
    if (argc == 3 && (mode == "write-step-wrapped-surface" || mode == "write-step-wrapped-cylinder" || mode == "write-step-wrapped-tube" || mode == "write-step-wrapped-frustum" || mode == "write-step-wrapped-circular-frustum" || mode == "write-step-wrapped-blind-bore" || mode == "write-step-wrapped-cavity" || mode == "write-step-wrapped-shoulder")) {
#ifdef PLAINSOLID_OCCT_STEP
      const double xy[9][2]={{2,0},{2,2},{0,2},{-2,2},{-2,0},{-2,-2},{0,-2},{2,-2},{2,0}};
      NCollection_Array2<gp_Pnt> poles(1,9,1,2);NCollection_Array2<double> weights(1,9,1,2);
      const double circularWeights[9]={1,1,2,2,4,2,2,1,1};
      for(int u=0;u<9;++u)for(int v=0;v<2;++v){const double scale=v&&(mode=="write-step-wrapped-frustum"||mode=="write-step-wrapped-circular-frustum")?.5:1;poles.SetValue(u+1,v+1,gp_Pnt(scale*xy[u][0],scale*xy[u][1],v?3:-2));weights.SetValue(u+1,v+1,mode=="write-step-wrapped-circular-frustum"?circularWeights[u]:(u%2?(mode!="write-step-wrapped-surface"?.75:std::sqrt(.5)):1));}
      NCollection_Array1<double> uk(1,5),vk(1,2);NCollection_Array1<int> um(1,5),vm(1,2);for(int i=1;i<=5;++i){uk.SetValue(i,i-1);um.SetValue(i,i==1||i==5?3:2);}vk.SetValue(1,-2);vk.SetValue(2,3);vm.SetValue(1,2);vm.SetValue(2,2);
      occ::handle<Geom_BSplineSurface> support=new Geom_BSplineSurface(poles,weights,uk,vk,um,vm,2,1);
      if(!support->IsUClosed()||support->IsUPeriodic()||support->IsVClosed())throw std::runtime_error("wrapped fixture semantics changed");
      const auto side=BRepBuilderAPI_MakeFace(support,1e-8).Face();TopoDS_Shape shape=side;
      if(mode!="write-step-wrapped-surface"){
        BRepBuilderAPI_Sewing sewing(1e-8);sewing.Add(side);int caps=0;
        for(TopExp_Explorer ex(side,TopAbs_EDGE);ex.More();ex.Next()) {auto edge=TopoDS::Edge(ex.Current());double lo,hi;auto curve=BRep_Tool::Curve(edge,lo,hi);if(curve.IsNull())continue;auto a=curve->Value(lo),b=curve->Value(hi),mid=curve->Value((lo+hi)/2);if(a.Z()!=b.Z()||a.Z()!=mid.Z())continue;
          edge.Orientation(TopAbs_FORWARD);BRepBuilderAPI_MakeWire wire(edge);auto cap=BRepBuilderAPI_MakeFace(gp_Pln(gp_Pnt(0,0,a.Z()),gp_Dir(0,0,1)),wire.Wire(),true).Face();if(a.Z()<0)cap.Reverse();sewing.Add(cap);++caps;
        }
        if(caps!=2)throw std::runtime_error("wrapped cylinder rim discovery failed");sewing.Perform();shape=BRepBuilderAPI_MakeSolid(TopoDS::Shell(sewing.SewedShape())).Solid();
      }
      if(mode=="write-step-wrapped-tube"||mode=="write-step-wrapped-blind-bore"||mode=="write-step-wrapped-cavity"||mode=="write-step-wrapped-shoulder"){gp_GTrsf scale;scale.SetValue(1,1,.5);scale.SetValue(2,2,.5);if(mode=="write-step-wrapped-blind-bore"||mode=="write-step-wrapped-cavity"){scale.SetValue(3,3,.5);scale.SetValue(3,4,mode=="write-step-wrapped-cavity"?.25:1.5);}if(mode=="write-step-wrapped-shoulder"){scale.SetValue(3,3,.5);scale.SetValue(3,4,4);}auto inner=BRepBuilderAPI_GTransform(shape,scale,true).Shape();if(mode=="write-step-wrapped-blind-bore"||mode=="write-step-wrapped-cavity"||mode=="write-step-wrapped-shoulder"){const auto innerSide=TopoDS::Face(BRepBuilderAPI_GTransform(side,scale,true).Shape());BRepBuilderAPI_Sewing sewing(1e-8);sewing.Add(innerSide);int caps=0;for(TopExp_Explorer ex(innerSide,TopAbs_EDGE);ex.More();ex.Next()){auto edge=TopoDS::Edge(ex.Current());double lo,hi;auto curve=BRep_Tool::Curve(edge,lo,hi);if(curve.IsNull())continue;auto a=curve->Value(lo),b=curve->Value(hi),mid=curve->Value((lo+hi)/2);if(a.Z()!=b.Z()||a.Z()!=mid.Z())continue;edge.Orientation(TopAbs_FORWARD);auto cap=BRepBuilderAPI_MakeFace(gp_Pln(gp_Pnt(0,0,a.Z()),gp_Dir(0,0,1)),BRepBuilderAPI_MakeWire(edge).Wire(),true).Face();if(a.Z()<(mode=="write-step-wrapped-cavity"?1.75:(mode=="write-step-wrapped-shoulder"?5.5:3)))cap.Reverse();sewing.Add(cap);++caps;}if(caps!=2)throw std::runtime_error("blind cutter caps");sewing.Perform();inner=BRepBuilderAPI_MakeSolid(TopoDS::Shell(sewing.SewedShape())).Solid();}if(mode=="write-step-wrapped-shoulder")shape=BRepAlgoAPI_Fuse(shape,inner).Shape();else shape=BRepAlgoAPI_Cut(shape,inner).Shape();}
      if(!BRepCheck_Analyzer(shape).IsValid())throw std::runtime_error("invalid wrapped spline fixture");STEPControl_Writer writer;if(writer.Transfer(shape,STEPControl_AsIs)!=IFSelect_RetDone||writer.Write(argv[1])!=IFSelect_RetDone)throw std::runtime_error("wrapped spline export failed");return 0;

#else
      throw std::runtime_error("reference executable built without STEP support");
#endif
    }
    if (argc == 3 && (mode == "write-step-axis-aligned-spline-shell" || mode == "write-step-orthogonal-affine-spline-shell" || mode == "write-step-affine-spline-shell")) {
#ifdef PLAINSOLID_OCCT_STEP
      auto face=[&](int a,int b,double u0,double u1,double v0,double v1,double fixed,const std::vector<gp_Pnt2d>& uv){
        NCollection_Array2<gp_Pnt> poles(1,2,1,2);for(int u=0;u<2;++u)for(int v=0;v<2;++v){double p[3]={0,0,0};p[a]=u?u1:u0;p[b]=v?v1:v0;p[3-a-b]=fixed;if(mode=="write-step-affine-spline-shell"){const double x=p[0],y=p[1],z=p[2];p[0]=x+y+7;p[1]=y+z+11;p[2]=z+13;}poles.SetValue(u+1,v+1,gp_Pnt(p[0],p[1],p[2]));}
        NCollection_Array1<double> uk(1,2),vk(1,2);uk.SetValue(1,u0);uk.SetValue(2,u1);vk.SetValue(1,v0);vk.SetValue(2,v1);NCollection_Array1<int> mults(1,2);mults.SetValue(1,2);mults.SetValue(2,2);
        occ::handle<Geom_BSplineSurface> support=new Geom_BSplineSurface(poles,uk,vk,mults,mults,1,1);BRepBuilderAPI_MakeWire wire;
        for(size_t i=0;i<uv.size();++i){auto p=uv[i],q=uv[(i+1)%uv.size()];bool vertical=p.X()==q.X();double lo=vertical?std::min(p.Y(),q.Y()):std::min(p.X(),q.X()),hi=vertical?std::max(p.Y(),q.Y()):std::max(p.X(),q.X());occ::handle<Geom2d_Line> pc=new Geom2d_Line(gp_Pnt2d(vertical?p.X():0,vertical?0:p.Y()),gp_Dir2d(vertical?0:1,vertical?1:0));auto edge=BRepBuilderAPI_MakeEdge(pc,support,lo,hi).Edge();if(vertical?q.Y()<p.Y():q.X()<p.X())edge.Reverse();wire.Add(edge);}
        auto boundary=wire.Wire();BRepLib::BuildCurves3d(boundary);return BRepBuilderAPI_MakeFace(support,boundary,true).Face();
      };
      const std::vector<gp_Pnt2d> outline={{0,0},{4,0},{4,2},{2,2},{2,4},{0,4}};
      BRepBuilderAPI_Sewing sewing(1e-8);sewing.Add(face(0,1,0,4,0,4,3,outline));sewing.Add(face(1,0,0,4,0,4,0,{{4,0},{4,2},{2,2},{2,4},{0,4},{0,0}}));
      for(size_t i=0;i<outline.size();++i){auto p=outline[i],q=outline[(i+1)%outline.size()];int a,b;double u0,u1,v0,v1,fixed;
        if(p.Y()==q.Y()){fixed=p.Y();if(q.X()>p.X()){a=0;b=2;u0=p.X();u1=q.X();v0=0;v1=3;}else{a=2;b=0;u0=0;u1=3;v0=q.X();v1=p.X();}}
        else{fixed=p.X();if(q.Y()>p.Y()){a=1;b=2;u0=p.Y();u1=q.Y();v0=0;v1=3;}else{a=2;b=1;u0=0;u1=3;v0=q.Y();v1=p.Y();}}
        sewing.Add(face(a,b,u0,u1,v0,v1,fixed,{{u0,v0},{u1,v0},{u1,v1},{u0,v1}}));
      }
      sewing.Perform();auto shape=BRepBuilderAPI_MakeSolid(TopoDS::Shell(sewing.SewedShape())).Solid();
      if(mode=="write-step-orthogonal-affine-spline-shell"){gp_Trsf tr;tr.SetValues(3,-4,0,7,4,3,0,11,0,0,5,13);shape=TopoDS::Solid(BRepBuilderAPI_Transform(shape,tr,true).Shape());}
      if(!BRepCheck_Analyzer(shape).IsValid())throw std::runtime_error("invalid axis aligned spline solid");STEPControl_Writer writer;if(writer.Transfer(shape,STEPControl_AsIs)!=IFSelect_RetDone||writer.Write(argv[1])!=IFSelect_RetDone)throw std::runtime_error("axis aligned spline solid export failed");return 0;
#else
      throw std::runtime_error("reference executable built without STEP support");
#endif
    }
    if (argc == 3 && (mode == "write-step-orthogonal-open-shell" || mode == "write-step-orthogonal-holed-open-shell")) {
#ifdef PLAINSOLID_OCCT_STEP
      auto face=[&](double u0,double u1,double v0,double v1,const std::vector<gp_Pnt2d>& uv){
        NCollection_Array2<gp_Pnt> poles(1,2,1,2);
        poles.SetValue(1,1,gp_Pnt(u0,v0,u0*v0/16));poles.SetValue(1,2,gp_Pnt(u0,v1,u0*v1/16));
        poles.SetValue(2,1,gp_Pnt(u1,v0,u1*v0/16));poles.SetValue(2,2,gp_Pnt(u1,v1,u1*v1/16));
        NCollection_Array1<double> uk(1,2),vk(1,2);uk.SetValue(1,u0);uk.SetValue(2,u1);vk.SetValue(1,v0);vk.SetValue(2,v1);
        NCollection_Array1<int> mults(1,2);mults.SetValue(1,2);mults.SetValue(2,2);
        occ::handle<Geom_BSplineSurface> support=new Geom_BSplineSurface(poles,uk,vk,mults,mults,1,1);
        BRepBuilderAPI_MakeWire wire;
        for(size_t i=0;i<uv.size();++i){const auto a=uv[i],b=uv[(i+1)%uv.size()];const bool vertical=a.X()==b.X();
          const double lo=vertical?std::min(a.Y(),b.Y()):std::min(a.X(),b.X()),hi=vertical?std::max(a.Y(),b.Y()):std::max(a.X(),b.X());
          occ::handle<Geom2d_Line> pc=new Geom2d_Line(gp_Pnt2d(vertical?a.X():0,vertical?0:a.Y()),gp_Dir2d(vertical?0:1,vertical?1:0));
          auto edge=BRepBuilderAPI_MakeEdge(pc,support,lo,hi).Edge();if(vertical?b.Y()<a.Y():b.X()<a.X())edge.Reverse();wire.Add(edge);
        }
        auto boundary=wire.Wire();BRepLib::BuildCurves3d(boundary);BRepBuilderAPI_MakeFace maker(support,boundary,true);
        if(mode=="write-step-orthogonal-holed-open-shell"&&u0==0) {
          const std::vector<gp_Pnt2d> hole={{.5,.5},{.5,1.5},{1.5,1.5},{1.5,.5}};BRepBuilderAPI_MakeWire inner;
          for(size_t i=0;i<hole.size();++i){const auto a=hole[i],b=hole[(i+1)%hole.size()];const bool vertical=a.X()==b.X();
            const double lo=vertical?std::min(a.Y(),b.Y()):std::min(a.X(),b.X()),hi=vertical?std::max(a.Y(),b.Y()):std::max(a.X(),b.X());
            occ::handle<Geom2d_Line> pc=new Geom2d_Line(gp_Pnt2d(vertical?a.X():0,vertical?0:a.Y()),gp_Dir2d(vertical?0:1,vertical?1:0));
            auto edge=BRepBuilderAPI_MakeEdge(pc,support,lo,hi).Edge();if(vertical?b.Y()<a.Y():b.X()<a.X())edge.Reverse();inner.Add(edge);
          }
          auto loop=inner.Wire();BRepLib::BuildCurves3d(loop);maker.Add(loop);
        }
        return maker.Face();
      };
      auto left=face(0,4,0,4,{{0,0},{4,0},{4,2},{2,2},{2,4},{0,4}});
      auto right=face(4,6,0,2,{{4,0},{6,0},{6,2},{4,2}});
      BRepBuilderAPI_Sewing sewing(1e-8);sewing.Add(left);sewing.Add(right);sewing.Perform();auto shape=sewing.SewedShape();
      if(shape.ShapeType()!=TopAbs_SHELL||!BRepCheck_Analyzer(shape).IsValid())throw std::runtime_error("invalid orthogonal open shell");
      STEPControl_Writer writer;if(writer.Transfer(shape,STEPControl_AsIs)!=IFSelect_RetDone||writer.Write(argv[1])!=IFSelect_RetDone)throw std::runtime_error("orthogonal open shell export failed");return 0;
#else
      throw std::runtime_error("reference executable built without STEP support");
#endif
    }
    if (argc == 3 && (mode == "write-step-orthogonal-holed-spline-face" || mode == "write-iges-orthogonal-holed-spline-face")) {
#ifdef PLAINSOLID_OCCT_STEP
      NCollection_Array2<gp_Pnt> poles(1,2,1,2);
      poles.SetValue(1,1,gp_Pnt(0,0,0));poles.SetValue(1,2,gp_Pnt(0,4,0));
      poles.SetValue(2,1,gp_Pnt(4,0,0));poles.SetValue(2,2,gp_Pnt(4,4,1));
      NCollection_Array1<double> knots(1,2);knots.SetValue(1,0);knots.SetValue(2,4);
      NCollection_Array1<int> mults(1,2);mults.SetValue(1,2);mults.SetValue(2,2);
      occ::handle<Geom_BSplineSurface> support=new Geom_BSplineSurface(poles,knots,knots,mults,mults,1,1);
      auto makeWire=[&](const std::vector<gp_Pnt2d>& uv){
        BRepBuilderAPI_MakeWire wire;
        for(size_t i=0;i<uv.size();++i){const auto a=uv[i],b=uv[(i+1)%uv.size()];const bool vertical=a.X()==b.X();
          const double lo=vertical?std::min(a.Y(),b.Y()):std::min(a.X(),b.X()),hi=vertical?std::max(a.Y(),b.Y()):std::max(a.X(),b.X());
          occ::handle<Geom2d_Line> pc=new Geom2d_Line(gp_Pnt2d(vertical?a.X():0,vertical?0:a.Y()),gp_Dir2d(vertical?0:1,vertical?1:0));
          auto edge=BRepBuilderAPI_MakeEdge(pc,support,lo,hi).Edge();if(vertical?b.Y()<a.Y():b.X()<a.X())edge.Reverse();wire.Add(edge);
        }
        auto result=wire.Wire();BRepLib::BuildCurves3d(result);return result;
      };
      auto outer=makeWire({{0,0},{4,0},{4,2},{2,2},{2,4},{0,4}});
      auto first=makeWire({{.5,.5},{1.5,.5},{1.5,1},{1,1},{1,1.5},{.5,1.5}});
      auto second=makeWire({{2.5,.5},{3.5,.5},{3.5,1.5},{2.5,1.5}});
      first.Reverse();second.Reverse();BRepBuilderAPI_MakeFace maker(support,outer,true);maker.Add(first);maker.Add(second);auto shape=maker.Face();
      if(!BRepCheck_Analyzer(shape).IsValid())throw std::runtime_error("invalid orthogonal holed spline face");
      if(mode=="write-iges-orthogonal-holed-spline-face") {
#ifdef PLAINSOLID_OCCT_IGES
        IGESControl_Writer writer("MM");if(!writer.AddShape(shape))throw std::runtime_error("orthogonal IGES transfer failed");
        writer.ComputeModel();if(!writer.Write(argv[1]))throw std::runtime_error("orthogonal IGES export failed");return 0;
#else
        throw std::runtime_error("reference executable built without IGES support");
#endif
      }
      STEPControl_Writer writer;if(writer.Transfer(shape,STEPControl_AsIs)!=IFSelect_RetDone||writer.Write(argv[1])!=IFSelect_RetDone)throw std::runtime_error("orthogonal holed spline face export failed");
      return 0;
#else
      throw std::runtime_error("reference executable built without STEP support");
#endif
    }
    if (argc == 3 && mode == "write-step-orthogonal-spline-face") {
#ifdef PLAINSOLID_OCCT_STEP
      NCollection_Array2<gp_Pnt> poles(1,2,1,2);
      poles.SetValue(1,1,gp_Pnt(0,0,0));poles.SetValue(1,2,gp_Pnt(0,2,0));
      poles.SetValue(2,1,gp_Pnt(42,0,0));poles.SetValue(2,2,gp_Pnt(42,2,21));
      NCollection_Array2<double> weights(1,2,1,2);weights.SetValue(1,1,1);weights.SetValue(1,2,1);weights.SetValue(2,1,2);weights.SetValue(2,2,2);
      NCollection_Array1<double> knots(1,2);knots.SetValue(1,0);knots.SetValue(2,1);
      NCollection_Array1<int> mults(1,2);mults.SetValue(1,2);mults.SetValue(2,2);
      occ::handle<Geom_BSplineSurface> support=new Geom_BSplineSurface(poles,weights,knots,knots,mults,mults,1,1);
      const double uv[6][2]={{.3125,0},{1,0},{1,.5},{.75,.5},{.75,1},{.3125,1}};
      BRepBuilderAPI_MakeWire wire;
      for(int i=0;i<6;++i){const auto a=uv[i];const auto b=uv[(i+1)%6];const bool vertical=a[0]==b[0];const double lo=vertical?std::min(a[1],b[1]):std::min(a[0],b[0]),hi=vertical?std::max(a[1],b[1]):std::max(a[0],b[0]);
        occ::handle<Geom2d_Line> pc=new Geom2d_Line(gp_Pnt2d(vertical?a[0]:0,vertical?0:a[1]),gp_Dir2d(vertical?0:1,vertical?1:0));
        auto edge=BRepBuilderAPI_MakeEdge(pc,support,lo,hi).Edge();if((vertical?b[1]<a[1]:b[0]<a[0]))edge.Reverse();wire.Add(edge);}
      const auto boundary=wire.Wire();BRepLib::BuildCurves3d(boundary);const auto shape=BRepBuilderAPI_MakeFace(support,boundary,true).Face();
      if(!BRepCheck_Analyzer(shape).IsValid())throw std::runtime_error("invalid bounded spline face");
      STEPControl_Writer writer;if(writer.Transfer(shape,STEPControl_AsIs)!=IFSelect_RetDone||writer.Write(argv[1])!=IFSelect_RetDone)throw std::runtime_error("bounded spline face export failed");
      return 0;
#else
      throw std::runtime_error("reference executable built without STEP support");
#endif
    }
    if (argc == 3 && mode == "write-step-bounded-spline-face") {
#ifdef PLAINSOLID_OCCT_STEP
      NCollection_Array2<gp_Pnt> poles(1,2,1,2);
      poles.SetValue(1,1,gp_Pnt(0,0,0));poles.SetValue(1,2,gp_Pnt(0,2,0));
      poles.SetValue(2,1,gp_Pnt(42,0,0));poles.SetValue(2,2,gp_Pnt(42,2,21));
      NCollection_Array2<double> weights(1,2,1,2);weights.SetValue(1,1,1);weights.SetValue(1,2,1);weights.SetValue(2,1,2);weights.SetValue(2,2,2);
      NCollection_Array1<double> knots(1,2);knots.SetValue(1,0);knots.SetValue(2,1);
      NCollection_Array1<int> mults(1,2);mults.SetValue(1,2);mults.SetValue(2,2);
      occ::handle<Geom_BSplineSurface> support=new Geom_BSplineSurface(poles,weights,knots,knots,mults,mults,1,1);
      const auto shape=BRepBuilderAPI_MakeFace(support,.3125,.75,.25,.75,1e-8).Face();
      if(!BRepCheck_Analyzer(shape).IsValid())throw std::runtime_error("invalid bounded spline face");
      STEPControl_Writer writer;if(writer.Transfer(shape,STEPControl_AsIs)!=IFSelect_RetDone||writer.Write(argv[1])!=IFSelect_RetDone)throw std::runtime_error("bounded spline face export failed");
      return 0;
#else
      throw std::runtime_error("reference executable built without STEP support");
#endif
    }
    if (argc == 3 && mode == "write-step-conical-sector") {
#ifdef PLAINSOLID_OCCT_STEP
      const auto shape=BRepPrimAPI_MakeCone(gp_Ax2(gp_Pnt(1,2,3),gp_Dir(0,0,1)),3,1,4,1.5707963267948966).Shape();
      if(!BRepCheck_Analyzer(shape).IsValid())throw std::runtime_error("invalid conical sector fixture");
      STEPControl_Writer writer;
      if(writer.Transfer(shape,STEPControl_AsIs)!=IFSelect_RetDone||writer.Write(argv[1])!=IFSelect_RetDone)throw std::runtime_error("conical sector export failed");
      return 0;
#else
      throw std::runtime_error("reference executable built without STEP support");
#endif
    }
    if (argc == 3 && mode == "write-step-cylinder-sector") {
#ifdef PLAINSOLID_OCCT_STEP
      const auto shape=BRepPrimAPI_MakeCylinder(gp_Ax2(gp_Pnt(1,2,3),gp_Dir(0,0,1)),2,4,1.5707963267948966).Shape();
      if(!BRepCheck_Analyzer(shape).IsValid())throw std::runtime_error("invalid cylinder sector fixture");
      STEPControl_Writer writer;
      if(writer.Transfer(shape,STEPControl_AsIs)!=IFSelect_RetDone||writer.Write(argv[1])!=IFSelect_RetDone)throw std::runtime_error("cylinder sector export failed");
      return 0;
#else
      throw std::runtime_error("reference executable built without STEP support");
#endif
    }
    if (argc == 3 && mode == "write-step-mixed-axial") {
#ifdef PLAINSOLID_OCCT_STEP
      const auto a=BRepPrimAPI_MakeCylinder(gp_Ax2(gp_Pnt(1,2,3),gp_Dir(0,0,1)),3,2).Shape();
      const auto b=BRepPrimAPI_MakeCone(gp_Ax2(gp_Pnt(1,2,5),gp_Dir(0,0,1)),2,1,2).Shape();
      const auto c=BRepPrimAPI_MakeCylinder(gp_Ax2(gp_Pnt(1,2,7),gp_Dir(0,0,1)),0.5,1).Shape();
      const auto shape=BRepAlgoAPI_Fuse(BRepAlgoAPI_Fuse(a,b).Shape(),c).Shape();
      if(!BRepCheck_Analyzer(shape).IsValid())throw std::runtime_error("invalid mixed axial fixture");
      STEPControl_Writer writer;
      if(writer.Transfer(shape,STEPControl_AsIs)!=IFSelect_RetDone||writer.Write(argv[1])!=IFSelect_RetDone)throw std::runtime_error("mixed axial export failed");
      return 0;
#else
      throw std::runtime_error("reference executable built without STEP support");
#endif
    }
    if (argc == 3 && mode == "write-step-hollow-cone") {
#ifdef PLAINSOLID_OCCT_STEP
      const auto outer=BRepPrimAPI_MakeCone(gp_Ax2(gp_Pnt(1,2,3),gp_Dir(0,0,1)),3,2,4).Shape();
      const auto inner=BRepPrimAPI_MakeCone(gp_Ax2(gp_Pnt(1,2,3),gp_Dir(0,0,1)),1,0.5,4).Shape();
      const auto shape=BRepAlgoAPI_Cut(outer,inner).Shape();
      if(!BRepCheck_Analyzer(shape).IsValid())throw std::runtime_error("invalid hollow cone fixture");
      STEPControl_Writer writer;
      if(writer.Transfer(shape,STEPControl_AsIs)!=IFSelect_RetDone||writer.Write(argv[1])!=IFSelect_RetDone)throw std::runtime_error("hollow cone export failed");
      return 0;
#else
      throw std::runtime_error("reference executable built without STEP support");
#endif
    }
    if (argc == 3 && mode == "write-step-tube") {
#ifdef PLAINSOLID_OCCT_STEP
      const auto outer=BRepPrimAPI_MakeCylinder(gp_Ax2(gp_Pnt(1,2,3),gp_Dir(0,0,1)),3,4).Shape();
      const auto inner=BRepPrimAPI_MakeCylinder(gp_Ax2(gp_Pnt(1,2,3),gp_Dir(0,0,1)),1,4).Shape();
      const auto shape=BRepAlgoAPI_Cut(outer,inner).Shape();
      if(!BRepCheck_Analyzer(shape).IsValid())throw std::runtime_error("invalid tube fixture");
      STEPControl_Writer writer;
      if(writer.Transfer(shape,STEPControl_AsIs)!=IFSelect_RetDone||writer.Write(argv[1])!=IFSelect_RetDone)throw std::runtime_error("tube export failed");
      return 0;
#else
      throw std::runtime_error("reference executable built without STEP support");
#endif
    }
    if (argc == 3 && mode == "write-step-cavity") {
#ifdef PLAINSOLID_OCCT_STEP
      const auto outer=BRepPrimAPI_MakeBox(10,10,10).Shape();
      const auto inner=BRepPrimAPI_MakeBox(gp_Pnt(2,2,2),3,3,3).Shape();
      const auto shape=BRepAlgoAPI_Cut(outer,inner).Shape();
      if(!BRepCheck_Analyzer(shape).IsValid())throw std::runtime_error("invalid cavity fixture");
      STEPControl_Writer writer;
      if(writer.Transfer(shape,STEPControl_AsIs)!=IFSelect_RetDone||writer.Write(argv[1])!=IFSelect_RetDone)throw std::runtime_error("cavity export failed");
      return 0;
#else
      throw std::runtime_error("reference executable built without STEP support");
#endif
    }
    if (argc == 3 && mode == "write-step-oblique-cylinder") {
#ifdef PLAINSOLID_OCCT_STEP
      const auto cylinder = BRepPrimAPI_MakeCylinder(gp_Ax2(gp_Pnt(1, 2, 3), gp_Dir(0, 0, 1)), 2, 8).Shape();
      const auto cap = BRepBuilderAPI_MakeFace(gp_Pln(gp_Pnt(1, 2, 7), gp_Dir(-0.25, 0, 1)), -100, 100, -100, 100).Face();
      const auto half = BRepPrimAPI_MakeHalfSpace(cap, gp_Pnt(1, 2, 3)).Solid();
      const auto shape = BRepAlgoAPI_Common(cylinder, half).Shape();
      if (!BRepCheck_Analyzer(shape).IsValid()) throw std::runtime_error("invalid oblique cylinder fixture");
      STEPControl_Writer writer;
      Interface_Static::SetIVal("write.surfacecurve.mode", 0);
      if (writer.Transfer(shape, STEPControl_AsIs) != IFSelect_RetDone || writer.Write(argv[1]) != IFSelect_RetDone)
        throw std::runtime_error("OCCT oblique cylinder export failed");
      return 0;
#else
      throw std::runtime_error("reference executable built without STEP support");
#endif
    }
    if (argc == 3 && (mode == "write-step-inch" || mode == "write-step-foot" || mode == "write-step" || mode == "write-step-curves" || mode == "write-step-surface" || mode == "write-step-sphere" || mode == "write-step-torus" || mode == "write-step-cylinder" || mode == "write-step-cone" || mode == "write-step-frustum")) {
#ifdef PLAINSOLID_OCCT_STEP
      STEPControl_Writer writer;
      if(mode == "write-step-inch") Interface_Static::SetCVal("write.step.unit", "INCH");
      if(mode == "write-step-foot") Interface_Static::SetCVal("write.step.unit", "FT");
      const bool curves = mode == "write-step-curves";
      const bool surface = mode == "write-step-surface";
      const TopoDS_Shape shape = mode == "write-step-sphere" ? BRepPrimAPI_MakeSphere(gp_Pnt(1, 2, 3), 2).Shape()
          : mode == "write-step-torus" ? BRepPrimAPI_MakeTorus(gp_Ax2(gp_Pnt(1, 2, 3), gp_Dir(0, 0, 1)), 3, 1).Shape()
          : mode == "write-step-cylinder" ? BRepPrimAPI_MakeCylinder(gp_Ax2(gp_Pnt(1, 2, 3), gp_Dir(0, 0, 1)), 2, 4).Shape()
          : mode == "write-step-cone" ? BRepPrimAPI_MakeCone(gp_Ax2(gp_Pnt(1, 2, 3), gp_Dir(0, 0, 1)), 2, 0, 4).Shape()
          : mode == "write-step-frustum" ? BRepPrimAPI_MakeCone(gp_Ax2(gp_Pnt(1, 2, 3), gp_Dir(0, 0, 1)), 2, 1, 4).Shape()
          : curves ? TopoDS_Shape(quarterCircle())
          : surface ? TopoDS_Shape(BRepBuilderAPI_MakeFace(quarterCylinder(), 1e-8).Face())
          : BRepPrimAPI_MakeBox(gp_Pnt(-1, 2, 3), 2, 3, 4).Shape();
      if (writer.Transfer(shape, curves ? STEPControl_GeometricCurveSet : STEPControl_AsIs) != IFSelect_RetDone
          || writer.Write(argv[1]) != IFSelect_RetDone)
        throw std::runtime_error("OCCT STEP export failed");
      return 0;
#else
      throw std::runtime_error("reference executable built without STEP support");
#endif
    }
#ifdef PLAINSOLID_OCCT_STEP
    if(argc==3 && mode=="write-step-long-edge-chain")return writeLongEdgeChainReference(argv[1],true);
    if(argc==3 && mode=="write-step-long-edge-treatment")return writeLongEdgeChainReference(argv[1],false);
    if(argc==3 && mode=="write-step-rounded-stock")return writeRoundedStockReference(argv[1]);
    if(argc==3 && mode=="write-step-rounded-pocket")return writeRoundedStockReference(argv[1],true);
    if(argc==3 && mode=="write-step-disk-pocket")return writeDiskPocketReference(argv[1]);
    if(argc==3 && mode=="write-step-circular-pocket")return writeCircularPocketReference(argv[1]);
    if(argc==3 && mode=="write-step-circular-through")return writeCircularPocketReference(argv[1],true);
    if(argc==3 && mode=="circular-pocket-extrusion")return circularHoleExtrusionReference(argv[1],1);
    if(argc==3 && mode=="circular-blind-extrusion")return circularHoleExtrusionReference(argv[1],2);
    if(argc==3 && mode=="circular-hole-extrusion")return circularHoleExtrusionReference(argv[1]);
    if(argc==3 && mode=="polygon-revolve-axis")return polygonRevolveReference(argv[1],true);
    if(argc==3 && mode=="polygon-revolve")return polygonRevolveReference(argv[1]);
#endif
    if (argc == 3 && mode == "write-iges") {
#ifdef PLAINSOLID_OCCT_IGES
      IGESControl_Writer writer("MM");
      const auto line = BRepBuilderAPI_MakeEdge(gp_Pnt(0, 0, 0), gp_Pnt(1, 2, 2)).Edge();
      const gp_Circ circle(gp_Ax2(gp_Pnt(0, 0, 3), gp_Dir(0, 0, 1)), 2);
      const auto arc = BRepBuilderAPI_MakeEdge(circle, 0, std::acos(0.0)).Edge();
      const auto spline = quarterCircle();
      if (!writer.AddShape(line) || !writer.AddShape(arc) || !writer.AddShape(spline)
          || !writer.Write(argv[1]))
        throw std::runtime_error("OCCT IGES export failed");
      return 0;
#else
      throw std::runtime_error("reference executable built without IGES support");
#endif
    }
#ifdef PLAINSOLID_OCCT_IGES
#ifdef PLAINSOLID_OCCT_STEP
    if(argc==3&&mode=="write-step-quadratic-annulus")return writeSTEPQuadraticTrim(argv[1]);
#endif
#ifdef PLAINSOLID_OCCT_STEP
    if(argc==3 && mode=="write-iges-concurrent-lens")return writeIGESPartitionedLens(argv[1],true);
    if(argc==3 && mode=="write-iges-affine-partitioned-lens")return writeAffinePartitionedIGESLens(argv[1]);
    if(argc==3 && mode=="write-iges-sheared-partitioned-lens")return writeShearedPartitionedIGESLens(argv[1]);
    if(argc==3 && mode=="write-iges-partitioned-lens")return writeIGESPartitionedLens(argv[1]);
    if(argc==3 && mode=="write-iges-raw-curve-lens")return writeIGESRefinedCurves(argv[1],true);
    if(argc==3 && mode=="write-iges-refined-curve-lens")return writeIGESRefinedCurves(argv[1]);
    if(argc==3 && mode=="write-iges-refined-lens")return writeIGESSplineLens(argv[1],false,true);
    if(argc==3 && mode=="write-iges-sheared-lens")return writeIGESSplineLens(argv[1],true);
    if(argc==3 && mode=="write-iges-spline-lens")return writeIGESSplineLens(argv[1]);
    if(argc==4 && mode=="iges-solid")return inspectIGESLens(argv[1],argv[3]);
#endif
    if(argc==4 && mode=="iges-topology")return inspectIGESCurvedTopology(argv[1],argv[3]);
    if(argc==3 && mode=="write-iges-curved-topology")return writeIGESCurvedTopology(argv[1]);
    if(argc==3 && mode=="write-iges-curved-sheet")return writeIGESCurvedSheet(argv[1]);
    if(argc==3 && (mode=="write-iges-curved-oval"||mode=="write-iges-curved-annulus"))return writeIGESCurvedOval(argv[1],mode=="write-iges-curved-annulus");
    if(argc==3 && (mode=="write-iges-quadratic-trim"||mode=="write-iges-quadratic-annulus"))return writeIGESQuadraticTrim(argv[1],mode=="write-iges-quadratic-annulus");
#endif
    if (argc == 3 && (mode == "write-iges-pcurve" || mode == "write-iges-trimmed")) {
#ifdef PLAINSOLID_OCCT_IGES
      IGESControl_Writer writer("MM");
      NCollection_Array2<gp_Pnt> poles(1,2,1,2);
      poles.SetValue(1,1,gp_Pnt(0,0,0));poles.SetValue(1,2,gp_Pnt(0,3,0));
      poles.SetValue(2,1,gp_Pnt(2,0,0));poles.SetValue(2,2,gp_Pnt(2,3,0));
      NCollection_Array1<double> knots(1,2);knots.SetValue(1,0);knots.SetValue(2,1);
      NCollection_Array1<int> mults(1,2);mults.SetValue(1,2);mults.SetValue(2,2);
      occ::handle<Geom_BSplineSurface> plane = new Geom_BSplineSurface(poles,knots,knots,mults,mults,1,1);
      GeomToIGES_GeomSurface surfaceConverter;surfaceConverter.SetModel(writer.Model());
      auto surface = surfaceConverter.TransferSurface(plane,0,1,0,1);
      if (mode == "write-iges-trimmed") {
        auto contour = [&](bool inner) {
          const double lo=inner?.375:.125, hi=inner?.625:.875;
          NCollection_Array1<gp_Pnt> uvp(1,5);
          uvp.SetValue(1,gp_Pnt(lo,lo,0));uvp.SetValue(2,inner?gp_Pnt(lo,hi,0):gp_Pnt(hi,lo,0));
          uvp.SetValue(3,gp_Pnt(hi,hi,0));uvp.SetValue(4,inner?gp_Pnt(hi,lo,0):gp_Pnt(lo,hi,0));uvp.SetValue(5,uvp.Value(1));
          NCollection_Array1<gp_Pnt> mp(1,5);for(int i=1;i<=5;++i)mp.SetValue(i,gp_Pnt(2*uvp.Value(i).X(),3*uvp.Value(i).Y(),0));
          NCollection_Array1<double> k(1,5);NCollection_Array1<int> mult(1,5);
          for(int i=1;i<=5;++i){k.SetValue(i,i-1);mult.SetValue(i,i==1||i==5?2:1);}
          occ::handle<Geom_BSplineCurve> uv=new Geom_BSplineCurve(uvp,k,mult,1),model=new Geom_BSplineCurve(mp,k,mult,1);
          GeomToIGES_GeomCurve converter;converter.SetModel(writer.Model());
          occ::handle<IGESGeom_CurveOnSurface> c=new IGESGeom_CurveOnSurface;
          c->Init(0,surface,converter.TransferCurve(uv,0,4),converter.TransferCurve(model,0,4),3);return c;
        };
        auto outer=contour(false),inner=contour(true);
        occ::handle<NCollection_HArray1<occ::handle<IGESGeom_CurveOnSurface>>> holes=new NCollection_HArray1<occ::handle<IGESGeom_CurveOnSurface>>(1,1);
        holes->SetValue(1,inner);occ::handle<IGESGeom_TrimmedSurface> trimmed=new IGESGeom_TrimmedSurface;
        trimmed->Init(surface,1,outer,holes);
        if(!writer.AddEntity(trimmed)||!writer.Write(argv[1]))throw std::runtime_error("OCCT trimmed plane export failed");
        return 0;
      }
      auto makeCurve = [](bool model) {
        NCollection_Array1<gp_Pnt> p(1,3);p.SetValue(1,gp_Pnt(.25,.25,0));p.SetValue(2,gp_Pnt(.75,.25,0));p.SetValue(3,gp_Pnt(.75,.75,0));
        if(model)for(int i=1;i<=3;++i)p.SetValue(i,gp_Pnt(2*p.Value(i).X(),3*p.Value(i).Y(),0));
        NCollection_Array1<double> w(1,3);w.SetValue(1,1);w.SetValue(2,std::sqrt(.5));w.SetValue(3,1);
        NCollection_Array1<double> k(1,2);k.SetValue(1,0);k.SetValue(2,1);
        NCollection_Array1<int> m(1,2);m.SetValue(1,3);m.SetValue(2,3);
        return occ::handle<Geom_BSplineCurve>(new Geom_BSplineCurve(p,w,k,m,2));
      };
      GeomToIGES_GeomCurve curveConverter;curveConverter.SetModel(writer.Model());
      auto uv = curveConverter.TransferCurve(makeCurve(false),0,1);
      auto model = curveConverter.TransferCurve(makeCurve(true),0,1);
      occ::handle<IGESGeom_CurveOnSurface> result = new IGESGeom_CurveOnSurface;
      result->Init(0,surface,uv,model,3);
      if(!writer.AddEntity(result)||!writer.Write(argv[1]))throw std::runtime_error("OCCT pcurve export failed");
      return 0;
#else
      throw std::runtime_error("reference executable built without IGES support");
#endif
    }
    if (argc == 3 && mode == "write-iges-surface") {
#ifdef PLAINSOLID_OCCT_IGES
      IGESControl_Writer writer("MM");
      auto surface = quarterCylinder();
      GeomToIGES_GeomSurface converter;
      converter.SetModel(writer.Model());
      auto iges = converter.TransferSurface(surface, 0, 1, 0, 1);
      if (iges.IsNull() || !writer.AddEntity(iges) || !writer.Write(argv[1]))
        throw std::runtime_error("OCCT IGES surface export failed");
      return 0;
#else
      throw std::runtime_error("reference executable built without IGES support");
#endif
    }
    std::ofstream out(argv[1]);
    out.exceptions(std::ios::failbit | std::ios::badbit);
    if (argc == 4 && (mode == "iges" || mode == "iges-surface")) {
#ifdef PLAINSOLID_OCCT_IGES
      IGESControl_Reader reader;
      if (reader.ReadFile(argv[3]) != IFSelect_RetDone || reader.TransferRoots() == 0)
        throw std::runtime_error("OCCT IGES import failed");
      const auto shape = reader.OneShape();
      if (shape.IsNull() || !BRepCheck_Analyzer(shape).IsValid())
        throw std::runtime_error("invalid OCCT IGES import");
      int count = 0;
      const bool surface = mode == "iges-surface";
      for (TopExp_Explorer ex(shape, surface ? TopAbs_FACE : TopAbs_EDGE); ex.More(); ex.Next()) ++count;
      GProp_GProps properties;
      if (surface) BRepGProp::SurfaceProperties(shape, properties);
      else BRepGProp::LinearProperties(shape, properties);
      out << std::setprecision(17) << (surface ? "{\"Faces\":" : "{\"Edges\":") << count
          << (surface ? ",\"Area\":" : ",\"Length\":") << properties.Mass() << ",\"Centroid\":[" << properties.CentreOfMass().X() << "," << properties.CentreOfMass().Y() << "," << properties.CentreOfMass().Z() << "]}\n";
      return 0;
#else
      throw std::runtime_error("reference executable built without IGES support");
#endif
    }
    if (argc == 4 && (mode == "step-curves" || mode == "step-surface")) {
#ifdef PLAINSOLID_OCCT_STEP
      STEPControl_Reader reader;
      if (reader.ReadFile(argv[3]) != IFSelect_RetDone || reader.TransferRoots() == 0)
        throw std::runtime_error("OCCT STEP curve import failed");
      const auto shape = reader.OneShape();
      if (shape.IsNull() || !BRepCheck_Analyzer(shape).IsValid())
        throw std::runtime_error("invalid OCCT STEP curve import");
      int count = 0;
      const bool surface = mode == "step-surface";
      for (TopExp_Explorer ex(shape, surface ? TopAbs_FACE : TopAbs_EDGE); ex.More(); ex.Next()) ++count;
      GProp_GProps measure;
      if (surface) BRepGProp::SurfaceProperties(shape, measure);
      else BRepGProp::LinearProperties(shape, measure);
      out << std::setprecision(17) << (surface ? "{\"Faces\":" : "{\"Edges\":") << count
          << (surface ? ",\"Area\":" : ",\"Length\":") << measure.Mass() << "}\n";
      return 0;
#else
      throw std::runtime_error("reference executable built without STEP support");
#endif
    }
    if (argc == 4 && mode == "step-mass-precise") {
#ifdef PLAINSOLID_OCCT_STEP
      STEPControl_Reader reader;
      Interface_Static::SetIVal("read.precision.mode", 1);
      Interface_Static::SetRVal("read.precision.val", 1e-10);
      Interface_Static::SetRVal("read.maxprecision.val", 1e-10);
      if (reader.ReadFile(argv[3]) != IFSelect_RetDone || reader.TransferRoots() == 0)
        throw std::runtime_error("OCCT precise STEP import failed");
      const auto shape = reader.OneShape();
      if (shape.IsNull() || !BRepCheck_Analyzer(shape).IsValid())
        throw std::runtime_error("invalid OCCT precise STEP import");
      GProp_GProps volume, area;
      BRepGProp::VolumeProperties(shape, volume, 1e-12);
      BRepGProp::SurfaceProperties(shape, area, 1e-12);
      const auto c = volume.CentreOfMass();
      out << std::setprecision(17) << "{\"Area\":" << area.Mass() << ",\"Volume\":" << volume.Mass()
          << ",\"Centroid\":[" << c.X() << ',' << c.Y() << ',' << c.Z() << "]}\n";
      return 0;
#else
      throw std::runtime_error("reference executable built without STEP support");
#endif
    }
    if (argc == 4) throw std::runtime_error("unknown reference mode");
    if (argc == 3) {
#ifdef PLAINSOLID_OCCT_STEP
      STEPControl_Reader reader;
      if (reader.ReadFile(argv[2]) != IFSelect_RetDone || reader.TransferRoots() == 0)
        throw std::runtime_error("OCCT STEP import failed");
      record(out, "step", reader.OneShape());
      return 0;
#else
      throw std::runtime_error("reference executable built without STEP support");
#endif
    }
    record(out, "box", BRepPrimAPI_MakeBox(gp_Pnt(-1, 2, 3), 2, 3, 4).Shape());
    const auto a = BRepPrimAPI_MakeSphere(gp_Pnt(0, 0, 0), 2).Shape();
    const auto b = BRepPrimAPI_MakeSphere(gp_Pnt(2.25, 0, 0), 1.75).Shape();
    record(out, "sphere", a);
    record(out, "cylinder", BRepPrimAPI_MakeCylinder(1.5, 2.25).Shape());
    record(out, "cone", BRepPrimAPI_MakeCone(0, 1.5, 2.25).Shape());
    record(out, "torus", BRepPrimAPI_MakeTorus(3, .75).Shape());
    record(out, "union", BRepAlgoAPI_Fuse(a, b).Shape());
    record(out, "intersection", BRepAlgoAPI_Common(a, b).Shape());
    record(out, "difference", BRepAlgoAPI_Cut(a, b).Shape());
    const auto boreBall = BRepPrimAPI_MakeSphere(3).Shape();
    const auto boreTool = BRepPrimAPI_MakeCylinder(gp_Ax2(gp_Pnt(0, 0, -4), gp_Dir(0, 0, 1)), std::sqrt(5.0), 8).Shape();
    record(out, "bored-sphere", BRepAlgoAPI_Cut(boreBall, boreTool).Shape());
    record(out, "spherical-core", BRepAlgoAPI_Common(boreBall, boreTool).Shape());
    const auto finiteBall = BRepPrimAPI_MakeSphere(5).Shape();
    const auto finiteBore = BRepPrimAPI_MakeCylinder(gp_Ax2(gp_Pnt(0, 0, 2), gp_Dir(0, 0, 1)), 3, 4).Shape();
    const auto finiteBand = BRepPrimAPI_MakeCylinder(gp_Ax2(gp_Pnt(0, 0, -4), gp_Dir(0, 0, 1)), 5, 8).Shape();
    const auto algebraicBand = BRepPrimAPI_MakeCylinder(gp_Ax2(gp_Pnt(0, 0, -4.75), gp_Dir(0, 0, 1)), 2, 9.5).Shape();
    record(out, "finite-sphere-cylinder-blind-cut", BRepAlgoAPI_Cut(finiteBall, finiteBore).Shape());
    record(out, "finite-sphere-cylinder-union", BRepAlgoAPI_Fuse(finiteBall, finiteBand).Shape());
    record(out, "finite-sphere-cylinder-common", BRepAlgoAPI_Common(finiteBall, finiteBore).Shape());
    record(out, "finite-sphere-cylinder-algebraic-common", BRepAlgoAPI_Common(finiteBall, algebraicBand).Shape());
    const auto offAxisBore = BRepPrimAPI_MakeCylinder(gp_Ax2(gp_Pnt(1, 0, -.75), gp_Dir(0, 0, 1)), 2, 7).Shape();
    record(out, "off-axis-sphere-blind-bore", BRepAlgoAPI_Cut(finiteBall, offAxisBore).Shape(), 1e-10);
    const auto wideOffsetBore = BRepPrimAPI_MakeCylinder(gp_Ax2(gp_Pnt(2, 0, .25), gp_Dir(0, 0, 1)), 1, 6).Shape();
    record(out, "wide-offset-sphere-blind-bore", BRepAlgoAPI_Cut(finiteBall, wideOffsetBore).Shape(), 1e-10);
    const auto throughOffsetBore = BRepPrimAPI_MakeCylinder(gp_Ax2(gp_Pnt(2, 0, -6), gp_Dir(0, 0, 1)), 1, 12).Shape();
    record(out, "offset-sphere-through-bore", BRepAlgoAPI_Cut(finiteBall, throughOffsetBore).Shape(), 1e-10);
    const auto clippedOffsetCore = BRepPrimAPI_MakeCylinder(gp_Ax2(gp_Pnt(2, 0, -2), gp_Dir(0, 0, 1)), 1, 8).Shape();
    record(out, "offset-sphere-cylinder-common", BRepAlgoAPI_Common(finiteBall, clippedOffsetCore).Shape(), 1e-10);
    const auto partialOffsetCore = BRepPrimAPI_MakeCylinder(gp_Ax2(gp_Pnt(2, 0, -2), gp_Dir(0, 0, 1)), 1, 6.25).Shape();
    record(out, "offset-sphere-cylinder-partial-common", BRepAlgoAPI_Common(finiteBall, partialOffsetCore).Shape(), 1e-10);
    const auto chordOffsetCore = BRepPrimAPI_MakeCylinder(gp_Ax2(gp_Pnt(2, 0, -2), gp_Dir(0, 0, 1)), 1, 6.875).Shape();
    record(out, "offset-sphere-cylinder-chord-common", BRepAlgoAPI_Common(finiteBall, chordOffsetCore).Shape(), 1e-10);
    const auto doubleOffsetCore = BRepPrimAPI_MakeCylinder(gp_Ax2(gp_Pnt(2, 0, -4.25), gp_Dir(0, 0, 1)), 1, 9).Shape();
    record(out, "offset-sphere-cylinder-double-common", BRepAlgoAPI_Common(finiteBall, doubleOffsetCore).Shape(), 1e-10);
    const auto offsetBoss = BRepPrimAPI_MakeCylinder(gp_Ax2(gp_Pnt(2, 0, -1), gp_Dir(0, 0, 1)), 1, 6.5).Shape();
    record(out, "offset-sphere-cylinder-union", BRepAlgoAPI_Fuse(finiteBall, offsetBoss).Shape(), 1e-10);
    const auto sameSideCore = BRepPrimAPI_MakeCylinder(gp_Ax2(gp_Pnt(2, 0, 4.125), gp_Dir(0, 0, 1)), 1, .625).Shape();
    record(out, "offset-sphere-cylinder-same-side-common", BRepAlgoAPI_Common(finiteBall, sameSideCore).Shape(), 1e-10);
    const auto coverSlab = BRepPrimAPI_MakeCylinder(gp_Ax2(gp_Pnt(1, 0, -2), gp_Dir(0, 0, 1)), 6, 9).Shape();
    record(out, "offset-sphere-cylinder-covered-slab", BRepAlgoAPI_Common(finiteBall, coverSlab).Shape(), 1e-10);
    const auto crossX = BRepPrimAPI_MakeCylinder(gp_Ax2(gp_Pnt(-4, 0, 0), gp_Dir(1, 0, 0)), 3, 8).Shape();
    const auto crossZ = BRepPrimAPI_MakeCylinder(gp_Ax2(gp_Pnt(0, 0, -5), gp_Dir(0, 0, 1)), 2, 10).Shape();
    record(out, "perpendicular-cylinder-common", BRepAlgoAPI_Common(crossX, crossZ).Shape(), 1e-10);
    const auto skewZ = BRepPrimAPI_MakeCylinder(gp_Ax2(gp_Pnt(0, .5, -5), gp_Dir(0, 0, 1)), 2, 10).Shape();
    record(out, "skew-orthogonal-cylinder-common", BRepAlgoAPI_Common(crossX, skewZ).Shape(), 1e-10);
    const auto overlapZ = BRepPrimAPI_MakeCylinder(gp_Ax2(gp_Pnt(0, 2, -5), gp_Dir(0, 0, 1)), 2, 10).Shape();
    record(out, "skew-orthogonal-overlap-common", BRepAlgoAPI_Common(crossX, overlapZ).Shape(), 1e-10);
    const auto obliqueA = BRepPrimAPI_MakeCylinder(gp_Ax2(gp_Pnt(-9, -12, 0), gp_Dir(3, 4, 0)), 2, 30).Shape();
    const auto obliqueB = BRepPrimAPI_MakeCylinder(gp_Ax2(gp_Pnt(-12, -9, 0), gp_Dir(4, 3, 0)), 2, 30).Shape();
    record(out, "oblique-equal-cylinder-common", BRepAlgoAPI_Common(obliqueA, obliqueB).Shape(), 1e-10);
    const auto generalA = BRepPrimAPI_MakeCylinder(gp_Ax2(gp_Pnt(-6, 0, 0), gp_Dir(2, 0, 0)), 1, 12).Shape();
    record(out, "oblique-unequal-cylinder-common", BRepAlgoAPI_Common(generalA, obliqueA).Shape(), 1e-10);
    const auto skewObliqueA = BRepPrimAPI_MakeCylinder(gp_Ax2(gp_Pnt(-10, 0, 0), gp_Dir(2, 0, 0)), 3, 20).Shape();
    const auto skewObliqueB = BRepPrimAPI_MakeCylinder(gp_Ax2(gp_Pnt(-15, -20, 2), gp_Dir(3, 4, 0)), 2, 50).Shape();
    record(out, "oblique-skew-cylinder-common", BRepAlgoAPI_Common(skewObliqueA, skewObliqueB).Shape(), 1e-10);
    const auto unionX = BRepPrimAPI_MakeCylinder(gp_Ax2(gp_Pnt(-3, 0, 0), gp_Dir(1, 0, 0)), 2, 6).Shape();
    const auto unionZ = BRepPrimAPI_MakeCylinder(gp_Ax2(gp_Pnt(0, 0, -4), gp_Dir(0, 0, 1)), 2, 8).Shape();
    record(out, "perpendicular-cylinder-union", BRepAlgoAPI_Fuse(unionX, unionZ).Shape(), 1e-10);
    const auto unionObliqueA = BRepPrimAPI_MakeCylinder(gp_Ax2(gp_Pnt(-12, -16, 0), gp_Dir(3, 4, 0)), 2, 40).Shape();
    const auto unionObliqueB = BRepPrimAPI_MakeCylinder(gp_Ax2(gp_Pnt(-16, -12, 0), gp_Dir(4, 3, 0)), 2, 40).Shape();
    record(out, "oblique-equal-cylinder-union", BRepAlgoAPI_Fuse(unionObliqueA, unionObliqueB).Shape(), 1e-10);
    const auto obliqueDifference = BRepAlgoAPI_Cut(unionObliqueA, unionObliqueB).Shape();
    int obliqueDifferenceCount = 0;
    for (TopExp_Explorer ex(obliqueDifference, TopAbs_SOLID); ex.More(); ex.Next()) {
      GProp_GProps props;
      BRepGProp::VolumeProperties(ex.Current(), props);
      const gp_Pnt p = props.CentreOfMass();
      record(out, 3*p.X()+4*p.Y() < 0 ? "oblique-cylinder-difference-minus" : "oblique-cylinder-difference-plus", ex.Current(), 1e-10);
      ++obliqueDifferenceCount;
    }
    if (obliqueDifferenceCount != 2) throw std::runtime_error("oblique cylinder subtraction did not yield two bodies");
    const auto splitStock = BRepPrimAPI_MakeCylinder(gp_Ax2(gp_Pnt(-5, 0, 0), gp_Dir(1, 0, 0)), 2, 10).Shape();
    const auto splitTool = BRepPrimAPI_MakeCylinder(gp_Ax2(gp_Pnt(0, 0, -4), gp_Dir(0, 0, 1)), 3, 8).Shape();
    const auto splitDifference = BRepAlgoAPI_Cut(splitStock, splitTool).Shape();
    int splitDifferenceCount = 0;
    for (TopExp_Explorer ex(splitDifference, TopAbs_SOLID); ex.More(); ex.Next()) {
      GProp_GProps props;
      BRepGProp::VolumeProperties(ex.Current(), props);
      record(out, props.CentreOfMass().X() < 0 ? "unequal-cylinder-split-minus" : "unequal-cylinder-split-plus", ex.Current(), 1e-10);
      ++splitDifferenceCount;
    }
    if (splitDifferenceCount != 2) throw std::runtime_error("unequal cylinder split did not yield two bodies");
    const auto obliqueSplitStock = BRepPrimAPI_MakeCylinder(gp_Ax2(gp_Pnt(-8, 0, 0), gp_Dir(2, 0, 0)), 1, 16).Shape();
    const auto obliqueSplitTool = BRepPrimAPI_MakeCylinder(gp_Ax2(gp_Pnt(-12, -16, 0), gp_Dir(3, 4, 0)), 2, 40).Shape();
    const auto obliqueSplit = BRepAlgoAPI_Cut(obliqueSplitStock, obliqueSplitTool).Shape();
    int obliqueSplitCount = 0;
    for (TopExp_Explorer ex(obliqueSplit, TopAbs_SOLID); ex.More(); ex.Next()) {
      GProp_GProps props;
      BRepGProp::VolumeProperties(ex.Current(), props);
      record(out, props.CentreOfMass().X() < 0 ? "oblique-cylinder-split-minus" : "oblique-cylinder-split-plus", ex.Current(), 1e-10);
      ++obliqueSplitCount;
    }
    if (obliqueSplitCount != 2) throw std::runtime_error("unequal oblique cylinder split did not yield two bodies");
    const auto boreBox = BRepPrimAPI_MakeBox(gp_Pnt(-2, -2, -1), 6, 5, 3).Shape();
    const auto boxBore = BRepPrimAPI_MakeCylinder(gp_Ax2(gp_Pnt(.5, .25, -2), gp_Dir(0, 0, 1)), .75, 5).Shape();
    record(out, "bored-box", BRepAlgoAPI_Cut(boreBox, boxBore).Shape());
    const auto obliqueBox = BRepPrimAPI_MakeBox(4, 5, 6).Shape();
    const auto obliqueBore = BRepPrimAPI_MakeCylinder(gp_Ax2(gp_Pnt(-1, 2, -1), gp_Dir(2, 0, 1)), .35, 10).Shape();
    record(out, "oblique-bored-box", BRepAlgoAPI_Cut(obliqueBox, obliqueBore).Shape());
    const auto twoBoreBox = BRepPrimAPI_MakeBox(8, 6, 4).Shape();
    const auto boreOne = BRepPrimAPI_MakeCylinder(gp_Ax2(gp_Pnt(2, 3, -1), gp_Dir(0, 0, 1)), .5, 6).Shape();
    const auto boreTwo = BRepPrimAPI_MakeCylinder(gp_Ax2(gp_Pnt(6, 3, -1), gp_Dir(0, 0, 1)), .75, 6).Shape();
    record(out, "double-bored-box", BRepAlgoAPI_Cut(BRepAlgoAPI_Cut(twoBoreBox, boreOne).Shape(), boreTwo).Shape());
    record(out, "corner-rounded-prism", roundRectanglePrism(6,4,.6,3));
    const auto sphericalPocket = BRepPrimAPI_MakeSphere(gp_Pnt(4,3,4.5),1).Shape();
    record(out, "spherical-pocket", BRepAlgoAPI_Cut(twoBoreBox,sphericalPocket).Shape());
    const auto ca = BRepPrimAPI_MakeCylinder(2, 3).Shape();
    const auto cb = BRepPrimAPI_MakeCylinder(gp_Ax2(gp_Pnt(1.5, 0, 0), gp_Dir(0, 0, 1)), 1.25, 3).Shape();
    record(out, "cylinder-union", BRepAlgoAPI_Fuse(ca, cb).Shape());
    record(out, "cylinder-intersection", BRepAlgoAPI_Common(ca, cb).Shape());
    record(out, "cylinder-difference", BRepAlgoAPI_Cut(ca, cb).Shape());
    const auto planarA = BRepPrimAPI_MakeBox(gp_Pnt(-1, -1, -1), 2, 2, 2).Shape();
    const auto planarB = BRepPrimAPI_MakeBox(gp_Pnt(-0.6, -1.1, -0.7), 2.1, 1.8, 2.0).Shape();
    gp_Trsf rotation;
    rotation.SetRotation(gp_Ax1(gp_Pnt(0, 0, 0), gp_Dir(1, 2, 3)), 0.25);
    const auto rotatedB = BRepBuilderAPI_Transform(planarB, rotation, true).Shape();
    record(out, "convex-union", BRepAlgoAPI_Fuse(planarA, rotatedB).Shape());
    record(out, "convex-intersection", BRepAlgoAPI_Common(planarA, rotatedB).Shape());
    record(out, "convex-difference", BRepAlgoAPI_Cut(planarA, rotatedB).Shape());
    const auto aligned = BRepPrimAPI_MakeBox(gp_Pnt(0, -1, -1), 2, 2, 2).Shape();
    record(out, "coplanar-union", BRepAlgoAPI_Fuse(planarA, aligned).Shape());
    record(out, "coplanar-difference", BRepAlgoAPI_Cut(planarA, aligned).Shape());
    const auto tunnelTool = BRepPrimAPI_MakeBox(gp_Pnt(-0.3, -0.4, -2), 0.6, 0.8, 4).Shape();
    record(out, "orthogonal-tunnel", BRepAlgoAPI_Cut(planarA, tunnelTool).Shape());
    const int cellsA[4][3] = {{0,0,0},{1,0,0},{0,1,0},{0,0,1}};
    const int cellsB[4][3] = {{0,0,0},{1,0,0},{0,1,0},{0,1,1}};
    const double mapA[3][4] = {{1,.25,0,0},{0,1,.25,0},{.25,0,1,0}};
    const double mapB[3][4] = {{1,0,.25,.5},{.25,1,0,.25},{0,.25,1,.125}};
    const auto branchA = branch(cellsA, mapA);
    const auto branchB = branch(cellsB, mapB);
    record(out, "nonconvex-union", BRepAlgoAPI_Fuse(branchA, branchB).Shape());
    record(out, "nonconvex-intersection", BRepAlgoAPI_Common(branchA, branchB).Shape());
    record(out, "nonconvex-difference", BRepAlgoAPI_Cut(branchA, branchB).Shape());
    out.close();
    return 0;
  } catch (const Standard_Failure& e) {
    std::cerr << e.what() << '\n';
  } catch (const std::exception& e) {
    std::cerr << e.what() << '\n';
  }
  return 1;
}
