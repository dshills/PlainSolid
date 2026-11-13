// Independent C0 two-peak graph roof with an internal multiplicity-two knot.
static TopoDS_Shape twoContourSplinePrism(){
 const double xs[5]={.0625,.28125,.5,.71875,.9375},ys[3]={.25,.5,.75};double z[5][3];
 for(int i=0;i<5;++i)for(int j=0;j<3;++j)z[i][j]=25./1024-(i%2?-49./1024:49./1024)-(j==1?-1./16:1./16);
 BRepBuilderAPI_Sewing sew(1e-9);
 auto patch=[&](int nu,int nv,std::function<gp_Pnt(int,int)> pole,bool reverse){
 NCollection_Array2<gp_Pnt> p(1,nu,1,nv);for(int i=0;i<nu;++i)for(int j=0;j<nv;++j)p.SetValue(i+1,j+1,pole(i,j));
 int uc=nu==5?3:2;NCollection_Array1<double> uk(1,uc),vk(1,2);NCollection_Array1<int> um(1,uc),vm(1,2);uk.SetValue(1,0);uk.SetValue(uc,1);if(uc==3)uk.SetValue(2,.5);um.Init(nu==2?2:3);if(uc==3)um.SetValue(2,2);vk.SetValue(1,0);vk.SetValue(2,1);vm.Init(nv);
 occ::handle<Geom_BSplineSurface> surf=new Geom_BSplineSurface(p,uk,vk,um,vm,nu==2?1:2,nv-1);TopoDS_Face f=BRepBuilderAPI_MakeFace(surf,1e-9).Face();if(reverse)f.Reverse();sew.Add(f);
 };
 patch(2,2,[&](int i,int j){return gp_Pnt(xs[i*4],ys[j*2],-1);},true);
 patch(5,3,[&](int i,int j){return gp_Pnt(xs[i],ys[j],z[i][j]);},false);
 for(int side=0;side<2;++side){int x=side*4;patch(3,2,[&,x](int i,int j){return gp_Pnt(xs[x],ys[i],j?z[x][i]:-1);},side==0);}
 for(int side=0;side<2;++side){int y=side*2;patch(5,2,[&,y](int i,int j){return gp_Pnt(xs[i],ys[y],j?z[i][y]:-1);},side==1);}
 sew.Perform();TopExp_Explorer sh(sew.SewedShape(),TopAbs_SHELL);if(!sh.More())throw std::runtime_error("two-contour shell missing");auto solid=BRepBuilderAPI_MakeSolid(TopoDS::Shell(sh.Current())).Solid();sh.Next();if(sh.More()||!BRepCheck_Analyzer(solid).IsValid())throw std::runtime_error("two-contour shell invalid");return solid;
}
static void twoContourRecord(std::ostream& out,const char* name,const TopoDS_Shape& s){
 splineShellRecord(out,name,s);
 NCollection_IndexedMap<TopoDS_Shape,TopTools_ShapeMapHasher> v,e,f;TopExp::MapShapes(s,TopAbs_VERTEX,v);TopExp::MapShapes(s,TopAbs_EDGE,e);TopExp::MapShapes(s,TopAbs_FACE,f);
 int holes=0;for(TopExp_Explorer face(s,TopAbs_FACE);face.More();face.Next()){int wires=0;for(TopExp_Explorer wire(face.Current(),TopAbs_WIRE);wire.More();wire.Next())++wires;if(wires>0)holes+=wires-1;}
 out<<"{\"Name\":\""<<name<<"-topology\",\"Vertices\":"<<v.Extent()<<",\"Edges\":"<<e.Extent()<<",\"Faces\":"<<f.Extent()<<",\"Euler\":"<<v.Extent()-e.Extent()+f.Extent()-holes<<"}\n";
}
static void twoContourSplineBooleans(std::ostream& out){auto a=BRepPrimAPI_MakeBox(1,1,1).Shape();auto b=twoContourSplinePrism();twoContourRecord(out,"two-b",b);twoContourRecord(out,"two-union",BRepAlgoAPI_Fuse(a,b).Shape());twoContourRecord(out,"two-common",BRepAlgoAPI_Common(a,b).Shape());twoContourRecord(out,"two-cut",BRepAlgoAPI_Cut(a,b).Shape());}
// The narrower box clips the second lobe through its center: one closed
// interface disk and one open half-disk, with two disjoint volume overlaps.
static void mixedSplineShellBooleans(std::ostream& out){auto a=BRepPrimAPI_MakeBox(23./32,1,1).Shape();auto b=twoContourSplinePrism();twoContourRecord(out,"mixed-b",b);twoContourRecord(out,"mixed-union",BRepAlgoAPI_Fuse(a,b).Shape());twoContourRecord(out,"mixed-common",BRepAlgoAPI_Common(a,b).Shape());twoContourRecord(out,"mixed-cut",BRepAlgoAPI_Cut(a,b).Shape());}
