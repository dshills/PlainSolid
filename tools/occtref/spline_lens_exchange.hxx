#include <StepData_StepModel.hxx>

#include <StepData_StepWriter.hxx>
#include <StepData_Protocol.hxx>
#pragma once
#ifdef PLAINSOLID_OCCT_STEP
static TopoDS_Solid makeSplineLens(bool sheared=false,bool refined=false){
 NCollection_Array1<double> knots(1,2);knots.SetValue(1,0);knots.SetValue(2,1);NCollection_Array1<int> mult(1,2);mult.SetValue(1,3);mult.SetValue(2,3);
 occ::handle<Geom_BSplineSurface> surfaces[2];const double square[3]={.25,-.25,.25};
 for(int side=0;side<2;++side){NCollection_Array2<gp_Pnt> poles(1,3,1,3);for(int i=0;i<3;++i)for(int j=0;j<3;++j)poles.SetValue(i+1,j+1,gp_Pnt(i*.5,j*.5,2+(side==0?1:-1)*(1./16-square[i]-(sheared?1.25:1)*square[j]-(sheared?(i*.5-.5)*(j*.5-.5):0))/2));surfaces[side]=new Geom_BSplineSurface(poles,knots,knots,mult,mult,2,2);}
 if(refined){surfaces[1]->InsertUKnot(.5,1,0,true);surfaces[1]->InsertVKnot(.5,2,0,true);}
 const int xy[9][2]={{1,0},{1,1},{0,1},{-1,1},{-1,0},{-1,-1},{0,-1},{1,-1},{1,0}};
 TopoDS_Vertex vertices[4];for(int i=0;i<4;++i)vertices[i]=BRepBuilderAPI_MakeVertex(gp_Pnt(.5+.25*xy[2*i][0]-(sheared?.125*xy[2*i][1]:0),.5+.25*xy[2*i][1],2)).Vertex();
 BRep_Builder builder;BRepBuilderAPI_MakeWire wire;
 for(int arc=0;arc<4;++arc){NCollection_Array1<gp_Pnt> poles(1,3);NCollection_Array1<gp_Pnt2d> uvp(1,3);NCollection_Array1<double> weights(1,3);for(int i=0;i<3;++i){double u=.5+.25*xy[2*arc+i][0]-(sheared?.125*xy[2*arc+i][1]:0),v=.5+.25*xy[2*arc+i][1];poles.SetValue(i+1,gp_Pnt(u,v,2));uvp.SetValue(i+1,gp_Pnt2d(u,v));weights.SetValue(i+1,i==2?2:1);}
 occ::handle<Geom_BSplineCurve> model=new Geom_BSplineCurve(poles,weights,knots,mult,2);occ::handle<Geom2d_BSplineCurve> uv=new Geom2d_BSplineCurve(uvp,weights,knots,mult,2);
 auto edge=BRepBuilderAPI_MakeEdge(model,vertices[arc],vertices[(arc+1)%4],0,1).Edge();for(auto surface:surfaces){builder.UpdateEdge(edge,uv,surface,TopLoc_Location(),1e-10);builder.Range(edge,surface,TopLoc_Location(),0,1);}builder.SameRange(edge,true);builder.SameParameter(edge,true);wire.Add(edge);
 }
 auto upper=BRepBuilderAPI_MakeFace(surfaces[0],wire.Wire(),true).Face();auto lower=BRepBuilderAPI_MakeFace(surfaces[1],wire.Wire(),true).Face();lower.Reverse();TopoDS_Shell shell;builder.MakeShell(shell);builder.Add(shell,upper);builder.Add(shell,lower);auto solid=BRepBuilderAPI_MakeSolid(shell).Solid();if(!BRepCheck_Analyzer(solid).IsValid())throw std::runtime_error("invalid spline lens");
 return solid;
}
static TopoDS_Solid makeRefinedCurveLens(){
 const bool sheared=false,refined=false;const double r=105./512;
 NCollection_Array1<double> knots(1,2);knots.SetValue(1,0);knots.SetValue(2,1);NCollection_Array1<int> mult(1,2);mult.SetValue(1,3);mult.SetValue(2,3);
 occ::handle<Geom_BSplineSurface> surfaces[2];const double square[3]={.25,-.25,.25};
 for(int side=0;side<2;++side){NCollection_Array2<gp_Pnt> poles(1,3,1,3);for(int i=0;i<3;++i)for(int j=0;j<3;++j)poles.SetValue(i+1,j+1,gp_Pnt(i*.5,j*.5,2+(side==0?1:-1)*(r*r-square[i]-(sheared?1.25:1)*square[j]-(sheared?(i*.5-.5)*(j*.5-.5):0))/2));surfaces[side]=new Geom_BSplineSurface(poles,knots,knots,mult,mult,2,2);}
 if(refined){surfaces[1]->InsertUKnot(.5,1,0,true);surfaces[1]->InsertVKnot(.5,2,0,true);}
 const int xy[9][2]={{1,0},{1,1},{0,1},{-1,1},{-1,0},{-1,-1},{0,-1},{1,-1},{1,0}};
 TopoDS_Vertex vertices[4];for(int i=0;i<4;++i)vertices[i]=BRepBuilderAPI_MakeVertex(gp_Pnt(.5+r*xy[2*i][0]-(sheared?.125*xy[2*i][1]:0),.5+r*xy[2*i][1],2)).Vertex();
 BRep_Builder builder;BRepBuilderAPI_MakeWire wire;
 for(int arc=0;arc<4;++arc){NCollection_Array1<gp_Pnt> poles(1,3);NCollection_Array1<gp_Pnt2d> uvp(1,3);NCollection_Array1<double> weights(1,3);for(int i=0;i<3;++i){double u=.5+r*xy[2*arc+i][0]-(sheared?.125*xy[2*arc+i][1]:0),v=.5+r*xy[2*arc+i][1];poles.SetValue(i+1,gp_Pnt(u,v,2));uvp.SetValue(i+1,gp_Pnt2d(u,v));weights.SetValue(i+1,i==2?2:1);}
 occ::handle<Geom_BSplineCurve> model=new Geom_BSplineCurve(poles,weights,knots,mult,2);occ::handle<Geom2d_BSplineCurve> uv=new Geom2d_BSplineCurve(uvp,weights,knots,mult,2);
 model->InsertKnot(.75,1,0,true);auto edge=BRepBuilderAPI_MakeEdge(model,vertices[arc],vertices[(arc+1)%4],0,1).Edge();for(int owner=0;owner<2;++owner){auto surface=surfaces[owner];auto local=occ::down_cast<Geom2d_BSplineCurve>(uv->Copy());local->InsertKnot(owner==0?.5:.25,1,0);builder.UpdateEdge(edge,local,surface,TopLoc_Location(),1e-10);builder.Range(edge,surface,TopLoc_Location(),0,1);}builder.SameRange(edge,true);builder.SameParameter(edge,true);wire.Add(edge);
 }
 auto upper=BRepBuilderAPI_MakeFace(surfaces[0],wire.Wire(),true).Face();auto lower=BRepBuilderAPI_MakeFace(surfaces[1],wire.Wire(),true).Face();lower.Reverse();TopoDS_Shell shell;builder.MakeShell(shell);builder.Add(shell,upper);builder.Add(shell,lower);auto solid=BRepBuilderAPI_MakeSolid(shell).Solid();if(!BRepCheck_Analyzer(solid).IsValid())throw std::runtime_error("invalid spline lens");
 return solid;
}
static int writeRefinedCurveLensSTEP(const char* path){STEPControl_Writer writer;if(writer.Transfer(makeRefinedCurveLens(),STEPControl_AsIs)!=IFSelect_RetDone)throw std::runtime_error("refined curve lens transfer");StepData_StepWriter output(writer.Model());output.FloatWriter().SetFormat("%.16E");output.SendModel(occ::down_cast<StepData_Protocol>(writer.Model()->Protocol()));std::ofstream stream(path);if(!output.Print(stream))throw std::runtime_error("refined curve lens export");return 0;}
static int writeSplineLensSTEP(const char* path,bool sheared=false,bool refined=false){auto solid=makeSplineLens(sheared,refined);
 STEPControl_Writer writer;if(writer.Transfer(solid,STEPControl_AsIs)!=IFSelect_RetDone||writer.Write(path)!=IFSelect_RetDone)throw std::runtime_error("spline lens export failed");return 0;
}
static TopoDS_Solid makePartitionedSplineLens();
static TopoDS_Solid makeConcurrentRationalLens();
static int writeSplineLensAssemblySTEP(const char* path,bool partitioned=false,bool refinedCurves=false,bool rationalCurves=false){
 occ::handle<TDocStd_Document> doc=new TDocStd_Document(TCollection_ExtendedString("BinXCAF"));auto shapes=XCAFDoc_DocumentTool::ShapeTool(doc->Main());auto part=shapes->AddShape(rationalCurves?makeConcurrentRationalLens():(refinedCurves?makeRefinedCurveLens():(partitioned?makePartitionedSplineLens():makeSplineLens())),false);TDataStd_Name::Set(part,TCollection_ExtendedString("Shared local lens"));if(rationalCurves){auto colors=XCAFDoc_DocumentTool::ColorTool(doc->Main());colors->SetColor(part,Quantity_Color(.25,.5,.75,Quantity_TOC_sRGB),XCAFDoc_ColorSurf);auto stock=shapes->GetShape(part);int role=0;for(TopExp_Explorer ex(stock,TopAbs_FACE);ex.More();ex.Next()){auto face=shapes->AddSubShape(part,ex.Current());colors->SetColor(face,role++==0?Quantity_Color(.75,.25,.5,Quantity_TOC_sRGB):Quantity_Color(.125,.5,.875,Quantity_TOC_sRGB),XCAFDoc_ColorSurf);}}auto assembly=shapes->NewShape();TDataStd_Name::Set(assembly,TCollection_ExtendedString("Lens group"));
 gp_Trsf a;a.SetTranslation(gp_Vec(4,0,0));auto left=shapes->AddComponent(assembly,part,TopLoc_Location(a));TDataStd_Name::Set(left,TCollection_ExtendedString("Translated"));
 gp_Trsf b;b.SetValues(0,0,1,8,0,1,0,0,-1,0,0,4);auto right=shapes->AddComponent(assembly,part,TopLoc_Location(b));TDataStd_Name::Set(right,TCollection_ExtendedString("Rotated"));shapes->UpdateAssemblies();STEPCAFControl_Writer writer;if(!writer.Transfer(doc))throw std::runtime_error("spline lens assembly transfer failed");if(partitioned||refinedCurves||rationalCurves){StepData_StepWriter output(writer.ChangeWriter().Model());output.FloatWriter().SetFormat("%.16E");output.SendModel(occ::down_cast<StepData_Protocol>(writer.ChangeWriter().Model()->Protocol()));std::ofstream stream(path);if(!output.Print(stream))throw std::runtime_error("partitioned assembly export failed");}else if(writer.Write(path)!=IFSelect_RetDone)throw std::runtime_error("spline lens assembly export failed");return 0;
}

