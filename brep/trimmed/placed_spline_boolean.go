package trimmed

import (
	"context"
	"fmt"
	"math/big"
	"slices"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"plainsolid/intersect"
)

// CombinePlacedSplineGraphSolids is CombineSplineGraphSolids for original
// splines with a common affine XY parameterization. The shared
// XY chart may be translated, sheared, scaled or reflected. Its affine
// coefficients must be exactly representable and its placement numerically
// invertible. Mixed bases and nonunit parameter domains are accepted when
// exact normalization and spline export are representable. Heights remain in
// the original world Z coordinate.
func CombinePlacedSplineGraphSolids(ctx context.Context, base, a, b *geom.BSplineSurface, op parametric.BooleanOp,
	tolerance float64, o intersect.SplinePlaneOptions) (Solid, SplineGraphBooleanHistory, error) {
	bad := SplineGraphBooleanHistory{TopA: -1, TopB: -1, Base: -1}
	if ctx == nil || base == nil || a == nil || b == nil {
		return Solid{}, bad, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return Solid{}, bad, e
	}
	x, y, z := base.Data(), a.Data(), b.Data()
	m, e := normalizeSplineGraphCharts(&x, &y, &z)
	inputs := [3]*geom.BSplineSurface{}
	if e != nil {
		m, inputs, e = alignExactPlacedGraphs(ctx, base, a, b, o.Patch, false)
		if e != nil {
			return Solid{}, bad, e
		}
	} else {
		for i, d := range [3]geom.BSplineSurfaceData{x, y, z} {
			inputs[i], e = geom.NewBSplineSurface(d.DegreeU, d.DegreeV, d.CountU, d.CountV, d.Poles, d.KnotsU, d.KnotsV, d.Weights)
			if e != nil {
				return Solid{}, bad, e
			}
		}
	}
	s, h, e := CombineSplineGraphSolids(ctx, inputs[0], inputs[1], inputs[2], op, tolerance, o)
	if e != nil || len(s.faces) == 0 {
		return s, h, e
	}
	s, e = s.Transformed(m)
	if e != nil {
		return Solid{}, bad, e
	}
	return s, h, nil
}

func normalizeSplineGraphCharts(x, y, z *geom.BSplineSurfaceData) (geom.Affine, error) {
	fail := func(reason string) (geom.Affine, error) {
		return geom.Affine{}, fmt.Errorf("trimmed: placed spline chart: %s", reason)
	}
	du, dv, nu, nv := x.DegreeU, x.DegreeV, x.CountU, x.CountV
	if du < 1 || dv < 1 || nu <= du || nv <= dv ||
		y.DegreeU != du || y.DegreeV != dv || y.CountU != nu || y.CountV != nv ||
		z.DegreeU != du || z.DegreeV != dv || z.CountU != nu || z.CountV != nv {
		return fail("expected matching spline basis")
	}
	ku, e := unitGraphAbscissas(du, nu, x.KnotsU)
	if e != nil {
		return geom.Affine{}, e
	}
	kv, e := unitGraphAbscissas(dv, nv, x.KnotsV)
	if e != nil {
		return geom.Affine{}, e
	}
	base, u, v := x.Poles[0], x.Poles[(nu-1)*nv], x.Poles[nv-1]
	m := geom.Identity()
	for axis, c := range [][3]float64{{base.X, u.X, v.X}, {base.Y, u.Y, v.Y}} {
		r := func(i int) *big.Rat { return new(big.Rat).SetFloat64(c[i]) }
		a := new(big.Rat).Sub(r(1), r(0))
		b := new(big.Rat).Sub(r(2), r(0))
		f, fa := a.Float64()
		g, ga := b.Float64()
		if !fa || !ga || !finite(f) || !finite(g) {
			return fail("affine chart coefficients are not exactly representable")
		}
		m[axis][0], m[axis][1], m[axis][3] = f, g, c[0]
	}
	if _, e := m.Inverse(); e != nil {
		return fail("singular or ill-conditioned chart placement")
	}
	value := func(row int, u, v *big.Rat) *big.Rat {
		p := new(big.Rat).SetFloat64(m[row][3])
		p.Add(p, new(big.Rat).Mul(new(big.Rat).SetFloat64(m[row][0]), u))
		p.Add(p, new(big.Rat).Mul(new(big.Rat).SetFloat64(m[row][1]), v))
		return p
	}
	for _, d := range []*geom.BSplineSurfaceData{x, y, z} {
		for i := range nu {
			for j := range nv {
				p := &d.Poles[i*nv+j]
				u, v := ku[i], kv[j]
				if value(0, u, v).Cmp(new(big.Rat).SetFloat64(p.X)) != 0 ||
					value(1, u, v).Cmp(new(big.Rat).SetFloat64(p.Y)) != 0 {
					return fail("XY control points do not define one common affine chart")
				}
				f, eu := geom.RepresentedExactScalar(u)
				g, ev := geom.RepresentedExactScalar(v)
				if eu != nil || ev != nil {
					return fail("normalized identity poles are not exactly representable")
				}
				p.X, p.Y = f, g
			}
		}
	}
	return m, nil
}

