package step

import (
	"errors"
	"fmt"
	"io"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"unicode/utf8"
)

// SphereBody explicitly specifies a complete outward spherical solid; no shape
// is guessed from arbitrary topology or sampled surfaces.
type SphereBody struct {
	Name   string
	Center geom.Vec3
	Radius float64
}

// SphereModel retains coordinates in raw file units, with LengthUnit in metres.
type SphereModel struct {
	Bodies     []SphereBody
	LengthUnit float64
}

// WriteSpheres emits exact spheres using one ADVANCED_FACE, SPHERICAL_SURFACE,
// and pole VERTEX_LOOP per MANIFOLD_SOLID_BREP. No tessellation is exported.
func WriteSpheres(w io.Writer, m SphereModel) error {
	if w == nil || len(m.Bodies) == 0 {
		return errors.New("step: missing sphere writer/model")
	}
	for _, s := range m.Bodies {
		if !utf8.ValidString(s.Name) {
			return errors.New("step: invalid sphere name")
		}
		if _, e := parametric.Sphere(s.Center, s.Radius); e != nil {
			return e
		}
	}
	return writeStandalone(w, m.LengthUnit, "", "ADVANCED_BREP_SHAPE_REPRESENTATION", func(emit func(string) int64) []int64 {
		ids := make([]int64, len(m.Bodies))
		for i, s := range m.Bodies {
			point := func(p geom.Vec3) int64 {
				return emit(fmt.Sprintf("CARTESIAN_POINT('',(%s,%s,%s))", real(p.X), real(p.Y), real(p.Z)))
			}
			center := point(s.Center)
			z := emit("DIRECTION('',(0.,0.,1.))")
			x := emit("DIRECTION('',(1.,0.,0.))")
			placement := emit(fmt.Sprintf("AXIS2_PLACEMENT_3D('',#%d,#%d,#%d)", center, z, x))
			surface := emit(fmt.Sprintf("SPHERICAL_SURFACE('',#%d,%s)", placement, real(s.Radius)))
			pole := point(s.Center.Add(geom.Vec3{Z: s.Radius}))
			vertex := emit(fmt.Sprintf("VERTEX_POINT('',#%d)", pole))
			loop := emit(fmt.Sprintf("VERTEX_LOOP('',#%d)", vertex))
			bound := emit(fmt.Sprintf("FACE_BOUND('',#%d,.T.)", loop))
			face := emit(fmt.Sprintf("ADVANCED_FACE('',(#%d),#%d,.T.)", bound, surface))
			shell := emit(fmt.Sprintf("CLOSED_SHELL('',(#%d))", face))
			ids[i] = emit(fmt.Sprintf("MANIFOLD_SOLID_BREP(%s,#%d)", quote(s.Name), shell))
		}
		return ids
	})
}
