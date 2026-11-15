#include <StepShape_EdgeCurve.hxx>
#include <StepShape_VertexPoint.hxx>
#include <StepGeom_CartesianPoint.hxx>
#include <StepGeom_Circle.hxx>
#include <StepGeom_Line.hxx>
#include <StepGeom_Vector.hxx>
#include <Geom_Circle.hxx>
#include <Geom_CartesianPoint.hxx>
#include <Geom_Line.hxx>
#include <StepData_StepModel.hxx>
#include <StepGeom_SurfaceCurve.hxx>
#include <StepGeom_Pcurve.hxx>
#include <StepGeom_Plane.hxx>
#include <StepGeom_CylindricalSurface.hxx>
#include <StepGeom_BSplineCurve.hxx>
#include <StepRepr_DefinitionalRepresentation.hxx>
#include <StepToGeom.hxx>
#include <Geom_Curve.hxx>
#include <Geom_Surface.hxx>
#include <Geom2d_Curve.hxx>
#include <BRepBuilderAPI_MakePolygon.hxx>
#include <BRepBuilderAPI_MakeFace.hxx>
#include <BRepPrimAPI_MakePrism.hxx>
#include <BRepFilletAPI_MakeFillet.hxx>
#include <BRepBuilderAPI_Transform.hxx>
#include <gp_Trsf.hxx>
#include <BRepGProp.hxx>
#include <BRepCheck_Analyzer.hxx>
#include <GProp_GProps.hxx>
#include <STEPControl_Writer.hxx>
#include <STEPControl_Reader.hxx>
#include <Interface_Static.hxx>
#include <TopExp_Explorer.hxx>
#include <TopoDS.hxx>
#include <algorithm>
#include <cmath>
#include <iostream>
#include <iomanip>
#include <string>

static int shiftedCylinderWitness(const char* path) {
 STEPControl_Reader reader;
 if (reader.ReadFile(path) != IFSelect_RetDone) return 6;
 const auto model = reader.StepModel();
 int owners = 0;
 double maxDistance = 0, withoutPhase = 0;
 for (int i=1; i<=model->NbEntities(); ++i) {
  const auto edge = Handle(StepShape_EdgeCurve)::DownCast(model->Value(i));
  if (edge.IsNull()) continue;
  const auto sc = Handle(StepGeom_SurfaceCurve)::DownCast(edge->EdgeGeometry());
  if (sc.IsNull()) return 9;
  const auto sourceCircle = Handle(StepGeom_Circle)::DownCast(sc->Curve3d());
  if (sourceCircle.IsNull()) continue;
  const auto circle = StepToGeom::MakeCircle(sourceCircle);
  if (circle.IsNull() || !edge->SameSense()) return 9;
  const auto start = Handle(StepShape_VertexPoint)::DownCast(edge->EdgeStart());
  const auto end = Handle(StepShape_VertexPoint)::DownCast(edge->EdgeEnd());
  if (start.IsNull() || end.IsNull()) return 9;
  const auto a = StepToGeom::MakeCartesianPoint(Handle(StepGeom_CartesianPoint)::DownCast(start->VertexGeometry()));
  const auto b = StepToGeom::MakeCartesianPoint(Handle(StepGeom_CartesianPoint)::DownCast(end->VertexGeometry()));
  if (a.IsNull() || b.IsNull()) return 9;
  const auto frame = circle->Position();
  const auto station = [&](const gp_Pnt& point) {
   const auto ray = gp_Vec(frame.Location(), point);
   return std::atan2(ray.Dot(gp_Vec(frame.YDirection())), ray.Dot(gp_Vec(frame.XDirection())));
  };
  const double first = station(a->Pnt());
  double last = station(b->Pnt());
  if (last < first) last += 2*std::acos(-1.0);
  for (int j=1; j<=sc->NbAssociatedGeometry(); ++j) {
   const auto pc = sc->AssociatedGeometryValue(j).Pcurve();
   if (pc.IsNull() || Handle(StepGeom_CylindricalSurface)::DownCast(pc->BasisSurface()).IsNull()) continue;
   const auto geometry = Handle(StepGeom_Line)::DownCast(pc->ReferenceToCurve()->ItemsValue(1));
   if (geometry.IsNull()) continue;
   const auto uv = StepToGeom::MakeCurve2d(geometry);
   const auto surface = StepToGeom::MakeSurface(pc->BasisSurface());
   if (uv.IsNull() || surface.IsNull()) return 9;
   const double phase = uv->Value(0).X();
   if (phase == 0) continue;
   ++owners;
   for (double t : {first, (first+last)/2, last}) {
    const auto p = uv->Value(t);
    const auto xyz = circle->Value(t);
    maxDistance = std::max(maxDistance, xyz.Distance(surface->Value(p.X(),p.Y())));
    withoutPhase = std::max(withoutPhase, xyz.Distance(surface->Value(p.X()-phase,p.Y())));
   }
  }
 }
 std::cout << std::setprecision(17) << "{\"Owners\":" << owners << ",\"Samples\":" << 3*owners
           << ",\"MaxSourceDistance\":" << maxDistance << ",\"WithoutPhase\":" << withoutPhase << "}\n";
 return owners == 2 ? 0 : 9;
}