static TopoDS_Solid makePartitionedSplineLens(){
 const double r=15./64,c=.5,knot=41./64,sq[3]={.25,-.25,.25};
 NCollection_Array1<double> knots(1,2);knots.SetValue(1,0);knots.SetValue(2,1);NCollection_Array1<int> mult(1,2);mult.SetValue(1,3);mult.SetValue(2,3);
 occ::handle<Geom_BSplineSurface> surfaces[2];for(int side=0;side<2;++side){NCollection_Array2<gp_Pnt> p(1,3,1,3);for(int i=0;i<3;++i)for(int j=0;j<3;++j)p.SetValue(i+1,j+1,gp_Pnt(i*.5,j*.5,2+(side? -1:1)*(r*r-sq[i]-sq[j])/2));surfaces[side]=new Geom_BSplineSurface(p,knots,knots,mult,mult,2,2);}surfaces[1]->InsertUKnot(knot,1,0,true);
 const double e[2]={c+r,c},n[2]={c,c+r},w[2]={c-r,c},south[2]={c,c-r},a[2]={c+3*r/5,c+4*r/5},b[2]={c+3*r/5,c-4*r/5};
 const double uv[6][3][2]={{{e[0],e[1]},{c+r,c+r/2},{a[0],a[1]}},{{a[0],a[1]},{c+r/3,c+r},{n[0],n[1]}},{{n[0],n[1]},{c-r,c+r},{w[0],w[1]}},{{w[0],w[1]},{c-r,c-r},{south[0],south[1]}},{{south[0],south[1]},{c+r/3,c-r},{b[0],b[1]}},{{b[0],b[1]},{c+r,c-r/2},{e[0],e[1]}}};
 const double weights[6][3]={{4,4,5},{5,6,8},{1,1,2},{1,1,2},{9,9,10},{10,12,18}};
 TopoDS_Vertex vertices[6];for(int i=0;i<6;++i)vertices[i]=BRepBuilderAPI_MakeVertex(gp_Pnt(uv[i][0][0],uv[i][0][1],2)).Vertex();BRep_Builder builder;BRepBuilderAPI_MakeWire wire;
 for(int part=0;part<6;++part){NCollection_Array1<gp_Pnt> p(1,3);NCollection_Array1<gp_Pnt2d> up(1,3);NCollection_Array1<double> wt(1,3);for(int i=0;i<3;++i){p.SetValue(i+1,gp_Pnt(uv[part][i][0],uv[part][i][1],2));up.SetValue(i+1,gp_Pnt2d(uv[part][i][0],uv[part][i][1]));wt.SetValue(i+1,weights[part][i]);}occ::handle<Geom_BSplineCurve> curve=new Geom_BSplineCurve(p,wt,knots,mult,2);occ::handle<Geom2d_BSplineCurve> pc=new Geom2d_BSplineCurve(up,wt,knots,mult,2);auto edge=BRepBuilderAPI_MakeEdge(curve,vertices[part],vertices[(part+1)%6],0,1).Edge();for(auto surface:surfaces){builder.UpdateEdge(edge,pc,surface,TopLoc_Location(),1e-10);builder.Range(edge,surface,TopLoc_Location(),0,1);}builder.SameRange(edge,true);builder.SameParameter(edge,true);wire.Add(edge);}
 auto upper=BRepBuilderAPI_MakeFace(surfaces[0],wire.Wire(),true).Face();auto lower=BRepBuilderAPI_MakeFace(surfaces[1],wire.Wire(),true).Face();lower.Reverse();TopoDS_Shell shell;builder.MakeShell(shell);builder.Add(shell,upper);builder.Add(shell,lower);auto solid=BRepBuilderAPI_MakeSolid(shell).Solid();if(!BRepCheck_Analyzer(solid).IsValid())throw std::runtime_error("invalid partitioned spline lens");return solid;
}
static int writePartitionedSplineLensSTEP(const char* path){auto solid=makePartitionedSplineLens();STEPControl_Writer writer;if(writer.Transfer(solid,STEPControl_AsIs)!=IFSelect_RetDone)throw std::runtime_error("partitioned lens transfer failed");StepData_StepWriter output(writer.Model());output.FloatWriter().SetFormat("%.16E");output.SendModel(occ::down_cast<StepData_Protocol>(writer.Model()->Protocol()));std::ofstream stream(path);if(!output.Print(stream))throw std::runtime_error("partitioned lens export failed");return 0;
}

