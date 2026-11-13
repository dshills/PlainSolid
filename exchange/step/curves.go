package step

import (
	"errors"

	"io"
	"math"
	"plainsolid/geom"
	"strconv"
	"strings"
)

// SplineCurve preserves a STEP spline's name and informational logical flags.
// Closed and SelfIntersect use "T", "F", or "U"; they do not prove topology.
type SplineCurve struct {
	Name                  string
	Geometry              *geom.BSpline
	Closed, SelfIntersect string
}

// CurveModel contains independent untrimmed B-spline curves in explicit units.
type CurveModel struct {
	Curves     []SplineCurve
	LengthUnit float64
}

func integer(v Value) (int, error) {
	if v.Kind != Number || strings.ContainsAny(v.Text, ".Ee") {
		return 0, errors.New("step: expected integer")
	}
	n, e := strconv.Atoi(v.Text)
	return n, e
}
func logical(v Value) (string, error) {
	if v.Kind != Enumeration || (v.Text != "T" && v.Text != "F" && v.Text != "U") {
		return "", errors.New("step: invalid logical")
	}
	return v.Text, nil
}

// DecodeBSplineCurve decodes an explicit-knot 3D spline entity from a parsed
// file. It accepts simple polynomial and complex rational STEP instances.
// maxPoints bounds both control points and (by degree/count) expanded knots.
func DecodeBSplineCurve(f *File, id int64, maxPoints int) (SplineCurve, error) {
	fail := func() (SplineCurve, error) {
		return SplineCurve{}, errors.New("step: unsupported or malformed B-spline curve")
	}
	if f == nil || maxPoints < 2 {
		return fail()
	}
	ent, ok := f.Entities[id]
	if !ok {
		return fail()
	}
	var name Value
	var base, knots, weights []Value
	rational := false
	if len(ent.Records) == 1 && ent.Records[0].Type == "B_SPLINE_CURVE_WITH_KNOTS" {
		a := ent.Records[0].Args
		if len(a) != 9 {
			return fail()
		}
		name = a[0]
		base = a[1:6]
		knots = a[6:]
	} else {
		seen := map[string]bool{}
		for _, r := range ent.Records {
			if seen[r.Type] {
				return fail()
			}
			seen[r.Type] = true
			switch r.Type {
			case "BOUNDED_CURVE", "CURVE", "GEOMETRIC_REPRESENTATION_ITEM":
				if len(r.Args) != 0 {
					return fail()
				}
			case "REPRESENTATION_ITEM":
				if len(r.Args) != 1 {
					return fail()
				}
				name = r.Args[0]
			case "B_SPLINE_CURVE":
				base = r.Args
			case "B_SPLINE_CURVE_WITH_KNOTS":
				knots = r.Args
			case "RATIONAL_B_SPLINE_CURVE":
				rational = true
				if len(r.Args) != 1 || r.Args[0].Kind != Aggregate {
					return fail()
				}
				weights = r.Args[0].Items
			default:
				return fail()
			}
		}
		for _, k := range []string{"BOUNDED_CURVE", "CURVE", "GEOMETRIC_REPRESENTATION_ITEM", "REPRESENTATION_ITEM", "B_SPLINE_CURVE", "B_SPLINE_CURVE_WITH_KNOTS", "RATIONAL_B_SPLINE_CURVE"} {
			if !seen[k] {
				return fail()
			}
		}
	}
	if len(base) != 5 || len(knots) != 3 {
		return fail()
	}
	label, e := textValue(name)
	if e != nil {
		return fail()
	}
	degree, e := integer(base[0])
	if e != nil || degree < 1 || degree >= maxPoints {
		return fail()
	}
	if base[1].Kind != Aggregate || len(base[1].Items) < degree+1 || len(base[1].Items) > maxPoints {
		return fail()
	}
	if base[2].Kind != Enumeration {
		return fail()
	}
	switch base[2].Text {
	case "POLYLINE_FORM", "CIRCULAR_ARC", "ELLIPTIC_ARC", "PARABOLIC_ARC", "HYPERBOLIC_ARC", "UNSPECIFIED":
	default:
		return fail()
	}
	closed, e := logical(base[3])
	if e != nil {
		return fail()
	}
	self, e := logical(base[4])
	if e != nil {
		return fail()
	}
	poles := make([]geom.Vec3, len(base[1].Items))
	for i, v := range base[1].Items {
		pid, e := ref(v)
		if e != nil {
			return fail()
		}
		p, e := coordinate(f, pid, "CARTESIAN_POINT", 3)
		if e != nil {
			return SplineCurve{}, e
		}
		poles[i] = p
	}
	if knots[0].Kind != Aggregate || knots[1].Kind != Aggregate || len(knots[0].Items) < 2 || len(knots[0].Items) != len(knots[1].Items) || knots[2].Kind != Enumeration {
		return fail()
	}
	switch knots[2].Text {
	case "UNIFORM_KNOTS", "QUASI_UNIFORM_KNOTS", "PIECEWISE_BEZIER_KNOTS", "UNSPECIFIED":
	default:
		return fail()
	}
	expected := len(poles) + degree + 1
	expanded := make([]float64, 0, expected)
	previous := 0.0
	for i, v := range knots[0].Items {
		n, e := integer(v)
		bound := degree
		if i == 0 || i == len(knots[0].Items)-1 {
			bound++
		}
		if e != nil || n < 1 || n > bound || n > expected-len(expanded) {
			return fail()
		}
		x, e := numeric(knots[1].Items[i])
		if e != nil || (i > 0 && x <= previous) {
			return fail()
		}
		previous = x
		for j := 0; j < n; j++ {
			expanded = append(expanded, x)
		}
	}
	if len(expanded) != expected {
		return fail()
	}
	var w []float64
	if rational {
		if len(weights) != len(poles) {
			return fail()
		}
		w = make([]float64, len(weights))
		for i, v := range weights {
			x, e := numeric(v)
			if e != nil || x <= 0 {
				return fail()
			}
			w[i] = x
		}
	}
	g, e := geom.NewBSpline(degree, poles, expanded, w)
	if e != nil {
		return SplineCurve{}, e
	}
	return SplineCurve{label, g, closed, self}, nil
}

