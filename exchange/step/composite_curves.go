package step

import (
	"bytes"
	"fmt"
	"io"
	"math/big"
	"plainsolid/geom"
	"strings"
	"unicode/utf8"
)

// CompositeSplineSegment retains a full clamped spline and its directed use.
// Curve logical flags are informational, as in SplineCurve; they prove no
// self-intersection property. SameSense=false traverses the original basis back.
type CompositeSplineSegment struct {
	Curve     SplineCurve
	SameSense bool
}

// CompositeSplineCurve proves positional joins only. Closed additionally proves
// the final-to-first endpoint join; it is not a simple wire or material boundary.
type CompositeSplineCurve struct {
	Name     string
	Segments []CompositeSplineSegment
	Closed   bool
}

// CompositeCurveModel is one rooted bounded wireframe product, without B-rep
// topology or assemblies. All curves own distinct segment and spline records.
type CompositeCurveModel struct {
	Name, RepresentationName, SetName string
	Curves                            []CompositeSplineCurve
	LengthUnit                        float64
}

func compositeFailure() error { return fmt.Errorf("step: unsupported exact composite spline graph") }
func validateCompositeCurve(c CompositeSplineCurve) error {
	if len(c.Segments) < 1 || len(c.Segments) > 64 || !utf8.ValidString(c.Name) || len(c.Name) > 1024 {
		return compositeFailure()
	}
	var first, previous geom.Vec3
	for i, s := range c.Segments {
		if s.Curve.Geometry == nil || !utf8.ValidString(s.Curve.Name) || len(s.Curve.Name) > 1024 {
			return compositeFailure()
		}
		g := s.Curve.Geometry
		p := g.Poles()
		k := g.Knots()
		degree := g.Degree()
		a, b := g.Domain()
		if len(p) < 2 || degree > 16 || len(p) > 4096 || k[0] != a || k[len(k)-1] != b {
			return compositeFailure()
		}
		for j := 0; j <= degree; j++ {
			if k[j] != a || k[len(k)-1-j] != b {
				return compositeFailure()
			}
		}
		constant := true
		for _, v := range p[1:] {
			if v != p[0] {
				constant = false
				break
			}
		}
		if constant {
			return compositeFailure()
		}
		start, end := p[0], p[len(p)-1]
		if !s.SameSense {
			start, end = end, start
		}
		if i == 0 {
			first = start
		} else if previous != start {
			return compositeFailure()
		}
		previous = end
	}
	if c.Closed {
		if previous != first {
			return compositeFailure()
		}
	} else if previous == first {
		return compositeFailure()
	}
	return nil
}

