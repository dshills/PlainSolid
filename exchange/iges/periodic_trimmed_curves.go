package iges

import (
	"context"
	"fmt"
	"math/big"

	"plainsolid/geom"
)

func (m *PeriodicTrimmedFace) proveEdges(ctx context.Context, graph periodicTrimGraph, cell geom.ExactPeriodicSurfaceBezierCell, uvIDs, modelIDs []int, remaining int) (int, error) {
	uv, _, err := readPeriodicTrimUVRectangle(graph, uvIDs)
	if err != nil {
		return 0, err
	}
	m.Edges, remaining, err = provePeriodicTrimRectangle(ctx, graph, cell, m.Bounds, uv, modelIDs, 1, remaining)
	return remaining, err
}

type periodicTrimUVEdge struct {
	id         int
	start, end [2]*big.Rat
}

func readPeriodicTrimUVRectangle(graph periodicTrimGraph, ids []int) (uv [4]periodicTrimUVEdge, bounds [2]geom.ExactParameterInterval, err error) {
	for i, uvID := range ids {
		rec, err := graph.take(uvID, 110)
		if err != nil {
			return uv, bounds, err
		}
		a, b, err := periodicTrimUVLine(rec)
		if err != nil {
			return uv, bounds, fmt.Errorf("iges: periodic UV line #%d: %w", uvID, err)
		}
		uv[i] = periodicTrimUVEdge{id: uvID, start: a, end: b}
		for _, p := range [2][2]*big.Rat{a, b} {
			for axis, q := range p {
				interval := &bounds[axis]
				if interval.Lower == nil || q.Cmp(interval.Lower) < 0 {
					interval.Lower = q
				}
				if interval.Upper == nil || q.Cmp(interval.Upper) > 0 {
					interval.Upper = q
				}
			}
		}
	}
	return uv, bounds, nil
}

func provePeriodicTrimRectangle(ctx context.Context, graph periodicTrimGraph, cell geom.ExactPeriodicSurfaceBezierCell,
	bounds [2]geom.ExactParameterInterval, uv [4]periodicTrimUVEdge, modelIDs []int, winding, remaining int) ([4]PeriodicTrimEdge, int, error) {
	var zero, pairs [4]PeriodicTrimEdge
	first := periodicTrimCorner(bounds, uv[0].start)
	for i, edge := range uv {
		start, end := periodicTrimCorner(bounds, edge.start), periodicTrimCorner(bounds, edge.end)
		if first < 0 || start != (first+i*winding+4)%4 || end != (start+winding+4)%4 {
			return zero, 0, fmt.Errorf("iges: periodic trim UV line #%d does not follow the directed rectangle: %w", edge.id, geom.ErrUnrepresentable)
		}
		side := start
		if winding < 0 {
			side = end
		}
		expected, err := periodicTrimCellEdge(ctx, cell, side)
		if err != nil {
			return zero, 0, err
		}
		if winding < 0 {
			expected, err = expected.Reparameterize(0, 1, true, geom.SurfacePatchOptions{})
			if err != nil {
				return zero, 0, err
			}
		}
		pair, used, err := provePeriodicTrimEdge(ctx, graph, edge.id, modelIDs[i], edge.start, edge.end, expected, remaining)
		if err != nil {
			return zero, 0, err
		}
		remaining -= used
		pairs[i] = pair
	}
	return pairs, remaining, nil
}

func provePeriodicTrimEdge(ctx context.Context, graph periodicTrimGraph, uvID, modelID int,
	a, b [2]*big.Rat, expected *geom.ExactSplineCurve, remaining int) (PeriodicTrimEdge, int, error) {
	var zero PeriodicTrimEdge
	source, err := graph.take(modelID, 126)
	if err != nil {
		return zero, 0, err
	}
	curve, domain, used, err := periodicTrimModelEdge(ctx, source, remaining)
	if err != nil {
		return zero, 0, fmt.Errorf("iges: periodic model edge #%d: %w", source.id, err)
	}
	same, err := geom.CompareExactCurvesContext(ctx, expected, curve, geom.ExactCurveComparisonOptions{NormalizeDomains: true, MaxSpans: 1})
	if err != nil {
		return zero, 0, err
	}
	if !same.Complete || !same.Equal {
		return zero, 0, fmt.Errorf("iges: periodic model edge #%d disagrees with UV line #%d: %w", source.id, uvID, geom.ErrUnrepresentable)
	}
	return PeriodicTrimEdge{UVSourceID: uvID, ModelSourceID: source.id, UVStart: a, UVEnd: b,
		ModelParameter: domain, ModelCurve: curve}, used, nil
}

