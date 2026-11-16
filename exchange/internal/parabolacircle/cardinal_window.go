package parabolacircle

import (
	"context"
	"fmt"
	"math/big"

	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// CardinalCircleWindow gives one oriented original circular arc by
// restriction of a finite-decimal full quarter carrier. All three clocks
// have domain [0, DomainEnd] and active interval [First, Last].
type CardinalCircleWindow struct {
	Model, UV              geom.ExactBezierSpan
	DomainEnd, First, Last int64
}

// PrepareCardinalCircleWindows uses only the original circle edges named by
// the source recipe. It requires a planar model arc and two original UV
// coedges with the same normalized polynomial clock. All coefficients of
// the derived restrictions are authenticated against the extracted spans.
func PrepareCardinalCircleWindows(ctx context.Context, b trimmed.ExactBezierBoundary, rings [2][]int,
	decimal func(*big.Rat) (string, error)) (map[int]CardinalCircleWindow, error) {
	return prepareCircleWindows(ctx, b, rings, decimal, false)
}

// PrepareAnchoredCircleWindows additionally tries exact quarter carriers
// starting at original vertices of the source-proved circle ring. This
// retains the linear active clock of subarcs authored from a non-cardinal
// junction, including subdivisions whose own endpoints have a different
// rational clock. Every carrier must match the two stored source UV laws.
func PrepareAnchoredCircleWindows(ctx context.Context, b trimmed.ExactBezierBoundary, ring []int,
	decimal func(*big.Rat) (string, error)) (map[int]CardinalCircleWindow, error) {
	return prepareCircleWindows(ctx, b, [2][]int{ring}, decimal, true)
}

func prepareCircleWindows(ctx context.Context, b trimmed.ExactBezierBoundary, rings [2][]int,
	decimal func(*big.Rat) (string, error), anchored bool) (map[int]CardinalCircleWindow, error) {
	if ctx == nil || decimal == nil {
		return nil, geom.ErrInvalid
	}
	windows := make(map[int]CardinalCircleWindow)
	for _, ring := range rings {
		var anchors []geom.Vec3
		if anchored {
			var err error
			anchors, err = circleRingVertices(b, ring)
			if err != nil {
				return nil, err
			}
		}
		for _, id := range ring {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			if id < 0 {
				continue
			}
			if id >= len(b.Edges) {
				return nil, geom.ErrConvergence
			}
			model := b.Edges[id].Model
			var uses []geom.ExactBezierSpan
			for _, face := range b.Faces {
				for _, loop := range face.Loops {
					for _, use := range loop {
						if use.Edge == id {
							if face.Spline == nil {
								return nil, geom.ErrConvergence
							}
							uses = append(uses, use.UV)
						}
					}
				}
			}
			if len(uses) != 2 {
				return nil, geom.ErrConvergence
			}
			if hasDecimalPoles(model, decimal) && hasDecimalPoles(uses[0], decimal) && hasDecimalPoles(uses[1], decimal) {
				continue
			}
			w, err := originalCircleWindow(model, [2]geom.ExactBezierSpan(uses), anchors...)
			if err != nil {
				return nil, fmt.Errorf("original circle edge %d: %w", id, err)
			}
			if !hasDecimalPoles(w.Model, decimal) || !hasDecimalPoles(w.UV, decimal) {
				return nil, geom.ErrUnrepresentable
			}
			windows[id] = w
		}
	}
	return windows, nil
}

func circleRingVertices(b trimmed.ExactBezierBoundary, ring []int) ([]geom.Vec3, error) {
	seen := make(map[int]bool, len(ring))
	var anchors []geom.Vec3
	for _, id := range ring {
		if id < 0 {
			continue
		}
		if id >= len(b.Edges) {
			return nil, geom.ErrConvergence
		}
		edge := b.Edges[id]
		for _, v := range [2]int{edge.Start, edge.End} {
			if v < 0 || v >= len(b.Vertices) || !b.Vertices[v].IsFinite() {
				return nil, geom.ErrConvergence
			}
			if !seen[v] {
				seen[v] = true
				anchors = append(anchors, b.Vertices[v])
			}
		}
	}
	return anchors, nil
}

func hasDecimalPoles(span geom.ExactBezierSpan, decimal func(*big.Rat) (string, error)) bool {
	for _, row := range span.Control {
		for _, x := range row[:3] {
			if _, err := decimal(new(big.Rat).Quo(x, row[3])); err != nil {
				return false
			}
		}
	}
	return true
}

func originalCircleWindow(model geom.ExactBezierSpan, uses [2]geom.ExactBezierSpan, anchors ...geom.Vec3) (CardinalCircleWindow, error) {
	var empty CardinalCircleWindow
	if len(model.Control) != 3 || model.Start != 0 || model.End != 1 {
		return empty, geom.ErrUnrepresentable
	}
	zero, one := new(big.Rat), big.NewRat(1, 1)
	mid := big.NewRat(1, 2)
	stations := [3]*big.Rat{zero, mid, one}
	var points [3][2]*big.Rat
	height := new(big.Rat).Quo(model.Control[0][2], model.Control[0][3])
	for i, t := range stations {
		h := circleWindowBlossom(model.Control, t, t)
		if new(big.Rat).Mul(height, h[3]).Cmp(h[2]) != 0 {
			return empty, geom.ErrUnrepresentable
		}
		for dim := range 2 {
			points[i][dim] = new(big.Rat).Quo(h[dim], h[3])
		}
	}
	center, radius, err := circleWindowCenter(points)
	if err != nil {
		return empty, err
	}
	rs := new(big.Rat).Mul(radius, radius)
	starts := make([][2]*big.Rat, 0, 4+len(anchors))
	for _, d := range [4][2]int64{{1, 0}, {0, 1}, {-1, 0}, {0, -1}} {
		starts = append(starts, [2]*big.Rat{
			new(big.Rat).Mul(radius, big.NewRat(d[0], 1)),
			new(big.Rat).Mul(radius, big.NewRat(d[1], 1)),
		})
	}
	for _, p := range anchors {
		if new(big.Rat).SetFloat64(p.Z).Cmp(height) != 0 {
			continue
		}
		x := new(big.Rat).Sub(new(big.Rat).SetFloat64(p.X), center[0])
		y := new(big.Rat).Sub(new(big.Rat).SetFloat64(p.Y), center[1])
		length := new(big.Rat).Add(new(big.Rat).Mul(x, x), new(big.Rat).Mul(y, y))
		if length.Cmp(rs) == 0 {
			starts = append(starts, [2]*big.Rat{x, y})
		}
	}
	for _, a := range starts {
		z := [2]*big.Rat{new(big.Rat).Neg(a[1]), new(big.Rat).Set(a[0])}
		limits := [2]*big.Rat{}
		for j, p := range [2][2]*big.Rat{points[0], points[2]} {
			radial := [2]*big.Rat{new(big.Rat).Sub(p[0], center[0]), new(big.Rat).Sub(p[1], center[1])}
			dot := new(big.Rat).Add(new(big.Rat).Mul(a[0], radial[0]), new(big.Rat).Mul(a[1], radial[1]))
			denominator := new(big.Rat).Add(rs, dot)
			if denominator.Sign() <= 0 {
				break
			}
			cross := new(big.Rat).Sub(new(big.Rat).Mul(a[0], radial[1]), new(big.Rat).Mul(a[1], radial[0]))
			t := cross.Quo(cross, denominator)
			if t.Sign() < 0 || t.Cmp(one) > 0 {
				break
			}
			limits[j] = t
		}
		if limits[1] == nil || limits[0].Cmp(limits[1]) == 0 {
			continue
		}
		modelParent := geom.ExactBezierSpan{End: 1, Control: make([][4]*big.Rat, 3)}
		for dim := range 2 {
			start := new(big.Rat).Add(center[dim], a[dim])
			end := new(big.Rat).Add(center[dim], z[dim])
			modelParent.Control[0][dim] = start
			modelParent.Control[1][dim] = new(big.Rat).Add(start, z[dim])
			modelParent.Control[2][dim] = new(big.Rat).Mul(big.NewRat(2, 1), end)
		}
		for j, w := range [3]int64{1, 1, 2} {
			modelParent.Control[j][3] = big.NewRat(w, 1)
			modelParent.Control[j][2] = new(big.Rat).Mul(height, modelParent.Control[j][3])
		}
		if limits[0].Cmp(limits[1]) > 0 {
			modelParent.Control[0], modelParent.Control[2] = modelParent.Control[2], modelParent.Control[0]
			for _, t := range limits {
				t.Sub(one, t)
			}
		}
		uvParent := geom.ExactBezierSpan{End: 1, Control: append([][4]*big.Rat(nil), modelParent.Control...)}
		for j := range uvParent.Control {
			uvParent.Control[j][2] = zero
		}
		if !matchesCircleWindow(model, modelParent, limits) || !matchesCircleWindow(uses[0], uvParent, limits) ||
			!matchesCircleWindow(uses[1], uvParent, limits) {
			continue
		}
		den0, den1 := limits[0].Denom(), limits[1].Denom()
		gcd := new(big.Int).GCD(nil, nil, den0, den1)
		scale := new(big.Int).Mul(new(big.Int).Quo(den0, gcd), den1)
		if !scale.IsInt64() || scale.Int64() > 65536 {
			return empty, geom.ErrConvergence
		}
		stretch := new(big.Rat).SetInt(scale)
		first, last := new(big.Rat).Mul(limits[0], stretch), new(big.Rat).Mul(limits[1], stretch)
		return CardinalCircleWindow{Model: modelParent, UV: uvParent, DomainEnd: scale.Int64(), First: first.Num().Int64(), Last: last.Num().Int64()}, nil
	}
	return empty, geom.ErrUnrepresentable
}

func circleWindowCenter(p [3][2]*big.Rat) ([2]*big.Rat, *big.Rat, error) {
	var u, v [2]*big.Rat
	for j := range u {
		u[j] = new(big.Rat).Sub(p[1][j], p[0][j])
		v[j] = new(big.Rat).Sub(p[2][j], p[0][j])
	}
	usq := new(big.Rat).Add(new(big.Rat).Mul(u[0], u[0]), new(big.Rat).Mul(u[1], u[1]))
	vsq := new(big.Rat).Add(new(big.Rat).Mul(v[0], v[0]), new(big.Rat).Mul(v[1], v[1]))
	det := new(big.Rat).Sub(new(big.Rat).Mul(u[0], v[1]), new(big.Rat).Mul(u[1], v[0]))
	if det.Sign() == 0 {
		return [2]*big.Rat{}, nil, geom.ErrUnrepresentable
	}
	det.Mul(det, big.NewRat(2, 1))
	x := new(big.Rat).Sub(new(big.Rat).Mul(usq, v[1]), new(big.Rat).Mul(vsq, u[1]))
	y := new(big.Rat).Sub(new(big.Rat).Mul(vsq, u[0]), new(big.Rat).Mul(usq, v[0]))
	x.Quo(x, det)
	y.Quo(y, det)
	rs := new(big.Rat).Add(new(big.Rat).Mul(x, x), new(big.Rat).Mul(y, y))
	n, d := new(big.Int).Sqrt(rs.Num()), new(big.Int).Sqrt(rs.Denom())
	if new(big.Int).Mul(n, n).Cmp(rs.Num()) != 0 || new(big.Int).Mul(d, d).Cmp(rs.Denom()) != 0 {
		return [2]*big.Rat{}, nil, geom.ErrUnrepresentable
	}
	c := [2]*big.Rat{new(big.Rat).Add(p[0][0], x), new(big.Rat).Add(p[0][1], y)}
	return c, new(big.Rat).SetFrac(n, d), nil
}

func matchesCircleWindow(span, parent geom.ExactBezierSpan, t [2]*big.Rat) bool {
	if len(span.Control) != 3 || span.Start != 0 || span.End != 1 {
		return false
	}
	h := [3][4]*big.Rat{
		circleWindowBlossom(parent.Control, t[0], t[0]),
		circleWindowBlossom(parent.Control, t[0], t[1]),
		circleWindowBlossom(parent.Control, t[1], t[1]),
	}
	for i, row := range span.Control {
		for j, v := range row {
			if new(big.Rat).Mul(v, h[0][3]).Cmp(new(big.Rat).Mul(h[i][j], span.Control[0][3])) != 0 {
				return false
			}
		}
	}
	return true
}

func circleWindowBlossom(h [][4]*big.Rat, a, b *big.Rat) (out [4]*big.Rat) {
	ac := new(big.Rat).Sub(big.NewRat(1, 1), a)
	bc := new(big.Rat).Sub(big.NewRat(1, 1), b)
	basis := [3]*big.Rat{
		new(big.Rat).Mul(ac, bc),
		new(big.Rat).Add(new(big.Rat).Mul(a, bc), new(big.Rat).Mul(ac, b)),
		new(big.Rat).Mul(a, b),
	}
	for j := range out {
		out[j] = new(big.Rat)
		for i, w := range basis {
			out[j].Add(out[j], new(big.Rat).Mul(h[i][j], w))
		}
	}
	return out
}
