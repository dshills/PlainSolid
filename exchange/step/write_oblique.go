package step

import (
	"errors"
	"fmt"
	"io"
	"math"
	"plainsolid/geom"
	"unicode/utf8"
)

// ObliqueCylinderBody is the full circular cylinder above Frame's XY plane
// and below Plane. Plane must face toward positive frame Z and intersect every
// generator strictly above the base. Radius is positive; no height is needed.
type ObliqueCylinderBody struct {
	Name   string
	Frame  geom.Frame
	Radius float64
	Plane  geom.Plane
}
type ObliqueCylinderModel struct {
	Bodies     []ObliqueCylinderBody
	LengthUnit float64
}

type obliqueExport struct {
	body                      ObliqueCylinderBody
	center, x, normal, radial geom.Vec3
	major                     float64
}

// WriteObliqueCylinders writes direct exact conic/line edge geometry with shared
// topology. It does not emit approximate parameter-space spline curves.
func WriteObliqueCylinders(w io.Writer, m ObliqueCylinderModel) error {
	if w == nil || len(m.Bodies) == 0 {
		return errors.New("step: missing oblique cylinder writer/model")
	}
	specs := make([]obliqueExport, len(m.Bodies))
	for i, b := range m.Bodies {
		if !utf8.ValidString(b.Name) || !finiteOblique(b.Radius) || b.Radius <= 0 {
			return errors.New("step: invalid oblique cylinder")
		}
		// Revalidate even zero-value frames and planes before emitting any records.
		f, e := geom.NewFrame(b.Frame.Origin(), b.Frame.XAxis(), b.Frame.ZAxis())
		if e != nil {
			return e
		}
		n := b.Plane.Normal()
		nz := n.Dot(f.ZAxis())
		if !n.IsFinite() || math.Abs(n.Length()-1) > 1e-10 || nz <= 1e-9 {
			return errors.New("step: cap must face positive cylinder axis")
		}
		h := -b.Plane.SignedDistance(f.Origin()) / nz
		horizontal := n.Sub(f.ZAxis().Scale(nz))
		slope := horizontal.Length() / nz
		if !finiteOblique(h) || h-b.Radius*slope <= 1e-9*math.Max(h, b.Radius) {
			return errors.New("step: cap intersects cylinder base")
		}
		q := f.XAxis()
		if horizontal.Length() > 1e-14 {
			q, e = horizontal.Scale(-1).Unit()
			if e != nil {
				return e
			}
		}
		x, e := q.Sub(f.ZAxis().Scale(n.Dot(q) / nz)).Unit()
		if e != nil {
			return e
		}
		major := b.Radius / nz
		center := f.Origin().Add(f.ZAxis().Scale(h))
		if !finiteOblique(major) || !center.IsFinite() {
			return errors.New("step: oblique geometry exceeds float64 range")
		}
		bottom, top := f.Origin().Add(q.Scale(b.Radius)), center.Add(x.Scale(major))
		if !bottom.IsFinite() || !top.IsFinite() || bottom == f.Origin() || top == center || top.Sub(bottom).Dot(f.ZAxis()) <= 0 {
			return errors.New("step: oblique cylinder coordinates overflow or collapse")
		}
		b.Frame = f
		specs[i] = obliqueExport{b, center, x, n, q, major}
	}
	return writeStandalone(w, m.LengthUnit, "", "ADVANCED_BREP_SHAPE_REPRESENTATION", func(emit func(string) int64) []int64 {
		point := func(p geom.Vec3) int64 {
			return emit(fmt.Sprintf("CARTESIAN_POINT('',(%s,%s,%s))", real(p.X), real(p.Y), real(p.Z)))
		}
		direction := func(p geom.Vec3) int64 {
			return emit(fmt.Sprintf("DIRECTION('',(%s,%s,%s))", real(p.X), real(p.Y), real(p.Z)))
		}
		place := func(o, x, z geom.Vec3) int64 {
			a, b, c := point(o), direction(z), direction(x)
			return emit(fmt.Sprintf("AXIS2_PLACEMENT_3D('',#%d,#%d,#%d)", a, b, c))
		}
		edge := func(a, b, g int64) int64 { return emit(fmt.Sprintf("EDGE_CURVE('',#%d,#%d,#%d,.T.)", a, b, g)) }
		use := func(e int64, sense bool) int64 {
			s := ".F."
			if sense {
				s = ".T."
			}
			return emit(fmt.Sprintf("ORIENTED_EDGE('',*,*,#%d,%s)", e, s))
		}
		face := func(surface int64, sense bool, uses []int64) int64 {
			loop := emit("EDGE_LOOP(''," + references(uses) + ")")
			bound := emit(fmt.Sprintf("FACE_BOUND('',#%d,.T.)", loop))
			s := ".F."
			if sense {
				s = ".T."
			}
			return emit(fmt.Sprintf("ADVANCED_FACE('',(#%d),#%d,%s)", bound, surface, s))
		}
		out := make([]int64, len(specs))
		for i, s := range specs {
			b := s.body
			f := b.Frame
			cp := place(f.Origin(), s.radial, f.ZAxis())
			ep := place(s.center, s.x, s.normal)
			cylinder := emit(fmt.Sprintf("CYLINDRICAL_SURFACE('',#%d,%s)", cp, real(b.Radius)))
			bottomPlane := emit(fmt.Sprintf("PLANE('',#%d)", cp))
			topPlane := emit(fmt.Sprintf("PLANE('',#%d)", ep))
			circle := emit(fmt.Sprintf("CIRCLE('',#%d,%s)", cp, real(b.Radius)))
			ellipse := emit(fmt.Sprintf("ELLIPSE('',#%d,%s,%s)", ep, real(s.major), real(b.Radius)))
			bottom := point(f.Origin().Add(s.radial.Scale(b.Radius)))
			top := point(s.center.Add(s.x.Scale(s.major)))
			bv := emit(fmt.Sprintf("VERTEX_POINT('',#%d)", bottom))
			tv := emit(fmt.Sprintf("VERTEX_POINT('',#%d)", top))
			d := direction(f.ZAxis())
			v := emit(fmt.Sprintf("VECTOR('',#%d,1.)", d))
			line := emit(fmt.Sprintf("LINE('',#%d,#%d)", bottom, v))
			be, te, se := edge(bv, bv, circle), edge(tv, tv, ellipse), edge(bv, tv, line)
			side := face(cylinder, true, []int64{use(te, false), use(se, false), use(be, true), use(se, true)})
			cap := face(topPlane, true, []int64{use(te, true)})
			base := face(bottomPlane, false, []int64{use(be, false)})
			shell := emit("CLOSED_SHELL(''," + references([]int64{side, cap, base}) + ")")
			out[i] = emit(fmt.Sprintf("MANIFOLD_SOLID_BREP(%s,#%d)", quote(b.Name), shell))
		}
		return out
	})
}
