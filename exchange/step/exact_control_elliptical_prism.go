package step

import (
	"context"
	"math/big"
	"plainsolid/brep/edgecheck"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

func exactPrismPoint(b mappedRationalBasis, x, y, z *big.Rat) (v [3]*big.Rat) {
	for i := range v {
		v[i] = new(big.Rat).Set(b.Origin[i])
		for j, axis := range [3][3]*big.Rat{b.X, b.Y, b.Z} {
			v[i].Add(v[i], new(big.Rat).Mul(axis[i], []*big.Rat{x, y, z}[j]))
		}
	}
	return
}
func exactPrismSurface(ctx context.Context, du, dv int, points [][3]*big.Rat, weights []*big.Rat) (*geom.ExactBezierSurface, error) {
	h := make([][4]*big.Rat, len(points))
	for i, p := range points {
		for j := range p {
			h[i][j] = new(big.Rat).Mul(p[j], weights[i])
		}
		h[i][3] = new(big.Rat).Set(weights[i])
	}
	p, e := geom.NewExactHomogeneousBezierSurface(ctx, du, dv, h, geom.SurfacePatchOptions{MaxPoles: 6, MaxCoefficientBits: 4096, MaxOperations: 100000})
	if e != nil {
		return nil, e
	}
	return geom.NewExactBezierSurface(p)
}
func exactPrismLine(a, b [2]int64) (trimmed.ExactQuadraticUV, error) {
	var h [3][3]*big.Rat
	for i := 0; i < 3; i++ {
		for j := 0; j < 2; j++ {
			h[i][j] = big.NewRat((2-int64(i))*a[j]+int64(i)*b[j], 2)
		}
		h[i][2] = big.NewRat(1, 1)
	}
	return trimmed.NewRationalEndpointQuadraticUV(h)
}
func exactControlEllipticalPrismParts(ctx context.Context, b mappedRationalBasis, a, r, h, tol float64) ([]trimmed.SewFace, error) {
	fail := func(e error) ([]trimmed.SewFace, error) { return nil, e }
	if ctx == nil {
		return fail(geom.ErrInvalid)
	}
	if e := exactPrismBasis(b); e != nil {
		return fail(e)
	}
	zero := new(big.Rat)
	ar, br, hr := new(big.Rat).SetFloat64(a), new(big.Rat).SetFloat64(r), new(big.Rat).SetFloat64(h)
	// Every support uses its original homogeneous rational coefficients. Cap UV
	// is the source ellipse coordinate box normalized to [0,1]^2.
	parts := []trimmed.SewFace{}
	for k, z := range []*big.Rat{zero, hr} {
		points := [][3]*big.Rat{}
		for _, x := range []*big.Rat{new(big.Rat).Neg(ar), ar} {
			for _, y := range []*big.Rat{new(big.Rat).Neg(br), br} {
				points = append(points, exactPrismPoint(b, x, y, z))
			}
		}
		s, e := exactPrismSurface(ctx, 1, 1, points, []*big.Rat{big.NewRat(1, 1), big.NewRat(1, 1), big.NewRat(1, 1), big.NewRat(1, 1)})
		if e != nil {
			return fail(e)
		}
		curves := []trimmed.ExactQuadraticUV{}
		for q := 0; q < 4; q++ {
			var control [3][3]*big.Rat
			for i, p := range [][2]int64{{1, 0}, {1, 1}, {0, 1}} {
				x, y := p[0], p[1]
				for j := 0; j < q; j++ {
					x, y = -y, x
				}
				w := int64(1)
				if i == 2 {
					w = 2
				}
				control[i] = [3]*big.Rat{big.NewRat((x+1)*w, 2), big.NewRat((y+1)*w, 2), big.NewRat(w, 1)}
			}
			c, e := trimmed.NewRationalEndpointQuadraticUV(control)
			if e != nil {
				return fail(e)
			}
			curves = append(curves, c)
		}
		part, e := trimmed.ExactQuadraticSewFace(ctx, s, curves, k == 0, geom.SurfacePatchOptions{MaxPoles: 6, MaxCoefficientBits: 4096, MaxOperations: 100000})
		if e != nil {
			return fail(e)
		}
		parts = append(parts, part)
	}
	for q := 0; q < 4; q++ {
		points := [][3]*big.Rat{}
		weights := []*big.Rat{}
		for i, p := range [][2]int64{{1, 0}, {1, 1}, {0, 1}} {
			x, y := p[0], p[1]
			for j := 0; j < q; j++ {
				x, y = -y, x
			}
			for _, z := range []*big.Rat{zero, hr} {
				points = append(points, exactPrismPoint(b, new(big.Rat).Mul(ar, big.NewRat(x, 1)), new(big.Rat).Mul(br, big.NewRat(y, 1)), z))
				w := int64(1)
				if i == 2 {
					w = 2
				}
				weights = append(weights, big.NewRat(w, 1))
			}
		}
		s, e := exactPrismSurface(ctx, 2, 1, points, weights)
		if e != nil {
			return fail(e)
		}
		uv := [4][2]int64{{0, 0}, {1, 0}, {1, 1}, {0, 1}}
		curves := []trimmed.ExactQuadraticUV{}
		for i, p := range uv {
			c, e := exactPrismLine(p, uv[(i+1)%4])
			if e != nil {
				return fail(e)
			}
			curves = append(curves, c)
		}
		part, e := trimmed.ExactQuadraticSewFace(ctx, s, curves, false, geom.SurfacePatchOptions{MaxPoles: 6, MaxCoefficientBits: 4096, MaxOperations: 100000})
		if e != nil {
			return fail(e)
		}
		parts = append(parts, part)
	}
	return parts, nil
}
func sewExactControlEllipticalPrism(ctx context.Context, b mappedRationalBasis, a, r, h, tol float64) (trimmed.Solid, [2][4]int, error) {
	fail := func(e error) (trimmed.Solid, [2][4]int, error) { return trimmed.Solid{}, [2][4]int{}, e }
	parts, e := exactControlEllipticalPrismParts(ctx, b, a, r, h, tol)
	if e != nil {
		return fail(e)
	}

	solid, history, e := trimmed.SewRationalQuadratic(ctx, parts, trimmed.SewOptions{Tolerance: tol, MaxFaces: 6, MaxUses: 24, Proof: edgecheck.Options{}})
	if e != nil {
		return fail(e)
	}
	var rims [2][4]int
	for k := range rims {
		for q := range rims[k] {
			rims[k][q] = history.Edges[k][q]
		}
	}
	return solid, rims, nil
}