// ReadCompositeCurves accepts direct full-domain spline segments only. Nesting,
// sharing, gaps, partial trims and stronger transition assertions are rejected.
// Original geometric decimals and normalization must preserve exact coefficients.
func ReadCompositeCurves(r io.Reader, l Limits) (CompositeCurveModel, error) {
	return readCompositeCurves(r, l, nil)
}
func readCompositeCurves(r io.Reader, l Limits, tangents *[][]bool, geometric ...bool) (CompositeCurveModel, error) {
	fail := func() (CompositeCurveModel, error) { return CompositeCurveModel{}, compositeFailure() }
	l, e := l.defaults()
	if e != nil {
		return CompositeCurveModel{}, e
	}
	l.MaxBytes = min(l.MaxBytes, 1<<20)
	l.MaxEntities = min(l.MaxEntities, 8192)
	l.MaxVertices = min(l.MaxVertices, 4096)
	f, e := Parse(r, l)
	if e != nil {
		return CompositeCurveModel{}, e
	}
	var repID int64
	for id, en := range f.Entities {
		if len(en.Records) == 1 && en.Records[0].Type == "GEOMETRICALLY_BOUNDED_WIREFRAME_SHAPE_REPRESENTATION" {
			if repID != 0 {
				return fail()
			}
			repID = id
		}
	}
	rep, e := simple(f, repID, "GEOMETRICALLY_BOUNDED_WIREFRAME_SHAPE_REPRESENTATION", 3)
	if e != nil {
		return fail()
	}
	repName, e := textValue(rep.Args[0])
	if e != nil {
		return fail()
	}
	items, e := listRefs(rep.Args[1], 1)
	if e != nil || len(items) != 1 {
		return fail()
	}
	set, e := simple(f, items[0], "GEOMETRIC_CURVE_SET", 2)
	if e != nil {
		return fail()
	}
	setName, e := textValue(set.Args[0])
	if e != nil {
		return fail()
	}
	roots, e := listRefs(set.Args[1], 1)
	if e != nil || len(roots) > 64 {
		return fail()
	}
	ctx, e := ref(rep.Args[2])
	if e != nil {
		return fail()
	}
	rc, ok := component(f.Entities[ctx], "REPRESENTATION_CONTEXT")
	if !ok || len(rc.Args) != 2 || rc.Args[0].Kind != String || rc.Args[1].Kind != String || rc.Args[0].Text != "" || rc.Args[1].Text != "" {
		return fail()
	}
	unit, e := analyticSupportContextUnits(f, ctx)
	if e != nil {
		return fail()
	}
	used := map[int64]bool{repID: true, items[0]: true}
	identity := map[int64]int64{}
	ids := make([]int64, len(splitProductKinds))
	for id, en := range f.Entities {
		identity[id] = id
		for _, rec := range en.Records {
			for i, k := range splitProductKinds {
				if rec.Type == k {
					if len(en.Records) != 1 || ids[i] != 0 {
						return fail()
					}
					ids[i] = id
				}
			}
		}
	}
	for _, id := range ids {
		if id == 0 {
			return fail()
		}
		used[id] = true
	}
	product, e := simple(f, ids[4], "PRODUCT", 4)
	if e != nil {
		return fail()
	}
	name, e := textValue(product.Args[1])
	if e != nil {
		return fail()
	}
	for i, want := range splitProductRecords(ids, repID, name, 2) {
		want = strings.Replace(want, "PRODUCT('split-cylinder',", "PRODUCT('1',", 1)
		if assemblyEntity(f.Entities[ids[i]], identity) != want {
			return fail()
		}
	}
	allowed := map[string]bool{"GEOMETRIC_REPRESENTATION_CONTEXT": true, "GLOBAL_UNIT_ASSIGNED_CONTEXT": true, "REPRESENTATION_CONTEXT": true, "LENGTH_UNIT": true, "NAMED_UNIT": true, "SI_UNIT": true, "PLANE_ANGLE_UNIT": true, "SOLID_ANGLE_UNIT": true}
	active := map[int64]bool{}
	var markContext func(int64) error
	markContext = func(id int64) error {
		if active[id] {
			return compositeFailure()
		}
		if used[id] {
			return nil
		}
		en, ok := f.Entities[id]
		if !ok {
			return compositeFailure()
		}
		active[id] = true
		var walk func(Value) error
		walk = func(v Value) error {
			if v.Kind == Reference {
				return markContext(v.Ref)
			}
			for _, x := range v.Items {
				if e := walk(x); e != nil {
					return e
				}
			}
			return nil
		}
		for _, rec := range en.Records {
			if !allowed[rec.Type] {
				return compositeFailure()
			}
			for _, v := range rec.Args {
				if e := walk(v); e != nil {
					return e
				}
			}
		}
		active[id] = false
		used[id] = true
		return nil
	}
	if e = markContext(ctx); e != nil {
		return fail()
	}
	for _, s := range []string{name, repName, setName} {
		if !utf8.ValidString(s) || len(s) > 1024 {
			return fail()
		}
	}
	out := CompositeCurveModel{Name: name, RepresentationName: repName, SetName: setName, LengthUnit: unit}
	count := 0
	var allTangents [][]bool
	take := func(id int64, kind string, n int) (Record, error) {
		if used[id] {
			return Record{}, compositeFailure()
		}
		r, e := simple(f, id, kind, n)
		if e == nil {
			used[id] = true
		}
		return r, e
	}
	for _, root := range roots {
		rec, e := take(root, "COMPOSITE_CURVE", 3)
		if e != nil {
			return fail()
		}
		cn, e := textValue(rec.Args[0])
		if e != nil {
			return fail()
		}
		self, e := logical(rec.Args[2])
		if e != nil || self != "U" {
			return fail()
		}
		segments, e := listRefs(rec.Args[1], 1)
		if e != nil || len(segments) > 64 {
			return fail()
		}
		c := CompositeSplineCurve{Name: cn}
		flags := make([]bool, len(segments))
		for i, sid := range segments {
			seg, e := take(sid, "COMPOSITE_CURVE_SEGMENT", 3)
			if e != nil {
				return fail()
			}
			transition := seg.Args[0]
			if transition.Kind != Enumeration {
				return fail()
			}
			if transition.Text == "CONT_SAME_GRADIENT" {
				if tangents == nil {
					return fail()
				}
				flags[i] = true
				transition.Text = "CONTINUOUS"
			}
			if i < len(segments)-1 && transition.Text != "CONTINUOUS" {
				return fail()
			}
			if i == len(segments)-1 {
				if transition.Text != "CONTINUOUS" && transition.Text != "DISCONTINUOUS" {
					return fail()
				}
				c.Closed = transition.Text == "CONTINUOUS"
			}
			sense, e := boolean(seg.Args[1])
			if e != nil {
				return fail()
			}
			cid, e := ref(seg.Args[2])
			if e != nil || used[cid] {
				return fail()
			}
			used[cid] = true
			for _, rr := range f.Entities[cid].Records {
				if rr.Type == "B_SPLINE_CURVE" && (len(rr.Args) != 5 || rr.Args[2].Kind != Enumeration || rr.Args[2].Text != "UNSPECIFIED") {
					return fail()
				}
				if rr.Type == "B_SPLINE_CURVE_WITH_KNOTS" {
					if len(rr.Args) != 3 && len(rr.Args) != 9 {
						return fail()
					}
					if len(rr.Args) == 9 && (rr.Args[3].Kind != Enumeration || rr.Args[3].Text != "UNSPECIFIED") {
						return fail()
					}
					v := rr.Args[len(rr.Args)-1]
					if v.Kind != Enumeration || v.Text != "UNSPECIFIED" {
						return fail()
					}
				}
			}
			curve, e := DecodeBSplineCurve(f, cid, l.MaxVertices-count)
			if e != nil {
				return fail()
			}
			if e = exactSplineWeightNormalization(f, cid, "RATIONAL_B_SPLINE_CURVE"); e != nil {
				return fail()
			}
			var walk func(Value) error
			walk = func(v Value) error {
				if v.Kind == Number {
					x, e := cylinderSupportNumber(v)
					if e != nil {
						return e
					}
					_, e = extrusionDirectionDecimal(new(big.Rat).SetFloat64(x))
					return e
				}
				if v.Kind == Reference {
					p, e := take(v.Ref, "CARTESIAN_POINT", 2)
					if e != nil {
						return e
					}
					pn, e := textValue(p.Args[0])
					if e != nil || pn != "" {
						return compositeFailure()
					}
					return walk(p.Args[1])
				}
				for _, x := range v.Items {
					if e := walk(x); e != nil {
						return e
					}
				}
				return nil
			}
			for _, rr := range f.Entities[cid].Records {
				for _, v := range rr.Args {
					if e = walk(v); e != nil {
						return fail()
					}
				}
			}
			count += len(curve.Geometry.Poles())
			c.Segments = append(c.Segments, CompositeSplineSegment{curve, sense})
		}
		if e = validateCompositeCurve(c); e != nil {
			return fail()
		}
		if tangents != nil {
			if e = validateCompositeTangents(c, flags, geometric...); e != nil {
				return fail()
			}
			allTangents = append(allTangents, flags)
		}
		out.Curves = append(out.Curves, c)
	}
	if len(used) != len(f.Entities) {
		return fail()
	}
	if tangents != nil {
		*tangents = allTangents
	}
	return out, nil
}

