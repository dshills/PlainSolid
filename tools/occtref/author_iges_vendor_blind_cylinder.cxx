// Independent OCCT Boolean blind bore, retained without source repair.
#include <IGESControl_Writer.hxx>
#include <BRepPrimAPI_MakeCylinder.hxx>
#include <BRepAlgoAPI_Cut.hxx>
#include <gp_Ax2.hxx>
int main(int argc,char**argv){if(argc!=2)return 2;auto a=BRepPrimAPI_MakeCylinder(3,4).Shape();auto b=BRepPrimAPI_MakeCylinder(gp_Ax2(gp_Pnt(0,0,2),gp_Dir(0,0,1)),1,3).Shape();BRepAlgoAPI_Cut cut(a,b);cut.Build();if(!cut.IsDone())return 3;IGESControl_Writer w("MM",1);if(!w.AddShape(cut.Shape()))return 4;return w.Write(argv[1])?0:5;}
