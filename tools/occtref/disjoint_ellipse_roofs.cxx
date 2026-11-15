// Independently authored complete common-base stocks. No PlainSolid import,
// tessellation or section curve: OCCT intersects the two original surfaces.
#include <BRepAlgoAPI_Common.hxx>
#include <BRepAlgoAPI_Cut.hxx>
#include <BRepAlgoAPI_Fuse.hxx>
#include <BRepBuilderAPI_MakeFace.hxx>
#include <BRepBuilderAPI_MakeSolid.hxx>
#include <BRepBuilderAPI_Sewing.hxx>
#include <BRepCheck_Analyzer.hxx>
#include <BRepGProp.hxx>
#include <GProp_GProps.hxx>
#include <Geom_BezierSurface.hxx>
#include <NCollection_Array2.hxx>
#include <TopExp_Explorer.hxx>
#include <TopoDS.hxx>
#include <TopoDS_Face.hxx>
#include <TopoDS_Solid.hxx>
#include <functional>
#include <iomanip>
#include <iostream>
#include <stdexcept>
#include <string>

static TopoDS_Solid stock(bool raised,bool vertical) {
  const double v[]={0,.5,1}, tu[]={.0625,-.0625,.0625},tv[]={.25,-.25,.25};
  auto roof=[&](int part,int i,int j){double u=(part+v[i])/2;return 3+u*v[j]/64+(raised?(1.0/64-tu[i]-tv[j]):0);};
  BRepBuilderAPI_Sewing sewing(1e-10);
  auto face=[&](int nu,int nv,bool reverse,const std::function<gp_Pnt(int,int)>& point){
    NCollection_Array2<gp_Pnt> poles(1,nu,1,nv);
    for(int i=0;i<nu;++i)for(int j=0;j<nv;++j){auto p=point(i,j);if(vertical)p=gp_Pnt(p.Y(),p.X(),p.Z());poles.SetValue(i+1,j+1,p);}
    occ::handle<Geom_BezierSurface> support=new Geom_BezierSurface(poles);
    auto f=BRepBuilderAPI_MakeFace(support,1e-10).Face();if(reverse!=vertical)f.Reverse();sewing.Add(f);
  };
  face(2,2,true,[&](int i,int j){return gp_Pnt(i,j,0);});
  for(int part=0;part<2;++part){
    face(3,3,false,[&](int i,int j){return gp_Pnt((part+v[i])/2,v[j],roof(part,i,j));});
    for(int k:{0,2})face(3,2,k==2,[&](int i,int j){return gp_Pnt((part+v[i])/2,v[k],j?roof(part,i,k):0);});
  }
  for(int k:{0,2})face(3,2,k==0,[&](int i,int j){return gp_Pnt(k/2,v[i],j?roof(k/2,k,i):0);});
  sewing.Perform();TopExp_Explorer shells(sewing.SewedShape(),TopAbs_SHELL);
  if(!shells.More())throw std::runtime_error("missing source shell");
  auto s=BRepBuilderAPI_MakeSolid(TopoDS::Shell(shells.Current())).Solid();shells.Next();
  if(shells.More()||!BRepCheck_Analyzer(s).IsValid())throw std::runtime_error("invalid source");
  GProp_GProps m;BRepGProp::VolumeProperties(s,m);if(m.Mass()<=0)throw std::runtime_error("wrong source orientation");
  return s;
}
int main(int argc,char** argv){
  bool vertical=argc==2&&std::string(argv[1])=="vertical";if(argc>2||(argc==2&&!vertical))return 3;
  auto a=stock(true,vertical),b=stock(false,vertical);std::cout<<std::setprecision(17)<<"[";bool first=true;
  for(int order=0;order<2;++order){auto x=order?b:a,y=order?a:b;
    for(int op=0;op<3;++op){TopoDS_Shape s;
      if(op==0){BRepAlgoAPI_Common f(x,y);if(!f.IsDone())return 2;s=f.Shape();}
      else if(op==1){BRepAlgoAPI_Fuse f(x,y);if(!f.IsDone())return 2;s=f.Shape();}
      else{BRepAlgoAPI_Cut f(x,y);if(!f.IsDone())return 2;s=f.Shape();}
      GProp_GProps mass;BRepGProp::VolumeProperties(s,mass,1e-10);int solids=0;
      for(TopExp_Explorer it(s,TopAbs_SOLID);it.More();it.Next())++solids;
      if(!first)std::cout<<",";first=false;
      std::cout<<"{\"Vertical\":"<<(vertical?"true":"false")<<",\"Order\":"<<order<<",\"Operation\":"<<op<<",\"Valid\":"<<(BRepCheck_Analyzer(s).IsValid()?"true":"false")<<",\"Solids\":"<<solids<<",\"Volume\":"<<mass.Mass()<<"}";
    }
  }
  std::cout<<"]\n";
}
