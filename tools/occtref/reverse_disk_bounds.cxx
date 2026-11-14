// Independently interpret and reverse STEP face-bound traversal through OCCT's
// actual STEP entity model. Preserve all original numeric tokens verbatim.
#include <STEPControl_Reader.hxx>
#include <StepData_StepModel.hxx>
#include <StepShape_FaceBound.hxx>
#include <StepShape_EdgeLoop.hxx>
#include <StepShape_OrientedEdge.hxx>
#include <fstream>
#include <sstream>
#include <map>
#include <regex>
int main(int argc,char**argv){if(argc!=3)return 2;STEPControl_Reader reader;if(reader.ReadFile(argv[1])!=IFSelect_RetDone)return 3;auto m=reader.StepModel();std::map<int,std::string> edits;
for(int i=1;i<=m->NbEntities();i++){auto b=occ::down_cast<StepShape_FaceBound>(m->Value(i));if(b.IsNull())continue;auto loop=occ::down_cast<StepShape_EdgeLoop>(b->Bound());if(loop.IsNull()||!b->Orientation())return 4;int id=m->IdentLabel(b),lid=m->IdentLabel(loop);edits[id]="FACE_BOUND('',#"+std::to_string(lid)+",.F.)";std::string list;
for(int j=loop->NbEdgeList();j>=1;j--){auto use=loop->EdgeListValue(j);int uid=m->IdentLabel(use);if(edits.count(uid))return 5;edits[uid]="ORIENTED_EDGE('',*,*,#"+std::to_string(m->IdentLabel(use->EdgeElement()))+","+(use->Orientation()?".F.":".T.")+")";if(!list.empty())list+=",";list+="#"+std::to_string(uid);}edits[lid]="EDGE_LOOP('',("+list+"))";}
std::ifstream in(argv[1]);std::stringstream all;all<<in.rdbuf();std::string s=all.str();std::regex re("#([0-9]+)[[:space:]]*=[^;]*;");std::ofstream out(argv[2]);size_t pos=0;for(std::sregex_iterator it(s.begin(),s.end(),re),end;it!=end;++it){const auto& x=*it;out<<s.substr(pos,x.position()-pos);int id=std::stoi(x[1]);auto e=edits.find(id);if(e==edits.end())out<<x.str();else out<<"#"<<id<<"="<<e->second<<";";pos=x.position()+x.length();}out<<s.substr(pos);return out?0:6;}
