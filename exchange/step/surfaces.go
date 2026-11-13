package step

import (
	"errors"
	"io"
	"plainsolid/geom"
	"strings"
)

// SplineSurface is an independent full-domain tensor-product patch. Logical
// flags are informational and never establish closure or shell topology.
type SplineSurface struct {
	Name                            string
	Geometry                        *geom.BSplineSurface
	ClosedU, ClosedV, SelfIntersect string
}

// SurfaceModel exchanges independent surface geometry in raw SI-scaled units.
type SurfaceModel struct {
	Surfaces   []SplineSurface
	LengthUnit float64
}

func expandSurfaceKnots(m, k Value, degree, count int) ([]float64, error) {
	fail := func() ([]float64, error) { return nil, errors.New("step: invalid surface knots") }
	if m.Kind != Aggregate || k.Kind != Aggregate || len(m.Items) < 2 || len(m.Items) != len(k.Items) {
		return fail()
	}
	expected := count + degree + 1
	out := make([]float64, 0, expected)
	previous := 0.0
	for i, v := range m.Items {
		n, e := integer(v)
		bound := degree
		if i == 0 || i == len(m.Items)-1 {
			bound++
		}
		if e != nil || n < 1 || n > bound || n > expected-len(out) {
			return fail()
		}
		x, e := numeric(k.Items[i])
		if e != nil || (i > 0 && x <= previous) {
			return fail()
		}
		previous = x
		for j := 0; j < n; j++ {
			out = append(out, x)
		}
	}
	if len(out) != expected {
		return fail()
	}
	return out, nil
}

// DecodeBSplineSurface decodes polynomial or rational explicit-knot STEP
// geometry. maxPoints limits the entire control net before allocation.
func DecodeBSplineSurface(f *File, id int64, maxPoints int) (SplineSurface, error) {
	fail := func() (SplineSurface, error) {
		return SplineSurface{}, errors.New("step: unsupported or malformed B-spline surface")
	}
	if f == nil || maxPoints < 4 {
		return fail()
	}
	ent, ok := f.Entities[id]
	if !ok {
		return fail()
	}
	var name Value
	var base, knots, weights []Value
	rational := false
	if len(ent.Records) == 1 && ent.Records[0].Type == "B_SPLINE_SURFACE_WITH_KNOTS" {
		a := ent.Records[0].Args
		if len(a) != 13 {
			return fail()
		}
		name = a[0]
		base = a[1:8]
		knots = a[8:]
	} else {
		seen := map[string]bool{}
		for _, r := range ent.Records {
			if seen[r.Type] {
				return fail()
			}
			seen[r.Type] = true
			switch r.Type {
			case "BOUNDED_SURFACE", "SURFACE", "GEOMETRIC_REPRESENTATION_ITEM":
				if len(r.Args) != 0 {
					return fail()
				}
			case "REPRESENTATION_ITEM":
				if len(r.Args) != 1 {
					return fail()
				}
				name = r.Args[0]
			case "B_SPLINE_SURFACE":
				base = r.Args
			case "B_SPLINE_SURFACE_WITH_KNOTS":
				knots = r.Args
			case "RATIONAL_B_SPLINE_SURFACE":
				rational = true
				if len(r.Args) != 1 || r.Args[0].Kind != Aggregate {
					return fail()
				}
				weights = r.Args[0].Items
			default:
				return fail()
			}
		}
		for _, k := range []string{"BOUNDED_SURFACE", "SURFACE", "GEOMETRIC_REPRESENTATION_ITEM", "REPRESENTATION_ITEM", "B_SPLINE_SURFACE", "B_SPLINE_SURFACE_WITH_KNOTS", "RATIONAL_B_SPLINE_SURFACE"} {
			if !seen[k] {
				return fail()
			}
		}
	}
	if len(base) != 7 || len(knots) != 5 {
		return fail()
	}
	label, e := textValue(name)
	if e != nil {
		return fail()
	}
	du, e := integer(base[0])
	if e != nil || du < 1 || du >= maxPoints {
		return fail()
	}
	dv, e := integer(base[1])
	if e != nil || dv < 1 || dv >= maxPoints {
		return fail()
	}
	if base[2].Kind != Aggregate || len(base[2].Items) < du+1 || len(base[2].Items) > maxPoints {
		return fail()
	}
	rows := base[2].Items
	nu := len(rows)
	if rows[0].Kind != Aggregate {
		return fail()
	}
	nv := len(rows[0].Items)
	if nv < dv+1 || nv > maxPoints || nu > maxPoints/nv {
		return fail()
	}
	if base[3].Kind != Enumeration {
		return fail()
	}
	switch base[3].Text {
	case "PLANE_SURF", "CYLINDRICAL_SURF", "CONICAL_SURF", "SPHERICAL_SURF", "TOROIDAL_SURF", "SURF_OF_REVOLUTION", "RULED_SURF", "GENERALISED_CONE", "QUADRIC_SURF", "SURF_OF_LINEAR_EXTRUSION", "UNSPECIFIED":
	default:
		return fail()
	}
	cu, e := logical(base[4])
	if e != nil {
		return fail()
	}
	cv, e := logical(base[5])
	if e != nil {
		return fail()
	}
	self, e := logical(base[6])
	if e != nil {
		return fail()
	}
	poles := make([]geom.Vec3, nu*nv)
	for u, row := range rows {
		if row.Kind != Aggregate || len(row.Items) != nv {
			return fail()
		}
		for v, x := range row.Items {
			pid, e := ref(x)
			if e != nil {
				return fail()
			}
			p, e := coordinate(f, pid, "CARTESIAN_POINT", 3)
			if e != nil {
				return SplineSurface{}, e
			}
			poles[u*nv+v] = p
		}
	}
	if knots[4].Kind != Enumeration {
		return fail()
	}
	switch knots[4].Text {
	case "UNIFORM_KNOTS", "QUASI_UNIFORM_KNOTS", "PIECEWISE_BEZIER_KNOTS", "UNSPECIFIED":
	default:
		return fail()
	}
	ku, e := expandSurfaceKnots(knots[0], knots[2], du, nu)
	if e != nil {
		return fail()
	}
	kv, e := expandSurfaceKnots(knots[1], knots[3], dv, nv)
	if e != nil {
		return fail()
	}
	var w []float64
	if rational {
		if len(weights) != nu {
			return fail()
		}
		w = make([]float64, len(poles))
		for u, row := range weights {
			if row.Kind != Aggregate || len(row.Items) != nv {
				return fail()
			}
			for v, x := range row.Items {
				value, e := numeric(x)
				if e != nil || value <= 0 {
					return fail()
				}
				w[u*nv+v] = value
			}
		}
	}
	g, e := geom.NewBSplineSurface(du, dv, nu, nv, poles, ku, kv, w)
	if e != nil {
		return SplineSurface{}, e
	}
	return SplineSurface{label, g, cu, cv, self}, nil
}

