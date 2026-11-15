#ifndef PLAINSOLID_OCCTREF_PLANAR_CONTACT_ANNULUS
#define PLAINSOLID_OCCTREF_PLANAR_CONTACT_ANNULUS

#include <BRepAlgoAPI_Cut.hxx>
#include <BRepPrimAPI_MakeBox.hxx>
#include <TopoDS_Shape.hxx>
#include <gp_Pnt.hxx>

// Independently builds square tubes with exact common annular end faces.
static TopoDS_Shape contactAnnulus(double z) {
    const auto outer = BRepPrimAPI_MakeBox(gp_Pnt(0.,0.,z),4.,4.,1.).Shape();
    const auto hole = BRepPrimAPI_MakeBox(gp_Pnt(1.,1.,z-1.),2.,2.,3.).Shape();
    return BRepAlgoAPI_Cut(outer, hole).Shape();
}

#endif
