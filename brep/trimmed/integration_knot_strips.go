package trimmed

import (
	"context"
	"errors"
	"math/big"
	"slices"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

type massKnotStripProofs struct {
	left int
}

// The old strip corrects a single chord. Replace it by cell-local strips
// correcting a broken chord, then add the signed polygon between the two
// chords. Only the integration domain changes.
func (b *massKnotStripProofs) split(ctx context.Context, surface geom.Surface, side Use, c differentiableTrim,
	knots [2][]float64) ([]triangleSurface, error) {
	wall, reversed := massOriginalWallTrim(c)
	var pieces []triangleSurface
	if wall.surface != nil {
		pieces = massWallKnotStrips(c, wall, reversed)
	} else if q, exact := c.(ExactBezierUV); exact && (len(knots[0]) > 2 || len(knots[1]) > 2) && b.left > 0 {
		var err error
		pieces, err = b.splitExactBezier(ctx, surface, q)
		if err != nil && !errors.Is(err, geom.ErrConvergence) && !errors.Is(err, geom.ErrUnrepresentable) {
			return nil, err
		}
	}
	if len(pieces) == 0 {
		return []triangleSurface{{p: [3]geom.Vec2{side.UVStart, side.UVEnd}, strip: c}}, nil
	}
	if pieces[0].p[0] != side.UVStart || pieces[len(pieces)-1].p[1] != side.UVEnd {
		return nil, geom.ErrConvergence
	}
	return pieces, nil
}

func (b *massKnotStripProofs) splitExactBezier(ctx context.Context, surface geom.Surface, q ExactBezierUV) ([]triangleSurface, error) {
	s, _ := parametric.SurfaceMapping(surface)
	// Placement has no effect on the original UV knot partition.
	cost := min(b.left, 400000)
	o := geom.SurfacePatchOptions{MaxOperations: cost}
	p, err := exactSplineSupportPatch(s, o)
	if err != nil {
		if errors.Is(err, geom.ErrDomain) || errors.Is(err, geom.ErrInvalid) {
			return nil, nil
		}
		b.left -= cost
		return nil, err
	}
	b.left -= p.ExactOperations()
	if b.left < 1 {
		return nil, geom.ErrConvergence
	}
	o.MaxOperations = min(b.left, 400000)
	cells, err := q.ModelCurveCells(ctx, p, geom.SurfaceCurveAgreementOptions{
		SurfacePatchOptions: o, MaxEvents: 64, MaxSubdivisions: 1024,
	})
	b.left -= max(1, cells.ExactOperations)
	if err != nil || !cells.Complete || len(cells.Cells) < 2 {
		return nil, err
	}
	previous := new(big.Rat)
	current := q
	var pieces []triangleSurface
	for _, cell := range cells.Cells[:len(cells.Cells)-1] {
		if b.left < 1 {
			return nil, geom.ErrConvergence
		}
		cut, err := cell.ClassifyEndpoint(ctx, 1, geom.ExactCurvePlaneEventOptions{MaxOperations: b.left})
		b.left -= max(1, cut.Operations)
		if err != nil {
			return nil, err
		}
		if cut.Kind != geom.ExactRootKindRational {
			return nil, geom.ErrUnrepresentable
		}
		local := new(big.Rat).Quo(new(big.Rat).Sub(cut.Rational, previous), new(big.Rat).Sub(big.NewRat(1, 1), previous))
		first, rest, err := current.Split(local)
		if err != nil {
			return nil, err
		}
		pieces = append(pieces, triangleSurface{p: [3]geom.Vec2{first.ends[0], first.ends[1]}, strip: first})
		current, previous = rest, cut.Rational
	}
	return append(pieces, triangleSurface{p: [3]geom.Vec2{current.ends[0], current.ends[1]}, strip: current}), nil
}

func massOriginalWallTrim(c differentiableTrim) (w wallTrim, reversed bool) {
	for {
		switch x := c.(type) {
		case reverseTrim:
			c, reversed = x.differentiableTrim, !reversed
		case affineGraphWallTrim:
			return x.wallTrim, reversed
		case wallTrim:
			return x, reversed
		default:
			return wallTrim{}, false
		}
	}
}

// A wall trim traces an exact original spline line affinely in the curve
// parameter. Only the represented interior knot stations may split it.
func massWallKnotStrips(c differentiableTrim, w wallTrim, reversed bool) []triangleSurface {
	u, v := massSplineKnotLines(w.surface)
	var cuts []float64
	for axis, knots := range [2][]float64{u, v} {
		if len(knots) < 3 {
			continue
		}
		a, z := [2]float64{w.a.X, w.a.Y}[axis], [2]float64{w.b.X, w.b.Y}[axis]
		if reversed {
			a, z = z, a
		}
		for _, k := range knots[1 : len(knots)-1] {
			if !(min(a, z) < k && k < max(a, z)) {
				continue
			}
			t := new(big.Rat).Quo(new(big.Rat).Sub(ratInset(k), ratInset(a)), new(big.Rat).Sub(ratInset(z), ratInset(a)))
			f, err := geom.RepresentedExactScalar(t)
			if err != nil || len(cuts) >= 62 {
				return nil
			}
			cuts = append(cuts, f)
		}
	}
	if len(cuts) == 0 {
		return nil
	}
	slices.Sort(cuts)
	cuts = append(append([]float64{0}, slices.Compact(cuts)...), 1)
	pieces := make([]triangleSurface, 0, len(cuts)-1)
	first, err := c.EvalUV(0)
	if err != nil {
		return nil
	}
	for i, start := range cuts[:len(cuts)-1] {
		end := cuts[i+1]
		next, err := c.EvalUV(end)
		if err != nil {
			return nil
		}
		pieces = append(pieces, triangleSurface{p: [3]geom.Vec2{first, next}, strip: massRestrictedWallTrim{source: c, start: start, end: end, a: first, b: next}})
		first = next
	}
	return pieces
}

type massRestrictedWallTrim struct {
	source     differentiableTrim
	start, end float64
	a, b       geom.Vec2
}

func (r massRestrictedWallTrim) EvalUV(t float64) (geom.Vec2, error) {
	if t == 0 {
		return r.a, nil
	}
	if t == 1 {
		return r.b, nil
	}
	return r.source.EvalUV(r.start + t*(r.end-r.start))
}

func (r massRestrictedWallTrim) DerivativeUV(t float64) (geom.Vec2, error) {
	d, err := r.source.DerivativeUV(r.start + t*(r.end-r.start))
	return scale2(d, r.end-r.start), err
}
