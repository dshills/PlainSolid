package step

import (
	"bytes"
	"context"
	"io"
	"plainsolid/geom"
	"slices"
	"sort"
)

// WrappedRationalStationAssembly retains one independently proved weighted
// station product, shared by 2..16 flat proper signed-coordinate occurrences.
// Original wrapped supports and model/pcurve topology remain product-local.
type WrappedRationalStationAssembly struct {
	Assembly                         Assembly
	Product                          AssemblyProduct
	Body                             WrappedRationalStationLoftBody
	SourceBodyID                     int64
	SourceFaceIDs, SourceWallFaceIDs []int64
	Occurrences                      []ExactNAUOOccurrence
}
type WrappedRationalStationAssemblyPart struct {
	ID, Name string
	Body     *WrappedRationalStationLoftBody
}
type WrappedRationalStationAssemblyModel struct {
	Products    []WrappedRationalStationAssemblyPart
	Occurrences []AssemblyInstance
	LengthUnit  float64
}

func ReadWrappedRationalStationAssembly(ctx context.Context, r io.Reader, l Limits, o WrappedSurfaceOptions) (WrappedRationalStationAssembly, error) {
	return readWeightedStationAssembly(ctx, r, l, o, func(raw []byte, l Limits, o WrappedSurfaceOptions) (WrappedRationalStationLoftBody, error) {
		return ReadWrappedRationalStationLoft(bytes.NewReader(raw), l, o)
	})
}

func readWeightedStationAssembly(ctx context.Context, r io.Reader, l Limits, o WrappedSurfaceOptions, local func([]byte, Limits, WrappedSurfaceOptions) (WrappedRationalStationLoftBody, error)) (WrappedRationalStationAssembly, error) {
	var zero WrappedRationalStationAssembly
	if ctx == nil || r == nil {
		return zero, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return zero, e
	}
	l, e := occurrenceColorSourceLimits(l)
	if e != nil {
		return zero, e
	}
	o, e = o.defaults()
	if e != nil {
		return zero, e
	}
	base, e := readExactFlatSingleProductAssembly(ctx, r, l)
	if e != nil {
		return zero, e
	}
	out := WrappedRationalStationAssembly{Assembly: base.Assembly, Product: base.Product, SourceBodyID: base.SourceBodyID, Occurrences: base.Occurrences}
	f, p, a := base.Assembly.Source, base.Product, base.Assembly
	closure, e := nauoClosure(f, []int64{p.Representations[0]}, l.MaxDepth)
	if e != nil {
		return zero, e
	}
	view := &File{Header: f.Header, Entities: map[int64]Entity{}}
	for _, id := range f.Order {
		if closure[id] {
			view.Order = append(view.Order, id)
			view.Entities[id] = f.Entities[id]
		}
	}
	raw, e := mappedPlanarSourceBytes(view)
	if e != nil {
		return zero, e
	}
	body, e := local(raw, l, o)
	if e != nil {
		return zero, e
	}
	if body.LengthUnit != a.LengthUnit {
		return zero, geom.ErrInvalid
	}
	out.Body = body
	out.SourceFaceIDs, e = bodyFaceIDs(f, base.SourceBodyID)
	if e != nil {
		return zero, e
	}
	for _, wall := range out.Body.Walls {
		want := append([]int64(nil), wall.SideEdges[:]...)
		sort.Slice(want, func(i, j int) bool { return want[i] < want[j] })
		var found int64
		for _, id := range out.SourceFaceIDs {
			uses, e := weightedAssemblyFaceUses(f, id)
			if e != nil {
				continue
			}
			got := []int64{}
			for _, u := range uses {
				got = append(got, u.id)
			}
			sort.Slice(got, func(i, j int) bool { return got[i] < got[j] })
			if slices.Equal(got, want) {
				if found != 0 {
					return zero, geom.ErrInvalid
				}
				found = id
			}
		}
		if found == 0 {
			return zero, geom.ErrInvalid
		}
		out.SourceWallFaceIDs = append(out.SourceWallFaceIDs, found)
	}
	if e = ctx.Err(); e != nil {
		return zero, e
	}
	return out, nil
}

