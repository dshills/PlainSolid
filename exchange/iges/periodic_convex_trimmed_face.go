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
	f, err := provePeriodicConvexOuter(ctx, &source)
	if err != nil {
		return PeriodicConvexTrimmedFace{}, err
	}
	if len(source.graph.claimed) != len(source.graph.records) {
		return PeriodicConvexTrimmedFace{}, geom.ErrUnrepresentable
	}
	return f, nil
}

func provePeriodicConvexOuter(ctx context.Context, source *periodicTrimSource) (PeriodicConvexTrimmedFace, error) {
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

	if _, err := provePeriodicConvexLoop(uv, 1); err != nil {
		return zero, err
	}
	if err := m.proveInjectivePatch(ctx, cell); err != nil {
		return zero, err
	}
	m.Edges, source.remaining, err = provePeriodicAffineEdges(ctx, source.graph, uv, source.model, m.Patch, bounds, source.remaining)
	if err != nil {
		return zero, err
	}
	return PeriodicConvexTrimmedFace(m), nil
}

func provePeriodicConvexLoop(uv [4]periodicTrimUVEdge, winding int) ([4][2]*big.Rat, error) {
	var points [4][2]*big.Rat
	diagonal := false
	for i, edge := range uv {
		next := uv[(i+1)%len(uv)]
		if edge.end[0].Cmp(next.start[0]) != 0 || edge.end[1].Cmp(next.start[1]) != 0 {
			return points, fmt.Errorf("iges: convex trim UV lines do not form one directed cycle at #%d: %w", edge.id, geom.ErrUnrepresentable)
		}
		points[i] = edge.start
		diagonal = diagonal || edge.start[0].Cmp(edge.end[0]) != 0 && edge.start[1].Cmp(edge.end[1]) != 0
	}
	if !periodicUVStrictlyConvex(points, winding) {
		direction := "counterclockwise"
		if winding < 0 {
			direction = "clockwise"
		}
		return points, fmt.Errorf("iges: expected a strictly convex %s UV loop: %w", direction, geom.ErrUnrepresentable)
	}
	if !diagonal {
		return points, fmt.Errorf("iges: convex trim requires a nonisoparametric UV line: %w", geom.ErrUnrepresentable)
	}
	return points, nil
}

func provePeriodicAffineEdges(ctx context.Context, graph periodicTrimGraph, uv [4]periodicTrimUVEdge,
	model []int, patch *geom.SurfacePatch, bounds [2]geom.ExactParameterInterval, remaining int) ([4]PeriodicTrimEdge, int, error) {
	var zero, pairs [4]PeriodicTrimEdge
	var width [2]*big.Rat
	for i, interval := range bounds {
		width[i] = new(big.Rat).Sub(interval.Upper, interval.Lower)
	}
	options := geom.SurfacePatchOptions{}
	for i, edge := range uv {
		var controls [2][4]*big.Rat
		for k, p := range [2][2]*big.Rat{edge.start, edge.end} {
			for axis, q := range p {
				controls[k][axis] = new(big.Rat).Quo(new(big.Rat).Sub(q, bounds[axis].Lower), width[axis])
			}
			controls[k][2], controls[k][3] = new(big.Rat), big.NewRat(1, 1)
		}
		line, err := geom.NewExactHomogeneousBezier(ctx, controls[:], options)
		if err != nil {
			return zero, 0, err
		}
		expected, err := patch.ComposeParameterCurve(ctx, line, options)
		if err != nil {
			return zero, 0, fmt.Errorf("iges: periodic affine composition for UV line #%d: %w", edge.id, err)
		}
		pair, used, err := provePeriodicTrimEdge(ctx, graph, edge.id, model[i], edge.start, edge.end, expected, remaining)
		if err != nil {
			return zero, 0, err
		}
		remaining -= used
		pairs[i] = pair
	}
	return pairs, remaining, nil
}

func periodicUVStrictlyConvex(points [4][2]*big.Rat, winding int) bool {
	for i, a := range points {
		b := points[(i+1)%len(points)]
		// Every nonincident vertex lies strictly on the interior side of
		// each directed edge. This excludes zero sides and self-crossing.
		for j := 2; j < len(points); j++ {
			if winding*periodicUVTurn(a, b, points[(i+j)%len(points)]) <= 0 {
				return false
			}
		}
	}
	return true
}

func periodicUVTurn(a, b, p [2]*big.Rat) int {
	du := new(big.Rat).Sub(b[0], a[0])
	dv := new(big.Rat).Sub(b[1], a[1])
	u := new(big.Rat).Sub(p[0], a[0])
	v := new(big.Rat).Sub(p[1], a[1])
	return v.Mul(v, du).Cmp(u.Mul(u, dv))
}
