#pragma once
#ifdef PLAINSOLID_OCCT_STEP
#include <Geom2d_BSplineCurve.hxx>
#include <BRepBuilderAPI_MakeVertex.hxx>
#include <BRepBuilderAPI_MakeEdge.hxx>
#include <gp_Pnt2d.hxx>
static TopoDS_Face makeCurvedOvalFace(bool rational=false,bool hole=false,bool restricted=false){
 NCollection_Array2<gp_Pnt> poles(1,3,1,3);NCollection_Array2<double> sw(1,3,1,3);const double square[3]={.25,-.25,.25},wu[3]={1,1.5,2},xp[3]={0,1,3};const double scale=rational?85085:1;
 for(int i=0;i<3;++i)for(int j=0;j<3;++j){poles.SetValue(i+1,j+1,gp_Pnt(scale*(rational?xp[i]:(restricted?.25+i*.25:i*.5)),scale*(restricted?.25+j*.25:j*.5),scale*3*(restricted?.25:1)*(square[i]+square[j])/(rational?wu[i]:1)));sw.SetValue(i+1,j+1,rational?wu[i]:1);}
 NCollection_Array1<double> k(1,2);k.SetValue(1,0);k.SetValue(2,1);NCollection_Array1<int> sm(1,2);sm.SetValue(1,3);sm.SetValue(2,3);
 occ::handle<Geom_BSplineSurface> surface=new Geom_BSplineSurface(poles,sw,k,k,sm,sm,2,2);
 const int xy[9][2]={{1,0},{1,1},{0,1},{-1,1},{-1,0},{-1,-1},{0,-1},{1,-1},{1,0}};auto makeWire=[&](double radius,bool clockwise){
 auto index=[&](int n){return clockwise?8-n:n;};
 TopoDS_Vertex vertices[4];for(int i=0;i<4;++i){double u=.5+radius*xy[index(2*i)][0],v=.5+radius*xy[index(2*i)][1];if(restricted){u=.25+.5*u;v=.25+.5*v;}vertices[i]=BRepBuilderAPI_MakeVertex(gp_Pnt(scale*(rational?3*u:u),scale*v,scale*3*((u-.5)*(u-.5)+(v-.5)*(v-.5))/(rational?1+u:1))).Vertex();}
 BRep_Builder builder;BRepBuilderAPI_MakeWire wire;
 const double choose2[3]={1,2,1},choose4[5]={1,4,6,4,1};
 for(int arc=0;arc<4;++arc){
  NCollection_Array1<gp_Pnt2d> up(1,3);NCollection_Array1<double> uw(1,3);double uh[3],vh[3],wh[3],du[3],dv[3],factor[3];
  for(int i=0;i<3;++i){double u=.5+radius*xy[index(2*arc+i)][0],v=.5+radius*xy[index(2*arc+i)][1],w=i==1?.5:1;up.SetValue(i+1,gp_Pnt2d(u,v));uw.SetValue(i+1,w);if(restricted){u=.25+.5*u;v=.25+.5*v;}uh[i]=u*w;vh[i]=v*w;wh[i]=w;du[i]=(u-.5)*w;dv[i]=(v-.5)*w;factor[i]=rational?w+uh[i]:w;}
  occ::handle<Geom2d_BSplineCurve> uv=new Geom2d_BSplineCurve(up,uw,k,sm,2);
  auto product=[&](const double* a,const double* b,int n){double numerator=0;for(int i=0;i<3;++i){int j=n-i;if(j>=0&&j<3)numerator+=choose2[i]*choose2[j]*a[i]*b[j];}return numerator/choose4[n];};
  NCollection_Array1<gp_Pnt> mp(1,5);NCollection_Array1<double> mw(1,5);NCollection_Array1<int> mm(1,2);mm.SetValue(1,5);mm.SetValue(2,5);
  for(int n=0;n<5;++n){double w=product(wh,factor,n);mp.SetValue(n+1,gp_Pnt(scale*(rational?3:1)*product(uh,factor,n)/w,scale*product(vh,factor,n)/w,scale*3*(product(du,du,n)+product(dv,dv,n))/w));mw.SetValue(n+1,w);}
  occ::handle<Geom_BSplineCurve> model=new Geom_BSplineCurve(mp,mw,k,mm,4);
  auto edge=BRepBuilderAPI_MakeEdge(model,vertices[arc],vertices[(arc+1)%4],0,1).Edge();builder.UpdateEdge(edge,uv,surface,TopLoc_Location(),1e-10);builder.Range(edge,surface,TopLoc_Location(),0,1);builder.SameRange(edge,true);builder.SameParameter(edge,true);wire.Add(edge);
 }
 return wire.Wire();
 };
 BRepBuilderAPI_MakeFace faceBuilder(surface,makeWire(3./16,false),true);if(hole)faceBuilder.Add(makeWire(3./32,true));auto face=faceBuilder.Face();if(!BRepCheck_Analyzer(face).IsValid())throw std::runtime_error("invalid exact curved oval face");
 return face;
}
static int writeCurvedOvalSTEP(const char* path,bool rational=false,bool hole=false){
 auto face=makeCurvedOvalFace(rational,hole);
 STEPControl_Writer writer;if(writer.Transfer(face,STEPControl_AsIs)!=IFSelect_RetDone||writer.Write(path)!=IFSelect_RetDone)throw std::runtime_error("curved oval STEP export failed");return 0;
}
static int writeCurvedOvalShellSTEP(const char* path){
 auto annulus=makeCurvedOvalFace(false,true),disk=makeCurvedOvalFace(false,false,true);
 BRepBuilderAPI_Sewing sewing(1e-9);sewing.Add(annulus);sewing.Add(disk);sewing.Perform();auto shape=sewing.SewedShape();
 if(shape.ShapeType()!=TopAbs_SHELL||!BRepCheck_Analyzer(shape).IsValid())throw std::runtime_error("invalid curved oval sheet");
 STEPControl_Writer writer;if(writer.Transfer(shape,STEPControl_AsIs)!=IFSelect_RetDone||writer.Write(path)!=IFSelect_RetDone)throw std::runtime_error("curved oval shell STEP export failed");return 0;
}
#endif
