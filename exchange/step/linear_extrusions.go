package step

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"math/big"
	"plainsolid/geom"
	"strings"
	"unicode/utf8"
)

// LinearExtrusionSurface preserves a full-domain spline profile swept along
// an exact signed coordinate unit axis. VStart/VEnd are finite source parameters.
// It describes surface geometry, not a regularity, embedding or solid proof.
type LinearExtrusionSurface struct {
	Name         string
	Profile      SplineCurve
	Axis         geom.Vec3
	VStart, VEnd float64
}
type LinearExtrusionModel struct {
	Surfaces   []LinearExtrusionSurface
	LengthUnit float64
}

// Surface constructs the equivalent tensor spline without rounding any
// represented profile pole translation. Nonrepresentable translations fail.
func (s LinearExtrusionSurface) Surface() (*geom.BSplineSurface, error) {
	if s.Profile.Geometry == nil || !utf8.ValidString(s.Name) || !finiteNumber(s.VStart) || !finiteNumber(s.VEnd) || s.VStart >= s.VEnd {
		return nil, geom.ErrDomain
	}
	axis := []float64{s.Axis.X, s.Axis.Y, s.Axis.Z}
	nonzero := 0
	for _, x := range axis {
		if x != 0 {
			if x != 1 && x != -1 {
				return nil, geom.ErrDomain
			}
			nonzero++
		}
	}
	if nonzero != 1 {
		return nil, geom.ErrDomain
	}
	c := s.Profile.Geometry
	u0, u1 := c.Domain()
	if !finiteNumber(u0) || !finiteNumber(u1) || u0 >= u1 {
		return nil, geom.ErrDomain
	}
	knots := c.Knots()
	for i := 0; i <= c.Degree(); i++ {
		if knots[i] != u0 || knots[len(knots)-1-i] != u1 {
			return nil, geom.ErrDomain
		}
	}
	if _, _, e := surfaceKnotStrings(knots, c.Degree()); e != nil {
		return nil, e
	}
	poles := c.Poles()
	weights := c.Weights()
	net := make([]geom.Vec3, 0, len(poles)*2)
	ws := make([]float64, 0, len(poles)*2)
	for i, p := range poles {
		for _, v := range []float64{s.VStart, s.VEnd} {
			vals := []float64{p.X, p.Y, p.Z}
			for j := range vals {
				q := new(big.Rat).Add(new(big.Rat).SetFloat64(vals[j]), new(big.Rat).Mul(new(big.Rat).SetFloat64(v), new(big.Rat).SetFloat64(axis[j])))
				f, exact := q.Float64()
				if !exact || !finiteNumber(f) {
					return nil, geom.ErrUnrepresentable
				}
				vals[j] = f
			}
			net = append(net, geom.Vec3{X: vals[0], Y: vals[1], Z: vals[2]})
			ws = append(ws, weights[i])
		}
	}
	return geom.NewBSplineSurface(c.Degree(), 1, len(poles), 2, net, c.Knots(), []float64{s.VStart, s.VStart, s.VEnd, s.VEnd}, ws)
}

// DecodeLinearExtrusionSurface accepts RECTANGULAR_TRIMMED_SURFACE over
// SURFACE_OF_LINEAR_EXTRUSION with positive senses and the full spline U range.
// Extrusion VECTOR magnitude must be exactly one; arbitrary direction
// normalization and partial U trimming are intentionally unsupported.
func DecodeLinearExtrusionSurface(f *File, id int64, maxPoints int) (LinearExtrusionSurface, error) {
	fields, e := decodeLinearExtrusionFields(f, id, maxPoints)
	if e != nil {
		return LinearExtrusionSurface{}, e
	}
	var axis [3]float64
	for i, r := range fields.ratios {
		x, exact := r.Float64()
		if !exact {
			return LinearExtrusionSurface{}, geom.ErrDomain
		}
		axis[i] = x
	}
	out := fields.surface
	out.Axis = geom.Vec3{X: axis[0], Y: axis[1], Z: axis[2]}
	if _, e = out.Surface(); e != nil {
		return LinearExtrusionSurface{}, e
	}
	return out, nil
}

