package step

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// OCCTExactDirectedCircularLoopExtrusion retains original product, surface,
// topology, and cap bound IDs. CapBounds follows RingOffsets on each cap.
type OCCTExactDirectedCircularLoopExtrusion struct {
	ExactDirectedCircularLoopExtrusionProduct
	Product   AssemblyProduct
	CapBounds [2][]int64
}

// ReadOCCTExactDirectedCircularLoopExtrusion proves a single model-only AP214
// extrusion from original finite lines and directed minor arcs. It accepts
// Z-axis planar/cylindrical walls and separate contained cap holes. A low cap
// without an explicit FACE_OUTER_BOUND must list the exterior first, as OCCT
// does. The native result is normalized to positive local Z height. It rejects
// pcurves, uncertain endpoints and geometry repairs.
func ReadOCCTExactDirectedCircularLoopExtrusion(ctx context.Context, r io.Reader, l Limits) (OCCTExactDirectedCircularLoopExtrusion, error) {
	var zero OCCTExactDirectedCircularLoopExtrusion
	a, body, l, err := readOCCTCircularLoopSource(ctx, r, l)
	if err != nil {
		return zero, err
	}
	f := a.Source
	m, err := readOCCTLoopModel(ctx, f, l, body, true)
	if err != nil {
		return zero, fmt.Errorf("step: directed loop topology: %w", err)
	}
	low := m.faces[m.lo]
	outer := 0
	marked := false
	for i, q := range low.trims {
		if q.outer {
			if marked {
				return zero, geom.ErrDomain
			}
			outer, marked = i, true
		}
	}
	frame, err := geom.NewFrame(geom.Vec3{Z: m.z0}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
	if err != nil {
		return zero, err
	}
	product := a.Products[0]
	d := ExactDirectedCircularLoopExtrusion{Name: product.Name, Frame: frame, Height: m.height, LengthUnit: a.LengthUnit}
	var flat []trimmed.ExactCircularLoopSegment
	offsets := []int{0}
	ordered := []occtLoopTrim{low.trims[outer]}
	ordered = append(ordered, low.trims[:outer]...)
	ordered = append(ordered, low.trims[outer+1:]...)
	for _, q := range ordered {
		ring := make([]trimmed.ExactCircularLoopSegment, 0, len(q.uses))
		for i := len(q.uses) - 1; i >= 0; i-- {
			u := q.uses[i]
			edge := m.edges[u.id]
			start, end := m.points[edge.b], m.points[edge.a]
			if !u.sense {
				start, end = end, start
			}
			if start.Z != m.z0 || end.Z != m.z0 {
				return zero, geom.ErrDomain
			}
			s := trimmed.ExactCircularLoopSegment{Start: geom.Vec2{X: start.X, Y: start.Y}, End: geom.Vec2{X: end.X, Y: end.Y}}
			if edge.kind == "CIRCLE" {
				c, e := simple(f, edge.model, "CIRCLE", 3)
				if e != nil {
					return zero, e
				}
				o, z, _, e := occtLoopFrame(f, c.Args[1].Ref)
				if e != nil || z[0].Sign() != 0 || z[1].Sign() != 0 || o[2].Cmp(low.origin[2]) != 0 {
					return zero, geom.ErrDomain
				}
				center, e := obliqueNURBSFloatPoint(o)
				if e != nil {
					return zero, e
				}
				s.Center = geom.Vec2{X: center.X, Y: center.Y}
				s.Radius, e = cylinderSupportNumber(c.Args[2])
				if e != nil || s.Radius <= 0 {
					return zero, geom.ErrDomain
				}
			}
			ring = append(ring, s)
		}
		if len(flat) == 0 {
			d.Outer = ring
		} else {
			d.Holes = append(d.Holes, ring)
		}
		flat = append(flat, ring...)
		offsets = append(offsets, len(flat))
	}
	native, err := trimmed.ExtrudeExactDirectedCircularLoops(ctx, d.Outer, d.Holes, 0, d.Height, trimmed.ExactCircularLoopOptions{})
	if err != nil {
		return zero, fmt.Errorf("step: directed loop native extrusion: %w", err)
	}
	out := OCCTExactDirectedCircularLoopExtrusion{Product: product, ExactDirectedCircularLoopExtrusionProduct: ExactDirectedCircularLoopExtrusionProduct{
		Descriptor: d, Native: native, Source: f, Body: body, Shell: m.shell, Caps: [2]int64{m.lo, m.hi}, RingOffsets: offsets,
	}}
	if err = occtLoopBindSource(f, &out.ExactDirectedCircularLoopExtrusionProduct, flat, m, true); err != nil {
		return zero, fmt.Errorf("step: directed loop source binding: %w", err)
	}
	for cap, id := range out.Caps {
		for _, start := range offsets[:len(offsets)-1] {
			for _, q := range m.faces[id].trims {
				for _, u := range q.uses {
					if u.id == out.Rims[cap][start] {
						out.CapBounds[cap] = append(out.CapBounds[cap], q.bound)
					}
				}
			}
		}
	}
	if err = ctx.Err(); err != nil {
		return zero, err
	}
	return out, nil
}

// WriteOCCTExactDirectedCircularLoopSource re-proves and emits the original
// product, cap bounds and directed model edges with their original entity IDs.
func WriteOCCTExactDirectedCircularLoopSource(ctx context.Context, w io.Writer, f *File, l Limits) error {
	if ctx == nil || w == nil || !independentNativeSourceRecordBound(f, 4096, 16) {
		return geom.ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	b, err := mappedPlanarSourceBytes(f)
	if err != nil {
		return err
	}
	if len(b) > 1<<20 {
		return geom.ErrConvergence
	}
	if _, err = ReadOCCTExactDirectedCircularLoopExtrusion(ctx, bytes.NewReader(b), l); err != nil {
		return err
	}
	n, err := w.Write(b)
	if err == nil && n != len(b) {
		return io.ErrShortWrite
	}
	return err
}
