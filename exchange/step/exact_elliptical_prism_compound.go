package step

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"plainsolid/geom"
)

// ExactEllipticalPrismCompound is a standard multi-root STEP material file.
// Its two independent products retain original model coordinates and source IDs.
// It does not infer assembly occurrences, Boolean union or spatial disjointness.
type ExactEllipticalPrismCompound struct {
	Members    [2]ExactEllipticalPrismFamily
	Source     *File
	LengthUnit float64
}

// ReadExactEllipticalPrismCompound accepts exactly two complete, dependency-
// disjoint rooted products. Only the exact neutral representation frame may alias across roots.
// Model geometry, contexts, units and product metadata remain independent. Each member passes the full
// existing exact family proof; unsupported inputs return an entirely empty result.
func ReadExactEllipticalPrismCompound(ctx context.Context, r io.Reader, l Limits, tol float64) (ExactEllipticalPrismCompound, error) {
	bad := func() (ExactEllipticalPrismCompound, error) {
		return ExactEllipticalPrismCompound{}, fmt.Errorf("step: unsupported independent elliptical prism compound")
	}
	if ctx == nil || r == nil || !finiteNumber(tol) || tol <= 0 {
		return bad()
	}
	if e := ctx.Err(); e != nil {
		return ExactEllipticalPrismCompound{}, e
	}
	l, e := l.defaults()
	if e != nil || l.MaxSolids < 2 {
		return bad()
	}
	l.MaxBytes = min(l.MaxBytes, 131072)
	l.MaxEntities = min(l.MaxEntities, 236)
	l.MaxValues = min(l.MaxValues, 8192)
	l.MaxDepth = min(l.MaxDepth, 16)
	f, e := Parse(r, l)
	if e != nil {
		return ExactEllipticalPrismCompound{}, e
	}
	roots, apds, categories := []int64{}, []int64{}, []int64{}
	for _, id := range f.Order {
		en := f.Entities[id]
		if len(en.Records) != 1 {
			continue
		}
		switch en.Records[0].Type {
		case "SHAPE_DEFINITION_REPRESENTATION":
			roots = append(roots, id)
		case "APPLICATION_PROTOCOL_DEFINITION":
			apds = append(apds, id)
		case "PRODUCT_RELATED_PRODUCT_CATEGORY":
			categories = append(categories, id)
		}
	}
	if len(roots) != 2 || len(apds) != 2 || len(categories) != 2 {
		return bad()
	}
	out := ExactEllipticalPrismCompound{Source: f}
	claimed := map[int64]bool{}
	sharedFrames := map[int64]bool{}
	faces, vertices, uses := 0, 0, 0
	for member, root := range roots {
		allowed := map[int64]bool{}
		sdr, e := simple(f, root, "SHAPE_DEFINITION_REPRESENTATION", 2)
		if e != nil || sdr.Args[1].Kind != Reference {
			return bad()
		}
		rep, e := simple(f, sdr.Args[1].Ref, "ADVANCED_BREP_SHAPE_REPRESENTATION", 3)
		if e != nil {
			return bad()
		}
		items, e := listRefs(rep.Args[1], 2)
		if e != nil || len(items) != 2 {
			return bad()
		}
		neutral, e := simple(f, items[0], "AXIS2_PLACEMENT_3D", 4)
		if e != nil {
			return bad()
		}
		allowed[items[0]] = true
		for _, v := range neutral.Args[1:] {
			if v.Kind != Reference {
				return bad()
			}
			allowed[v.Ref] = true
		}
		keep, active := map[int64]bool{}, map[int64]bool{}
		var visit func(int64, int) error
		visit = func(id int64, depth int) error {
			if e := ctx.Err(); e != nil {
				return e
			}
			if active[id] || depth >= l.MaxDepth {
				return geom.ErrInvalid
			}
			if keep[id] {
				return nil
			}
			en, ok := f.Entities[id]
			if !ok {
				return geom.ErrInvalid
			}
			active[id] = true
			var value func(Value) error
			value = func(v Value) error {
				if v.Kind == Reference {
					return visit(v.Ref, depth+1)
				}
				for _, x := range v.Items {
					if e := value(x); e != nil {
						return e
					}
				}
				return nil
			}
			for _, rec := range en.Records {
				for _, v := range rec.Args {
					if e := value(v); e != nil {
						return e
					}
				}
			}
			delete(active, id)
			keep[id] = true
			return nil
		}
		if e = visit(root, 0); e != nil {
			return bad()
		}
		attached := 0
		for _, id := range apds {
			rec, e := simple(f, id, "APPLICATION_PROTOCOL_DEFINITION", 4)
			if e != nil {
				return bad()
			}
			if rec.Args[3].Kind == Reference && keep[rec.Args[3].Ref] {
				if e = visit(id, 0); e != nil {
					return bad()
				}
				attached++
			}
		}
		if attached != 1 {
			return bad()
		}
		attached = 0
		for _, id := range categories {
			rec, e := simple(f, id, "PRODUCT_RELATED_PRODUCT_CATEGORY", 3)
			if e != nil {
				return bad()
			}
			ids, e := listRefs(rec.Args[2], 1)
			if e != nil || len(ids) != 1 {
				return bad()
			}
			if keep[ids[0]] {
				if e = visit(id, 0); e != nil {
					return bad()
				}
				attached++
			}
		}
		if attached != 1 {
			return bad()
		}
		view := &File{Header: f.Header, Entities: map[int64]Entity{}}
		for _, id := range f.Order {
			if !keep[id] {
				continue
			}
			if claimed[id] && (!allowed[id] || !sharedFrames[id]) {
				return bad()
			}
			if allowed[id] {
				sharedFrames[id] = true
			}
			claimed[id] = true
			view.Entities[id] = f.Entities[id]
			view.Order = append(view.Order, id)
		}
		leaf := l
		leaf.MaxFaces -= faces
		leaf.MaxVertices -= vertices
		leaf.MaxEdgeUses -= uses
		m, e := readExactEllipticalPrismFamilyFile(ctx, view, leaf, tol)
		if e != nil {
			return ExactEllipticalPrismCompound{}, e
		}
		faces += len(m.Solid.Faces())
		vertices += len(m.Solid.Vertices())
		for _, face := range m.Solid.Faces() {
			uses += len(face.Outer)
			for _, hole := range face.Holes {
				uses += len(hole)
			}
		}
		if faces > l.MaxFaces || vertices > l.MaxVertices || uses > l.MaxEdgeUses {
			return bad()
		}
		if member == 0 {
			out.LengthUnit = m.LengthUnit
		} else if m.LengthUnit != out.LengthUnit {
			return bad()
		}
		out.Members[member] = m
	}
	if len(claimed) != len(f.Entities) {
		return bad()
	}
	return out, nil
}