// CombineAlignedSplineGraphSolids accepts an affine XY chart on each input,
// with charts that may differ by independent rescaling, U/V exchange or reversal. It aligns exact
// control nets to the base chart, then applies the shared-base Boolean.
// Other parameter transformations are rejected. Ancestry uses original
// operands; resulting trims use the aligned chart.
func CombineAlignedSplineGraphSolids(ctx context.Context, base, a, b *geom.BSplineSurface, op parametric.BooleanOp,
	tolerance float64, o intersect.SplinePlaneOptions) (Solid, SplineGraphBooleanHistory, error) {
	bad := SplineGraphBooleanHistory{TopA: -1, TopB: -1, Base: -1}
	if ctx == nil || base == nil || a == nil || b == nil {
		return Solid{}, bad, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return Solid{}, bad, e
	}
	target := base.Data()
	align := func(s *geom.BSplineSurface) (*geom.BSplineSurface, error) {
		for _, transpose := range []bool{false, true} {
			x := s
			var e error
			if transpose {
				x, e = x.Transposed()
				if e != nil {
					continue
				}
			}
			for _, u := range []bool{false, true} {
				for _, v := range []bool{false, true} {
					if e := ctx.Err(); e != nil {
						return nil, e
					}
					y := x
					if u {
						y, e = y.ReversedU()
						if e != nil {
							continue
						}
					}
					if v {
						y, e = y.ReversedV()
						if e != nil {
							continue
						}
					}
					if sameGraphXYNet(target, y.Data()) {
						return y, nil
					}
				}
			}
		}
		return nil, fmt.Errorf("trimmed: could not align spline graph chart")
	}
	x, e := align(a)
	if e == nil {
		y, e := align(b)
		if e == nil {
			return CombinePlacedSplineGraphSolids(ctx, base, x, y, op, tolerance, o)
		}
	}
	m, inputs, e := alignExactPlacedGraphs(ctx, base, a, b, o.Patch, true)
	if e != nil {
		return Solid{}, bad, e
	}
	s, h, e := CombineSplineGraphSolids(ctx, inputs[0], inputs[1], inputs[2], op, tolerance, o)
	if e != nil || len(s.faces) == 0 {
		return s, h, e
	}
	s, e = s.Transformed(m)
	if e != nil {
		return Solid{}, bad, e
	}
	return s, h, nil
}

func sameGraphXYNet(a, b geom.BSplineSurfaceData) bool {
	if a.DegreeU != b.DegreeU || a.DegreeV != b.DegreeV || a.CountU != b.CountU || a.CountV != b.CountV ||
		!slices.Equal(a.KnotsU, b.KnotsU) || !slices.Equal(a.KnotsV, b.KnotsV) {
		return false
	}
	for i, p := range a.Poles {
		q := b.Poles[i]
		if p.X != q.X || p.Y != q.Y {
			return false
		}
	}
	return true
}
