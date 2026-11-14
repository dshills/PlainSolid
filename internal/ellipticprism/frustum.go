package ellipticprism

import (
	"context"
	"math/big"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// CircularFrustum constructs a positive-Z, positive-radius full conical frustum
// from exact quarter circles and a linear radius interpolation. Positive radii
// and height prove embedding; Sew proves all continuous edge identities.
func CircularFrustum(ctx context.Context, origin geom.Vec3, lower, upper, height, tol float64) (trimmed.Solid, [2][4]int, error) {
	f, e := geom.NewFrame(origin, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
	if e != nil {
		return trimmed.Solid{}, [2][4]int{}, e
	}
	return CircularFrustumFrame(ctx, f, lower, upper, height, tol)
}

// CircularFrustumFrame retains exact represented controls in the supplied frame.
func CircularFrustumFrame(ctx context.Context, frame geom.Frame, lower, upper, height, tol float64) (trimmed.Solid, [2][4]int, error) {
	p, e := circularFrustumParts(ctx, frame, lower, upper, height, tol)
	if e != nil {
		return trimmed.Solid{}, [2][4]int{}, e
	}
	return sewParts(ctx, p, tol)
}
func circularFrustumParts(ctx context.Context, frame geom.Frame, lower, upper, height, tol float64) ([]trimmed.SewFace, error) {
	origin := frame.Origin()
	bad := func(e error) ([]trimmed.SewFace, error) { return nil, e }
	if ctx == nil || !origin.IsFinite() || !finite(lower) || !finite(upper) || !finite(height) || !finite(tol) || lower <= 0 || upper <= 0 || height <= 0 || tol <= 0 {
		return bad(geom.ErrInvalid)
	}
	if e := ctx.Err(); e != nil {
		return bad(e)
	}
	point := func(x, y, z float64) (geom.Vec3, error) {
		axes := []geom.Vec3{frame.XAxis(), frame.YAxis(), frame.ZAxis()}
		o := []float64{origin.X, origin.Y, origin.Z}
		v := [3]float64{}
		for i := 0; i < 3; i++ {
			q := new(big.Rat).SetFloat64(o[i])
			for j, c := range []float64{x, y, z} {
				a := []float64{axes[j].X, axes[j].Y, axes[j].Z}
				q.Add(q, new(big.Rat).Mul(new(big.Rat).SetFloat64(c), new(big.Rat).SetFloat64(a[i])))
			}
			var exact bool
			v[i], exact = q.Float64()
			if !exact || !finite(v[i]) {
				return geom.Vec3{}, geom.ErrUnrepresentable
			}
		}
		return geom.Vec3{X: v[0], Y: v[1], Z: v[2]}, nil
	}
	var uv, world [2][4][]geom.Vec3
	var caps [2]geom.Frame
	for k, r := range []float64{lower, upper} {
		z := 0.0
		var e error
		if k == 1 {
			z = height
		}
		capOrigin, e := point(0, 0, z)
		if e != nil {
			return bad(e)
		}
		caps[k], e = geom.NewFrame(capOrigin, frame.XAxis(), frame.ZAxis())
		if e != nil {
			return bad(e)
		}
		for q := 0; q < 4; q++ {
			for _, v := range [][2]float64{{1, 0}, {1, 1}, {0, 1}} {
				x, y := v[0], v[1]
				for j := 0; j < q; j++ {
					x, y = -y, x
				}
				x *= r
				y *= r
				uv[k][q] = append(uv[k][q], geom.Vec3{X: x, Y: y})
				w, e := point(x, y, z)
				if e != nil {
					return bad(e)
				}
				world[k][q] = append(world[k][q], w)
			}
		}
	}
	p, e := partsEndControls(ctx, uv, world, caps, tol)
	if e != nil {
		return bad(e)
	}
	return p, nil
}
