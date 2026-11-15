#include <BRepAlgoAPI_Common.hxx>
#include <BRepAlgoAPI_Cut.hxx>
#include <BRepAlgoAPI_Fuse.hxx>
#include <BRepBuilderAPI_MakeFace.hxx>
#include <BRepBuilderAPI_MakePolygon.hxx>
#include <BRepCheck_Analyzer.hxx>
#include <BRepGProp.hxx>
#include <BRepPrimAPI_MakePrism.hxx>
#include <GProp_GProps.hxx>
#include <NCollection_IndexedMap.hxx>
#include <Standard_Failure.hxx>
#include <TopExp.hxx>
#include <TopExp_Explorer.hxx>
#include <TopTools_ShapeMapHasher.hxx>
#include <gp_Pln.hxx>

#include <algorithm>
#include <iomanip>
#include <iostream>
#include <stdexcept>
#include <string>
#include <vector>

namespace {
using ShapeMap = NCollection_IndexedMap<TopoDS_Shape, TopTools_ShapeMapHasher>;

TopoDS_Wire readWire(double z, bool hole) {
  int count;
  if (!(std::cin >> count) || count < 3 || count > 256) {
    throw std::runtime_error("invalid ring vertex count");
  }
  std::vector<gp_Pnt> vertices;
  for (int i = 0; i < count; ++i) {
    double x, y;
    if (!(std::cin >> x >> y)) {
      throw std::runtime_error("invalid ring vertex");
    }
    vertices.emplace_back(x, y, z);
  }
  if (hole) {
    std::reverse(vertices.begin(), vertices.end());
  }
  BRepBuilderAPI_MakePolygon wire;
  for (const auto& p : vertices) {
    wire.Add(p);
  }
  wire.Close();
  return wire.Wire();
}

TopoDS_Shape readPrism() {
  double low, high;
  int holes;
  if (!(std::cin >> low >> high >> holes) || low >= high || holes < 0 || holes > 32) {
    throw std::runtime_error("invalid prism controls");
  }
  const gp_Pln plane(gp_Pnt(0, 0, low), gp_Dir(0, 0, 1));
  BRepBuilderAPI_MakeFace face(plane, readWire(low, false), true);
  for (int i = 0; i < holes; ++i) {
    face.Add(readWire(low, true));
  }
  const auto shape = BRepPrimAPI_MakePrism(face.Face(), gp_Vec(0, 0, high - low)).Shape();
  if (shape.IsNull() || !BRepCheck_Analyzer(shape).IsValid()) {
    throw std::runtime_error("invalid independently extruded prism");
  }
  return shape;
}

template<class Operation>
TopoDS_Shape combine(const TopoDS_Shape& first, const TopoDS_Shape& second) {
  Operation op(first, second);
  if (!op.IsDone() || op.HasErrors()) {
    throw std::runtime_error("OCCT Boolean operation failed");
  }
  return op.Shape();
}

void writeMaterial(int index, const TopoDS_Shape& shape) {
  if (shape.IsNull() || !BRepCheck_Analyzer(shape).IsValid()) {
    throw std::runtime_error("invalid reference Boolean shape");
  }
  ShapeMap solids;
  TopExp::MapShapes(shape, TopAbs_SOLID, solids);
  std::vector<int> eulers;
  GProp_GProps volume;
  double area = 0;
  int shellCount = 0;
  for (int i = 1; i <= solids.Extent(); ++i) {
    const auto& solid = solids(i);
    GProp_GProps mass, surface;
    BRepGProp::VolumeProperties(solid, mass, 1.e-12, true);
    BRepGProp::SurfaceProperties(solid, surface, 1.e-12);
    if (mass.Mass() <= 0) {
      throw std::runtime_error("reference solid has nonpositive volume");
    }
    volume.Add(mass);
    area += surface.Mass();
    ShapeMap vertices, edges, faces, shells;
    TopExp::MapShapes(solid, TopAbs_VERTEX, vertices);
    TopExp::MapShapes(solid, TopAbs_EDGE, edges);
    TopExp::MapShapes(solid, TopAbs_FACE, faces);
    TopExp::MapShapes(solid, TopAbs_SHELL, shells);
    shellCount += shells.Extent();
    int euler = vertices.Extent() - edges.Extent();
    for (int f = 1; f <= faces.Extent(); ++f) {
      int wires = 0;
      for (TopExp_Explorer exp(faces(f), TopAbs_WIRE); exp.More(); exp.Next()) {
        ++wires;
      }
      euler += 2 - wires;
    }
    eulers.push_back(euler);
  }
  const auto p = volume.CentreOfMass();
  std::cout << std::setprecision(17) << "{\"index\":" << index << ",\"volume\":" << volume.Mass()
            << ",\"area\":" << area << ",\"centroid\":[" << p.X() << ',' << p.Y() << ',' << p.Z()
            << "],\"solids\":" << solids.Extent() << ",\"shells\":" << shellCount << ",\"eulers\":[";
  for (std::size_t i = 0; i < eulers.size(); ++i) {
    if (i > 0) {
      std::cout << ',';
    }
    std::cout << eulers[i];
  }
  std::cout << "]}\n";
}
}  // namespace

int main() {
  int count;
  if (!(std::cin >> count) || count < 1 || count > 256) {
    return 2;
  }
  for (int index = 0; index < count; ++index) {
    try {
      std::string mode;
      if (!(std::cin >> mode)) {
        throw std::runtime_error("missing operation");
      }
      const auto first = readPrism();
      const auto second = readPrism();
      if (mode == "common") {
        writeMaterial(index, combine<BRepAlgoAPI_Common>(first, second));
      } else if (mode == "fuse") {
        writeMaterial(index, combine<BRepAlgoAPI_Fuse>(first, second));
      } else if (mode == "cut") {
        writeMaterial(index, combine<BRepAlgoAPI_Cut>(first, second));
      } else {
        throw std::runtime_error("unknown operation");
      }
    } catch (const Standard_Failure& e) {
      std::cerr << "OCCT case " << index << ": " << e.what() << '\n';
      return 3;
    } catch (const std::exception& e) {
      std::cerr << "OCCT case " << index << ": " << e.what() << '\n';
      return 4;
    }
  }
}
