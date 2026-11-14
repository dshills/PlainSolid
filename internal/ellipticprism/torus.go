package ellipticprism

import (
	"context"
	"math/big"
	"plainsolid/brep/edgecheck"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// RingTorus sews sixteen exact rational circle-product charts. R>r>0 proves
// positive radial distance and injective interiors, with only declared seams.
func RingTorus(ctx context.Context, R, r, tol float64) (trimmed.Solid, [2][4]int, error) {
	bad := func(e error) (trimmed.Solid, [2][4]int, error) { return trimmed.Solid{}, [2][4]int{}, e }
	if ctx == nil || !finite(R) || !finite(r) || !finite(tol) || r <= 0 || R <= r || tol <= 0 {
		return bad(geom.ErrInvalid)
	}
	quarter := func(q, i int) (float64, float64) {
		p := [][2]float64{{1, 0}, {1, 1}, {0, 1}}[i]
		x, y := p[0], p[1]
		for j := 0; j < q; j++ {
			x, y = -y, x
		}
		return x, y
	}
	parts := []trimmed.SewFace{}
	knots := []float64{0, 0, 0, 1, 1, 1}
	w := []float64{1, 1, 2}
	for u := 0; u < 4; u++ {
		for v := 0; v < 4; v++ {
			if e := ctx.Err(); e != nil {
				return bad(e)
			}
			poles := []geom.Vec3{}
			weights := []float64{}
			for i := 0; i < 3; i++ {
				cu, su := quarter(u, i)
				for j := 0; j < 3; j++ {
					cv, sv := quarter(v, j)
					rhoQ := new(big.Rat).Add(new(big.Rat).SetFloat64(R), new(big.Rat).Mul(new(big.Rat).SetFloat64(r), new(big.Rat).SetFloat64(cv)))
					rho, exact := rhoQ.Float64()
					if !exact || !finite(rho) {
						return bad(geom.ErrUnrepresentable)
					}
					poles = append(poles, geom.Vec3{X: rho * cu, Y: rho * su, Z: r * sv})
					weights = append(weights, w[i]*w[j])
				}
			}
			s, e := geom.NewBSplineSurface(2, 2, 3, 3, poles, knots, knots, weights)
			if e != nil {
				return bad(e)
			}
			p, e := trimmed.RectangularSplineFace(s, false)
			if e != nil {
				return bad(e)
			}
			parts = append(parts, p)
		}
	}
	solid, h, e := trimmed.Sew(ctx, parts, trimmed.SewOptions{Tolerance: tol, MaxFaces: 16, MaxUses: 64, Proof: edgecheck.Options{}})
	if e != nil {
		return bad(e)
	}
	var seams [2][4]int
	for q := 0; q < 4; q++ {
		seams[0][q] = h.Edges[4*q][0]
		seams[1][q] = h.Edges[q][3]
	}
	return solid, seams, nil
}
