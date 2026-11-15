#ifndef PLAINSOLID_OCCTREF_PLANAR_CONTACT_INSETS
#define PLAINSOLID_OCCTREF_PLANAR_CONTACT_INSETS

#include <BRepBuilderAPI_MakeFace.hxx>
#include <BRepBuilderAPI_MakePolygon.hxx>
#include <BRepCheck_Analyzer.hxx>
#include <BRepPrimAPI_MakeBox.hxx>
#include <BRepPrimAPI_MakePrism.hxx>
#include <TopoDS_Shape.hxx>
#include <gp_Pnt.hxx>
#include <gp_Vec.hxx>
#include <array>

// Separate original bodies: a plate and a bridge with two disjoint feet in
// the plate face interior. No kernel-specific join is used to author them.
static TopoDS_Shape contactInsets(double part) {
    if (part == 0.) return BRepPrimAPI_MakeBox(5.,4.,1.).Shape();
    const std::array<gp_Pnt,8> profile{{
        {1.,1.,1.},{2.,1.,1.},{2.,1.,2.},{3.,1.,2.},
        {3.,1.,1.},{4.,1.,1.},{4.,1.,3.},{1.,1.,3.}
    }};
    BRepBuilderAPI_MakePolygon wire;
    for (const auto& p : profile) wire.Add(p);
    wire.Close();
    BRepBuilderAPI_MakeFace face(wire.Wire());
    if (!face.IsDone()) return {};
    const auto stock = BRepPrimAPI_MakePrism(face.Face(), gp_Vec(0.,1.,0.)).Shape();
    if (!BRepCheck_Analyzer(stock).IsValid()) return {};
    return stock;
}

#endif
