// Independent pinned OCCT construction: six bilinear B-spline patches form
// 0<=x,y<=2, 0<=z<=1+xy/4. No Go geometry or exchange file is consumed.
#include <Geom_BSplineSurface.hxx>
#include <NCollection_Array2.hxx>
#include <NCollection_Array1.hxx>
#include <BRepBuilderAPI_Sewing.hxx>
#include <BRepBuilderAPI_MakeSolid.hxx>
#include <TopoDS.hxx>
#include <TopExp_Explorer.hxx>
static TopoDS_Shape splineShellPrism(){
 BRepBuilderAPI_Sewing sewing(1e-9);
 auto patch=[&](std::function<gp_Pnt(double,double)> eval,bool reverse){
 NCollection_Array2<gp_Pnt> poles(1,2,1,2);for(int i=1;i<=2;++i)for(int j=1;j<=2;++j)poles.SetValue(i,j,eval(i-1,j-1));
 NCollection_Array1<double> knots(1,2);knots.SetValue(1,0);knots.SetValue(2,1);NCollection_Array1<int> mult(1,2);mult.Init(2);
 occ::handle<Geom_BSplineSurface> surface=new Geom_BSplineSurface(poles,knots,knots,mult,mult,1,1);
 TopoDS_Face face=BRepBuilderAPI_MakeFace(surface,1e-9).Face();if(reverse)face.Reverse();sewing.Add(face);
 };
 patch([](double u,double v){return gp_Pnt(2*u,2*v,0);},true);
 patch([](double u,double v){return gp_Pnt(2*u,2*v,1+u*v);},false);
 for(double x:{0.,2.})patch([x](double u,double v){return gp_Pnt(x,2*u,v*(1+x*u/2));},x==0);
 for(double y:{0.,2.})patch([y](double u,double v){return gp_Pnt(2*u,y,v*(1+u*y/2));},y==2);
 sewing.Perform();auto sewn=sewing.SewedShape();TopExp_Explorer shells(sewn,TopAbs_SHELL);if(!shells.More())throw std::runtime_error("spline reference shell missing");
 auto solid=BRepBuilderAPI_MakeSolid(TopoDS::Shell(shells.Current())).Solid();shells.Next();if(shells.More()||!BRepCheck_Analyzer(solid).IsValid())throw std::runtime_error("invalid spline reference shell");return solid;
}
static void splineShellRecord(std::ostream& out,const char* name,const TopoDS_Shape& shape){
 if(!BRepCheck_Analyzer(shape).IsValid())throw std::runtime_error("invalid spline Boolean reference");
 GProp_GProps volume,area;BRepGProp::VolumeProperties(shape,volume,1e-10);BRepGProp::SurfaceProperties(shape,area,1e-10);const auto c=volume.CentreOfMass();
 int shells=0;for(TopExp_Explorer x(shape,TopAbs_SHELL);x.More();x.Next())++shells;
 out<<std::setprecision(17)<<"{\"Name\":\""<<name<<"\",\"Volume\":"<<volume.Mass()<<",\"Area\":"<<area.Mass()<<",\"Centroid\":["<<c.X()<<','<<c.Y()<<','<<c.Z()<<"],\"Shells\":"<<shells<<"}\n";
}
static void splineShellBooleans(std::ostream& out){
 auto a=splineShellPrism();gp_Trsf scale;scale.SetScale(gp_Pnt(0,0,0),.25);scale.SetTranslationPart(gp_Vec(.5,.5,.25));auto inside=BRepBuilderAPI_Transform(a,scale,true).Shape();
 gp_Trsf move;move.SetTranslation(gp_Vec(3,0,0));auto disjoint=BRepBuilderAPI_Transform(a,move,true).Shape();
 splineShellRecord(out,"nested-union",BRepAlgoAPI_Fuse(a,inside).Shape());splineShellRecord(out,"nested-common",BRepAlgoAPI_Common(a,inside).Shape());splineShellRecord(out,"nested-cut",BRepAlgoAPI_Cut(a,inside).Shape());
 splineShellRecord(out,"disjoint-union",BRepAlgoAPI_Fuse(a,disjoint).Shape());splineShellRecord(out,"disjoint-common",BRepAlgoAPI_Common(a,disjoint).Shape());splineShellRecord(out,"disjoint-cut",BRepAlgoAPI_Cut(a,disjoint).Shape());
 gp_Trsf mirror;mirror.SetMirror(gp_Ax2(gp_Pnt(2,0,0),gp_Dir(1,0,0)));auto joined=BRepBuilderAPI_Transform(a,mirror,true).Shape();
 splineShellRecord(out,"joined-union",BRepAlgoAPI_Fuse(a,joined).Shape());splineShellRecord(out,"joined-common",BRepAlgoAPI_Common(a,joined).Shape());splineShellRecord(out,"joined-cut",BRepAlgoAPI_Cut(a,joined).Shape());
 gp_Trsf insetMap;insetMap.SetValues(.5,0,0,.5,0,.5,0,.5,0,0,-.5,0);auto inset=BRepBuilderAPI_Transform(a,insetMap,true).Shape();
 splineShellRecord(out,"inset-union",BRepAlgoAPI_Fuse(a,inset).Shape());splineShellRecord(out,"inset-common",BRepAlgoAPI_Common(a,inset).Shape());splineShellRecord(out,"inset-cut",BRepAlgoAPI_Cut(a,inset).Shape());
 gp_Trsf overlapMap;overlapMap.SetValues(1,0,0,1,0,1,0,.5,0,0,-1,0);auto overlap=BRepBuilderAPI_Transform(a,overlapMap,true).Shape();
 splineShellRecord(out,"overlap-union",BRepAlgoAPI_Fuse(a,overlap).Shape());splineShellRecord(out,"overlap-common",BRepAlgoAPI_Common(a,overlap).Shape());splineShellRecord(out,"overlap-cut",BRepAlgoAPI_Cut(a,overlap).Shape());
}

static void openSplineBoxBooleans(std::ostream& out){auto a=BRepPrimAPI_MakeBox(1,1,1).Shape();auto b=BRepPrimAPI_MakeBox(gp_Pnt(.5,.5,-.5),1,1,1).Shape();splineShellRecord(out,"open-union",BRepAlgoAPI_Fuse(a,b).Shape());splineShellRecord(out,"open-common",BRepAlgoAPI_Common(a,b).Shape());splineShellRecord(out,"open-cut",BRepAlgoAPI_Cut(a,b).Shape());}

// Two separate transverse section wires, independently authored primitives.
static void multipleOpenSplineBoxBooleans(std::ostream& out){auto a=BRepPrimAPI_MakeBox(1,1,1).Shape();auto b=BRepPrimAPI_MakeBox(gp_Pnt(.25,.25,-.5),.5,.5,2).Shape();splineShellRecord(out,"multiple-open-union",BRepAlgoAPI_Fuse(a,b).Shape());splineShellRecord(out,"multiple-open-common",BRepAlgoAPI_Common(a,b).Shape());splineShellRecord(out,"multiple-open-cut",BRepAlgoAPI_Cut(a,b).Shape());}