// WriteExactEllipticalPrismCompound emits two independent standard product roots
// with deterministic ID rebasing. It preserves each original product's numeric
// geometry, metadata and unit declarations. Shared source objects are copied by
// value into separate graph records; no instance/placement is invented.
func WriteExactEllipticalPrismCompound(ctx context.Context, w io.Writer, sources [2]*File, l Limits, tol float64) error {
	if ctx == nil || w == nil {
		return geom.ErrInvalid
	}
	var out bytes.Buffer
	fmt.Fprintln(&out, "ISO-10303-21;\nHEADER;\nFILE_DESCRIPTION(('Independent exact ellipse materials'),'2;1');\nFILE_NAME('ellipse-compound.step','1970-01-01T00:00:00',(''),(''),'PlainSolid','PlainSolid','');\nFILE_SCHEMA(('AUTOMOTIVE_DESIGN'));\nENDSEC;\nDATA;")
	next := int64(1)
	for _, source := range sources {
		if source == nil || (len(source.Entities) != 76 && len(source.Entities) != 118) || !ellipseOwnedSourceBound(source, len(source.Entities)) {
			return geom.ErrInvalid
		}
		raw, e := mappedPlanarSourceBytes(source)
		if e != nil {
			return e
		}
		if _, e = ReadExactEllipticalPrismFamily(ctx, bytes.NewReader(raw), l, tol); e != nil {
			return e
		}
		ids := map[int64]int64{}
		for _, id := range source.Order {
			ids[id] = next
			next++
		}
		for _, id := range source.Order {
			fmt.Fprintf(&out, "#%d=%s;\n", ids[id], assemblyEntity(source.Entities[id], ids))
		}
	}
	fmt.Fprintln(&out, "ENDSEC;END-ISO-10303-21;")
	if _, e := ReadExactEllipticalPrismCompound(ctx, bytes.NewReader(out.Bytes()), l, tol); e != nil {
		return e
	}
	if e := ctx.Err(); e != nil {
		return e
	}
	n, e := w.Write(out.Bytes())
	if e == nil && n != out.Len() {
		e = io.ErrShortWrite
	}
	return e
}
