package trimmed

import (
	"context"
	"math/big"

	"plainsolid/geom"
)

// An open partition fragment is the union of its original outer boundary
// path and the section cut. At a generic horizontal ray, count every proper
// crossing using the exact restrictions of both kinds of edges.
func classifyHoleAgainstOpenFragment(ctx context.Context, chart int, support *geom.SurfacePatch, f Face,
	cut []AlgebraicTubeFaceCutPiece, outer []AlgebraicTubeFaceBoundaryUse, uv [2]*big.Rat,
	o geom.ExactCurvePlaneEventOptions, projection ...[2]int) (inside bool, operations int, err error) {
	if o.MaxOperations < 1 {
		return false, 0, geom.ErrConvergence
	}
	exact := func() geom.ExactCurvePlaneEventOptions {
		p := o
		p.MaxOperations -= operations
		return p
	}
	patch := func() geom.SurfacePatchOptions {
		return geom.SurfacePatchOptions{MaxOperations: o.MaxOperations - operations, MaxCoefficientBits: o.MaxCoefficientBits}
	}
	compare := func(a, z *geom.ExactAlgebraicRoot) (int, error) {
		p := exact()
		if p.MaxOperations < 1 {
			return 0, geom.ErrConvergence
		}
		q, e := a.Compare(ctx, z, p)
		operations += q.Operations
		if e == nil && !q.Proven {
			e = geom.ErrConvergence
		}
		return q.Order, e
	}
	u, used, e := geom.NewExactRationalRoot(ctx, uv[0], exact())
	operations += used
	if e != nil {
		return false, operations, e
	}
	ray, used, e := tubeRayCrossingsAxis(ctx, chart, support, cut, uv[1], 0, exact(), projection...)
	operations += used
	if e != nil {
		return false, operations, e
	}
	n := 0
	count := func(x *geom.ExactAlgebraicRoot) error {
		c, e := compare(u, x)
		if e != nil {
			return e
		}
		if c == 0 {
			return geom.ErrConvergence
		}
		if c < 0 {
			n++
		}
		return nil
	}
	for _, h := range ray {
		if e := count(h.Parameters[2*chart]); e != nil {
			return false, operations, e
		}
	}
	for _, fragment := range outer {
		if e := ctx.Err(); e != nil {
			return false, operations, e
		}
		source := f.Outer[fragment.Use]
		p := patch()
		if p.MaxOperations < 1 {
			return false, operations, geom.ErrConvergence
		}
		spans, used, e := tubeFaceTrimSpans(ctx, source, SplineTubeTrimSource{}, p)
		operations += used
		if e != nil {
			return false, operations, e
		}
		for _, span := range spans {
			lo, hi := ratInset(span.source.ParameterStart), ratInset(span.source.ParameterEnd)
			if q, ok := shortSplineTrim(source.Curve); ok {
				a, z := ratInset(q.Range.Min), ratInset(q.Range.Max)
				if q.Reversed {
					a, z = z, a
				}
				width := new(big.Rat).Sub(z, a)
				lo.Quo(new(big.Rat).Sub(lo, a), width)
				hi.Quo(new(big.Rat).Sub(hi, a), width)
			}
			p := patch()
			if p.MaxOperations < 1 {
				return false, operations, geom.ErrConvergence
			}
			c, e := exactBezierTrimParameterCurve(ctx, span.controls, p)
			if e != nil {
				return false, operations, e
			}
			operations += c.ExactOperations()
			p = patch()
			if p.MaxOperations < 1 {
				return false, operations, geom.ErrConvergence
			}
			cell, e := geom.NewExactComposedBezierCell(ctx, c, p)
			if e != nil {
				return false, operations, e
			}
			opts := exact()
			if opts.MaxOperations < 1 {
				return false, operations, geom.ErrConvergence
			}
			x, e := cell.IntersectRawPlane(ctx, [4]*big.Rat{nil, big.NewRat(1, 1), nil, new(big.Rat).Neg(uv[1])}, opts)
			operations += x.Operations
			if e != nil {
				return false, operations, e
			}
			if !x.Complete || x.Coincident {
				return false, operations, geom.ErrConvergence
			}
			for _, h := range x.Events {
				t := h.Root()
				if lo.Sign() != 0 || hi.Cmp(big.NewRat(1, 1)) != 0 {
					opts := exact()
					if opts.MaxOperations < 1 {
						return false, operations, geom.ErrConvergence
					}
					t, used, e = t.AffineMap(ctx, lo, new(big.Rat).Sub(hi, lo), opts)
					operations += used
					if e != nil {
						return false, operations, e
					}
				}
				a, e := compare(fragment.Start, t)
				if e != nil {
					return false, operations, e
				}
				z, e := compare(t, fragment.End)
				if e != nil {
					return false, operations, e
				}
				if a > 0 || z > 0 {
					continue
				}
				if a == 0 || z == 0 || h.EndpointMask() != 0 || h.Multiplicity() != 1 {
					return false, operations, geom.ErrConvergence
				}
				opts := exact()
				if opts.MaxOperations < 1 {
					return false, operations, geom.ErrConvergence
				}
				x, used, e := h.CoordinateRoot(ctx, 0, opts)
				operations += used
				if e != nil {
					return false, operations, e
				}
				if e := count(x); e != nil {
					return false, operations, e
				}
			}
		}
	}
	return n%2 == 1, operations, nil
}
