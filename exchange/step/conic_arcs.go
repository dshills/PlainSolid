package step

import (
	"errors"
	"fmt"
	"io"
	"math"
	"plainsolid/geom"
	"strings"
	"unicode/utf8"
)

// ConicArc stores native Go parameters: parabola t=y/(2*f), and hyperbola
// t=tanh(STEP_parameter/2). Only the positive hyperbola branch is supported.
type ConicArc struct {
	Name       string
	Geometry   geom.Curve
	Start, End float64
}
type ConicArcModel struct {
	Curves     []ConicArc
	LengthUnit float64
}

// DecodeConicArc accepts a forward TRIMMED_CURVE with finite explicit parameter
// selectors over a 3D PARABOLA or HYPERBOLA. Optional Cartesian selectors must
// agree with the exact endpoint. Reversed or unbounded curves are rejected.
func DecodeConicArc(f *File, id int64) (ConicArc, int64, error) {
	fail := func() (ConicArc, int64, error) { return ConicArc{}, 0, errors.New("step: unsupported open-conic arc") }
	if f == nil {
		return fail()
	}
	r, e := simple(f, id, "TRIMMED_CURVE", 6)
	if e != nil {
		return fail()
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
	ent := f.Entities[basis]
	if len(ent.Records) != 1 {
		return fail()
	}
	br := ent.Records[0]
	if br.Type != "PARABOLA" && br.Type != "HYPERBOLA" {
		return fail()
	}
	n := 3
	if br.Type == "HYPERBOLA" {
		n = 4
	}
	br, e = simple(f, basis, br.Type, n)
	if e != nil {
		return fail()
	}
	if _, e = textValue(br.Args[0]); e != nil {
		return fail()
	}
	pid, e := ref(br.Args[1])
	if e != nil {
		return fail()
	}
	frame, e := placement(f, pid)
	if e != nil {
		return fail()
	}
	a, e := numeric(br.Args[2])
	if e != nil {
		return fail()
	}
	var curve geom.Curve
	convert := func(u float64) float64 { return u / (2 * a) }
	if br.Type == "PARABOLA" {
		curve, e = geom.NewParabola(frame, a)
	} else {
		b, err := numeric(br.Args[3])
		if err != nil {
			return fail()
		}
		curve, e = geom.NewHyperbola(frame, a, b)
		convert = func(u float64) float64 { return math.Tanh(u / 2) }
	}
	if e != nil {
		return fail()
	}
	ends := [2]float64{}
	for i := range ends {
		list := r.Args[2+i]
		if list.Kind != Aggregate || len(list.Items) < 1 || len(list.Items) > 2 {
			return fail()
		}
		found := false
		pointID := int64(0)
		for _, v := range list.Items {
			if v.Kind == Typed && v.Text == "PARAMETER_VALUE" && len(v.Items) == 1 && !found {
				u, e := numeric(v.Items[0])
				if e != nil {
					return fail()
				}
				ends[i] = convert(u)
				found = true
			} else if v.Kind == Reference && pointID == 0 {
				pointID = v.Ref
			} else {
				return fail()
			}
		}
		if !found || !finiteOblique(ends[i]) {
			return fail()
		}
		p, e := curve.Eval(ends[i])
		if e != nil {
			return fail()
		}
		if pointID != 0 {
			q, e := triple(f, pointID, "CARTESIAN_POINT")
			if e != nil || p.Distance(q) > 1e-9*math.Max(1, p.Sub(frame.Origin()).Length()) {
				return fail()
			}
		}
	}
	if ends[0] >= ends[1] {
		return fail()
	}
	return ConicArc{name, curve, ends[0], ends[1]}, basis, nil
}

func WriteConicArcs(w io.Writer, m ConicArcModel) error {
	if w == nil || len(m.Curves) == 0 {
		return errors.New("step: missing conic arc writer/model")
	}
	type spec struct {
		arc        ConicArc
		frame      geom.Frame
		a, b, u, v float64
		kind       string
	}
	specs := make([]spec, len(m.Curves))
	for i, c := range m.Curves {
		if !utf8.ValidString(c.Name) || !finiteOblique(c.Start) || !finiteOblique(c.End) || c.Start >= c.End {
			return errors.New("step: invalid conic arc interval")
		}
		s := spec{arc: c}
		switch g := c.Geometry.(type) {
		case geom.Parabola:
			s.frame, s.a = g.Parameters()
			s.kind = "PARABOLA"
			s.u, s.v = 2*s.a*c.Start, 2*s.a*c.End
		case geom.Hyperbola:
			var branch int
			s.frame, s.a, s.b, branch = g.Parameters()
			if branch != 1 {
				return errors.New("step: negative hyperbola branch unsupported")
			}
			s.kind = "HYPERBOLA"
			s.u, s.v = 2*math.Atanh(c.Start), 2*math.Atanh(c.End)
		default:
			return errors.New("step: expected parabola or positive hyperbola")
		}
		if !finiteOblique(s.u) || !finiteOblique(s.v) || s.u >= s.v {
			return errors.New("step: conic parameter overflow")
		}
		for _, t := range []float64{c.Start, c.End} {
			if _, e := c.Geometry.Eval(t); e != nil {
				return e
			}
		}
		specs[i] = s
	}
	return writeStandalone(w, m.LengthUnit, "GEOMETRIC_CURVE_SET", "GEOMETRICALLY_BOUNDED_WIREFRAME_SHAPE_REPRESENTATION", func(emit func(string) int64) []int64 {
		point := func(p geom.Vec3, kind string) int64 {
			return emit(fmt.Sprintf("%s('',(%s,%s,%s))", kind, real(p.X), real(p.Y), real(p.Z)))
		}
		ids := make([]int64, len(specs))
		for i, s := range specs {
			f := s.frame
			o, z, x := point(f.Origin(), "CARTESIAN_POINT"), point(f.ZAxis(), "DIRECTION"), point(f.XAxis(), "DIRECTION")
			pl := emit(fmt.Sprintf("AXIS2_PLACEMENT_3D('',#%d,#%d,#%d)", o, z, x))
			params := real(s.a)
			if s.kind == "HYPERBOLA" {
				params += "," + real(s.b)
			}
			basis := emit(fmt.Sprintf("%s('',#%d,%s)", s.kind, pl, params))
			ids[i] = emit(fmt.Sprintf("TRIMMED_CURVE(%s,#%d,(PARAMETER_VALUE(%s)),(PARAMETER_VALUE(%s)),.T.,.PARAMETER.)", quote(s.arc.Name), basis, real(s.u), real(s.v)))
		}
		return ids
	})
}

// ReadConicArcs imports bounded open conics in a geometric curve set.
func ReadConicArcs(r io.Reader, l Limits) (ConicArcModel, error) {
	l, e := l.defaults()
	if e != nil {
		return ConicArcModel{}, e
	}
	f, e := Parse(r, l)
	if e != nil {
		return ConicArcModel{}, e
	}
	m := ConicArcModel{}
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
				c, basis, e := DecodeConicArc(f, cid)
				if e != nil {
					return m, e
				}
				if basis != cid && used[basis] {
					return ConicArcModel{}, errors.New("step: repeated conic basis")
				}
				count += 2
				if count > l.MaxVertices {
					return ConicArcModel{}, errors.New("step: conic resource limit")
				}
				used[cid] = true
				used[basis] = true
				m.Curves = append(m.Curves, c)
			}
		}
	}
	for id, ent := range f.Entities {
		for _, r := range ent.Records {
			if (r.Type == "PARABOLA" || r.Type == "HYPERBOLA" || r.Type == "TRIMMED_CURVE") && !used[id] {
				return ConicArcModel{}, errors.New("step: unrepresented conic curve")
			}
		}
	}
	if len(m.Curves) == 0 {
		return m, errors.New("step: no supported curves")
	}
	return m, nil
}
