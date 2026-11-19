// Oracle helper of the general Boolean trial (specs/general-boolean, A6).
//
// Usage: general_boolean CASES.json
//
// The file holds {"cases":[{"name":…,"shapes":[…],"steps":[…]}]}. A shape is
// a box (min, max), a cylinder (origin, axis, radius, height) or a prism
// (outer, holes, offset), with an optional rigid placement of three rows of
// four numbers. A step is {"op":"union"|"common"|"cut","a":i,"b":j}; its
// operands are shapes, numbered from 0, or the results of earlier steps,
// numbered on from the last shape. For every step the helper prints one line
// of JSON: validity, volume, area, centroid, the number of solids and the
// number of faces after faces on one surface are merged. There is no code
// for any one case.
#include <BRepAlgoAPI_Common.hxx>
#include <BRepAlgoAPI_Cut.hxx>
#include <BRepAlgoAPI_Fuse.hxx>
#include <BRepBuilderAPI_MakeFace.hxx>
#include <BRepBuilderAPI_MakePolygon.hxx>
#include <BRepBuilderAPI_Transform.hxx>
#include <BRepCheck_Analyzer.hxx>
#include <BRepGProp.hxx>
#include <BRepPrimAPI_MakeBox.hxx>
#include <BRepPrimAPI_MakeCylinder.hxx>
#include <BRepPrimAPI_MakePrism.hxx>
#include <GProp_GProps.hxx>
#include <ShapeUpgrade_UnifySameDomain.hxx>
#include <TopExp_Explorer.hxx>
#include <TopoDS.hxx>
#include <gp_Ax2.hxx>
#include <gp_Dir.hxx>
#include <gp_Pnt.hxx>
#include <gp_Trsf.hxx>
#include <gp_Vec.hxx>
#include <cstdlib>
#include <fstream>
#include <iomanip>
#include <iostream>
#include <map>
#include <memory>
#include <sstream>
#include <stdexcept>
#include <string>
#include <vector>

// A value of the case file: a number, a string, a list or an object.
struct Value {
    double number = 0;
    std::string text;
    std::vector<Value> list;
    std::map<std::string, Value> fields;
    const Value& at(const std::string& key) const {
        const auto it = fields.find(key);
        if (it == fields.end()) throw std::runtime_error("missing field " + key);
        return it->second;
    }
    bool has(const std::string& key) const { return fields.count(key) != 0; }
};

struct Reader {
    const std::string& s;
    size_t i = 0;
    void space() { while (i < s.size() && (s[i] == ' ' || s[i] == '\n' || s[i] == '\t' || s[i] == '\r')) ++i; }
    void expect(char c) {
        space();
        if (i >= s.size() || s[i] != c) throw std::runtime_error(std::string("expected ") + c);
        ++i;
    }
    bool next(char c) {
        space();
        if (i < s.size() && s[i] == c) { ++i; return true; }
        return false;
    }
    std::string string() {
        expect('"');
        std::string out;
        while (i < s.size() && s[i] != '"') out += s[i++];
        expect('"');
        return out;
    }
    Value value() {
        Value v;
        space();
        if (i >= s.size()) throw std::runtime_error("unexpected end");
        if (s[i] == '"') {
            v.text = string();
        } else if (next('[')) {
            if (!next(']')) {
                do v.list.push_back(value()); while (next(','));
                expect(']');
            }
        } else if (next('{')) {
            if (!next('}')) {
                do {
                    const std::string key = string();
                    expect(':');
                    v.fields[key] = value();
                } while (next(','));
                expect('}');
            }
        } else {
            char* end = nullptr;
            v.number = std::strtod(s.c_str() + i, &end);
            if (end == s.c_str() + i) throw std::runtime_error("expected a value");
            i = end - s.c_str();
        }
        return v;
    }
};

static gp_Pnt point(const Value& v) { return gp_Pnt(v.list.at(0).number, v.list.at(1).number, v.list.at(2).number); }
static gp_Vec vec(const Value& v) { return gp_Vec(v.list.at(0).number, v.list.at(1).number, v.list.at(2).number); }

static TopoDS_Wire wire(const Value& loop) {
    BRepBuilderAPI_MakePolygon polygon;
    for (const auto& p : loop.list) polygon.Add(point(p));
    polygon.Close();
    return polygon.Wire();
}

