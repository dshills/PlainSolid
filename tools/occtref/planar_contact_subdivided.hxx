#ifndef PLAINSOLID_OCCTREF_PLANAR_CONTACT_SUBDIVIDED
#define PLAINSOLID_OCCTREF_PLANAR_CONTACT_SUBDIVIDED

#include <BRepBuilderAPI_MakeFace.hxx>
#include <BRepBuilderAPI_MakePolygon.hxx>
#include <BRepBuilderAPI_MakeSolid.hxx>
#include <BRepBuilderAPI_Sewing.hxx>
#include <BRepCheck_Analyzer.hxx>
#include <BRepGProp.hxx>
#include <BRepLib.hxx>
#include <BRepPrimAPI_MakeBox.hxx>
#include <GProp_GProps.hxx>
#include <TopExp_Explorer.hxx>
#include <TopoDS.hxx>
#include <TopoDS_Shell.hxx>
#include <gp_Ax3.hxx>
#include <gp_Pln.hxx>
#include <array>

// Sew four independent original triangular faces into the lower stock's cap.
// The upper stock remains an ordinary box with a single contact face.
static TopoDS_Shape contactSubdivided(double z) {
    if (z != 0.) return BRepPrimAPI_MakeBox(gp_Pnt(16.,16.,z),6.,8.,1.).Shape();
    const auto stock = BRepPrimAPI_MakeBox(gp_Pnt(16.,16.,0.),6.,8.,1.).Shape();
    BRepBuilderAPI_Sewing sewing;
    int removed = 0;
    for (TopExp_Explorer face(stock, TopAbs_FACE); face.More(); face.Next()) {
        GProp_GProps measure;
        BRepGProp::SurfaceProperties(face.Current(), measure);
        if (measure.CentreOfMass().Z() == 1.) ++removed;
        else sewing.Add(face.Current());
    }
    if (removed != 1) return {};
    const std::array<gp_Pnt,4> corners{{{16.,16.,1.},{22.,16.,1.},{22.,24.,1.},{16.,24.,1.}}};
    // Integer 3-4-5 spokes and a nonzero origin keep finite IGES endpoints exact.
    const gp_Pnt midpoint(19.,20.,1.);
    const gp_Pln plane(gp_Ax3(gp_Pnt(16.,16.,1.), gp_Dir(0.,0.,1.), gp_Dir(1.,0.,0.)));
    for (std::size_t i = 0; i < corners.size(); ++i) {
        BRepBuilderAPI_MakePolygon wire(corners[i], corners[(i+1)%corners.size()], midpoint, true);
        BRepBuilderAPI_MakeFace face(plane, wire.Wire());
        if (!face.IsDone()) return {};
        sewing.Add(face.Face());
    }
    sewing.Perform();
    const auto sewn = sewing.SewedShape();
    if (sewn.IsNull() || sewn.ShapeType() != TopAbs_SHELL) return {};
    BRepBuilderAPI_MakeSolid build(TopoDS::Shell(sewn));
    if (!build.IsDone()) return {};
    auto solid = build.Solid();
    if (!BRepLib::OrientClosedSolid(solid) || !BRepCheck_Analyzer(solid).IsValid()) return {};
    return solid;
}

#endif
