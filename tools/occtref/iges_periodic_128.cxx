// Independent type-128 periodic tensor authorship and D1 readback.
#include <Geom_BSplineSurface.hxx>
#include <IGESControl_Reader.hxx>
#include <IGESControl_Writer.hxx>
#include <IGESData_IGESModel.hxx>
#include <IGESGeom_BSplineSurface.hxx>
#include <IGESToBRep_BasicSurface.hxx>
#include <Standard_Failure.hxx>

#include <array>
#include <iomanip>
#include <iostream>
#include <initializer_list>
#include <string_view>

using Surface = occ::handle<Geom_BSplineSurface>;

template<typename T>
static NCollection_Array1<T> array(std::initializer_list<T> values)
{
    NCollection_Array1<T> result(1, static_cast<int>(values.size()));
    int i = 1;
    for (const auto value : values) {
        result(i++) = value;
    }
    return result;
}

static Surface author(bool doubly, bool exchangeAxes)
{
    const std::array<int, 7> x{7, 5, 0, -5, -6, -3, 4}, y{0, 4, 6, 4, -2, -6, -5};
    const std::array<double, 5> ring{1.25, 1.125, .875, .75, 1.0};
    const std::array<int, 5> z{0, 3, 2, -1, -3};
    const int countV = doubly ? 5 : 4;
    NCollection_Array2<gp_Pnt> poles(1, 7, 1, countV);
    NCollection_Array2<double> weights(1, 7, 1, countV);
    for (int i = 0; i < 7; ++i) {
        for (int j = 0; j < countV; ++j) {
            const double scale = doubly ? ring[j] : 1 + j / 8.;
            const double height = doubly ? z[j] : 2*j + i % 3 / 8.;
            poles(i+1,j+1) = gp_Pnt(scale*x[i] + j/8., scale*y[i], height);
            weights(i+1,j+1) = 1 + ((3*i+j)%7)/(doubly ? 10. : 8.);
        }
    }
    const auto knotsU = array<double>({-1.25, 0., 1.75, 3.5, 5.5, 6.75});
    const auto multU = array<int>({2, 1, 1, 2, 1, 2});
    const auto knotsV = doubly ? array<double>({2.4, 3.375, 4., 6.5, 8.75, 9.6})
                              : array<double>({2.5, 4., 7.5});
    const auto multV = doubly ? array<int>({1, 1, 1, 1, 1, 1}) : array<int>({3, 1, 3});
    Surface s = new Geom_BSplineSurface(poles, weights, knotsU, knotsV, multU, multV, 3, 2, true, doubly);
    if (exchangeAxes) {
        s->ExchangeUV();
    }
    return s;
}

static void report(const Surface& s)
{
    double u0, u1, v0, v1;
    s->Bounds(u0,u1,v0,v1);
    std::cout << std::setprecision(17)
              << "{\"periodic\":[" << (s->IsUPeriodic()?1:0) << "," << (s->IsVPeriodic()?1:0)
              << "],\"counts\":[" << s->NbUPoles() << "," << s->NbVPoles()
              << "],\"bounds\":[" << u0 << "," << u1 << "," << v0 << "," << v1 << "],\"samples\":[";
    const std::array<std::array<double,2>,9> samples{{{0,.1875},{1,.1875},{.00000001,.1875},{.99999999,.1875},
                                                      {.28125,.6875},{.71875,0},{.71875,1},
                                                      {.71875,.00000001},{.71875,.99999999}}};
    for (std::size_t i = 0; i < samples.size(); ++i) {
        const double u = u0 + (u1-u0)*samples[i][0], v = v0 + (v1-v0)*samples[i][1];
        gp_Pnt p;
        gp_Vec du,dv;
        s->D1(u,v,p,du,dv);
        if(i) std::cout << ",";
        std::cout << "{\"u\":" << u << ",\"v\":" << v
                  << ",\"point\":[" << p.X() << "," << p.Y() << "," << p.Z()
                  << "],\"du\":[" << du.X() << "," << du.Y() << "," << du.Z()
                  << "],\"dv\":[" << dv.X() << "," << dv.Y() << "," << dv.Z() << "]}";
    }
    std::cout << "]}\n";
}

int main(int argc,char** argv)
{
    if (argc != 3) return 2;
    try {
        const std::string_view mode(argv[1]);
        if (mode == "write-u" || mode == "write-v" || mode == "write-uv") {
            const Surface s = author(mode == "write-uv", mode == "write-v");
            IGESControl_Writer writer("MM",0);
            if ((!s->IsUPeriodic() && !s->IsVPeriodic()) || !writer.AddGeom(s) || !writer.Write(argv[2])) return 3;
            report(s);
            return 0;
        }
        if (mode != "read") return 2;
        IGESControl_Reader reader;
        if (reader.ReadFile(argv[2]) != IFSelect_RetDone) return 4;
        auto model = reader.IGESModel();
        if (model->NbEntities() != 1) return 5;
        auto raw = occ::down_cast<IGESGeom_BSplineSurface>(model->Entity(1));
        if (raw.IsNull()) return 6;
        std::cout << "{\"rawPeriodic\":[" << (raw->IsPeriodicU()?1:0) << "," << (raw->IsPeriodicV()?1:0)
                  << "],\"rawClosed\":[" << (raw->IsClosedU()?1:0) << "," << (raw->IsClosedV()?1:0)
                  << "],\"expandedCounts\":[" << raw->NbPolesU() << "," << raw->NbPolesV() << "]}\n";
        IGESToBRep_BasicSurface transfer;
        transfer.SetModel(model);
        const Surface s = occ::down_cast<Geom_BSplineSurface>(transfer.TransferBasicSurface(raw));
        if (s.IsNull()) return 7;
        report(s);
        return 0;
    } catch (const Standard_Failure& e) {
        std::cerr << e.what() << "\n";
        return 8;
    }
}
