package step

import (
	"bytes"
	"fmt"
	"io"
	"math/big"
	"plainsolid/geom"
	"unicode/utf8"
)

// RevolutionSurface preserves angular U in radians and the full original
// meridian parameter V. Geometry is an analytic sweep, not a tensor spline.
type RevolutionSurface struct {
	Name                 string
	Profile              SplineCurve
	Origin, Axis         geom.Vec3
	AngleStart, AngleEnd float64
}
type RevolutionSurfaceModel struct {
	Surfaces   []RevolutionSurface
	LengthUnit float64
}

func (s RevolutionSurface) Surface() (geom.RevolutionSurface, error) {
	if !utf8.ValidString(s.Name) || s.Profile.Closed == "T" || s.Profile.SelfIntersect == "T" {
		return geom.RevolutionSurface{}, geom.ErrDomain
	}
	return geom.NewRevolutionSurface(s.Profile.Geometry, s.Origin, s.Axis, s.AngleStart, s.AngleEnd)
}

// revolutionUnits requires an unambiguous source length scale and exactly one
// unscaled SI radian unit in every global geometric context. Entity decoding
// never silently assumes radians in a file without unit evidence.
func revolutionUnits(f *File) (float64, error) {
	if f == nil {
		return 0, geom.ErrDomain
	}
	unit := 0.
	contexts := 0
	for id, en := range f.Entities {
		r, ok := component(en, "GLOBAL_UNIT_ASSIGNED_CONTEXT")
		if !ok {
			continue
		}
		if len(r.Args) != 1 {
			return 0, geom.ErrDomain
		}
		seen := map[string]bool{}
		for _, part := range en.Records {
			if seen[part.Type] {
				return 0, geom.ErrDomain
			}
			seen[part.Type] = true
		}
		u, e := contextUnits(f, id)
		if e != nil {
			return 0, e
		}
		if unit != 0 && u != unit {
			return 0, geom.ErrDomain
		}
		unit = u
		contexts++
		ids, e := listRefs(r.Args[0], 1)
		if e != nil {
			return 0, e
		}
		angles := 0
		for _, uid := range ids {
			v := f.Entities[uid]
			if _, ok := component(v, "PLANE_ANGLE_UNIT"); !ok {
				continue
			}
			angles++
			seen := map[string]bool{}
			for _, part := range v.Records {
				if seen[part.Type] {
					return 0, geom.ErrDomain
				}
				seen[part.Type] = true
				switch part.Type {
				case "SI_UNIT", "NAMED_UNIT", "PLANE_ANGLE_UNIT":
				default:
					return 0, geom.ErrDomain
				}
			}
			if len(v.Records) != 3 {
				return 0, geom.ErrDomain
			}
			si, ok := component(v, "SI_UNIT")
			if !ok || len(si.Args) != 2 || si.Args[0].Kind != Omitted || si.Args[1].Kind != Enumeration || si.Args[1].Text != "RADIAN" {
				return 0, geom.ErrDomain
			}
			if _, ok := component(v, "CONVERSION_BASED_UNIT"); ok {
				return 0, geom.ErrDomain
			}
		}
		if angles != 1 {
			return 0, geom.ErrDomain
		}
	}
	if contexts == 0 {
		return 0, geom.ErrDomain
	}
	return unit, nil
}
func revolutionSourcePoint(f *File, id int64, kind string) ([3]*big.Rat, error) {
	var out [3]*big.Rat
	r, e := simple(f, id, kind, 2)
	if e != nil || r.Args[1].Kind != Aggregate || len(r.Args[1].Items) != 3 {
		return out, geom.ErrDomain
	}
	if _, e = textValue(r.Args[0]); e != nil {
		return out, e
	}
	for i, v := range r.Args[1].Items {
		out[i], e = liftSourceRat(v)
		if e != nil {
			return out, e
		}
	}
	return out, nil
}
func revolutionExactMeridian(f *File, cid int64, origin [3]*big.Rat, axis int, normalized []float64) error {
	en := f.Entities[cid]
	r, ok := component(en, "B_SPLINE_CURVE")
	var refs Value
	if ok && len(r.Args) == 5 {
		refs = r.Args[1]
	} else {
		r, ok = component(en, "B_SPLINE_CURVE_WITH_KNOTS")
		if !ok || len(r.Args) != 9 {
			return geom.ErrDomain
		}
		refs = r.Args[2]
	}
	ids, e := listRefs(refs, 2)
	if e != nil || len(ids) > 4096 {
		return geom.ErrDomain
	}
	if w, ok := component(en, "RATIONAL_B_SPLINE_CURVE"); ok {
		if len(w.Args) != 1 || w.Args[0].Kind != Aggregate || len(w.Args[0].Items) != len(ids) {
			return geom.ErrDomain
		}
		maximum := new(big.Rat)
		raw := make([]*big.Rat, len(ids))
		for i, v := range w.Args[0].Items {
			q, e := liftSourceRat(v)
			if e != nil || q.Sign() <= 0 {
				return geom.ErrDomain
			}
			raw[i] = q
			if q.Cmp(maximum) > 0 {
				maximum.Set(q)
			}
		}
		if len(normalized) != len(raw) {
			return geom.ErrDomain
		}
		for i, q := range raw {
			ratio := new(big.Rat).Quo(q, maximum)
			if ratio.Cmp(new(big.Rat).SetFloat64(normalized[i])) != 0 {
				return geom.ErrDomain
			}
		}

	}
	heightSign := 0
	var previous, first0, first1 *big.Rat
	perp0, perp1 := (axis+1)%3, (axis+2)%3
	for i, id := range ids {
		p, e := revolutionSourcePoint(f, id, "CARTESIAN_POINT")
		if e != nil {
			return e
		}
		q0 := new(big.Rat).Sub(p[perp0], origin[perp0])
		q1 := new(big.Rat).Sub(p[perp1], origin[perp1])
		if i == 0 {
			first0, first1 = q0, q1
		}
		cross := new(big.Rat).Sub(new(big.Rat).Mul(q0, first1), new(big.Rat).Mul(q1, first0))
		dot := new(big.Rat).Add(new(big.Rat).Mul(q0, first0), new(big.Rat).Mul(q1, first1))
		if cross.Sign() != 0 || dot.Sign() <= 0 {
			return geom.ErrDomain
		}
		if previous != nil {
			sign := p[axis].Cmp(previous)
			if sign == 0 {
				return geom.ErrDomain
			}
			if i == 1 {
				heightSign = sign
			} else if sign != heightSign {
				return geom.ErrDomain
			}
		}
		previous = p[axis]
	}
	return nil
}