static TopoDS_Solid makeConcurrentRationalLens(){
 const double r=263340./4194304.,c=.5,ku=c+3*r/5,kv=c+4*r/5;
 NCollection_Array1<double> whole(1,2);whole.SetValue(1,0);whole.SetValue(2,1);NCollection_Array1<int> full(1,2);full.SetValue(1,3);full.SetValue(2,3);
 occ::handle<Geom_BSplineSurface> surfaces[2];for(int side=0;side<2;++side){double k=side?ku:kv;double xy[4]={0,k/2,(1+k)/2,1},sq[4]={.25,.25-k/2,-.25+k/2,.25},plain[3]={0,.5,1},plainSq[3]={.25,-.25,.25};int nu=side?4:3,nv=side?3:4;NCollection_Array2<gp_Pnt> poles(1,nu,1,nv);for(int u=0;u<nu;++u)for(int v=0;v<nv;++v)poles.SetValue(u+1,v+1,gp_Pnt(side?xy[u]:plain[u],side?plain[v]:xy[v],2+(side?-1:1)*(r*r-(side?sq[u]:plainSq[u])-(side?plainSq[v]:sq[v]))/2));NCollection_Array1<double> ks(1,3);ks.SetValue(1,0);ks.SetValue(2,k);ks.SetValue(3,1);NCollection_Array1<int> ms(1,3);ms.SetValue(1,3);ms.SetValue(2,1);ms.SetValue(3,3);surfaces[side]=new Geom_BSplineSurface(poles,side?ks:whole,side?whole:ks,side?ms:full,side?full:ms,2,2);}
 const double uv[7][3][2]={{{c+r,c},{c+r,c+r/2},{c+3*r/5,c+4*r/5}},{{c+3*r/5,c+4*r/5},{c+r/3,c+r},{c,c+r}},{{c,c+r},{c-r/3,c+r},{c-3*r/5,c+4*r/5}},{{c-3*r/5,c+4*r/5},{c-r,c+r/2},{c-r,c}},{{c-r,c},{c-r,c-r},{c,c-r}},{{c,c-r},{c+r/3,c-r},{c+3*r/5,c-4*r/5}},{{c+3*r/5,c-4*r/5},{c+r,c-r/2},{c+r,c}}};
 const double weights[7][3]={{4,4,5},{5,6,8},{9,9,10},{10,12,18},{1,1,2},{9,9,10},{10,12,18}};
 TopoDS_Vertex vertices[7];for(int i=0;i<7;++i)vertices[i]=BRepBuilderAPI_MakeVertex(gp_Pnt(uv[i][0][0],uv[i][0][1],2)).Vertex();BRep_Builder builder;BRepBuilderAPI_MakeWire wire;
 for(int part=0;part<7;++part){NCollection_Array1<gp_Pnt> p(1,3);NCollection_Array1<gp_Pnt2d> up(1,3);NCollection_Array1<double> wt(1,3);for(int j=0;j<3;++j){p.SetValue(j+1,gp_Pnt(uv[part][j][0],uv[part][j][1],2));up.SetValue(j+1,gp_Pnt2d(uv[part][j][0],uv[part][j][1]));wt.SetValue(j+1,weights[part][j]);}occ::handle<Geom_BSplineCurve> model=new Geom_BSplineCurve(p,wt,whole,full,2);if(part%3==0)model->InsertKnot(.5,1,0,true);auto edge=BRepBuilderAPI_MakeEdge(model,vertices[part],vertices[(part+1)%7],0,1).Edge();for(int owner=0;owner<2;++owner){occ::handle<Geom2d_BSplineCurve> pc=new Geom2d_BSplineCurve(up,wt,whole,full,2);if(part%3==owner+1)pc->InsertKnot(.5,1,0);builder.UpdateEdge(edge,pc,surfaces[owner],TopLoc_Location(),1e-10);builder.Range(edge,surfaces[owner],TopLoc_Location(),0,1);}builder.SameRange(edge,true);builder.SameParameter(edge,true);wire.Add(edge);}
 auto upper=BRepBuilderAPI_MakeFace(surfaces[0],wire.Wire(),true).Face();auto lower=BRepBuilderAPI_MakeFace(surfaces[1],wire.Wire(),true).Face();lower.Reverse();TopoDS_Shell shell;builder.MakeShell(shell);builder.Add(shell,upper);builder.Add(shell,lower);auto solid=BRepBuilderAPI_MakeSolid(shell).Solid();if(!BRepCheck_Analyzer(solid).IsValid())throw std::runtime_error("invalid concurrent rational lens");return solid;
}
static int writeConcurrentRationalLensSTEP(const char* path){auto solid=makeConcurrentRationalLens();STEPControl_Writer writer;if(writer.Transfer(solid,STEPControl_AsIs)!=IFSelect_RetDone)throw std::runtime_error("concurrent lens transfer");StepData_StepWriter output(writer.Model());output.FloatWriter().SetFormat("%.16E");output.SendModel(occ::down_cast<StepData_Protocol>(writer.Model()->Protocol()));std::ofstream stream(path);if(!output.Print(stream))throw std::runtime_error("concurrent lens export");return 0;
}

#endif