static int convertedUnitWitness(const char* path, double modelSpeed, double ownerScale, bool grouped = false) {
 STEPControl_Reader reader;
 if (reader.ReadFile(path) != IFSelect_RetDone) return 6;
 const auto model = reader.StepModel();
 int planeLines=0, planeCircles=0, cylinderLines=0, cylinderCircles=0, rationalCaps=0;
 double maxDistance=0, withoutMap=0, maxCapDistance=0;
 for (int i=1; i<=model->NbEntities(); ++i) {
  const auto edge = Handle(StepShape_EdgeCurve)::DownCast(model->Value(i));
  if (edge.IsNull()) continue;
  const auto sc = Handle(StepGeom_SurfaceCurve)::DownCast(edge->EdgeGeometry());
  if (sc.IsNull()) return 9;
  const auto curve = StepToGeom::MakeCurve(sc->Curve3d());
  const auto stepLine = Handle(StepGeom_Line)::DownCast(sc->Curve3d());
  const auto line = Handle(Geom_Line)::DownCast(curve);
  const auto circle = Handle(Geom_Circle)::DownCast(curve);
  const auto start = Handle(StepShape_VertexPoint)::DownCast(edge->EdgeStart());
  const auto end = Handle(StepShape_VertexPoint)::DownCast(edge->EdgeEnd());
  if (start.IsNull() || end.IsNull() || (line.IsNull() && circle.IsNull())) return 9;
  const auto a = StepToGeom::MakeCartesianPoint(Handle(StepGeom_CartesianPoint)::DownCast(start->VertexGeometry()));
  const auto b = StepToGeom::MakeCartesianPoint(Handle(StepGeom_CartesianPoint)::DownCast(end->VertexGeometry()));
  if (a.IsNull() || b.IsNull()) return 9;
  double speed = 1;
  double parameter;
  if (!line.IsNull()) {
   if (stepLine.IsNull() || stepLine->Dir().IsNull()) return 9;
   speed = stepLine->Dir()->Magnitude();
   if (speed != modelSpeed) return 9;
   const auto axis = line->Position();
   const auto station = [&](const gp_Pnt& point) { return gp_Vec(axis.Location(), point).Dot(gp_Vec(axis.Direction())); };
   parameter = (station(a->Pnt()) + station(b->Pnt())) / (2 * speed);
  } else {
   const auto chart = circle->Position();
   const auto angle = [&](const gp_Pnt& point) {
    const auto ray = gp_Vec(chart.Location(), point);
    return std::atan2(ray.Dot(gp_Vec(chart.YDirection())), ray.Dot(gp_Vec(chart.XDirection())));
   };
   parameter = (angle(a->Pnt()) + angle(b->Pnt())) / 2;
  }
  const auto point = curve->Value(parameter * speed);
  for (int j=1; j<=sc->NbAssociatedGeometry(); ++j) {
   const auto pc = sc->AssociatedGeometryValue(j).Pcurve();
   if (pc.IsNull()) return 9;
   const auto geometry = Handle(StepGeom_Curve)::DownCast(pc->ReferenceToCurve()->ItemsValue(1));
   if (geometry.IsNull()) return 9;
   const auto uv = StepToGeom::MakeCurve2d(geometry);
   const auto surface = StepToGeom::MakeSurface(pc->BasisSurface());
   if (uv.IsNull() || surface.IsNull()) return 9;
   const auto lift = [&](double t) { const auto p=uv->Value(t); return surface->Value(p.X(),p.Y()); };
   double ownerParameter = parameter;
   if (!Handle(StepGeom_Plane)::DownCast(pc->BasisSurface()).IsNull()) {
    if (!line.IsNull()) {
     const auto ownerLine = Handle(StepGeom_Line)::DownCast(geometry);
     if (ownerLine.IsNull() || ownerLine->Dir().IsNull() || ownerLine->Dir()->Magnitude() != 1) return 9;
     ++planeLines;
     ownerParameter *= ownerScale;
     withoutMap = std::max(withoutMap, point.Distance(lift(parameter)));
    } else if (!Handle(StepGeom_Circle)::DownCast(geometry).IsNull()) {
     ++planeCircles;
    } else {
     if (!grouped || Handle(StepGeom_BSplineCurve)::DownCast(geometry).IsNull()) return 9;
     const auto frame = circle->Position();
     const auto angle = [&](double t) {
      const auto ray = gp_Vec(frame.Location(), lift(t));
      return std::atan2(ray.Dot(gp_Vec(frame.YDirection())), ray.Dot(gp_Vec(frame.XDirection())));
     };
     double lo=uv->FirstParameter(), hi=lo+(uv->LastParameter()-lo)/3;
     if (angle(lo) >= parameter || angle(hi) <= parameter) return 9;
     for (int k=0; k<60; ++k) {
      const double mid=(lo+hi)/2;
      if (angle(mid)<parameter) lo=mid;
      else hi=mid;
     }
     ownerParameter=(lo+hi)/2;
     maxCapDistance=std::max(maxCapDistance, point.Distance(lift(ownerParameter)));
     ++rationalCaps;
    }
   } else {
    if (Handle(StepGeom_CylindricalSurface)::DownCast(pc->BasisSurface()).IsNull() ||
        Handle(StepGeom_BSplineCurve)::DownCast(geometry).IsNull()) return 9;
    if (line.IsNull()) ++cylinderCircles;
    else ++cylinderLines;
   }
   maxDistance = std::max(maxDistance, point.Distance(lift(ownerParameter)));
  }
 }
 std::cout << std::setprecision(17) << "{\"PlaneLines\":" << planeLines << ",\"PlaneCircles\":" << planeCircles
           << ",\"CylinderLines\":" << cylinderLines << ",\"CylinderCircles\":" << cylinderCircles
           << ",\"MaxSourceDistance\":" << maxDistance << ",\"WithoutPlanarMap\":" << withoutMap;
 if (grouped) std::cout << ",\"RationalCaps\":" << rationalCaps << ",\"MaxCapDistance\":" << maxCapDistance;
 std::cout << "}\n";
 return planeLines==18 && planeCircles==2 && cylinderLines==(grouped ? 4 : 2) &&
        cylinderCircles==(grouped ? 4 : 2) && rationalCaps==(grouped ? 2 : 0) ? 0 : 9;
}

