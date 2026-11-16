package trimmed

import (
	"context"
	"math/big"
	"slices"

	"plainsolid/brep/edgecheck"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// ExtractExactBezierBoundaryAtRationalKnots also separates model edges at the
// exact rational boundaries of their original support knot cells. Both face
// uses share each new model vertex and keep their original support charts.
// Algebraic stations or model vertices needing rounding cause refusal.
func (s Solid) ExtractExactBezierBoundaryAtRationalKnots(ctx context.Context, o edgecheck.Options) (ExactBezierBoundary, error) {
	return s.extractExactBezierBoundary(ctx, o, true)
}

type exactBoundaryKnotPart struct {
	edge int
	ends [2]*big.Rat
}

func (b *ExactBezierBoundary) splitRationalKnotEdge(ctx context.Context, id int, edge parametric.Edge,
	o edgecheck.Options) ([]exactBoundaryKnotPart, error) {
	source, ok := edge.Curve.(surfaceTrimCurve)
	if !ok || edge.Range != (parametric.Interval{Max: 1}) {
		return nil, geom.ErrConvergence
	}
	work := o.MaxWork
	if work == 0 {
		work = 2000000
	}
	limits := geom.SurfacePatchOptions{MaxOperations: work, MaxCoefficientBits: o.MaxBits}
	cells, err := exactBoundaryGraphCells(ctx, source.surface, source.a, source.b, source.curve,
		geom.SurfaceCurveAgreementOptions{SurfacePatchOptions: limits, MaxEvents: o.MaxIntervals})
	if err != nil {
		return nil, err
	}
	if !cells.Complete || len(cells.Cells) < 2 {
		return nil, geom.ErrConvergence
	}
	proof := geom.ExactCurvePlaneEventOptions{MaxOperations: work - cells.ExactOperations, MaxCoefficientBits: o.MaxBits}
	parts := make([]exactBoundaryKnotPart, len(cells.Cells))
	start := edge.Start
	last := new(big.Rat)
	one := big.NewRat(1, 1)
	for i, cell := range cells.Cells {
		if proof.MaxOperations < 1 {
			return nil, geom.ErrConvergence
		}
		event, err := cell.ClassifyEndpoint(ctx, 1, proof)
		if err != nil {
			return nil, err
		}
		proof.MaxOperations -= event.Operations
		switch event.Kind {
		case geom.ExactRootKindRational:
		case geom.ExactRootKindIrrational:
			return nil, geom.ErrUnrepresentable
		default:
			return nil, geom.ErrConvergence
		}
		next := event.Rational
		if last.Cmp(next) >= 0 || next.Cmp(one) > 0 || proof.MaxOperations < 1 {
			return nil, geom.ErrConvergence
		}
		ends := [2]*big.Rat{last, next}
		curve, used, err := cell.ExactBezierCurve(ctx, ends, proof)
		if err != nil {
			return nil, err
		}
		proof.MaxOperations -= used
		if proof.MaxOperations < 1 {
			return nil, geom.ErrConvergence
		}
		limits.MaxOperations = proof.MaxOperations
		span, used, err := curve.BezierControlNet(ctx, limits)
		if err != nil {
			return nil, err
		}
		proof.MaxOperations -= used
		if !exactBoundaryEnd(span.Control[0], b.Vertices[start]) {
			return nil, geom.ErrConvergence
		}
		endControl := span.Control[len(span.Control)-1]
		end, neutralID := edge.End, id
		if i < len(parts)-1 {
			point, err := geom.RepresentedExactPoint([3]*big.Rat{
				new(big.Rat).Quo(endControl[0], endControl[3]),
				new(big.Rat).Quo(endControl[1], endControl[3]),
				new(big.Rat).Quo(endControl[2], endControl[3]),
			})
			if err != nil {
				return nil, err
			}
			if slices.Contains(b.Vertices, point) {
				return nil, geom.ErrConvergence
			}
			end = len(b.Vertices)
			b.Vertices = append(b.Vertices, point)
		} else if next.Cmp(one) != 0 || !exactBoundaryEnd(endControl, b.Vertices[end]) {
			return nil, geom.ErrConvergence
		}
		if start == end {
			return nil, geom.ErrConvergence
		}
		segment := ExactBezierBoundaryEdge{Start: start, End: end, Model: span}
		if i == 0 {
			b.Edges[id] = segment
		} else {
			neutralID = len(b.Edges)
			b.Edges = append(b.Edges, segment)
		}
		parts[i] = exactBoundaryKnotPart{edge: neutralID, ends: ends}
		start, last = end, next
	}
	return parts, nil
}

func (b *ExactBezierBoundary) splitRationalKnotUse(ctx context.Context, surface geom.Surface, use Use,
	parts []exactBoundaryKnotPart, limits geom.SurfacePatchOptions) ([]ExactBezierBoundaryUse, error) {
	var original geom.ExactBezierSpan
	wall, isWall := surface.(splineGraphAffineWall)
	if !isWall {
		var err error
		original, err = exactBoundaryUV(ctx, use, limits)
		if err != nil {
			return nil, err
		}
		if use.Reversed {
			slices.Reverse(original.Control)
		}
	}
	out := make([]ExactBezierBoundaryUse, len(parts))
	for i, part := range parts {
		var span geom.ExactBezierSpan
		var err error
		if isWall {
			span, err = exactBoundaryKnotWallUV(ctx, wall, b.Edges[part.edge].Model, limits)
		} else {
			span, err = restrictBoundaryKnotUV(ctx, original, part.ends, limits)
		}
		if err != nil {
			return nil, err
		}
		out[i] = ExactBezierBoundaryUse{Edge: part.edge, Reversed: use.Reversed, UV: span}
	}
	a, z := use.UVStart, use.UVEnd
	if use.Reversed {
		a, z = z, a
	}
	first, last := out[0].UV.Control, out[len(out)-1].UV.Control
	if !exactBoundaryEnd(first[0], geom.Vec3{X: a.X, Y: a.Y}) ||
		!exactBoundaryEnd(last[len(last)-1], geom.Vec3{X: z.X, Y: z.Y}) {
		return nil, geom.ErrConvergence
	}
	if use.Reversed {
		slices.Reverse(out)
	}
	return out, nil
}

func restrictBoundaryKnotUV(ctx context.Context, span geom.ExactBezierSpan, ends [2]*big.Rat,
	limits geom.SurfacePatchOptions) (geom.ExactBezierSpan, error) {
	if len(span.Control) < 2 || len(span.Control) > maxSourceGraphBezierControls {
		return geom.ExactBezierSpan{}, geom.ErrConvergence
	}
	uv := make([][3]*big.Rat, len(span.Control))
	for i, p := range span.Control {
		uv[i] = [3]*big.Rat{p[0], p[1], p[3]}
	}
	if ends[1].Cmp(big.NewRat(1, 1)) < 0 {
		uv, _ = splitHomogeneousUV(uv, ends[1])
	}
	if ends[0].Sign() > 0 {
		_, uv = splitHomogeneousUV(uv, new(big.Rat).Quo(ends[0], ends[1]))
	}
	controls := make([][4]*big.Rat, len(uv))
	for i, p := range uv {
		controls[i] = [4]*big.Rat{p[0], p[1], new(big.Rat), p[2]}
	}
	return exactBoundarySpan(ctx, controls, limits)
}

// Zero-residual coedge agreement and injectivity of the original affine wall
// identify its UV restriction from the exact model fragment.
func exactBoundaryKnotWallUV(ctx context.Context, wall splineGraphAffineWall, model geom.ExactBezierSpan,
	limits geom.SurfacePatchOptions) (geom.ExactBezierSpan, error) {
	dir := [2]*big.Rat{ratInset(wall.along.X), ratInset(wall.along.Y)}
	axis := 0
	if dir[0].Sign() == 0 {
		axis = 1
	}
	if dir[axis].Sign() == 0 {
		return geom.ExactBezierSpan{}, geom.ErrDegenerate
	}
	origin := [3]*big.Rat{ratInset(wall.origin.X), ratInset(wall.origin.Y), ratInset(wall.origin.Z)}
	controls := make([][4]*big.Rat, len(model.Control))
	for i, p := range model.Control {
		offset := func(axis int) *big.Rat {
			return new(big.Rat).Sub(p[axis], new(big.Rat).Mul(origin[axis], p[3]))
		}
		u := new(big.Rat).Quo(offset(axis), dir[axis])
		if new(big.Rat).Mul(u, dir[1-axis]).Cmp(offset(1-axis)) != 0 {
			return geom.ExactBezierSpan{}, geom.ErrConvergence
		}
		controls[i] = [4]*big.Rat{u, offset(2), new(big.Rat), p[3]}
	}
	return exactBoundarySpan(ctx, controls, limits)
}
