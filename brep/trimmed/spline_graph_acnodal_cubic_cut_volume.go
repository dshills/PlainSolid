package trimmed

import (
	"context"
	"fmt"
	"math/big"

	"plainsolid/geom"
)

func acnodalCubicCutVolume(ctx context.Context, roofs [2]*geom.BSplineSurface, patches [2]*geom.SurfacePatch,
	p *geom.AcnodalCubicSection, o geom.GraphPointContactOptions) (*big.Rat, error) {
	var heights [2]*big.Rat
	for i, original := range roofs {
		weights := original.Data().Weights
		for _, w := range weights {
			if w != weights[0] {
				return nil, fmt.Errorf("trimmed: exact acnodal Cut volume requires polynomial source heights: %w", geom.ErrDomain)
			}
		}
		uv := p.Regions[1].UV[i]
		jet, err := patches[i].EvaluateRational(ctx, uv[0], uv[1], geom.SurfacePatchEvaluationOptions{
			MaxOperations: o.MaxOperations, MaxCoefficientBits: o.MaxCoefficientBits,
		})
		if err != nil {
			return nil, err
		}
		heights[i] = jet.Point[2]
	}

	// With constant denominators, each original knot cell has a scalar
	// multiple of the same signed height polynomial. Continuity across a
	// knot forces adjacent amplitudes to agree, since the cubic is nonzero
	// somewhere on every longitudinal and transverse knot line.
	k, g, tau := p.Scale, p.Gap, p.Arc.Edges[1].Range[1]
	t2 := new(big.Rat).Mul(tau, tau)
	span := new(big.Rat).Quo(t2, k)
	// The interior witness is x=g+span/2, y=0. Solve for the amplitude
	// of its positive gap k*x²*(x-g)-y² from the original heights.
	half := new(big.Rat).Mul(span, big.NewRat(1, 2))
	x := new(big.Rat).Add(g, half)
	witness := new(big.Rat).Mul(x, x)
	witness.Mul(witness, k).Mul(witness, half)
	if witness.Sign() <= 0 {
		return nil, geom.ErrConvergence
	}
	amplitude := new(big.Rat).Quo(new(big.Rat).Sub(heights[0], heights[1]), witness)
	tx, ty := p.IsolatedTangent[0], p.IsolatedTangent[1]
	jacobian := new(big.Rat).Sub(new(big.Rat).Mul(tx[0], ty[1]), new(big.Rat).Mul(tx[1], ty[0]))
	jacobian.Abs(jacobian)
	// Integrating across y gives (4/3)*amplitude*(t*x)^3.
	// With x=g+t²/k and dx=2t/k dt, its integral contributes
	// 2*tau^5/k times the cubic below, with span=tau²/k.
	g2 := new(big.Rat).Mul(g, g)
	poly := new(big.Rat).Mul(span, big.NewRat(1, 11))
	poly.Add(poly, new(big.Rat).Mul(g, big.NewRat(1, 3))).Mul(poly, span)
	poly.Add(poly, new(big.Rat).Mul(g2, big.NewRat(3, 7))).Mul(poly, span)
	poly.Add(poly, new(big.Rat).Mul(g2, new(big.Rat).Mul(g, big.NewRat(1, 5))))
	t5 := new(big.Rat).Mul(new(big.Rat).Mul(t2, t2), tau)
	volume := new(big.Rat).Mul(poly, t5)
	volume.Mul(volume, amplitude).Mul(volume, jacobian).Mul(volume, big.NewRat(8, 3)).Quo(volume, k)
	bits := o.MaxCoefficientBits
	if bits == 0 {
		bits = 16384
	}
	for _, q := range []*big.Rat{witness, amplitude, jacobian, poly, volume} {
		if q.Num().BitLen() > bits || q.Denom().BitLen() > bits {
			return nil, geom.ErrConvergence
		}
	}
	if volume.Sign() <= 0 {
		return nil, geom.ErrConvergence
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return volume, nil
}