// ReadCurves imports independent spline items grouped in GEOMETRIC_CURVE_SET
// under GEOMETRICALLY_BOUNDED_WIREFRAME_SHAPE_REPRESENTATION. MaxVertices limits
// total decoded control points. Assemblies, trimming and non-spline items fail.
func ReadCurves(r io.Reader, l Limits) (CurveModel, error) {
	l, e := l.defaults()
	if e != nil {
		return CurveModel{}, e
	}
	f, e := Parse(r, l)
	if e != nil {
		return CurveModel{}, e
	}
	m := CurveModel{}
	used := map[int64]bool{}
	count := 0
	for _, id := range f.Order {
		ent := f.Entities[id]
		for _, rec := range ent.Records {
			if strings.Contains(rec.Type, "TRANSFORMATION") || strings.Contains(rec.Type, "REPRESENTATION_RELATIONSHIP") || rec.Type == "MAPPED_ITEM" || rec.Type == "REPRESENTATION_MAP" {
				return m, errors.New("step: curve placement/assembly unsupported")
			}
		}
		if len(ent.Records) != 1 {
			for _, r := range ent.Records {
				if strings.HasSuffix(r.Type, "SHAPE_REPRESENTATION") {
					return m, errors.New("step: complex wireframe representation unsupported")
				}
			}
			continue
		}
		kind := ent.Records[0].Type
		if strings.HasSuffix(kind, "SHAPE_REPRESENTATION") && kind != "GEOMETRICALLY_BOUNDED_WIREFRAME_SHAPE_REPRESENTATION" {
			return m, errors.New("step: unsupported curve representation")
		}
		if kind != "GEOMETRICALLY_BOUNDED_WIREFRAME_SHAPE_REPRESENTATION" {
			continue
		}
		rep, e := simple(f, id, kind, 3)
		if e != nil {
			return m, e
		}
		ctx, e := ref(rep.Args[2])
		if e != nil {
			return m, e
		}
		unit, e := contextUnits(f, ctx)
		if e != nil {
			return m, e
		}
		if m.LengthUnit != 0 && m.LengthUnit != unit {
			return m, errors.New("step: mixed curve units")
		}
		m.LengthUnit = unit
		items, e := listRefs(rep.Args[1], 1)
		if e != nil {
			return m, e
		}
		for _, item := range items {
			if x := f.Entities[item]; len(x.Records) == 1 && x.Records[0].Type == "AXIS2_PLACEMENT_3D" {
				if _, e = placement(f, item); e != nil {
					return m, e
				}
				continue
			}
			set, e := simple(f, item, "GEOMETRIC_CURVE_SET", 2)
			if e != nil {
				return m, e
			}
			ids, e := listRefs(set.Args[1], 1)
			if e != nil {
				return m, e
			}
			for _, cid := range ids {
				if used[cid] {
					return m, errors.New("step: repeated curve instance")
				}
				c, basis, e := fullDomainCurve(f, cid, l.MaxVertices-count)
				if e != nil {
					return m, e
				}
				if basis != cid && used[basis] {
					return CurveModel{}, errors.New("step: repeated spline basis")
				}
				count += len(c.Geometry.Poles())
				used[cid] = true
				used[basis] = true
				m.Curves = append(m.Curves, c)
			}
		}
	}
	for id, ent := range f.Entities {
		for _, r := range ent.Records {
			if r.Type == "B_SPLINE_CURVE_WITH_KNOTS" && !used[id] {
				return CurveModel{}, errors.New("step: unrepresented spline curve")
			}
		}
	}
	if len(m.Curves) == 0 {
		return m, errors.New("step: no supported curves")
	}
	return m, nil
}