// WriteCompositeCurves stages the rooted graph and exact typed readback before
// output. Nonrepresentable source decimals fail rather than round the basis.
func WriteCompositeCurves(w io.Writer, m CompositeCurveModel) error {
	return writeCompositeCurves(w, m, nil)
}
func writeCompositeCurves(w io.Writer, m CompositeCurveModel, tangents [][]bool, geometric ...bool) error {
	if w == nil || len(m.Curves) < 1 || len(m.Curves) > 64 {
		return compositeFailure()
	}
	for _, s := range []string{m.Name, m.RepresentationName, m.SetName} {
		if !utf8.ValidString(s) || len(s) > 1024 {
			return compositeFailure()
		}
	}
	if tangents != nil {
		if len(tangents) != len(m.Curves) {
			return compositeFailure()
		}
	}
	flat := CurveModel{LengthUnit: m.LengthUnit}
	poles := 0
	for _, c := range m.Curves {
		if e := validateCompositeCurve(c); e != nil {
			return e
		}
		for _, s := range c.Segments {
			poles += len(s.Curve.Geometry.Poles())
			if poles > 4096 {
				return compositeFailure()
			}
			flat.Curves = append(flat.Curves, s.Curve)
		}
	}
	if len(flat.Curves) > 4096 {
		return compositeFailure()
	}
	if tangents != nil {
		for i, c := range m.Curves {
			if e := validateCompositeTangents(c, tangents[i], geometric...); e != nil {
				return e
			}
		}
	}
	var initial bytes.Buffer
	if e := WriteCurves(&initial, flat); e != nil {
		return e
	}
	f, e := Parse(&initial, Limits{MaxBytes: 1 << 20, MaxEntities: 8192})
	if e != nil {
		return e
	}
	var max, setID int64
	for id, en := range f.Entities {
		if id > max {
			max = id
		}
		if len(en.Records) == 1 {
			switch en.Records[0].Type {
			case "GEOMETRIC_CURVE_SET":
				setID = id
				en.Records[0].Args[0].Text = m.SetName
			case "GEOMETRICALLY_BOUNDED_WIREFRAME_SHAPE_REPRESENTATION":
				en.Records[0].Args[0].Text = m.RepresentationName
			case "PRODUCT":
				en.Records[0].Args[1].Text = m.Name
			}
		}
	}
	leaves, e := listRefs(f.Entities[setID].Records[0].Args[1], 1)
	if e != nil {
		return e
	}
	at := 0
	var roots []Value
	add := func(r Record) int64 {
		max++
		f.Entities[max] = Entity{ID: max, Records: []Record{r}}
		f.Order = append(f.Order, max)
		return max
	}
	for ci, c := range m.Curves {
		var segs []Value
		for i, s := range c.Segments {
			tr := "CONTINUOUS"
			if i == len(c.Segments)-1 && !c.Closed {
				tr = "DISCONTINUOUS"
			}
			if tangents != nil && tangents[ci][i] {
				tr = "CONT_SAME_GRADIENT"
			}
			sense := "F"
			if s.SameSense {
				sense = "T"
			}
			sid := add(Record{Type: "COMPOSITE_CURVE_SEGMENT", Args: []Value{{Kind: Enumeration, Text: tr}, {Kind: Enumeration, Text: sense}, {Kind: Reference, Ref: leaves[at]}}})
			at++
			segs = append(segs, Value{Kind: Reference, Ref: sid})
		}
		id := add(Record{Type: "COMPOSITE_CURVE", Args: []Value{{Kind: String, Text: c.Name}, {Kind: Aggregate, Items: segs}, {Kind: Enumeration, Text: "U"}}})
		roots = append(roots, Value{Kind: Reference, Ref: id})
	}
	f.Entities[setID].Records[0].Args[1].Items = roots
	// WriteCurves uses shortest roundtrip numbers; replace geometric real spellings
	// with their exact finite-decimal binary64 value before strict re-import.
	var exact func(*Value) error
	exact = func(v *Value) error {
		if v.Kind == Number && strings.ContainsAny(v.Text, ".Ee") {
			x, e := numeric(*v)
			if e != nil {
				return e
			}
			s, e := extrusionDirectionDecimal(new(big.Rat).SetFloat64(x))
			if e != nil {
				return e
			}
			v.Text = s
		}
		for i := range v.Items {
			if e := exact(&v.Items[i]); e != nil {
				return e
			}
		}
		return nil
	}
	for _, en := range f.Entities {
		for _, r := range en.Records {
			for i := range r.Args {
				if e = exact(&r.Args[i]); e != nil {
					return e
				}
			}
		}
	}
	raw := bytes.Replace(splitProductFile(f, nil, nil), []byte("PlainSolid split cylinder product"), []byte("PlainSolid composite spline curves"), 1)
	var decoded [][]bool
	var ptr *[][]bool
	if tangents != nil {
		ptr = &decoded
	}
	if _, e = readCompositeCurves(bytes.NewReader(raw), Limits{}, ptr, geometric...); e != nil {
		return e
	}
	n, e := w.Write(raw)
	if e == nil && n != len(raw) {
		e = io.ErrShortWrite
	}
	return e
}
