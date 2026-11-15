package step

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math/big"
	"plainsolid/geom"
)

// nonplanarCavityGenerated authors the two proved original carrier sets, then
// changes only the newly generated unit-domain UV LINE representation to an
// equivalent bounded polynomial spline. This is not an external LINE importer.
func nonplanarCavityGenerated(ctx context.Context, m NonplanarSplineCavitySource) ([]byte, error) {
	if ctx == nil {
		return nil, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	if len(m.Outer.Faces) != 6 || len(m.Inner.Faces) != 6 {
		return nil, geom.ErrInvalid
	}
	outer, inner := m.Outer, m.Inner
	outer.Name = m.Name
	inner.Name = ""
	var raw bytes.Buffer
	if e := WriteSplineShells(&raw, SplineShellModel{Bodies: []SplineShellBody{outer, inner}, LengthUnit: m.LengthUnit}); e != nil {
		return nil, e
	}
	f, e := Parse(&raw, Limits{})
	if e != nil {
		return nil, e
	}
	// The generic writer emits unit-weight complex splines. This bounded
	// authoring path emits their exactly identical polynomial entity form.
	for id, en := range f.Entities {
		records := map[string]Record{}
		for _, r := range en.Records {
			records[r.Type] = r
		}
		for _, kind := range []string{"CURVE", "SURFACE"} {
			w, ok := records["RATIONAL_B_SPLINE_"+kind]
			if !ok {
				continue
			}
			var ones func(Value) bool
			ones = func(v Value) bool {
				if v.Kind == Aggregate {
					for _, x := range v.Items {
						if !ones(x) {
							return false
						}
					}
					return true
				}
				q, e := affineSplineSourceRat(v)
				return e == nil && q.Cmp(big.NewRat(1, 1)) == 0
			}
			if len(w.Args) != 1 || !ones(w.Args[0]) {
				return nil, geom.ErrInvalid
			}
			b, k, name := records["B_SPLINE_"+kind], records["B_SPLINE_"+kind+"_WITH_KNOTS"], records["REPRESENTATION_ITEM"]
			if len(name.Args) != 1 {
				return nil, geom.ErrInvalid
			}
			args := []Value{name.Args[0]}
			args = append(args, b.Args...)
			args = append(args, k.Args...)
			en.Records = []Record{{Type: "B_SPLINE_" + kind + "_WITH_KNOTS", Args: args}}
		}
		if len(en.Records) == 1 {
			r := en.Records[0]
			switch r.Type {
			case "MECHANICAL_CONTEXT":
				r.Type = "PRODUCT_CONTEXT"
			case "DESIGN_CONTEXT":
				r.Type = "PRODUCT_DEFINITION_CONTEXT"
			case "PRODUCT_DEFINITION_FORMATION_WITH_SPECIFIED_SOURCE":
				if len(r.Args) != 4 || r.Args[3].Kind != Enumeration || r.Args[3].Text != "NOT_KNOWN" {
					return nil, geom.ErrInvalid
				}
				r.Type = "PRODUCT_DEFINITION_FORMATION"
				r.Args = r.Args[:3]
			}
			en.Records[0] = r
		}
		f.Entities[id] = en
	}
	incoming := map[int64]int{}
	var walk func(Value)
	walk = func(v Value) {
		if v.Kind == Reference {
			incoming[v.Ref]++
		}
		for _, x := range v.Items {
			walk(x)
		}
	}
	next := int64(1)
	for id, en := range f.Entities {
		next = max(next, id+1)
		for _, rec := range en.Records {
			for _, v := range rec.Args {
				walk(v)
			}
		}
	}
	num := func(s string) Value { return Value{Kind: Number, Text: s} }
	refv := func(id int64) Value { return Value{Kind: Reference, Ref: id} }
	count := 0
	for _, id := range append([]int64(nil), f.Order...) {
		line, e := simple(f, id, "LINE", 3)
		if e != nil {
			continue
		}
		if incoming[id] != 1 || line.Args[1].Kind != Reference || line.Args[2].Kind != Reference {
			return nil, geom.ErrInvalid
		}
		pid, vid := line.Args[1].Ref, line.Args[2].Ref
		vector, e := simple(f, vid, "VECTOR", 3)
		if e != nil || vector.Args[1].Kind != Reference {
			return nil, geom.ErrInvalid
		}
		did := vector.Args[1].Ref
		if incoming[pid] != 1 || incoming[vid] != 1 || incoming[did] != 1 {
			return nil, fmt.Errorf("generated UV ownership %d: point=%d vector=%d direction=%d: %w", id, incoming[pid], incoming[vid], incoming[did], geom.ErrInvalid)
		}
		p, d, e := lineGeometry(f, id, 2)
		if e != nil {
			return nil, e
		}
		if (p.X != 0 && p.X != 1) || (p.Y != 0 && p.Y != 1) || !((d.X == 1 && d.Y == 0 && p.X == 0) || (d.X == 0 && d.Y == 1 && p.Y == 0)) {
			return nil, geom.ErrInvalid
		}
		end := next
		next++
		f.Entities[end] = Entity{ID: end, Records: []Record{{Type: "CARTESIAN_POINT", Args: []Value{{Kind: String}, {Kind: Aggregate, Items: []Value{num(real(p.X + d.X)), num(real(p.Y + d.Y))}}}}}}
		f.Order = append(f.Order, end)
		f.Entities[id] = Entity{ID: id, Records: []Record{{Type: "B_SPLINE_CURVE_WITH_KNOTS", Args: []Value{line.Args[0], num("1"), {Kind: Aggregate, Items: []Value{refv(pid), refv(end)}}, {Kind: Enumeration, Text: "UNSPECIFIED"}, {Kind: Enumeration, Text: "F"}, {Kind: Enumeration, Text: "F"}, {Kind: Aggregate, Items: []Value{num("2"), num("2")}}, {Kind: Aggregate, Items: []Value{num("0"), num("1")}}, {Kind: Enumeration, Text: "PIECEWISE_BEZIER_KNOTS"}}}}}
		delete(f.Entities, vid)
		delete(f.Entities, did)
		count++
	}
	if count != 48 {
		return nil, fmt.Errorf("generated UV count %d: %w", count, geom.ErrInvalid)
	}
	out, e := joinWrappedVoid(f)
	if e != nil {
		return nil, e
	}
	if _, e = ReadNonplanarSplineCavitySource(ctx, bytes.NewReader(out), Limits{}); e != nil {
		return nil, fmt.Errorf("generated cavity reproof: %w", e)
	}
	return out, nil
}

// WriteNonplanarSplineCavity emits generated standard BREP_WITH_VOIDS geometry
// after re-proving the supplied source-bound material and every native claim.
func WriteNonplanarSplineCavity(ctx context.Context, w io.Writer, m NonplanarSplineCavitySource, l Limits) error {
	if w == nil {
		return geom.ErrInvalid
	}
	q, e := checkedNonplanarSplineCavity(ctx, m, l)
	if e != nil {
		return e
	}
	n, e := io.WriteString(w, q.Generated)
	if e == nil && n != len(q.Generated) {
		e = io.ErrShortWrite
	}
	return e
}
