// Independently authored OCCT support-intersection fixtures; no native output
// or transformed vertices from the Go implementation are used here.
#include <BRepPrimAPI_MakeHalfSpace.hxx>
#include <gp_Pln.hxx>
#include <TopExp.hxx>
#include <NCollection_IndexedMap.hxx>
#include <TopTools_ShapeMapHasher.hxx>
#include <array>
static void supportRecord(std::ostream& out,const char* name,const std::vector<std::array<double,4>>& p){
  TopoDS_Shape s=BRepPrimAPI_MakeBox(gp_Pnt(-10,-10,-10),20,20,20).Shape();
  for(const auto& h:p){
    const double n2=h[0]*h[0]+h[1]*h[1]+h[2]*h[2];
    gp_Pnt on(h[0]*h[3]/n2,h[1]*h[3]/n2,h[2]*h[3]/n2);
    gp_Pnt inside(on.X()-h[0],on.Y()-h[1],on.Z()-h[2]);
    auto face=BRepBuilderAPI_MakeFace(gp_Pln(on,gp_Dir(h[0],h[1],h[2]))).Face();
    auto half=BRepPrimAPI_MakeHalfSpace(face,inside).Solid();
    BRepAlgoAPI_Common common(s,half);common.Build();
    if(!common.IsDone())throw std::runtime_error("draft halfspace common failed");
    s=common.Shape();
  }
  if(!BRepCheck_Analyzer(s).IsValid())throw std::runtime_error("invalid draft reference");
  GProp_GProps v,a;BRepGProp::VolumeProperties(s,v);BRepGProp::SurfaceProperties(s,a);
  NCollection_IndexedMap<TopoDS_Shape, TopTools_ShapeMapHasher> vertices,edges,faces,solids;
  TopExp::MapShapes(s,TopAbs_VERTEX,vertices);TopExp::MapShapes(s,TopAbs_EDGE,edges);
  TopExp::MapShapes(s,TopAbs_FACE,faces);TopExp::MapShapes(s,TopAbs_SOLID,solids);
  const auto c=v.CentreOfMass();
  out<<std::setprecision(17)<<"{\"Name\":\""<<name<<"\",\"Area\":"<<a.Mass()<<",\"Volume\":"<<v.Mass()<<",\"Centroid\":["<<c.X()<<','<<c.Y()<<','<<c.Z()<<"],\"Vertices\":"<<vertices.Extent()<<",\"Edges\":"<<edges.Extent()<<",\"Faces\":"<<faces.Extent()<<",\"Solids\":"<<solids.Extent()<<"}\n";
}
static void draftRecord(std::ostream& out,const char* name,double k,bool single,bool oblique){
 std::vector<std::array<double,4>> p={{{0,0,-1,0}},{{0,0,1,1}},{{0,-1,-k,0}},{{1,0,-k,1}},{{0,1,-k,1}},{{-1,0,-k,0}}};
 if(single){p[2][2]=p[4][2]=p[5][2]=0;}
 if(oblique){const double n=1/std::sqrt(2.0);p[3][2]=-.25*n;p[3][3]=1+.125*n;}
 supportRecord(out,name,p);
}
static void draftRecords(std::ostream& out){
 draftRecord(out,"single",.5,true,false);
 draftRecord(out,"four",.25,false,false);
 draftRecord(out,"collapse",-.75,false,false);
 draftRecord(out,"oblique",0,true,true);
 // Cut a unit cube along x/0.5+y/0.25 <= 1/0.5+1/0.25-1.
 // This independently specifies .25 setback on x=1 and .5 on y=1.
 std::vector<std::array<double,4>> p={{{0,0,-1,0}},{{0,0,1,1}},{{0,-1,0,0}},{{1,0,0,1}},{{0,1,0,1}},{{-1,0,0,0}}};
 p.push_back({{2,4,0,5}});supportRecord(out,"chamfer-two",p);p.pop_back();
 // Four equal 1/8 setbacks, authored as diagonal support inequalities.
 p.push_back({{1,1,0,1.875}});p.push_back({{1,-1,0,.875}});
 p.push_back({{-1,1,0,.875}});p.push_back({{-1,-1,0,-.125}});
 supportRecord(out,"chamfer-four",p);
 p.resize(8);supportRecord(out,"defeature-two",p);
 p.resize(6);supportRecord(out,"defeature-all",p);
}