int main(int argc,char**argv){if(argc!=3)return 2;TopoDS_Shape shape;
if(std::string(argv[1])=="converted-metre-witness") return convertedUnitWitness(argv[2], .001, .001);
if(std::string(argv[1])=="converted-inch-witness") return convertedUnitWitness(argv[2], 3.937007874016E-02, 5.0/127.0);
if(std::string(argv[1])=="grouped-converted-metre-witness") return convertedUnitWitness(argv[2], .001, .001, true);
if(std::string(argv[1])=="grouped-converted-inch-witness") return convertedUnitWitness(argv[2], 3.937007874016E-02, 5.0/127.0, true);
if(std::string(argv[1])=="shifted-cylinder-witness") return shiftedCylinderWitness(argv[2]);
if(std::string(argv[1])=="cap-trim") {
 STEPControl_Reader reader;if(reader.ReadFile(argv[2])!=IFSelect_RetDone)return 6;auto model=reader.StepModel();int count=0;double minStation=10,maxStation=0,maxEndpoint=0;
 for(int i=1;i<=model->NbEntities();++i){auto edge=Handle(StepShape_EdgeCurve)::DownCast(model->Value(i));if(edge.IsNull())continue;auto sc=Handle(StepGeom_SurfaceCurve)::DownCast(edge->EdgeGeometry());if(sc.IsNull())continue;auto circleSource=Handle(StepGeom_Circle)::DownCast(sc->Curve3d());if(circleSource.IsNull())continue;auto circle=StepToGeom::MakeCircle(circleSource);for(int j=1;j<=sc->NbAssociatedGeometry();++j){auto pc=sc->AssociatedGeometryValue(j).Pcurve();if(pc.IsNull()||Handle(StepGeom_Plane)::DownCast(pc->BasisSurface()).IsNull())continue;auto bs=Handle(StepGeom_BSplineCurve)::DownCast(pc->ReferenceToCurve()->ItemsValue(1));if(bs.IsNull())continue;auto uv=StepToGeom::MakeCurve2d(bs);auto surf=StepToGeom::MakeSurface(pc->BasisSurface());auto point=[&](double t){auto p=uv->Value(t);return surf->Value(p.X(),p.Y());};double lo=uv->FirstParameter(),hi=lo+(uv->LastParameter()-lo)/3;auto axis=circle->Position().XDirection();for(int k=0;k<60;++k){double mid=(lo+hi)/2;double x=gp_Vec(circle->Location(),point(mid)).Dot(gp_Vec(axis));if(x>0)lo=mid;else hi=mid;}double station=(lo+hi)/2;minStation=std::min(minStation,station);maxStation=std::max(maxStation,station);
 auto start=Handle(StepShape_VertexPoint)::DownCast(edge->EdgeStart());auto end=Handle(StepShape_VertexPoint)::DownCast(edge->EdgeEnd());if(start.IsNull()||end.IsNull()||!edge->SameSense())return 11;auto a=StepToGeom::MakeCartesianPoint(Handle(StepGeom_CartesianPoint)::DownCast(start->VertexGeometry()));auto b=StepToGeom::MakeCartesianPoint(Handle(StepGeom_CartesianPoint)::DownCast(end->VertexGeometry()));if(a.IsNull()||b.IsNull())return 12;maxEndpoint=std::max(maxEndpoint,point(uv->FirstParameter()).Distance(a->Pnt()));maxEndpoint=std::max(maxEndpoint,point(station).Distance(b->Pnt()));++count;}}
 std::cout<<std::setprecision(17)<<"{\"Caps\":"<<count<<",\"MinStation\":"<<minStation<<",\"MaxStation\":"<<maxStation<<",\"MaxEndpointDistance\":"<<maxEndpoint<<"}\n";return count==2?0:10;
}
if(std::string(argv[1])=="cap-witness") {
 STEPControl_Reader r;if(r.ReadFile(argv[2])!=IFSelect_RetDone)return 6;auto model=r.StepModel();int count=0;double minSquared=1,maxSquared=0;
 for(int i=1;i<=model->NbEntities();++i){auto sc=Handle(StepGeom_SurfaceCurve)::DownCast(model->Value(i));if(sc.IsNull())continue;for(int j=1;j<=sc->NbAssociatedGeometry();++j){auto pc=sc->AssociatedGeometryValue(j).Pcurve();if(pc.IsNull()||Handle(StepGeom_Plane)::DownCast(pc->BasisSurface()).IsNull())continue;auto bs=Handle(StepGeom_BSplineCurve)::DownCast(pc->ReferenceToCurve()->ItemsValue(1));if(bs.IsNull())continue;auto uv=StepToGeom::MakeCurve2d(bs);auto c=StepToGeom::MakeCurve(sc->Curve3d());auto surface=StepToGeom::MakeSurface(pc->BasisSurface());if(uv.IsNull()||c.IsNull()||surface.IsNull())return 9;double t=2.094395102393/4;auto p=uv->Value(t);double d=surface->Value(p.X(),p.Y()).SquareDistance(c->Value(t));minSquared=std::min(minSquared,d);maxSquared=std::max(maxSquared,d);++count;}}
 std::cout<<std::setprecision(17)<<"{\"Caps\":"<<count<<",\"MinSquared\":"<<minSquared<<",\"MaxSquared\":"<<maxSquared<<"}\n";return count==2?0:10;
}
const std::string mode = argv[1];
const bool threeNonDyadic = mode == "write-three-default-tilted-nondyadic";
const bool three = mode == "write-three-default" || mode == "write-three-default-tilted" || threeNonDyadic;
const bool groupedConvertedMetre = mode == "write-both-default-tilted-converted-metre";
const bool groupedConvertedInch = mode == "write-both-default-tilted-converted-inch" ||
                                 mode == "write-both-default-tilted-converted-inch-unscaled";
const bool groupedTilted = mode == "write-both-default-tilted" || mode == "write-both-default-tilted-metre" ||
                           mode == "write-both-default-tilted-inch" || groupedConvertedMetre || groupedConvertedInch;
const bool convertedMetre = mode == "write-default-tilted-translated-converted-metre" || groupedConvertedMetre;
const bool convertedInch = mode == "write-default-tilted-translated-converted-inch" ||
                           mode == "write-default-tilted-translated-converted-inch-unscaled" || groupedConvertedInch;
const bool metre = mode == "write-default-tilted-translated-metre" || mode == "write-both-default-tilted-metre" || convertedMetre;
const bool inch = mode == "write-default-tilted-translated-inch" || mode == "write-both-default-tilted-inch" || convertedInch;
const bool tiltedDefault = mode == "write-default-tilted" || mode == "write-default-tilted-translated" ||
                           mode == "write-three-default-tilted" || threeNonDyadic || groupedTilted || metre || inch;
if(mode=="write" || mode=="write-x" || std::string(argv[1])=="write-minus-x" || std::string(argv[1])=="write-translated" || std::string(argv[1])=="write-both" || std::string(argv[1])=="write-both-x" || std::string(argv[1])=="write-default" || std::string(argv[1])=="write-both-default" || std::string(argv[1])=="write-both-default-x" || std::string(argv[1])=="write-both-default-minus-x" || std::string(argv[1])=="write-both-default-translated" || tiltedDefault || three){
 BRepBuilderAPI_MakePolygon p;p.Add(gp_Pnt(0,0,0));p.Add(gp_Pnt(4,0,0));p.Add(gp_Pnt(0,3,0));p.Close();auto stock=BRepPrimAPI_MakePrism(BRepBuilderAPI_MakeFace(p.Wire()).Face(),gp_Vec(0,0,5)).Shape();TopoDS_Edge target;
 for(TopExp_Explorer e(stock,TopAbs_EDGE);e.More();e.Next()){GProp_GProps g;BRepGProp::LinearProperties(e.Current(),g);if(g.CentreOfMass().Distance(gp_Pnt(4,0,2.5))<1e-12)target=TopoDS::Edge(e.Current());}
 if(target.IsNull())return 3;BRepFilletAPI_MakeFillet fillet(stock);fillet.Add(5.0/16,target);
 if(std::string(argv[1])=="write-both" || std::string(argv[1])=="write-both-x" || std::string(argv[1])=="write-both-default" || std::string(argv[1])=="write-both-default-x" || std::string(argv[1])=="write-both-default-minus-x" || std::string(argv[1])=="write-both-default-translated" || groupedTilted || three){TopoDS_Edge other;for(TopExp_Explorer e(stock,TopAbs_EDGE);e.More();e.Next()){GProp_GProps g;BRepGProp::LinearProperties(e.Current(),g);if(g.CentreOfMass().Distance(gp_Pnt(0,0,2.5))<1e-12)other=TopoDS::Edge(e.Current());}if(other.IsNull())return 8;fillet.Add(.25,other);}
 if (three) {
  TopoDS_Edge third;
  for (TopExp_Explorer e(stock, TopAbs_EDGE); e.More(); e.Next()) {
   GProp_GProps g;
   BRepGProp::LinearProperties(e.Current(), g);
   if (g.CentreOfMass().Distance(gp_Pnt(0,3,2.5)) < 1e-12) third = TopoDS::Edge(e.Current());
  }
  if (third.IsNull()) return 8;
  fillet.Add(threeNonDyadic ? 3.0/16 : 5.0/32, third);
 }
 fillet.Build();if(!fillet.IsDone())return 4;shape=fillet.Shape();
 if (mode != "write" && mode != "write-both" && mode != "write-default" && mode != "write-both-default" && mode != "write-three-default") {
  gp_Trsf pose;
  if (mode == "write-x" || mode == "write-both-x" || mode == "write-both-default-x") {
   pose.SetValues(0,0,1,2, 1,0,0,-3, 0,1,0,0);
  } else if (mode == "write-minus-x" || mode == "write-both-default-minus-x") {
   pose.SetValues(0,0,-1,2, -1,0,0,-3, 0,1,0,0);
  } else if (tiltedDefault) {
   const bool translated = !groupedTilted && (mode == "write-default-tilted-translated" || metre || inch);
   pose.SetValues(.6,0,.8,translated ? 1 : 0, 0,1,0,0, -.8,0,.6,translated ? 2 : 0);
  } else {
   pose.SetTranslation(gp_Vec(2,0,3));
  }
  shape = BRepBuilderAPI_Transform(shape,pose,true).Shape();
 }
 STEPControl_Writer w;
 if (metre || inch) {
  if (convertedMetre || convertedInch) {
   // Author in millimetres and let STEPControl convert the model and charts.
   // 127 mm is exactly five inches, so scaled dyadic model coordinates
   // retain terminating inch decimals even though the LINE speed does not.
   if (mode != "write-default-tilted-translated-converted-inch-unscaled" && mode != "write-both-default-tilted-converted-inch-unscaled") {
    gp_Trsf scale;
    scale.SetScale(gp_Pnt(0,0,0), convertedMetre ? 1000.0 : 127.0);
    shape = BRepBuilderAPI_Transform(shape,scale,true).Shape();
   }
   w.Model()->SetLocalLengthUnit(1.0);
  } else {
   w.Model()->SetLocalLengthUnit(metre ? 1000.0 : 25.4);
  }
  if (!Interface_Static::SetCVal("write.step.unit", metre ? "M" : "INCH")) return 9;
 }
 Interface_Static::SetCVal("write.step.schema","AP214IS");Interface_Static::SetIVal("write.surfacecurve.mode",(mode=="write-default" || mode=="write-both-default" || mode=="write-both-default-x" || mode=="write-both-default-minus-x" || mode=="write-both-default-translated" || tiltedDefault || three)?1:0);if(w.Transfer(shape,STEPControl_AsIs)!=IFSelect_RetDone||w.Write(argv[2])!=IFSelect_RetDone)return 5;
}else if(std::string(argv[1])=="read"){STEPControl_Reader r;if(r.ReadFile(argv[2])!=IFSelect_RetDone||r.TransferRoots()==0)return 6;shape=r.OneShape();}else return 2;
if(!BRepCheck_Analyzer(shape).IsValid())return 7;GProp_GProps v,a;BRepGProp::VolumeProperties(shape,v,1e-12);BRepGProp::SurfaceProperties(shape,a,1e-12);int faces=0,solids=0;for(TopExp_Explorer e(shape,TopAbs_FACE);e.More();e.Next())faces++;for(TopExp_Explorer e(shape,TopAbs_SOLID);e.More();e.Next())solids++;
std::cout<<std::setprecision(17)<<"{\"Volume\":"<<v.Mass()<<",\"Area\":"<<a.Mass()<<",\"Faces\":"<<faces<<",\"Solids\":"<<solids<<",\"Centroid\":["<<v.CentreOfMass().X()<<","<<v.CentreOfMass().Y()<<","<<v.CentreOfMass().Z()<<"]}\n";
}
