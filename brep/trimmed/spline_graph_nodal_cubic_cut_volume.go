package trimmed

import (
	"context"
	"fmt"
	"math/big"

	"plainsolid/geom"
)

func nodalCubicCutVolumes(ctx context.Context, roofs [2]*geom.BSplineSurface, section nodalCubicNativeSection,
	o NodalCubicCutOptions) ([2]*big.Rat, error) {
	var out [2]*big.Rat
	p := section.proof
	var heights [2]*big.Rat
	for i, source := range roofs {
		weights := source.Data().Weights
		for _, weight := range weights {
			if weight != weights[0] {
				return out, fmt.Errorf("trimmed: exact nodal Cut volume requires polynomial source heights: %w", geom.ErrDomain)
			}
		}
		patch, err := geom.NewSurfacePatch(source, o.Section.Patch)
		if err != nil {
			return out, err
		}
		uv := p.Regions[1].UV[i]
		jet, err := patch.EvaluateRational(ctx, uv[0], uv[1], geom.SurfacePatchEvaluationOptions{
			MaxOperations: o.Proof.MaxOperations, MaxCoefficientBits: o.Proof.MaxCoefficientBits,
		})
		if err != nil {
			return out, err
		}
		heights[i] = jet.Point[2]
	}
	// Both source denominators are constant. The complete homogeneous
	// residual determines the entire height difference up to a constant.
	// The certified loop witness is x=-depth/2, y=0, where the positive
	// gap is amplitude*scale*depth³/8.
	a, k := p.LoopDepth, p.Scale
	a2 := new(big.Rat).Mul(a, a)
	a3 := new(big.Rat).Mul(a, a2)
	amplitude := new(big.Rat).Sub(heights[0], heights[1])
	amplitude.Mul(amplitude, big.NewRat(8, 1)).Quo(amplitude, new(big.Rat).Mul(k, a3))
	width := new(big.Rat).Sub(ratInset(section.domain.UMax), ratInset(section.domain.UMin))
	height := new(big.Rat).Sub(ratInset(section.domain.VMax), ratInset(section.domain.VMin))
	factor := new(big.Rat).Mul(amplitude, big.NewRat(4, 3))
	factor.Mul(factor, width).Mul(factor, height)

	// Integrating the vertical gap over y gives (4/3)*amplitude*|t*x|³.
	// With x=t²/scale-depth, the remaining antiderivative is
	// (2/scale)*t⁵*(-depth³/5 + 3*depth²*s/7 - depth*s²/3 + s³/11)
	// with s=t²/scale. Reverse its sign in the loop, where x is negative.
	primitive := func(t *big.Rat) *big.Rat {
		t2 := new(big.Rat).Mul(t, t)
		s := new(big.Rat).Quo(t2, k)
		poly := new(big.Rat).Mul(big.NewRat(1, 11), s)
		poly.Sub(poly, new(big.Rat).Mul(a, big.NewRat(1, 3))).Mul(poly, s)
		poly.Add(poly, new(big.Rat).Mul(a2, big.NewRat(3, 7))).Mul(poly, s)
		poly.Sub(poly, new(big.Rat).Mul(a3, big.NewRat(1, 5)))
		t5 := new(big.Rat).Mul(new(big.Rat).Mul(t2, t2), t)
		return poly.Mul(poly, t5).Mul(poly, big.NewRat(2, 1)).Quo(poly, k)
	}
	atNode := primitive(p.Edges[3].Range[0])
	atExit := primitive(p.Edges[3].Range[1])
	volumes := [2]*big.Rat{
		new(big.Rat).Mul(factor, new(big.Rat).Neg(atNode)),
		new(big.Rat).Mul(factor, new(big.Rat).Sub(atExit, atNode)),
	}
	bits := o.Proof.MaxCoefficientBits
	if bits == 0 {
		bits = 16384
	}
	for _, q := range []*big.Rat{amplitude, factor, atNode, atExit, volumes[0], volumes[1]} {
		if q.Num().BitLen() > bits || q.Denom().BitLen() > bits {
			return out, geom.ErrConvergence
		}
	}
	if volumes[0].Sign() <= 0 || volumes[1].Sign() <= 0 {
		return out, geom.ErrConvergence
	}
	if err := ctx.Err(); err != nil {
		return out, err
	}
	return volumes, nil
}
