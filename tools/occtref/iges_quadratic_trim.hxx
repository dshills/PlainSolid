#ifdef PLAINSOLID_OCCT_IGES
static int writeIGESQuadraticTrim(const char* path, bool annulus=false) {
 IGESControl_Writer writer("MM");
 NCollection_Array2<gp_Pnt> poles(1,2,1,2);
 poles.SetValue(1,1,gp_Pnt(0,0,0));poles.SetValue(1,2,gp_Pnt(0,3,0));poles.SetValue(2,1,gp_Pnt(2,0,0));poles.SetValue(2,2,gp_Pnt(2,3,0));
 NCollection_Array1<double> knots(1,2);knots.SetValue(1,0);knots.SetValue(2,1);NCollection_Array1<int> mult(1,2);mult.SetValue(1,2);mult.SetValue(2,2);
 occ::handle<Geom_BSplineSurface> plane=new Geom_BSplineSurface(poles,knots,knots,mult,mult,1,1);
 GeomToIGES_GeomSurface sc;sc.SetModel(writer.Model());auto support=sc.TransferSurface(plane,0,1,0,1);
 const int xy[9][2]={{1,0},{1,1},{0,1},{-1,1},{-1,0},{-1,-1},{0,-1},{1,-1},{1,0}};
 NCollection_Array1<gp_Pnt> uvp(1,9),mp(1,9);NCollection_Array1<double> weights(1,9),k(1,5);NCollection_Array1<int> m(1,5);
 for(int i=1;i<=9;++i){double x=.5+.25*xy[i-1][0],y=.5+.25*xy[i-1][1];uvp.SetValue(i,gp_Pnt(x,y,0));mp.SetValue(i,gp_Pnt(2*x,3*y,0));weights.SetValue(i,i%2?1:.75);}
 for(int i=1;i<=5;++i){k.SetValue(i,i-1);m.SetValue(i,i==1||i==5?3:2);}
 occ::handle<Geom_BSplineCurve> uv=new Geom_BSplineCurve(uvp,weights,k,m,2),model=new Geom_BSplineCurve(mp,weights,k,m,2);
 GeomToIGES_GeomCurve cc;cc.SetModel(writer.Model());occ::handle<IGESGeom_CurveOnSurface> contour=new IGESGeom_CurveOnSurface;
 contour->Init(0,support,cc.TransferCurve(uv,0,4),cc.TransferCurve(model,0,4),3);
 occ::handle<IGESGeom_TrimmedSurface> face=new IGESGeom_TrimmedSurface;occ::handle<NCollection_HArray1<occ::handle<IGESGeom_CurveOnSurface>>> holes;
 if(annulus){
  for(int i=1;i<=9;++i){int j=9-i;double x=.5+.125*xy[j][0],y=.5+.125*xy[j][1];uvp.SetValue(i,gp_Pnt(x,y,0));mp.SetValue(i,gp_Pnt(2*x,3*y,0));}
  occ::handle<Geom_BSplineCurve> iu=new Geom_BSplineCurve(uvp,weights,k,m,2),im=new Geom_BSplineCurve(mp,weights,k,m,2);
  occ::handle<IGESGeom_CurveOnSurface> inner=new IGESGeom_CurveOnSurface;inner->Init(0,support,cc.TransferCurve(iu,0,4),cc.TransferCurve(im,0,4),3);
  holes=new NCollection_HArray1<occ::handle<IGESGeom_CurveOnSurface>>(1,1);holes->SetValue(1,inner);
 }
 face->Init(support,1,contour,holes);
 if(!writer.AddEntity(face)||!writer.Write(path))throw std::runtime_error("quadratic trim IGES export failed");return 0;
}
#endif
#if defined(PLAINSOLID_OCCT_IGES) && defined(PLAINSOLID_OCCT_STEP)
static int writeSTEPQuadraticTrim(const char* path) {
 const std::string temporary=std::string(path)+".iges";
 writeIGESQuadraticTrim(temporary.c_str(),true);
 IGESControl_Reader reader;if(reader.ReadFile(temporary.c_str())!=IFSelect_RetDone||reader.TransferRoots()==0)throw std::runtime_error("quadratic fixture IGES transfer failed");
 const auto shape=reader.OneShape();if(shape.IsNull()||!BRepCheck_Analyzer(shape).IsValid())throw std::runtime_error("quadratic fixture invalid");
 STEPControl_Writer writer;if(writer.Transfer(shape,STEPControl_AsIs)!=IFSelect_RetDone||writer.Write(path)!=IFSelect_RetDone)throw std::runtime_error("quadratic STEP fixture export failed");
 std::remove(temporary.c_str());return 0;
}
#endif
