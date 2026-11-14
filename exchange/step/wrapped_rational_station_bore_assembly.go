package step

import (
	"bytes"
	"context"
	"io"
	"plainsolid/geom"
)

// WrappedRationalStationBoreAssembly retains one through-bore product with
// independently authenticated inner/outer station supports and 2..16 uses.
// Wall role slices are ordered bottom to top separately for each owner.
type WrappedRationalStationBoreAssembly struct {
	Assembly                                                      Assembly
	Product                                                       AssemblyProduct
	Body                                                          WrappedRationalStationBoreBody
	SourceBodyID                                                  int64
	SourceFaceIDs, SourceOuterWallFaceIDs, SourceInnerWallFaceIDs []int64
	Occurrences                                                   []ExactNAUOOccurrence
}
type WrappedRationalStationBoreAssemblyPart struct {
	ID, Name string
	Body     *WrappedRationalStationBoreBody
}
type WrappedRationalStationBoreAssemblyModel struct {
	Products    []WrappedRationalStationBoreAssemblyPart
	Occurrences []AssemblyInstance
	LengthUnit  float64
}

func ReadWrappedRationalStationBoreAssembly(ctx context.Context, r io.Reader, l Limits, o WrappedSurfaceOptions) (WrappedRationalStationBoreAssembly, error) {
	var zero WrappedRationalStationBoreAssembly
	var body WrappedRationalStationBoreBody
	a, e := readWeightedStationAssembly(ctx, r, l, o, func(raw []byte, l Limits, o WrappedSurfaceOptions) (WrappedRationalStationLoftBody, error) {
		var e error
		body, e = ReadWrappedRationalStationBore(bytes.NewReader(raw), l, o)
		if e != nil {
			return WrappedRationalStationLoftBody{}, e
		}
		walls := append(append([]WrappedSplineFace(nil), body.Outer.Walls...), body.Inner.Walls...)
		return WrappedRationalStationLoftBody{Name: body.Name, Walls: walls, CapNames: body.CapNames, Solid: body.Solid, LengthUnit: body.LengthUnit}, nil
	})
	if e != nil {
		return zero, e
	}
	n := len(body.Outer.Walls)
	if len(a.SourceWallFaceIDs) != n+len(body.Inner.Walls) {
		return zero, geom.ErrInvalid
	}
	return WrappedRationalStationBoreAssembly{a.Assembly, a.Product, body, a.SourceBodyID, a.SourceFaceIDs, append([]int64(nil), a.SourceWallFaceIDs[:n]...), append([]int64(nil), a.SourceWallFaceIDs[n:]...), a.Occurrences}, nil
}

func WriteWrappedRationalStationBoreAssembly(ctx context.Context, w io.Writer, m WrappedRationalStationBoreAssemblyModel, l Limits, o WrappedSurfaceOptions) error {
	if ctx == nil || w == nil || len(m.Products) != 2 || len(m.Occurrences) < 2 || len(m.Occurrences) > 16 {
		return geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return e
	}
	graph := AssemblyModel{LengthUnit: m.LengthUnit, Occurrences: m.Occurrences}
	counts := make([]int, 2)
	bodies := 0
	for i, p := range m.Products {
		if len(p.ID) > 32768 || len(p.Name) > 32768 {
			return geom.ErrInvalid
		}
		graph.Products = append(graph.Products, AssemblyPart{ID: p.ID, Name: p.Name})
		if p.Body != nil {
			if len(p.Body.Name) > 32768 || len(p.Body.CapNames[0]) > 32768 || len(p.Body.CapNames[1]) > 32768 {
				return geom.ErrInvalid
			}
			for _, side := range []WrappedRationalStationLoftBody{p.Body.Outer, p.Body.Inner} {
				for _, wall := range side.Walls {
					if len(wall.Name) > 32768 {
						return geom.ErrInvalid
					}
				}
			}
			if p.Body.LengthUnit != m.LengthUnit || len(p.Body.Outer.Walls) > 16 || len(p.Body.Inner.Walls) > 16 {
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
		if !planarSplineRigid(u.Transform) || len(u.ID) > 32768 || len(u.Name) > 32768 || len(u.Parent) > 32768 || len(u.Child) > 32768 {
			return geom.ErrInvalid
		}
	}
	var staged bytes.Buffer
	e := writeAssemblyGeometry(&staged, graph, counts, func(i int) (*File, []int64, error) {
		var b bytes.Buffer
		if e := WriteWrappedRationalStationBore(&b, *m.Products[i].Body); e != nil {
			return nil, nil, e
		}
		f, e := Parse(&b, l)
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
	if _, e = ReadWrappedRationalStationBoreAssembly(ctx, bytes.NewReader(staged.Bytes()), l, o); e != nil {
		return e
	}
	n, e := w.Write(staged.Bytes())
	if e == nil && n != staged.Len() {
		return io.ErrShortWrite
	}
	return e
}
func WriteWrappedRationalStationBoreAssemblySource(ctx context.Context, w io.Writer, f *File, l Limits, o WrappedSurfaceOptions) error {
	if ctx == nil || w == nil || !independentNativeSourceRecordBound(f, 4096, 8) {
		return geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return e
	}
	b, e := mappedPlanarSourceBytes(f)
	if e != nil {
		return e
	}
	if _, e = ReadWrappedRationalStationBoreAssembly(ctx, bytes.NewReader(b), l, o); e != nil {
		return e
	}
	n, e := w.Write(b)
	if e == nil && n != len(b) {
		return io.ErrShortWrite
	}
	return e
}
