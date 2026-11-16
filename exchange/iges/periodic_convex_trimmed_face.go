package iges

import (
	"context"
	"fmt"
	"io"
	"math/big"

	"plainsolid/geom"
)

// PeriodicConvexTrimmedFace retains one strictly convex four-edge trim
// with at least one diagonal UV edge. Patch evaluates the enclosing
// rectangle, with the retained directed edge pairs delimiting material.
// Bounds and KnotOwners retain the original physical chart.
type PeriodicConvexTrimmedFace PeriodicTrimmedFace

// OriginalIGES returns the complete retained source file.
func (f PeriodicConvexTrimmedFace) OriginalIGES() string { return f.original }

// ReadPeriodicConvexTrimmedFace proves an outer-only type-144 face with
// four affine UV edges strictly within one original knot cell. A fixed
// projection certifies the full enclosing rectangle. Each retained model
// curve must equal the exact composition of its original directed UV
// line with the periodic surface over the entire edge.
func ReadPeriodicConvexTrimmedFace(ctx context.Context, r io.Reader, l Limits) (PeriodicConvexTrimmedFace, error) {
	source, err := readPeriodicTrimSourceForAxes(ctx, r, l, 4, periodicTrimAnyAxes)
	if err != nil {
		return PeriodicConvexTrimmedFace{}, err
	}
	return provePeriodicConvexTrimmedFace(ctx, source)
}

func provePeriodicConvexTrimmedFace(ctx context.Context, source periodicTrimSource) (PeriodicConvexTrimmedFace, error) {
	var zero PeriodicConvexTrimmedFace
	m := source.face
	cell, err := m.proveOriginalKnotCell(ctx)
	if err != nil {
		return zero, err
	}
	var uv [4]periodicTrimUVEdge
	bounds, err := readPeriodicTrimUVEdges(source.graph, source.uv, uv[:])
	if err != nil {
		return zero, err
	}
	for axis, interval := range bounds {
		if interval.Lower.Cmp(m.Bounds[axis].Lower) != 0 || interval.Upper.Cmp(m.Bounds[axis].Upper) != 0 {
			return zero, fmt.Errorf("iges: convex trim bounds disagree with the original type-128 bounds: %w", geom.ErrUnrepresentable)
		}
	}

	var points [4][2]*big.Rat
	diagonal := false
	for i, edge := range uv {
		next := uv[(i+1)%len(uv)]
		if edge.end[0].Cmp(next.start[0]) != 0 || edge.end[1].Cmp(next.start[1]) != 0 {
			return zero, fmt.Errorf("iges: convex trim UV lines do not form one directed cycle at #%d: %w", edge.id, geom.ErrUnrepresentable)
		}
		points[i] = edge.start
		diagonal = diagonal || edge.start[0].Cmp(edge.end[0]) != 0 && edge.start[1].Cmp(edge.end[1]) != 0
	}
	if !periodicUVStrictlyConvex(points) {
		return zero, fmt.Errorf("iges: expected a strictly convex counterclockwise UV loop: %w", geom.ErrUnrepresentable)
	}
	if !diagonal {
		return zero, fmt.Errorf("iges: convex trim requires a nonisoparametric UV line: %w", geom.ErrUnrepresentable)
	}
	if err := m.proveInjectivePatch(ctx, cell); err != nil {
		return zero, err
	}

	var width [2]*big.Rat
	for i, interval := range bounds {
		width[i] = new(big.Rat).Sub(interval.Upper, interval.Lower)
	}
	remaining := source.remaining
	for i, edge := range uv {
		var controls [2][4]*big.Rat
		for k, p := range [2][2]*big.Rat{edge.start, edge.end} {
			for axis, q := range p {
				controls[k][axis] = new(big.Rat).Quo(new(big.Rat).Sub(q, bounds[axis].Lower), width[axis])
			}
			controls[k][2], controls[k][3] = new(big.Rat), big.NewRat(1, 1)
		}
		options := geom.SurfacePatchOptions{}
		line, err := geom.NewExactHomogeneousBezier(ctx, controls[:], options)
		if err != nil {
			return zero, err
		}
		expected, err := m.Patch.ComposeParameterCurve(ctx, line, options)
		if err != nil {
			return zero, fmt.Errorf("iges: periodic affine composition for UV line #%d: %w", edge.id, err)
		}
		pair, used, err := provePeriodicTrimEdge(ctx, source.graph, edge.id, source.model[i], edge.start, edge.end, expected, remaining)
		if err != nil {
			return zero, err
		}
		remaining -= used
		m.Edges[i] = pair
	}
	if len(source.graph.claimed) != len(source.graph.records) {
		return zero, geom.ErrUnrepresentable
	}
	return PeriodicConvexTrimmedFace(m), nil
}

func periodicUVStrictlyConvex(points [4][2]*big.Rat) bool {
	for i, a := range points {
		b := points[(i+1)%len(points)]
		du := new(big.Rat).Sub(b[0], a[0])
		dv := new(big.Rat).Sub(b[1], a[1])
		// Every nonincident vertex lies strictly on the left of the
		// directed edge. This also excludes zero sides and self-crossing.
		for j := 2; j < len(points); j++ {
			p := points[(i+j)%len(points)]
			u := new(big.Rat).Sub(p[0], a[0])
			v := new(big.Rat).Sub(p[1], a[1])
			if v.Mul(v, du).Cmp(u.Mul(u, dv)) <= 0 {
				return false
			}
		}
	}
	return true
}
