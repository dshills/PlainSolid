#include <BRepAlgoAPI_Cut.hxx>
#include <BRepBuilderAPI_MakeFace.hxx>
#include <BRepBuilderAPI_MakePolygon.hxx>
#include <BRepPrimAPI_MakeBox.hxx>
#include <BRepPrimAPI_MakePrism.hxx>
#include <Interface_Static.hxx>
#include <STEPControl_Writer.hxx>
#include <TopExp_Explorer.hxx>
#include <gp_Pnt.hxx>
#include <gp_Vec.hxx>
#include <cstring>
#include "planar_contact_annulus.hxx"
#include "planar_contact_subdivided.hxx"

int main(int argc, char** argv) {
    const bool cavity = argc == 3 && std::strcmp(argv[1], "voids") == 0;
    const bool divided = argc == 3 && std::strcmp(argv[1], "face-subdivided") == 0;
    const bool faceContact = divided || (argc == 3 && std::strcmp(argv[1], "face-annulus") == 0);
    const bool interval = argc == 3 && std::strcmp(argv[1], "edge-interval") == 0;
    const bool edge = interval || (argc == 3 && std::strcmp(argv[1], "edge") == 0);
    if (argc != 2 && !cavity && !edge && !faceContact) return 2;
    STEPControl_Writer writer;
    if (!Interface_Static::SetCVal("write.step.schema", "AP214IS") ||
        !Interface_Static::SetIVal("write.surfacecurve.mode", (cavity || divided) ? 0 : 1)) return 3;
    if (cavity) {
        BRepBuilderAPI_MakePolygon wire;
        for (const gp_Pnt& p : {gp_Pnt(0,0,0),gp_Pnt(8,0,0),gp_Pnt(8,4,0),gp_Pnt(6,4,0),gp_Pnt(6,8,0),gp_Pnt(0,8,0)}) {
            wire.Add(p);
        }
        wire.Close();
        const auto face = BRepBuilderAPI_MakeFace(wire.Wire()).Face();
        const auto base = BRepPrimAPI_MakePrism(face, gp_Vec(0,0,8)).Shape();
        const auto bore = BRepPrimAPI_MakeBox(gp_Pnt(2,2,2),2,2,2).Shape();
        const auto cut = BRepAlgoAPI_Cut(base, bore).Shape();
        TopExp_Explorer pieces(cut, TopAbs_SOLID);
        if (!pieces.More() || writer.Transfer(pieces.Current(), STEPControl_AsIs) != IFSelect_RetDone) return 4;
        pieces.Next();
        if (pieces.More()) return 4;
        const auto separate = BRepPrimAPI_MakeBox(gp_Pnt(12,12,12),1,1,1).Shape();
        if (writer.Transfer(separate, STEPControl_AsIs) != IFSelect_RetDone) return 4;
    } else if (faceContact) {
        for (double z : {0., 1.}) {
            const auto part = divided ? contactSubdivided(z) : contactAnnulus(z);
            if (part.IsNull()) return 4;
            TopExp_Explorer solid(part, TopAbs_SOLID);
            if (!solid.More() || writer.Transfer(solid.Current(), STEPControl_AsIs) != IFSelect_RetDone) return 4;
            solid.Next();
            if (solid.More()) return 4;
        }
    } else if (edge) {
        for (double p : {0., 1.}) {
            const auto stock = BRepPrimAPI_MakeBox(gp_Pnt(p,p,interval ? p : 0.),1,1,interval ? 3. : 1.).Shape();
            if (writer.Transfer(stock, STEPControl_AsIs) != IFSelect_RetDone) return 4;
        }
    } else {
        for (double p : {0.0, 1.0, 4.0}) {
            const auto stock = BRepPrimAPI_MakeBox(gp_Pnt(p, p, p), 1.0, 1.0, 1.0).Shape();
            if (writer.Transfer(stock, STEPControl_AsIs) != IFSelect_RetDone) return 4;
        }
    }
    return writer.Write(argv[argc-1]) == IFSelect_RetDone ? 0 : 5;
}
