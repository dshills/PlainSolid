#include <BRepPrimAPI_MakeBox.hxx>
#include <Interface_Static.hxx>
#include <STEPControl_Writer.hxx>
#include <gp_Pnt.hxx>
int main(int argc, char** argv) {
    if (argc != 2) return 2;
    STEPControl_Writer writer;
    if (!Interface_Static::SetCVal("write.step.schema", "AP214IS") ||
        !Interface_Static::SetIVal("write.surfacecurve.mode", 1)) return 3;
    for (double p : {0.0, 1.0, 4.0}) {
        const auto stock = BRepPrimAPI_MakeBox(gp_Pnt(p, p, p), 1.0, 1.0, 1.0).Shape();
        if (writer.Transfer(stock, STEPControl_AsIs) != IFSelect_RetDone) return 4;
    }
    return writer.Write(argv[1]) == IFSelect_RetDone ? 0 : 5;
}