type linearExtrusionFields struct {
	surface LinearExtrusionSurface
	ratios  [3]*big.Rat
}

func decodeLinearExtrusionFields(f *File, id int64, maxPoints int) (linearExtrusionFields, error) {
	fail := func() (linearExtrusionFields, error) {
		return linearExtrusionFields{}, errors.New("step: unsupported bounded linear extrusion")
	}
	if f == nil || maxPoints < 4 {
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
	senseU, e := boolean(trim.Args[6])
	if e != nil || !senseU {
		return fail()
	}
	senseV, e := boolean(trim.Args[7])
	if e != nil || !senseV {
		return fail()
	}
	basis, e := ref(trim.Args[1])
	if e != nil {
		return fail()
	}
	sweep, e := simple(f, basis, "SURFACE_OF_LINEAR_EXTRUSION", 3)
	if e != nil {
		return fail()
	}
	if _, e = textValue(sweep.Args[0]); e != nil {
		return fail()
	}
	curve, e := ref(sweep.Args[1])
	if e != nil {
		return fail()
	}
	profile, e := DecodeBSplineCurve(f, curve, maxPoints/2)
	if e != nil {
		return fail()
	}
	if e = exactSplineWeightNormalization(f, curve, "RATIONAL_B_SPLINE_CURVE"); e != nil {
		return linearExtrusionFields{}, e
	}
	u0, u1 := profile.Geometry.Domain()
	var bounds [4]float64
	for i := range bounds {
		bounds[i], e = numeric(trim.Args[2+i])
		if e != nil {
			return fail()
		}
	}
	kr, ok := component(f.Entities[curve], "B_SPLINE_CURVE_WITH_KNOTS")
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
	lo, e0 := liftSourceRat(trim.Args[2])
	hi, e1 := liftSourceRat(trim.Args[3])
	klo, e2 := liftSourceRat(kv.Items[0])
	khi, e3 := liftSourceRat(kv.Items[len(kv.Items)-1])
	if e0 != nil || e1 != nil || e2 != nil || e3 != nil || lo.Cmp(klo) != 0 || hi.Cmp(khi) != 0 || bounds[0] != u0 || bounds[1] != u1 {
		return fail()
	}
	vid, e := ref(sweep.Args[2])
	if e != nil {
		return fail()
	}
	vec, e := simple(f, vid, "VECTOR", 3)
	if e != nil {
		return fail()
	}
	if _, e = textValue(vec.Args[0]); e != nil {
		return fail()
	}
	mag, e := liftSourceRat(vec.Args[2])
	if e != nil || mag.Cmp(big.NewRat(1, 1)) != 0 {
		return fail()
	}
	did, e := ref(vec.Args[1])
	if e != nil {
		return fail()
	}
	direction, e := simple(f, did, "DIRECTION", 2)
	if e != nil || direction.Args[1].Kind != Aggregate || len(direction.Args[1].Items) != 3 {
		return fail()
	}
	if _, e = textValue(direction.Args[0]); e != nil {
		return fail()
	}
	var ratios [3]*big.Rat
	for i, x := range direction.Args[1].Items {
		ratios[i], e = liftSourceRat(x)
		if e != nil {
			return fail()
		}
	}
	out := LinearExtrusionSurface{Name: name, Profile: profile, VStart: bounds[2], VEnd: bounds[3]}
	return linearExtrusionFields{out, ratios}, nil
}

// ReadLinearExtrusions reads standalone bounded surface geometry, preserving
// source units and parameters. Assemblies, face topology and unused swept
// geometry are rejected. MaxVertices bounds the total derived control net.
func ReadLinearExtrusions(r io.Reader, l Limits) (LinearExtrusionModel, error) {
	l, e := l.defaults()
	if e != nil {
		return LinearExtrusionModel{}, e
	}
	f, e := Parse(r, l)
	if e != nil {
		return LinearExtrusionModel{}, e
	}
	roots, unit, e := linearExtrusionRoots(f, l)
	if e != nil {
		return LinearExtrusionModel{}, e
	}
	m := LinearExtrusionModel{LengthUnit: unit}
	count := 0
	for _, id := range roots {
		s, e := DecodeLinearExtrusionSurface(f, id, l.MaxVertices-count)
		if e != nil {
			return LinearExtrusionModel{}, e
		}
		count += 2 * len(s.Profile.Geometry.Poles())
		m.Surfaces = append(m.Surfaces, s)
	}
	return m, nil
}

func linearExtrusionRoots(f *File, l Limits) ([]int64, float64, error) {
	fail := func() ([]int64, float64, error) {
		return nil, 0, errors.New("step: unsupported linear extrusion model ownership")
	}
	var roots []int64
	var unit float64
	used := map[int64]bool{}
	reps := 0
	for _, id := range f.Order {
		en := f.Entities[id]
		for _, rec := range en.Records {
			if !linearExtrusionRecordAllowed(rec.Type) {
				return fail()
			}
			if strings.Contains(rec.Type, "TRANSFORMATION") || strings.Contains(rec.Type, "REPRESENTATION_RELATIONSHIP") || rec.Type == "MAPPED_ITEM" || rec.Type == "REPRESENTATION_MAP" || rec.Type == "ADVANCED_FACE" || rec.Type == "MANIFOLD_SOLID_BREP" {
				return fail()
			}
			if strings.HasSuffix(rec.Type, "SHAPE_REPRESENTATION") && rec.Type != "GEOMETRICALLY_BOUNDED_SURFACE_SHAPE_REPRESENTATION" {
				return fail()
			}
		}
		if len(en.Records) != 1 || en.Records[0].Type != "GEOMETRICALLY_BOUNDED_SURFACE_SHAPE_REPRESENTATION" {
			continue
		}
		reps++
		if reps != 1 {
			return fail()
		}
		rep, e := simple(f, id, "GEOMETRICALLY_BOUNDED_SURFACE_SHAPE_REPRESENTATION", 3)
		if e != nil {
			return fail()
		}
		ctx, e := ref(rep.Args[2])
		if e != nil {
			return fail()
		}
		unit, e = contextUnits(f, ctx)
		if e != nil {
			return fail()
		}
		items, e := listRefs(rep.Args[1], 1)
		if e != nil || len(items) != 1 {
			return fail()
		}
		setID := items[0]
		set, e := simple(f, setID, "GEOMETRIC_SET", 2)
		if e != nil {
			return fail()
		}
		used[setID] = true
		ids, e := listRefs(set.Args[1], 1)
		if e != nil || len(ids) > l.MaxFaces {
			return fail()
		}
		for _, sid := range ids {
			if used[sid] {
				return fail()
			}
			used[sid] = true
			trim, e := simple(f, sid, "RECTANGULAR_TRIMMED_SURFACE", 8)
			if e != nil {
				return fail()
			}
			bid, e := ref(trim.Args[1])
			if e != nil {
				return fail()
			}
			if used[bid] {
				return fail()
			}
			used[bid] = true
			sweep, e := simple(f, bid, "SURFACE_OF_LINEAR_EXTRUSION", 3)
			if e != nil {
				return fail()
			}
			cid, e := ref(sweep.Args[1])
			if e != nil {
				return fail()
			}
			if used[cid] {
				return fail()
			}
			used[cid] = true
			roots = append(roots, sid)
		}
	}
	for id, en := range f.Entities {
		for _, rec := range en.Records {
			switch rec.Type {
			case "GEOMETRIC_SET", "RECTANGULAR_TRIMMED_SURFACE", "SURFACE_OF_LINEAR_EXTRUSION", "B_SPLINE_CURVE_WITH_KNOTS", "B_SPLINE_SURFACE_WITH_KNOTS", "B_SPLINE_CURVE", "RATIONAL_B_SPLINE_CURVE", "BOUNDED_CURVE", "CURVE":
				if !used[id] {
					return fail()
				}
			}
		}
	}
	if len(roots) == 0 {
		return fail()
	}
	return roots, unit, nil
}

// WriteLinearExtrusions emits swept source profiles, not approximated tensor
// replacements. Validation and staging complete before any caller output.
func WriteLinearExtrusions(w io.Writer, m LinearExtrusionModel) error {
	l, _ := (Limits{}).defaults()
	if w == nil || len(m.Surfaces) == 0 || len(m.Surfaces) > l.MaxFaces || !validLengthUnit(m.LengthUnit) {
		return geom.ErrDomain
	}
	curves := make([]SplineCurve, len(m.Surfaces))
	count := 0
	for i, s := range m.Surfaces {
		if s.Profile.Geometry == nil {
			return geom.ErrDomain
		}
		count += 2 * len(s.Profile.Geometry.Poles())
		if count > l.MaxVertices {
			return geom.ErrDomain
		}
		if _, e := s.Surface(); e != nil {
			return e
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
		return geom.ErrInvalid
	}
	return writeStandalone(w, m.LengthUnit, "GEOMETRIC_SET", "GEOMETRICALLY_BOUNDED_SURFACE_SHAPE_REPRESENTATION", func(emit func(string) int64) []int64 {
		cids := copySplineGeometry(emit, f, roots)
		out := make([]int64, len(cids))
		for i, cid := range cids {
			s := m.Surfaces[i]
			d := emit(fmt.Sprintf("DIRECTION('',(%s,%s,%s))", real(s.Axis.X), real(s.Axis.Y), real(s.Axis.Z)))
			v := emit(fmt.Sprintf("VECTOR('',#%d,1.)", d))
			basis := emit(fmt.Sprintf("SURFACE_OF_LINEAR_EXTRUSION('',#%d,#%d)", cid, v))
			u0, u1 := s.Profile.Geometry.Domain()
			out[i] = emit(fmt.Sprintf("RECTANGULAR_TRIMMED_SURFACE(%s,#%d,%s,%s,%s,%s,.T.,.T.)", quote(s.Name), basis, real(u0), real(u1), real(s.VStart), real(s.VEnd)))
		}
		return out
	})
}

func linearExtrusionRecordAllowed(kind string) bool {
	switch kind {
	case "RECTANGULAR_TRIMMED_SURFACE", "SURFACE_OF_LINEAR_EXTRUSION", "GEOMETRIC_SET", "GEOMETRICALLY_BOUNDED_SURFACE_SHAPE_REPRESENTATION",
		"B_SPLINE_CURVE_WITH_KNOTS", "B_SPLINE_CURVE", "BOUNDED_CURVE", "CURVE", "RATIONAL_B_SPLINE_CURVE", "REPRESENTATION_ITEM", "GEOMETRIC_REPRESENTATION_ITEM",
		"CARTESIAN_POINT", "DIRECTION", "VECTOR",
		"GEOMETRIC_REPRESENTATION_CONTEXT", "GLOBAL_UNIT_ASSIGNED_CONTEXT", "GLOBAL_UNCERTAINTY_ASSIGNED_CONTEXT", "UNCERTAINTY_MEASURE_WITH_UNIT", "REPRESENTATION_CONTEXT",
		"LENGTH_UNIT", "PLANE_ANGLE_UNIT", "SOLID_ANGLE_UNIT", "NAMED_UNIT", "SI_UNIT", "CONVERSION_BASED_UNIT", "DIMENSIONAL_EXPONENTS", "LENGTH_MEASURE_WITH_UNIT", "MEASURE_WITH_UNIT",
		"APPLICATION_CONTEXT", "APPLICATION_PROTOCOL_DEFINITION", "MECHANICAL_CONTEXT", "PRODUCT_CONTEXT", "DESIGN_CONTEXT", "PRODUCT_DEFINITION_CONTEXT", "PRODUCT", "PRODUCT_DEFINITION_FORMATION_WITH_SPECIFIED_SOURCE", "PRODUCT_DEFINITION_FORMATION", "PRODUCT_DEFINITION", "PRODUCT_DEFINITION_SHAPE", "SHAPE_DEFINITION_REPRESENTATION":
		return true
	}
	return false
}