// WriteWrappedRationalStationAssembly stages one weighted local body and a
// geometry-free root with shared flat uses. Cached native Solid is not authority.
func WriteWrappedRationalStationAssembly(ctx context.Context, w io.Writer, m WrappedRationalStationAssemblyModel, l Limits, o WrappedSurfaceOptions) error {
	if ctx == nil || w == nil || len(m.Products) != 2 || len(m.Occurrences) < 2 || len(m.Occurrences) > 16 {
		return geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return e
	}
	for _, p := range m.Products {
		if len(p.ID) > 32768 || len(p.Name) > 32768 {
			return geom.ErrInvalid
		}
		if p.Body != nil {
			if len(p.Body.Name) > 32768 || len(p.Body.CapNames[0]) > 32768 || len(p.Body.CapNames[1]) > 32768 || len(p.Body.Walls) > 16 {
				return geom.ErrInvalid
			}
			for _, wall := range p.Body.Walls {
				if len(wall.Name) > 32768 {
					return geom.ErrInvalid
				}
			}
		}
	}
	for _, u := range m.Occurrences {
		if len(u.ID) > 32768 || len(u.Name) > 32768 || len(u.Parent) > 32768 || len(u.Child) > 32768 {
			return geom.ErrInvalid
		}
	}
	graph := AssemblyModel{LengthUnit: m.LengthUnit, Occurrences: m.Occurrences}
	counts := make([]int, 2)
	bodies := 0
	for i, p := range m.Products {
		graph.Products = append(graph.Products, AssemblyPart{ID: p.ID, Name: p.Name})
		if p.Body != nil {
			if p.Body.LengthUnit != m.LengthUnit {
				return geom.ErrInvalid
			}
			counts[i] = 1
			bodies++
		}
	}
	if bodies != 1 {
		return geom.ErrInvalid
	}
	for _, u := range m.Occurrences {
		if !planarSplineRigid(u.Transform) {
			return geom.ErrInvalid
		}
	}
	var staged bytes.Buffer
	e := writeAssemblyGeometry(&staged, graph, counts, func(i int) (*File, []int64, error) {
		var raw bytes.Buffer
		if e := WriteWrappedRationalStationLoft(&raw, *m.Products[i].Body); e != nil {
			return nil, nil, e
		}
		f, e := Parse(&raw, l)
		if e != nil {
			return nil, nil, e
		}
		var ids []int64
		for _, id := range f.Order {
			if _, e := simple(f, id, "MANIFOLD_SOLID_BREP", 2); e == nil {
				ids = append(ids, id)
			}
		}
		return f, ids, nil
	})
	if e != nil {
		return e
	}
	if _, e = ReadWrappedRationalStationAssembly(ctx, bytes.NewReader(staged.Bytes()), l, o); e != nil {
		return e
	}
	n, e := w.Write(staged.Bytes())
	if e == nil && n != staged.Len() {
		return io.ErrShortWrite
	}
	return e
}
func WriteWrappedRationalStationAssemblySource(ctx context.Context, w io.Writer, f *File, l Limits, o WrappedSurfaceOptions) error {
	if ctx == nil || w == nil || !independentNativeSourceRecordBound(f, 4096, 8) {
		return geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return e
	}
	raw, e := mappedPlanarSourceBytes(f)
	if e != nil {
		return e
	}
	if _, e = ReadWrappedRationalStationAssembly(ctx, bytes.NewReader(raw), l, o); e != nil {
		return e
	}
	n, e := w.Write(raw)
	if e == nil && n != len(raw) {
		return io.ErrShortWrite
	}
	return e
}

// This maps identities only after the selected local material reader proved
// orientation. Inner bore bounds can be false; no source record is changed.
func weightedAssemblyFaceUses(f *File, id int64) ([]analyticUse, error) {
	face, e := simple(f, id, "ADVANCED_FACE", 4)
	if e != nil {
		return nil, e
	}
	ids, e := listRefs(face.Args[1], 1)
	if e != nil || len(ids) != 1 {
		return nil, geom.ErrInvalid
	}
	b, e := simple(f, ids[0], "FACE_BOUND", 3)
	if e != nil {
		b, e = simple(f, ids[0], "FACE_OUTER_BOUND", 3)
	}
	if e != nil {
		return nil, e
	}
	if _, e = boolean(b.Args[2]); e != nil {
		return nil, e
	}
	loop, e := simple(f, b.Args[1].Ref, "EDGE_LOOP", 2)
	if e != nil {
		return nil, e
	}
	edges, e := listRefs(loop.Args[1], 4)
	if e != nil || len(edges) != 4 {
		return nil, geom.ErrInvalid
	}
	var out []analyticUse
	for _, eid := range edges {
		u, e := simple(f, eid, "ORIENTED_EDGE", 5)
		if e != nil {
			return nil, e
		}
		sense, e := boolean(u.Args[4])
		if e != nil {
			return nil, e
		}
		out = append(out, analyticUse{u.Args[3].Ref, sense})
	}
	return out, nil
}