// fullDomainCurve accepts the common full-domain TRIMMED_CURVE wrapper only;
// actual clipping, reverse sense and Cartesian-only trimming remain unsupported.
func fullDomainCurve(f *File, id int64, maxPoints int) (SplineCurve, int64, error) {
	ent := f.Entities[id]
	if len(ent.Records) != 1 || ent.Records[0].Type != "TRIMMED_CURVE" {
		c, e := DecodeBSplineCurve(f, id, maxPoints)
		return c, id, e
	}
	r, e := simple(f, id, "TRIMMED_CURVE", 6)
	if e != nil {
		return SplineCurve{}, 0, e
	}
	fail := func() (SplineCurve, int64, error) {
		return SplineCurve{}, 0, errors.New("step: only forward full-domain parameter spline trims supported")
	}
	name, e := textValue(r.Args[0])
	if e != nil {
		return fail()
	}
	basis, e := ref(r.Args[1])
	if e != nil {
		return fail()
	}
	sense, e := boolean(r.Args[4])
	if e != nil || !sense || r.Args[5].Kind != Enumeration || (r.Args[5].Text != "PARAMETER" && r.Args[5].Text != "UNSPECIFIED") {
		return fail()
	}
	c, e := DecodeBSplineCurve(f, basis, maxPoints)
	if e != nil {
		return SplineCurve{}, 0, e
	}
	a, b := c.Geometry.Domain()
	poles := c.Geometry.Poles()
	scale := 0.0
	for _, p := range poles {
		scale = math.Max(scale, p.Distance(poles[0]))
	}
	if !finiteNumber(scale) {
		return fail()
	}
	for i, want := range []float64{a, b} {
		v := r.Args[2+i]
		if v.Kind != Aggregate || len(v.Items) < 1 || len(v.Items) > 2 {
			return fail()
		}
		haveParam, havePoint := false, false
		for _, x := range v.Items {
			switch x.Kind {
			case Typed:
				if haveParam || x.Text != "PARAMETER_VALUE" || len(x.Items) != 1 {
					return fail()
				}
				t, e := numeric(x.Items[0])
				if e != nil || t != want {
					return fail()
				}
				haveParam = true
			case Reference:
				if havePoint {
					return fail()
				}
				p, e := coordinate(f, x.Ref, "CARTESIAN_POINT", 3)
				if e != nil {
					return fail()
				}
				q, e := c.Geometry.Eval(want)
				if e != nil || p.Distance(q) > 1e-9*scale {
					return fail()
				}
				havePoint = true
			default:
				return fail()
			}
		}
		if !haveParam {
			return fail()
		}
	}
	c.Name = name
	return c, basis, nil
}
