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
	bad := func(e error) (trimmed.Solid, [2][4]int, error) { return trimmed.Solid{}, [2][4]int{}, e }
	if ctx == nil || !origin.IsFinite() || !finite(lower) || !finite(upper) || !finite(height) || !finite(tol) || lower <= 0 || upper <= 0 || height <= 0 || tol <= 0 {
		return bad(geom.ErrInvalid)
	}
	if e := ctx.Err(); e != nil {
		return bad(e)
	}
	add := func(a, b float64) (float64, error) {
		v, ok := new(big.Rat).Add(new(big.Rat).SetFloat64(a), new(big.Rat).SetFloat64(b)).Float64()
		if !ok || !finite(v) {
			return 0, geom.ErrUnrepresentable
		}
		return v, nil
	}
	var uv, world [2][4][]geom.Vec3
	var caps [2]geom.Frame
	for k, r := range []float64{lower, upper} {
		z := origin.Z
		var e error
		if k == 1 {
			z, e = add(z, height)
			if e != nil {
				return bad(e)
			}
		}
		caps[k], e = geom.NewFrame(geom.Vec3{X: origin.X, Y: origin.Y, Z: z}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
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
				wx, e := add(origin.X, x)
				if e != nil {
					return bad(e)
				}
				wy, e := add(origin.Y, y)
				if e != nil {
					return bad(e)
				}
				world[k][q] = append(world[k][q], geom.Vec3{X: wx, Y: wy, Z: z})
			}
		}
	}
	p, e := partsEndControls(ctx, uv, world, caps, tol)
	if e != nil {
		return bad(e)
	}
	return sewParts(ctx, p, tol)
}