static TopoDS_Shape build(const Value& shape) {
    const std::string kind = shape.at("kind").text;
    TopoDS_Shape out;
    if (kind == "box") {
        out = BRepPrimAPI_MakeBox(point(shape.at("min")), point(shape.at("max"))).Shape();
    } else if (kind == "cylinder") {
        const gp_Ax2 axes(point(shape.at("origin")), gp_Dir(vec(shape.at("axis"))));
        out = BRepPrimAPI_MakeCylinder(axes, shape.at("radius").number, shape.at("height").number).Shape();
    } else if (kind == "prism") {
        BRepBuilderAPI_MakeFace face(wire(shape.at("outer")), true);
        if (shape.has("holes")) {
            for (const auto& hole : shape.at("holes").list) {
                // An inner wire runs against the outer one. Whether the file
                // gives it that way is settled by the area of the face.
                BRepBuilderAPI_MakeFace with(face.Face(), wire(hole));
                GProp_GProps before, after;
                BRepGProp::SurfaceProperties(face.Face(), before);
                BRepGProp::SurfaceProperties(with.Face(), after);
                if (after.Mass() > before.Mass()) {
                    with = BRepBuilderAPI_MakeFace(face.Face(), TopoDS::Wire(wire(hole).Reversed()));
                }
                face = with;
            }
        }
        out = BRepPrimAPI_MakePrism(face.Face(), vec(shape.at("offset"))).Shape();
    } else {
        throw std::runtime_error("unknown kind " + kind);
    }
    if (shape.has("placement")) {
        const auto& m = shape.at("placement").list;
        gp_Trsf t;
        t.SetValues(m.at(0).list.at(0).number, m.at(0).list.at(1).number, m.at(0).list.at(2).number, m.at(0).list.at(3).number,
                    m.at(1).list.at(0).number, m.at(1).list.at(1).number, m.at(1).list.at(2).number, m.at(1).list.at(3).number,
                    m.at(2).list.at(0).number, m.at(2).list.at(1).number, m.at(2).list.at(2).number, m.at(2).list.at(3).number);
        out = BRepBuilderAPI_Transform(out, t, true).Shape();
    }
    return out;
}

static int count(const TopoDS_Shape& shape, TopAbs_ShapeEnum kind) {
    int n = 0;
    for (TopExp_Explorer it(shape, kind); it.More(); it.Next()) ++n;
    return n;
}

int main(int argc, char** argv) {
    if (argc != 2) {
        std::cerr << "usage: " << argv[0] << " CASES.json\n";
        return 2;
    }
    std::ifstream in(argv[1]);
    std::stringstream text;
    text << in.rdbuf();
    const std::string source = text.str();
    try {
        Reader reader{source};
        const Value file = reader.value();
        std::cout << std::setprecision(17);
        for (const auto& c : file.at("cases").list) {
            std::vector<TopoDS_Shape> shapes;
            for (const auto& s : c.at("shapes").list) shapes.push_back(build(s));
            int step = 0;
            for (const auto& s : c.at("steps").list) {
                const TopoDS_Shape& a = shapes.at(static_cast<size_t>(s.at("a").number));
                const TopoDS_Shape& b = shapes.at(static_cast<size_t>(s.at("b").number));
                const std::string op = s.at("op").text;
                std::unique_ptr<BRepAlgoAPI_BooleanOperation> boolean;
                if (op == "union") boolean = std::make_unique<BRepAlgoAPI_Fuse>(a, b);
                else if (op == "common") boolean = std::make_unique<BRepAlgoAPI_Common>(a, b);
                else if (op == "cut") boolean = std::make_unique<BRepAlgoAPI_Cut>(a, b);
                else throw std::runtime_error("unknown op " + op);
                TopoDS_Shape result = boolean->IsDone() ? boolean->Shape() : TopoDS_Shape();
                const bool done = boolean->IsDone() && !result.IsNull();
                const int solids = done ? count(result, TopAbs_SOLID) : 0;
                const bool valid = done && BRepCheck_Analyzer(result).IsValid();
                double volume = 0, area = 0;
                gp_Pnt centre(0, 0, 0);
                int faces = 0;
                if (done && solids > 0) {
                    GProp_GProps v, s2;
                    BRepGProp::VolumeProperties(result, v);
                    BRepGProp::SurfaceProperties(result, s2);
                    volume = v.Mass();
                    area = s2.Mass();
                    centre = v.CentreOfMass();
                    ShapeUpgrade_UnifySameDomain unify(result, true, true, false);
                    unify.Build();
                    faces = count(unify.Shape(), TopAbs_FACE);
                }
                std::cout << "{\"Name\":\"" << c.at("name").text << "\",\"Step\":" << step++
                          << ",\"Done\":" << (done ? "true" : "false") << ",\"Valid\":" << (valid ? "true" : "false")
                          << ",\"Volume\":" << volume << ",\"Area\":" << area
                          << ",\"Centroid\":[" << centre.X() << "," << centre.Y() << "," << centre.Z() << "]"
                          << ",\"Solids\":" << solids << ",\"Faces\":" << faces << "}\n";
                shapes.push_back(result);
            }
        }
    } catch (const std::exception& e) {
        std::cerr << "general_boolean: " << e.what() << "\n";
        return 1;
    }
    return 0;
}
