#include <BRepAlgoAPI_Cut.hxx>
#include <BRepAlgoAPI_Fuse.hxx>
#include <BRepCheck_Analyzer.hxx>
#include <BRepGProp.hxx>
#include <BRepPrimAPI_MakeBox.hxx>
#include <BRepPrimAPI_MakeCylinder.hxx>
#include <GProp_GProps.hxx>
#include <TopExp_Explorer.hxx>
#include <gp_Ax2.hxx>
#include <gp_Dir.hxx>
#include <gp_Pnt.hxx>
#include <iomanip>
#include <iostream>
#include <string_view>

int main(int argc, char** argv) {
    const bool full = argc == 2 && std::string_view(argv[1]) == "complete";
    if (argc != 1 && !full) return 1;
    const auto cylinder = [](double x, double y, double z, double r, double height) {
        return BRepPrimAPI_MakeCylinder(gp_Ax2(gp_Pnt(x,y,z),gp_Dir(0.,0.,1.)),r,height).Shape();
    };
    const auto stock = BRepPrimAPI_MakeBox(gp_Pnt(0.,0.,-2.),8.,6.,2.).Shape();
    BRepAlgoAPI_Cut hole(stock,cylinder(1.,1.,-3.,.5,4.));
    BRepAlgoAPI_Cut annulus(cylinder(4.,3.,0.,2.,3.),cylinder(4.,3.,-1.,.5,5.));
    if (!hole.IsDone() || !annulus.IsDone()) return 2;
    TopoDS_Shape lower=hole.Shape(), upper=annulus.Shape();
    if (full) {
        BRepAlgoAPI_Cut bothLower(lower,cylinder(6.,4.,-3.,.5,4.));
        const auto stockUpper=BRepPrimAPI_MakeBox(gp_Pnt(0.,0.,0.),8.,6.,3.).Shape();
        BRepAlgoAPI_Cut oneUpper(stockUpper,cylinder(1.,1.,-1.,.5,5.));
        if (!bothLower.IsDone() || !oneUpper.IsDone()) return 4;
        BRepAlgoAPI_Cut bothUpper(oneUpper.Shape(),cylinder(6.,4.,-1.,.5,5.));
        if (!bothUpper.IsDone()) return 4;
        lower=bothLower.Shape();
        upper=bothUpper.Shape();
    }
    BRepAlgoAPI_Fuse joined(lower,upper);
    if (!joined.IsDone() || joined.Shape().IsNull() || !BRepCheck_Analyzer(joined.Shape()).IsValid()) return 3;
    int solids=0;
    for (TopExp_Explorer it(joined.Shape(),TopAbs_SOLID); it.More();it.Next()) ++solids;
    GProp_GProps vol,area;
    BRepGProp::VolumeProperties(joined.Shape(),vol);
    BRepGProp::SurfaceProperties(joined.Shape(),area);
    std::cout << std::setprecision(17) << "{\"Solids\":" << solids
              << ",\"Volume\":" << vol.Mass() << ",\"Area\":" << area.Mass() << "}\n";
}