// DecodeRevolutionSurface reads a bounded support only. All source global unit
// contexts must agree and use radians. It does not import surrounding faces.
func DecodeRevolutionSurface(f *File, id int64, maxPoints int) (RevolutionSurface, error) {
	if _, e := revolutionUnits(f); e != nil {
		return RevolutionSurface{}, e
	}
	return decodeRevolutionSurface(f, id, maxPoints)
}
func decodeRevolutionSurface(f *File, id int64, maxPoints int) (RevolutionSurface, error) {
	fail := func() (RevolutionSurface, error) { return RevolutionSurface{}, geom.ErrDomain }
	if f == nil || maxPoints < 2 {
		return fail()
	}
	trim, e := simple(f, id, "RECTANGULAR_TRIMMED_SURFACE", 8)
	if e != nil {
		return fail()
	}
	name, e := textValue(trim.Args[0])
	if e != nil {
		return fail()
	}
	for _, v := range trim.Args[6:] {
		sense, e := boolean(v)
		if e != nil || !sense {
			return fail()
		}
	}
	bid, e := ref(trim.Args[1])
	if e != nil {
		return fail()
	}
	basis, e := simple(f, bid, "SURFACE_OF_REVOLUTION", 3)
	if e != nil {
		return fail()
	}
	if _, e = textValue(basis.Args[0]); e != nil {
		return fail()
	}
	cid, e := ref(basis.Args[1])
	if e != nil {
		return fail()
	}
	profile, e := DecodeBSplineCurve(f, cid, min(maxPoints, 4096))
	if e != nil {
		return fail()
	}
	axisID, e := ref(basis.Args[2])
	if e != nil {
		return fail()
	}
	placement, e := simple(f, axisID, "AXIS1_PLACEMENT", 3)
	if e != nil {
		return fail()
	}
	if _, e = textValue(placement.Args[0]); e != nil {
		return fail()
	}
	pid, e := ref(placement.Args[1])
	if e != nil {
		return fail()
	}
	did, e := ref(placement.Args[2])
	if e != nil {
		return fail()
	}
	origin, e := revolutionSourcePoint(f, pid, "CARTESIAN_POINT")
	if e != nil {
		return fail()
	}
	axis, e := revolutionSourcePoint(f, did, "DIRECTION")
	if e != nil {
		return fail()
	}
	ax := -1
	var ov, av [3]float64
	for i := 0; i < 3; i++ {
		if axis[i].Sign() != 0 {
			if ax >= 0 || new(big.Rat).Abs(axis[i]).Cmp(big.NewRat(1, 1)) != 0 {
				return fail()
			}
			ax = i
		}
		ov[i], _ = origin[i].Float64()
		av[i], _ = axis[i].Float64()
	}
	if ax < 0 {
		return fail()
	}
	if e = revolutionExactMeridian(f, cid, origin, ax, profile.Geometry.Weights()); e != nil {
		return fail()
	}
	var bounds [4]float64
	var exact [4]*big.Rat
	for i := range bounds {
		exact[i], e = liftSourceRat(trim.Args[2+i])
		if e != nil {
			return fail()
		}
		bounds[i], e = numeric(trim.Args[2+i])
		if e != nil {
			return fail()
		}
	}
	if exact[0].Cmp(big.NewRat(-3, 1)) < 0 || exact[1].Cmp(big.NewRat(3, 1)) > 0 || exact[0].Cmp(exact[1]) >= 0 || new(big.Rat).Sub(exact[1], exact[0]).Cmp(big.NewRat(3, 1)) > 0 {
		return fail()
	}
	kr, ok := component(f.Entities[cid], "B_SPLINE_CURVE_WITH_KNOTS")
	if !ok {
		return fail()
	}
	var kv Value
	if len(kr.Args) == 9 {
		kv = kr.Args[7]
	} else if len(kr.Args) == 3 {
		kv = kr.Args[1]
	} else {
		return fail()
	}
	if kv.Kind != Aggregate || len(kv.Items) < 2 {
		return fail()
	}
	lo, e0 := liftSourceRat(kv.Items[0])
	hi, e1 := liftSourceRat(kv.Items[len(kv.Items)-1])
	if e0 != nil || e1 != nil || lo.Cmp(exact[2]) != 0 || hi.Cmp(exact[3]) != 0 {
		return fail()
	}
	v0, v1 := profile.Geometry.Domain()
	if bounds[2] != v0 || bounds[3] != v1 {
		return fail()
	}
	s := RevolutionSurface{Name: name, Profile: profile, Origin: geom.Vec3{X: ov[0], Y: ov[1], Z: ov[2]}, Axis: geom.Vec3{X: av[0], Y: av[1], Z: av[2]}, AngleStart: bounds[0], AngleEnd: bounds[1]}
	if _, e = s.Surface(); e != nil {
		return fail()
	}
	return s, nil
}
func ReadRevolutionSurfaces(r io.Reader, l Limits) (RevolutionSurfaceModel, error) {
	l, e := l.defaults()
	if e != nil {
		return RevolutionSurfaceModel{}, e
	}
	f, e := Parse(r, l)
	if e != nil {
		return RevolutionSurfaceModel{}, e
	}
	unit, e := revolutionUnits(f)
	if e != nil {
		return RevolutionSurfaceModel{}, e
	}
	roots, rootUnit, e := boundedSweptSurfaceRoots(f, l, "SURFACE_OF_REVOLUTION")
	if e != nil || rootUnit != unit {
		return RevolutionSurfaceModel{}, geom.ErrDomain
	}
	m := RevolutionSurfaceModel{LengthUnit: unit}
	count := 0
	for _, id := range roots {
		s, e := decodeRevolutionSurface(f, id, l.MaxVertices-count)
		if e != nil {
			return RevolutionSurfaceModel{}, e
		}
		count += len(s.Profile.Geometry.Poles())
		m.Surfaces = append(m.Surfaces, s)
	}
	return m, nil
}
func WriteRevolutionSurfaces(w io.Writer, m RevolutionSurfaceModel) error {
	l, _ := (Limits{}).defaults()
	if w == nil || len(m.Surfaces) == 0 || len(m.Surfaces) > l.MaxFaces || !validLengthUnit(m.LengthUnit) {
		return geom.ErrDomain
	}
	curves := make([]SplineCurve, len(m.Surfaces))
	count := 0
	for i, s := range m.Surfaces {
		if _, e := s.Surface(); e != nil {
			return e
		}
		count += len(s.Profile.Geometry.Poles())
		if count > l.MaxVertices {
			return geom.ErrDomain
		}
		curves[i] = s.Profile
	}
	var raw bytes.Buffer
	if e := WriteCurves(&raw, CurveModel{Curves: curves, LengthUnit: m.LengthUnit}); e != nil {
		return e
	}
	f, e := Parse(bytes.NewReader(raw.Bytes()), l)
	if e != nil {
		return e
	}
	var roots []int64
	for _, id := range f.Order {
		if _, ok := component(f.Entities[id], "B_SPLINE_CURVE_WITH_KNOTS"); ok {
			roots = append(roots, id)
		}
	}
	if len(roots) != len(curves) {
		return geom.ErrDomain
	}
	return writeStandalone(w, m.LengthUnit, "GEOMETRIC_SET", "GEOMETRICALLY_BOUNDED_SURFACE_SHAPE_REPRESENTATION", func(emit func(string) int64) []int64 {
		ids := copySplineGeometry(emit, f, roots)
		out := make([]int64, len(ids))
		for i, id := range ids {
			s := m.Surfaces[i]
			p := emit(fmt.Sprintf("CARTESIAN_POINT('',(%s,%s,%s))", real(s.Origin.X), real(s.Origin.Y), real(s.Origin.Z)))
			d := emit(fmt.Sprintf("DIRECTION('',(%s,%s,%s))", real(s.Axis.X), real(s.Axis.Y), real(s.Axis.Z)))
			a := emit(fmt.Sprintf("AXIS1_PLACEMENT('',#%d,#%d)", p, d))
			basis := emit(fmt.Sprintf("SURFACE_OF_REVOLUTION('',#%d,#%d)", id, a))
			v0, v1 := s.Profile.Geometry.Domain()
			out[i] = emit(fmt.Sprintf("RECTANGULAR_TRIMMED_SURFACE(%s,#%d,%s,%s,%s,%s,.T.,.T.)", quote(s.Name), basis, real(s.AngleStart), real(s.AngleEnd), real(v0), real(v1)))
		}
		return out
	})
}
