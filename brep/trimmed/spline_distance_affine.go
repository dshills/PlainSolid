package trimmed

import (
	"context"
	"errors"
	"math/big"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// A proved affine patch admits an exact orthogonal projection to its support
// plane. When the projection is in its trim, it attains the lower bound for
// every point on the face. Otherwise it is only a lower bound. The fast path
// treats an unresolved boundary projection like any other unproved trim box.
func affineSplineDistanceProjection(ctx context.Context, p *geom.SurfacePatch, f splineDistanceFace,
	query geom.Vec3, o geom.SurfacePatchOptions) (r splineAffineProjection, err error) {
	c, e := geom.NewAffineSurfaceChart(ctx, p, geom.AffineSurfaceChartOptions{MaxOperations: o.MaxOperations, MaxCoefficientBits: o.MaxCoefficientBits})
	if errors.Is(e, geom.ErrConvergence) || errors.Is(e, geom.ErrDegenerate) {
		return r, nil
	}
	if e != nil {
		return r, e
	}
	r.chart = c
	d := c.Domain()
	start, e := c.EvalExact(ratInset(d.UMin), ratInset(d.VMin))
	if e != nil {
		return r, e
	}
	x, e := c.EvalExact(ratInset(d.UMax), ratInset(d.VMin))
	if e != nil {
		return r, e
	}
	y, e := c.EvalExact(ratInset(d.UMin), ratInset(d.VMax))
	if e != nil {
		return r, e
	}
	var u, v, q [3]*big.Rat
	for i, value := range [3]float64{query.X, query.Y, query.Z} {
		u[i] = new(big.Rat).Sub(x[i], start[i])
		v[i] = new(big.Rat).Sub(y[i], start[i])
		q[i] = new(big.Rat).Sub(ratInset(value), start[i])
	}
	dot := func(a, b [3]*big.Rat) *big.Rat {
		sum := new(big.Rat)
		for i := range a {
			sum.Add(sum, new(big.Rat).Mul(a[i], b[i]))
		}
		return sum
	}
	uu, uv, vv := dot(u, u), dot(u, v), dot(v, v)
	qu, qv := dot(q, u), dot(q, v)
	det := new(big.Rat).Sub(new(big.Rat).Mul(uu, vv), new(big.Rat).Mul(uv, uv))
	if det.Sign() <= 0 {
		return r, geom.ErrDegenerate
	}
	a := new(big.Rat).Sub(new(big.Rat).Mul(qu, vv), new(big.Rat).Mul(qv, uv))
	a.Quo(a, det)
	b := new(big.Rat).Sub(new(big.Rat).Mul(qv, uu), new(big.Rat).Mul(qu, uv))
	b.Quo(b, det)
	r.lower = new(big.Rat).Sub(dot(q, q), new(big.Rat).Mul(a, qu))
	r.lower.Sub(r.lower, new(big.Rat).Mul(b, qv))
	if a.Sign() < 0 || b.Sign() < 0 || a.Cmp(big.NewRat(1, 1)) > 0 || b.Cmp(big.NewRat(1, 1)) > 0 {
		return r, nil
	}
	axis := func(f *big.Rat, lo, hi float64) *big.Rat {
		return new(big.Rat).Add(ratInset(lo), new(big.Rat).Mul(f, new(big.Rat).Sub(ratInset(hi), ratInset(lo))))
	}
	ua, va := axis(a, d.UMin, d.UMax), axis(b, d.VMin, d.VMax)
	r.uv = rationalUVBounds(ua, va)
	loc, known := parametric.Inside, f.rectangle
	if !known {
		loc, known, e = classifyTrimBox(ctx, f.face, r.uv, 48)
		if e != nil {
			return r, e
		}
	}
	if !known || loc != parametric.Inside {
		return r, nil
	}
	xyz, e := c.EvalExact(ua, va)
	if e != nil {
		return r, e
	}
	r.point, r.hit = rationalPointBounds(xyz), true
	return r, nil
}

type splineAffineProjection struct {
	chart *geom.AffineSurfaceChart
	lower *big.Rat
	uv    geom.UVBox
	point geom.Box
	hit   bool
}

// The closest point on a straight UV edge of an affine support is a rational
// segment projection. Keep its exact parameter through the UV interpolation.
func affineSplineEdgeWitness(c *geom.AffineSurfaceChart, u Use, query geom.Vec3) (geom.UVBox, geom.Box, error) {
	a, e := c.EvalExact(ratInset(u.UVStart.X), ratInset(u.UVStart.Y))
	if e != nil {
		return geom.UVBox{}, geom.Box{}, e
	}
	b, e := c.EvalExact(ratInset(u.UVEnd.X), ratInset(u.UVEnd.Y))
	if e != nil {
		return geom.UVBox{}, geom.Box{}, e
	}
	q := [3]float64{query.X, query.Y, query.Z}
	n, d := new(big.Rat), new(big.Rat)
	for i := range a {
		delta := new(big.Rat).Sub(b[i], a[i])
		n.Add(n, new(big.Rat).Mul(delta, new(big.Rat).Sub(ratInset(q[i]), a[i])))
		d.Add(d, new(big.Rat).Mul(delta, delta))
	}
	if d.Sign() <= 0 {
		return geom.UVBox{}, geom.Box{}, geom.ErrDegenerate
	}
	if n.Sign() <= 0 {
		n.SetInt64(0)
	} else if n.Cmp(d) >= 0 {
		n.SetInt64(1)
	} else {
		n.Quo(n, d)
	}
	coord := func(a, b float64) *big.Rat {
		return new(big.Rat).Add(ratInset(a), new(big.Rat).Mul(n, new(big.Rat).Sub(ratInset(b), ratInset(a))))
	}
	x, y := coord(u.UVStart.X, u.UVEnd.X), coord(u.UVStart.Y, u.UVEnd.Y)
	p, e := c.EvalExact(x, y)
	if e != nil {
		return geom.UVBox{}, geom.Box{}, e
	}
	return rationalUVBounds(x, y), rationalPointBounds(p), nil
}

func rationalUVBounds(x, y *big.Rat) geom.UVBox {
	xl, xh := rationalFloatRange(x)
	yl, yh := rationalFloatRange(y)
	return geom.UVBox{Min: geom.Vec2{X: xl, Y: yl}, Max: geom.Vec2{X: xh, Y: yh}}
}

func rationalPointBounds(p [3]*big.Rat) geom.Box {
	xl, xh := rationalFloatRange(p[0])
	yl, yh := rationalFloatRange(p[1])
	zl, zh := rationalFloatRange(p[2])
	return geom.Box{Min: geom.Vec3{X: xl, Y: yl, Z: zl}, Max: geom.Vec3{X: xh, Y: yh, Z: zh}}
}

func rationalFloatRange(x *big.Rat) (float64, float64) {
	r, _ := geom.RationalOutwardRange(x)
	return r.Min, r.Max
}
