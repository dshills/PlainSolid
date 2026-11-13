package trimmed

import (
	"context"
	"errors"
	"fmt"
	"math/big"

	"plainsolid/geom"
)

func alignExactPlacedGraphs(ctx context.Context, base, a, b *geom.BSplineSurface, o geom.SurfacePatchOptions, symmetry bool) (geom.Affine, [3]*geom.BSplineSurface, error) {
	var out [3]*geom.BSplineSurface
	if e := ctx.Err(); e != nil {
		return geom.Affine{}, out, e
	}
	world, inverse, e := exactGraphFrame(base)
	if e != nil {
		return geom.Affine{}, out, e
	}
	unit := geom.ParameterRectangle{UMax: 1, VMax: 1}
	for i, s := range [3]*geom.BSplineSurface{base, a, b} {
		var last error
		count := 1
		if i != 0 && symmetry {
			count = 8
		}
		for j := range count {
			if e := ctx.Err(); e != nil {
				return geom.Affine{}, out, e
			}
			x := s
			if j&4 != 0 {
				x, e = x.Transposed()
				if e != nil {
					last = e
					continue
				}
			}
			p, e := geom.NewSurfacePatch(x, o)
			if e != nil {
				return geom.Affine{}, out, e
			}
			p, e = p.Reparameterize(unit, j&1 != 0, j&2 != 0, o)
			if e != nil {
				last = e
				continue
			}
			p, e = p.Transformed(inverse)
			if e != nil {
				return geom.Affine{}, out, e
			}
			candidate, e := p.ToBSplineSurface()
			if e != nil {
				last = e
				continue
			}
			e = requireIdentitySplineChart(ctx, candidate, o)
			if e == nil {
				out[i] = candidate
				break
			}
			last = e
		}
		if out[i] == nil {
			return geom.Affine{}, out, fmt.Errorf("trimmed: exact graph chart alignment for operand %d: %w", i, last)
		}
	}
	return world, out, nil
}

func exactGraphFrame(s *geom.BSplineSurface) (geom.Affine, geom.Affine, error) {
	d := s.Data()
	if d.CountU < 2 || d.CountV < 2 || len(d.Poles) == 0 || d.KnotsU[0] != d.KnotsU[d.DegreeU] || d.KnotsV[0] != d.KnotsV[d.DegreeV] ||
		d.KnotsU[len(d.KnotsU)-1] != d.KnotsU[d.CountU] || d.KnotsV[len(d.KnotsV)-1] != d.KnotsV[d.CountV] {
		return geom.Affine{}, geom.Affine{}, geom.ErrInvalid
	}
	a, b, c := d.Poles[0], d.Poles[(d.CountU-1)*d.CountV], d.Poles[d.CountV-1]
	sub := func(a, b float64) *big.Rat { return new(big.Rat).Sub(ratInset(a), ratInset(b)) }
	x, y := [2]*big.Rat{sub(b.X, a.X), sub(b.Y, a.Y)}, [2]*big.Rat{sub(c.X, a.X), sub(c.Y, a.Y)}
	det := new(big.Rat).Sub(new(big.Rat).Mul(x[0], y[1]), new(big.Rat).Mul(x[1], y[0]))
	if det.Sign() == 0 {
		return geom.Affine{}, geom.Affine{}, geom.ErrDegenerate
	}
	m, inv := geom.Identity(), geom.Identity()
	assign := func(f *float64, r *big.Rat) bool {
		x, e := geom.RepresentedExactScalar(r)
		*f = x
		return e == nil
	}
	for i, tr := range []float64{a.X, a.Y} {
		if !assign(&m[i][0], x[i]) || !assign(&m[i][1], y[i]) {
			return geom.Affine{}, geom.Affine{}, geom.ErrUnrepresentable
		}
		m[i][3] = tr
	}
	row := [2][2]*big.Rat{{y[1], new(big.Rat).Neg(y[0])}, {new(big.Rat).Neg(x[1]), x[0]}}
	for i, r := range row {
		u, v := new(big.Rat).Quo(r[0], det), new(big.Rat).Quo(r[1], det)
		tr := new(big.Rat).Add(new(big.Rat).Mul(u, ratInset(a.X)), new(big.Rat).Mul(v, ratInset(a.Y)))
		if !assign(&inv[i][0], u) || !assign(&inv[i][1], v) || !assign(&inv[i][3], tr.Neg(tr)) {
			return geom.Affine{}, geom.Affine{}, geom.ErrUnrepresentable
		}
	}
	if _, e := m.Inverse(); e != nil {
		return geom.Affine{}, geom.Affine{}, errors.Join(geom.ErrDegenerate, e)
	}
	return m, inv, nil
}