func periodicTrimUVLine(r topologyRecord) (a, b [2]*big.Rat, err error) {
	if len(r.values) < 7 {
		return a, b, geom.ErrUnrepresentable
	}
	if err = noPointers(r.values[7:]); err != nil {
		return a, b, err
	}
	p, err := igesPlanarCoordinates(r.values[1:4])
	if err != nil {
		return a, b, err
	}
	q, err := igesPlanarCoordinates(r.values[4:7])
	if err != nil {
		return a, b, err
	}
	if p[2].Sign() != 0 || q[2].Sign() != 0 {
		return a, b, geom.ErrUnrepresentable
	}
	return [2]*big.Rat{p[0], p[1]}, [2]*big.Rat{q[0], q[1]}, nil
}

var periodicTrimCorners = [4][2]int{{0, 0}, {1, 0}, {1, 1}, {0, 1}}

func periodicTrimCorner(bounds [2]geom.ExactParameterInterval, p [2]*big.Rat) int {
	limits := [2][2]*big.Rat{
		{bounds[0].Lower, bounds[0].Upper},
		{bounds[1].Lower, bounds[1].Upper},
	}
	for i, c := range periodicTrimCorners {
		if p[0].Cmp(limits[0][c[0]]) == 0 && p[1].Cmp(limits[1][c[1]]) == 0 {
			return i
		}
	}
	return -1
}

func periodicTrimCellEdge(ctx context.Context, cell geom.ExactPeriodicSurfaceBezierCell, start int) (*geom.ExactSplineCurve, error) {
	a, b := periodicTrimCorners[start], periodicTrimCorners[(start+1)%4]
	du, dv := cell.DegreeU, cell.DegreeV
	stepU, stepV := b[0]-a[0], b[1]-a[1]
	degree := du
	if stepU == 0 {
		degree = dv
	}
	controls := make([][4]*big.Rat, degree+1)
	for i := range controls {
		u, v := a[0]*du+i*stepU, a[1]*dv+i*stepV
		controls[i] = cell.Control[u*(dv+1)+v]
	}
	return geom.NewExactHomogeneousBezier(ctx, controls, geom.SurfacePatchOptions{})
}

func periodicTrimModelEdge(ctx context.Context, source topologyRecord, remaining int) (*geom.ExactSplineCurve, geom.ExactParameterInterval, int, error) {
	var zero geom.ExactParameterInterval
	en, err := decode(126, source.values, remaining)
	if err != nil {
		return nil, zero, 0, err
	}
	if err := en.validate(); err != nil {
		return nil, zero, 0, err
	}
	s := en.Spline
	n := s.Curve.Degree() + 1
	if s.Closed || n != len(s.Curve.Poles()) || n > 26 {
		return nil, zero, 0, fmt.Errorf("expected one clamped model spline edge: %w", geom.ErrUnrepresentable)
	}
	values := source.values[7:]
	domain := geom.ExactParameterInterval{}
	domain.Lower, err = igesPlanarExact(values[6*n])
	if err != nil {
		return nil, zero, 0, err
	}
	domain.Upper, err = igesPlanarExact(values[6*n+1])
	if err != nil || domain.Lower.Cmp(domain.Upper) >= 0 {
		return nil, zero, 0, geom.ErrUnrepresentable
	}
	for i, p := range values[:2*n] {
		want := domain.Lower
		if i >= n {
			want = domain.Upper
		}
		knot, err := igesPlanarExact(p)
		if err != nil || knot.Cmp(want) != 0 {
			return nil, zero, 0, fmt.Errorf("model edge bounds must be its full clamped knot range: %w", geom.ErrUnrepresentable)
		}
	}
	controls := make([][4]*big.Rat, n)
	for i := range controls {
		w, err := igesPlanarExact(values[2*n+i])
		if err != nil || w.Sign() <= 0 || s.Polynomial && i > 0 && controls[0][3].Cmp(w) != 0 {
			return nil, zero, 0, geom.ErrUnrepresentable
		}
		p, err := igesPlanarCoordinates(values[3*n+3*i : 3*n+3*i+3])
		if err != nil {
			return nil, zero, 0, err
		}
		controls[i] = [4]*big.Rat{p[0].Mul(p[0], w), p[1].Mul(p[1], w), p[2].Mul(p[2], w), w}
	}
	curve, err := geom.NewExactHomogeneousBezier(ctx, controls, geom.SurfacePatchOptions{})
	return curve, domain, n, err
}
