package step

import (
	"context"
	"math/big"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

func sewEllipticalAnnularPrism(ctx context.Context, b mappedRationalBasis, a, r, ia, ir, h, tol float64) (trimmed.Solid, error) {
	return sewOrientedEllipticalAnnularPrism(ctx, b, a, r, ia, ir, h, tol, false)
}
func sewOrientedEllipticalAnnularPrism(ctx context.Context, b mappedRationalBasis, a, r, ia, ir, h, tol float64, orthogonal bool) (trimmed.Solid, error) {
	bad := func(e error) (trimmed.Solid, error) { return trimmed.Solid{}, e }
	outer, e := exactControlEllipticalPrismParts(ctx, b, a, r, h, tol)
	if e != nil {
		return bad(e)
	}
	innerBasis := b
	if orthogonal {
		innerBasis = orthogonalAnnularBasis(b)
	}
	inner, e := exactControlEllipticalPrismParts(ctx, innerBasis, ia, ir, h, tol)
	if e != nil {
		return bad(e)
	}
	ratios := [2]*big.Rat{new(big.Rat).Quo(new(big.Rat).SetFloat64(ia), new(big.Rat).SetFloat64(a)), new(big.Rat).Quo(new(big.Rat).SetFloat64(ir), new(big.Rat).SetFloat64(r))}
	if orthogonal {
		ratios = [2]*big.Rat{new(big.Rat).Neg(new(big.Rat).Quo(new(big.Rat).SetFloat64(ir), new(big.Rat).SetFloat64(a))), new(big.Rat).Quo(new(big.Rat).SetFloat64(ia), new(big.Rat).SetFloat64(r))}
	}
	for k := 0; k < 2; k++ {
		curves := []trimmed.ExactQuadraticUV{}
		for q := 3; q >= 0; q-- {
			old := outer[k].Face.Outer[q].Curve.(trimmed.ExactQuadraticUV).HomogeneousControls()
			var control [3][3]*big.Rat
			for i := 0; i < 3; i++ {
				w := old[2-i][2]
				control[i][2] = new(big.Rat).Set(w)
				for j := 0; j < 2; j++ {
					half := new(big.Rat).Quo(w, big.NewRat(2, 1))
					axis := j
					if orthogonal {
						axis = 1 - j
					}
					control[i][j] = new(big.Rat).Add(half, new(big.Rat).Mul(ratios[j], new(big.Rat).Sub(old[2-i][axis], half)))
				}
			}
			c, e := trimmed.NewRationalEndpointQuadraticUV(control)
			if e != nil {
				return bad(e)
			}
			curves = append(curves, c)
		}
		hole, e := trimmed.ExactQuadraticSewFace(ctx, outer[k].Face.Surface, curves, outer[k].Face.Reversed, geom.SurfacePatchOptions{})
		if e != nil {
			return bad(e)
		}
		nv, ne := len(outer[k].Vertices), len(outer[k].Edges)
		for i := range hole.Edges {
			hole.Edges[i].Start += nv
			hole.Edges[i].End += nv
		}
		for i := range hole.Face.Outer {
			hole.Face.Outer[i].Edge += ne
		}
		outer[k].Vertices = append(outer[k].Vertices, hole.Vertices...)
		outer[k].Edges = append(outer[k].Edges, hole.Edges...)
		outer[k].Face.Holes = [][]trimmed.Use{hole.Face.Outer}
	}
	for _, p := range inner[2:] {
		p.Face.Reversed = !p.Face.Reversed
		outer = append(outer, p)
	}
	solid, _, e := trimmed.SewRationalQuadratic(ctx, outer, trimmed.SewOptions{Tolerance: tol, MaxFaces: 10, MaxUses: 48})
	return solid, e
}
