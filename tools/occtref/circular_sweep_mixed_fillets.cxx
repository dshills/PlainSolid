#include <BRepAlgoAPI_Cut.hxx>
#include <BRepCheck_Analyzer.hxx>
#include <BRepFilletAPI_MakeFillet.hxx>
#include <BRepGProp.hxx>
#include <BRepPrimAPI_MakeBox.hxx>
#include <BRepPrimAPI_MakeCylinder.hxx>
#include <BRep_Tool.hxx>
#include <GProp_GProps.hxx>
#include <NCollection_IndexedMap.hxx>
#include <TopExp.hxx>
#include <TopTools_ShapeMapHasher.hxx>
#include <TopoDS.hxx>
#include <TopoDS_Vertex.hxx>
#include <gp_Ax2.hxx>
#include <gp_Dir.hxx>
#include <gp_Pnt.hxx>
#include <array>
#include <iomanip>
#include <iostream>
#include <initializer_list>

static TopoDS_Shape round(const TopoDS_Shape& shape, std::initializer_list<std::array<double,3>> sites) {
    BRepFilletAPI_MakeFillet f(shape);
    NCollection_IndexedMap<TopoDS_Shape,TopTools_ShapeMapHasher> edges;
    TopExp::MapShapes(shape,TopAbs_EDGE,edges);
    int count=0;
    for(int i=1;i<=edges.Extent();++i) {
        const auto edge=TopoDS::Edge(edges(i));
        TopoDS_Vertex from,to;
        TopExp::Vertices(edge,from,to);
        if(from.IsNull()||to.IsNull()) continue;
        const auto a=BRep_Tool::Pnt(from), b=BRep_Tool::Pnt(to);
        if(a.X()!=b.X()||a.Y()!=b.Y()||a.Z()==b.Z()) continue;
        for(const auto& s:sites) {
            if(a.X()==s[0]&&a.Y()==s[1]) { f.Add(s[2],edge); ++count; }
        }
    }
    if(count!=static_cast<int>(sites.size())) return {};
    f.Build();
    return f.IsDone()?f.Shape():TopoDS_Shape{};
}

int main(){
    const auto stock=BRepPrimAPI_MakeBox(gp_Pnt(0.,0.,-1.),10.,8.,2.).Shape();
    const auto prior=round(stock,{{10.,0.,1.}});
    if(prior.IsNull())return 2;
    const auto square=BRepPrimAPI_MakeBox(gp_Pnt(4.,3.,-2.),2.,2.,4.).Shape();
    const auto circle=BRepPrimAPI_MakeCylinder(gp_Ax2(gp_Pnt(8.,5.,-2.),gp_Dir(0.,0.,1.)),.5,4.).Shape();
    BRepAlgoAPI_Cut a(prior,square);
    if(!a.IsDone()) return 3;
    BRepAlgoAPI_Cut b(a.Shape(),circle);
    if(!b.IsDone()) return 3;
    const auto result=round(b.Shape(),{{0.,0.,.5},{4.,3.,.25}});
    if(result.IsNull()||!BRepCheck_Analyzer(result).IsValid())return 4;
    GProp_GProps vol,area;
    BRepGProp::VolumeProperties(result,vol);
    BRepGProp::SurfaceProperties(result,area);
    std::cout<<std::setprecision(17)<<"{\"Volume\":"<<vol.Mass()<<",\"Area\":"<<area.Mass()<<"}\n";
}
