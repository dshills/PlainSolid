package step

import (
	"io"
	"plainsolid/geom"
)

const planarSplineCylinder PlanarSplineFamily = "cylinder"

// CylinderMaterialFamily names the supported original local material graph.
type CylinderMaterialFamily string

const (
	CylinderMaterialNode     CylinderMaterialFamily = "assembly"
	CylinderMaterialPlanar   CylinderMaterialFamily = "planar"
	CylinderMaterialSpline   CylinderMaterialFamily = "spline"
	CylinderMaterialCylinder CylinderMaterialFamily = "cylinder"
)

type CylinderMaterialPart struct {
	ID, Name  string
	Family    CylinderMaterialFamily
	Planar    []Body
	Splines   []SplineShellBody
	Cylinders []CylinderBody
}
type CylinderMaterialAssemblyModel struct {
	Products    []CylinderMaterialPart
	Occurrences []AssemblyInstance
	LengthUnit  float64
}

// CylinderMaterialProduct retains actual source body IDs and local decoded
// geometry. Cylinder descriptors are recovered only after the full three-face
// cylinder proof; Analytic retains the corresponding validated native material.
type CylinderMaterialProduct struct {
	Family        CylinderMaterialFamily
	Planar        []Body
	Splines       []SplineShellBody
	Cylinders     []CylinderBody
	Analytic      []AnalyticBody
	SourceBodyIDs []int64
}
type CylinderMaterialAssembly struct {
	Assembly Assembly
	Products map[int64]CylinderMaterialProduct
}

func readAssemblyCylinder(f *File, id int64) (AnalyticBody, error) {
	family, e := planarSplineBodyFamily(f, id, true)
	if e != nil || family != planarSplineCylinder {
		return AnalyticBody{}, planarSplineFailure()
	}
	b, e := simple(f, id, "MANIFOLD_SOLID_BREP", 2)
	if e != nil {
		return AnalyticBody{}, e
	}
	name, e := textValue(b.Args[0])
	if e != nil {
		return AnalyticBody{}, e
	}
	sid, e := ref(b.Args[1])
	if e != nil {
		return AnalyticBody{}, e
	}
	shell, e := simple(f, sid, "CLOSED_SHELL", 2)
	if e != nil {
		return AnalyticBody{}, e
	}
	faces, e := listRefs(shell.Args[1], 3)
	if e != nil || len(faces) != 3 {
		return AnalyticBody{}, planarSplineFailure()
	}
	return cylinderBody(f, name, faces)
}

// WriteCylinderMaterialAssembly extends the heterogeneous workflow by exactly
// the complete finite cylinder family. At least one cylinder product and one
// planar or certified spline product are required. All poses are proper signed
// coordinate rigid placements, with independent product-local geometry.
func WriteCylinderMaterialAssembly(w io.Writer, m CylinderMaterialAssemblyModel) error {
	cylinders := map[string][]CylinderBody{}
	n := PlanarSplineAssemblyModel{Occurrences: m.Occurrences, LengthUnit: m.LengthUnit}
	for _, p := range m.Products {
		cylinders[p.ID] = p.Cylinders
		n.Products = append(n.Products, PlanarSplinePart{ID: p.ID, Name: p.Name, Family: PlanarSplineFamily(p.Family), Planar: p.Planar, Splines: p.Splines})
	}
	return writePlanarSplineAssembly(w, n, true, cylinders)
}

// ReadCylinderMaterialAssembly dispatches only from complete support-family
// graphs. A cylinder must have exactly one cylindrical side and two planar caps;
// sectors, cones, tubes, spheres and mixed supports inside other bodies fail.
// Existing planar/spline-only APIs retain their original acceptance domain.
func ReadCylinderMaterialAssembly(r io.Reader, l Limits) (CylinderMaterialAssembly, error) {
	cylinders := map[int64][]AnalyticBody{}
	a, e := readPlanarSplineAssembly(r, l, true, cylinders)
	if e != nil {
		return CylinderMaterialAssembly{}, e
	}
	out := CylinderMaterialAssembly{Assembly: a.Assembly, Products: map[int64]CylinderMaterialProduct{}}
	for id, p := range a.Products {
		v := CylinderMaterialProduct{Family: CylinderMaterialFamily(p.Family), Planar: p.Planar, Splines: p.Splines, Analytic: cylinders[id], SourceBodyIDs: p.SourceBodyIDs}
		for _, b := range cylinders[id] {
			found := false
			for _, f := range b.Solid.Faces() {
				if s, ok := f.Surface.(geom.CylinderSurface); ok {
					if found || f.V.Min != 0 || f.V.Max <= 0 {
						return CylinderMaterialAssembly{}, planarSplineFailure()
					}
					frame, radius := s.Parameters()
					v.Cylinders = append(v.Cylinders, CylinderBody{Name: b.Name, Frame: frame, Radius: radius, Height: f.V.Max})
					found = true
				}
			}
			if !found {
				return CylinderMaterialAssembly{}, planarSplineFailure()
			}
		}
		out.Products[id] = v
	}
	return out, nil
}
