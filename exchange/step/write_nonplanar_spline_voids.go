package step

import (
	"bytes"
	"context"
	"io"
	"plainsolid/geom"
)

func nonplanarVoidsGenerated(ctx context.Context, m NonplanarSplineVoidsSource) ([]byte, error) {
	if ctx == nil || len(m.Voids) < 2 || len(m.Voids) > 4 || len(m.Outer.Faces) != 6 {
		return nil, geom.ErrInvalid
	}
	outer := m.Outer
	outer.Name = m.Name
	bodies := []SplineShellBody{outer}
	for _, b := range m.Voids {
		if len(b.Faces) != 6 {
			return nil, geom.ErrInvalid
		}
		b.Name = ""
		bodies = append(bodies, b)
	}
	f, e := nonplanarPolynomialShellFile(ctx, bodies, m.LengthUnit)
	if e != nil {
		return nil, e
	}
	var ids []int64
	next := int64(1)
	for _, id := range f.Order {
		next = max(next, id+1)
		if _, e := simple(f, id, "MANIFOLD_SOLID_BREP", 2); e == nil {
			ids = append(ids, id)
		}
	}
	if len(ids) != len(bodies) {
		return nil, geom.ErrInvalid
	}
	root, e := simple(f, ids[0], "MANIFOLD_SOLID_BREP", 2)
	if e != nil {
		return nil, e
	}
	removed := map[int64]bool{}
	var voids []Value
	for _, id := range ids[1:] {
		b, e := simple(f, id, "MANIFOLD_SOLID_BREP", 2)
		if e != nil {
			return nil, e
		}
		f.Entities[next] = Entity{ID: next, Records: []Record{{Type: "ORIENTED_CLOSED_SHELL", Args: []Value{{Kind: String}, {Kind: Derived}, b.Args[1], {Kind: Enumeration, Text: "F"}}}}}
		f.Order = append(f.Order, next)
		voids = append(voids, Value{Kind: Reference, Ref: next})
		next++
		delete(f.Entities, id)
		removed[id] = true
	}
	f.Entities[ids[0]] = Entity{ID: ids[0], Records: []Record{{Type: "BREP_WITH_VOIDS", Args: []Value{root.Args[0], root.Args[1], {Kind: Aggregate, Items: voids}}}}}
	for id, en := range f.Entities {
		if len(en.Records) != 1 || en.Records[0].Type != "ADVANCED_BREP_SHAPE_REPRESENTATION" {
			continue
		}
		r := en.Records[0]
		var items []Value
		for _, v := range r.Args[1].Items {
			if v.Kind == Reference && removed[v.Ref] {
				continue
			}
			items = append(items, v)
		}
		r.Args[1] = Value{Kind: Aggregate, Items: items}
		en.Records[0] = r
		f.Entities[id] = en
	}
	out := serializeWrappedProofFile(f)
	if _, e = ReadNonplanarSplineVoidsSource(ctx, bytes.NewReader(out), Limits{}); e != nil {
		return nil, e
	}
	return out, nil
}

// WriteNonplanarSplineVoids authors all retained source carriers into one
// re-proved standard BREP_WITH_VOIDS, without flattening or merging void shells.
func WriteNonplanarSplineVoids(ctx context.Context, w io.Writer, m NonplanarSplineVoidsSource, l Limits) error {
	if w == nil {
		return geom.ErrInvalid
	}
	q, e := checkedNonplanarSplineVoids(ctx, m, l)
	if e != nil {
		return e
	}
	n, e := io.WriteString(w, q.Generated)
	if e == nil && n != len(q.Generated) {
		e = io.ErrShortWrite
	}
	return e
}