// ReadSurfaces imports full-domain explicit-knot surface patches from a
// GEOMETRIC_SET and bounded surface shape representation. MaxVertices limits
// total control-net points. Face trimming, topology and assemblies fail.
func ReadSurfaces(r io.Reader, l Limits) (SurfaceModel, error) {
	l, e := l.defaults()
	if e != nil {
		return SurfaceModel{}, e
	}
	f, e := Parse(r, l)
	if e != nil {
		return SurfaceModel{}, e
	}
	return readSurfacesFile(f, l)
}

func readSurfacesFile(f *File, l Limits) (SurfaceModel, error) {
	m := SurfaceModel{}
	used := map[int64]bool{}
	count := 0
	for _, id := range f.Order {
		ent := f.Entities[id]
		for _, rec := range ent.Records {
			if strings.Contains(rec.Type, "TRANSFORMATION") || strings.Contains(rec.Type, "REPRESENTATION_RELATIONSHIP") || rec.Type == "MAPPED_ITEM" || rec.Type == "REPRESENTATION_MAP" {
				return m, errors.New("step: surface placement/assembly unsupported")
			}
		}
		if len(ent.Records) != 1 {
			for _, r := range ent.Records {
				if strings.HasSuffix(r.Type, "SHAPE_REPRESENTATION") {
					return m, errors.New("step: complex surface representation unsupported")
				}
			}
			continue
		}
		kind := ent.Records[0].Type
		if strings.HasSuffix(kind, "SHAPE_REPRESENTATION") && kind != "GEOMETRICALLY_BOUNDED_SURFACE_SHAPE_REPRESENTATION" {
			return m, errors.New("step: unsupported surface representation")
		}
		if kind != "GEOMETRICALLY_BOUNDED_SURFACE_SHAPE_REPRESENTATION" {
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
			return m, errors.New("step: mixed surface units")
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
			set, e := simple(f, item, "GEOMETRIC_SET", 2)
			if e != nil {
				return m, e
			}
			ids, e := listRefs(set.Args[1], 1)
			if e != nil {
				return m, e
			}
			for _, sid := range ids {
				if used[sid] {
					return m, errors.New("step: repeated surface instance")
				}
				s, e := DecodeBSplineSurface(f, sid, l.MaxVertices-count)
				if e != nil {
					return m, e
				}
				count += len(s.Geometry.Data().Poles)
				used[sid] = true
				m.Surfaces = append(m.Surfaces, s)
			}
		}
	}
	for id, ent := range f.Entities {
		for _, r := range ent.Records {
			if r.Type == "B_SPLINE_SURFACE_WITH_KNOTS" && !used[id] {
				return m, errors.New("step: unrepresented spline surface")
			}
		}
	}
	if len(m.Surfaces) == 0 {
		return m, errors.New("step: no supported surfaces")
	}
	return m, nil
}
