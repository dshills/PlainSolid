#ifdef PLAINSOLID_OCCT_IGES
#include <TopExp.hxx>
#include <IGESSolid_Shell.hxx>
// Independent homogeneous Bernstein product construction, without PlainSolid data.
static int writeIGESCurvedOval(const char* path, bool annulus, double scale=1, bool fill=false, IGESControl_Writer* shared=nullptr) {
 IGESControl_Writer local("MM");IGESControl_Writer& writer=shared?*shared:local;
 NCollection_Array2<gp_Pnt> p(1,3,1,3); const double sq[3]={.25,.25-.5*scale,(scale-.5)*(scale-.5)};
 for(int i=0;i<3;++i)for(int j=0;j<3;++j)p.SetValue(i+1,j+1,gp_Pnt(i*.5*scale,j*.5*scale,3*(sq[i]+sq[j])));
 NCollection_Array1<double> sk(1,2);sk.SetValue(1,0);sk.SetValue(2,1);NCollection_Array1<int> sm(1,2);sm.SetValue(1,3);sm.SetValue(2,3);
 occ::handle<Geom_BSplineSurface> surface=new Geom_BSplineSurface(p,sk,sk,sm,sm,2,2);
 GeomToIGES_GeomSurface sc;sc.SetModel(writer.Model());auto support=sc.TransferSurface(surface,0,1,0,1);
 auto contour=[&](double radius,bool reverse){
  const int xy[9][2]={{1,0},{1,1},{0,1},{-1,1},{-1,0},{-1,-1},{0,-1},{1,-1},{1,0}};
  NCollection_Array1<gp_Pnt> up(1,9),mp(1,17);NCollection_Array1<double> uw(1,9),mw(1,17),k(1,5);NCollection_Array1<int> um(1,5),mm(1,5);
  for(int i=0;i<9;++i){int j=reverse?8-i:i;up.SetValue(i+1,gp_Pnt((.5+radius*xy[j][0])/scale,(.5+radius*xy[j][1])/scale,0));uw.SetValue(i+1,i%2?.5:1);}
  const double c2[3]={1,2,1},c4[5]={1,4,6,4,1};
  for(int a=0;a<4;++a){double u[3],v[3],w[3],du[3],dv[3];for(int i=0;i<3;++i){auto q=up.Value(2*a+i+1);w[i]=uw.Value(2*a+i+1);u[i]=scale*q.X()*w[i];v[i]=scale*q.Y()*w[i];du[i]=(scale*q.X()-.5)*w[i];dv[i]=(scale*q.Y()-.5)*w[i];}
   auto product=[&](const double* x,const double* y,int n){double s=0;for(int i=0;i<3;++i){int j=n-i;if(j>=0&&j<3)s+=c2[i]*c2[j]*x[i]*y[j];}return s/c4[n];};
   for(int n=0;n<5;++n){double z=product(w,w,n);mw.SetValue(4*a+n+1,z);mp.SetValue(4*a+n+1,gp_Pnt(product(u,w,n)/z,product(v,w,n)/z,3*(product(du,du,n)+product(dv,dv,n))/z));}
  }
  for(int i=1;i<=5;++i){k.SetValue(i,i-1);um.SetValue(i,i==1||i==5?3:2);mm.SetValue(i,i==1||i==5?5:4);}
  occ::handle<Geom_BSplineCurve> uv=new Geom_BSplineCurve(up,uw,k,um,2),model=new Geom_BSplineCurve(mp,mw,k,mm,4);
  GeomToIGES_GeomCurve cc;cc.SetModel(writer.Model());occ::handle<IGESGeom_CurveOnSurface> c=new IGESGeom_CurveOnSurface;c->Init(0,support,cc.TransferCurve(uv,0,4),cc.TransferCurve(model,0,4),3);return c;
 };
 occ::handle<NCollection_HArray1<occ::handle<IGESGeom_CurveOnSurface>>> holes;
 if(annulus){holes=new NCollection_HArray1<occ::handle<IGESGeom_CurveOnSurface>>(1,1);holes->SetValue(1,contour(3./32,true));}
 occ::handle<IGESGeom_TrimmedSurface> face=new IGESGeom_TrimmedSurface;face->Init(support,1,contour(fill?3./32:3./16,false),holes);
 if(!writer.AddEntity(face)||(!shared&&!writer.Write(path)))throw std::runtime_error("curved oval IGES export failed");return 0;
}
static int writeIGESCurvedSheet(const char* path) {
 IGESControl_Writer writer("MM");
 writeIGESCurvedOval(path,true,1,false,&writer);writeIGESCurvedOval(path,false,2,true,&writer);
 if(!writer.Write(path))throw std::runtime_error("sheet write");return 0;
}
static int writeIGESCurvedTopology(const char* path) {
 const std::string temp=std::string(path)+".sheet.igs";writeIGESCurvedSheet(temp.c_str());
 IGESControl_Reader reader;if(reader.ReadFile(temp.c_str())!=IFSelect_RetDone||reader.TransferRoots()==0)throw std::runtime_error("topology source read");
 BRepBuilderAPI_Sewing sewing(1e-9);sewing.Add(reader.OneShape());sewing.Perform();auto shape=sewing.SewedShape();
 if(shape.IsNull()||!BRepCheck_Analyzer(shape).IsValid())throw std::runtime_error("topology sewing invalid");
 IGESControl_Writer writer("MM",1);if(!writer.AddShape(shape))throw std::runtime_error("topology transfer");
 // OCCT TransferShell defaults to form1 even for open source shells. Explicitly
 // mark the fixture open; do not let incidence contradict its IGES declaration.
 for(int i=1;i<=writer.Model()->NbEntities();++i){auto sh=occ::down_cast<IGESSolid_Shell>(writer.Model()->Entity(i));if(!sh.IsNull())sh->SetClosed(false);}
 if(!writer.Write(path))throw std::runtime_error("topology write");std::remove(temp.c_str());return 0;
}
static int inspectIGESCurvedTopology(const char* out,const char* path) {
 IGESControl_Reader reader;if(reader.ReadFile(path)!=IFSelect_RetDone||reader.TransferRoots()==0)throw std::runtime_error("topology transfer");auto shape=reader.OneShape();if(!BRepCheck_Analyzer(shape).IsValid())throw std::runtime_error("invalid topology");
 NCollection_IndexedMap<TopoDS_Shape,TopTools_ShapeMapHasher> faces,edges,vertices,shells;TopExp::MapShapes(shape,TopAbs_FACE,faces);TopExp::MapShapes(shape,TopAbs_EDGE,edges);TopExp::MapShapes(shape,TopAbs_VERTEX,vertices);TopExp::MapShapes(shape,TopAbs_SHELL,shells);GProp_GProps props;BRepGProp::SurfaceProperties(shape,props);
 std::ofstream f(out);f<<std::setprecision(17)<<"{\"Faces\":"<<faces.Extent()<<",\"Edges\":"<<edges.Extent()<<",\"Vertices\":"<<vertices.Extent()<<",\"Shells\":"<<shells.Extent()<<",\"Area\":"<<props.Mass()<<"}\n";return 0;
}
#endif
