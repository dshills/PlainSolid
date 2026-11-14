// OCCT 4dbfdcc: intersect every transferred solid, including spline faces.
#include <STEPControl_Reader.hxx>
#include <IntCurvesFace_ShapeIntersector.hxx>
#include <TopExp_Explorer.hxx>
#include <gp_Lin.hxx>
#include <algorithm>
#include <cmath>
#include <cstdlib>
#include <iomanip>
#include <iostream>
#include <vector>
int main(int argc,char**argv){if(argc!=10)return 2;STEPControl_Reader r;if(r.ReadFile(argv[1])!=IFSelect_RetDone||r.TransferRoots()<1)return 3;double x[8];for(int i=0;i<8;i++)x[i]=std::strtod(argv[i+2],nullptr);gp_Vec d(x[3],x[4],x[5]);double scale=d.Magnitude();gp_Lin line(gp_Pnt(x[0],x[1],x[2]),gp_Dir(d));std::vector<double> hits;
for(TopExp_Explorer ex(r.OneShape(),TopAbs_SOLID);ex.More();ex.Next()){IntCurvesFace_ShapeIntersector q;q.Load(ex.Current(),1e-9);q.Perform(line,x[6]*scale,x[7]*scale);if(!q.IsDone())return 4;std::vector<double> local;for(int i=1;i<=q.NbPnt();i++)local.push_back(q.WParameter(i)/scale);std::sort(local.begin(),local.end());local.erase(std::unique(local.begin(),local.end(),[](double a,double b){return std::abs(a-b)<1e-8;}),local.end());hits.insert(hits.end(),local.begin(),local.end());}
std::sort(hits.begin(),hits.end());std::cout<<std::setprecision(17)<<"[";for(size_t i=0;i<hits.size();i++){if(i)std::cout<<",";std::cout<<hits[i];}std::cout<<"]\n";}
