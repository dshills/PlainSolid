package trimmed

import (
	"context"
	"math/big"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"runtime"
)

// Split only the numerical integration domain. Surface evaluators, original
// trims and topology remain unchanged. Exact clipping ensures adjacent pieces
// use identical knot intersections before conversion to quadrature coordinates.
func massSplineKnotLines(surface geom.Surface) (u, v []float64) {
	base, _ := parametric.SurfaceMapping(surface)
	switch p := base.(type) {
	case *geom.BSplineSurface:
		data := p.Data()
		unique := func(knots []float64) []float64 {
			var out []float64
			for _, x := range knots {
				if len(out) == 0 || out[len(out)-1] != x {
					out = append(out, x)
				}
			}
			return out
		}
		return unique(data.KnotsU), unique(data.KnotsV)
	case *geom.SurfacePatch:
		return p.KnotBreaks()
	}
	return nil, nil
}

type massExactUV [2]*big.Rat

func massEqualUV(a, b massExactUV) bool { return a[0].Cmp(b[0]) == 0 && a[1].Cmp(b[1]) == 0 }
func massClipHalfplane(poly []massExactUV, axis int, knot *big.Rat, positive bool) []massExactUV {
	var out []massExactUV
	inside := func(p massExactUV) bool {
		sign := p[axis].Cmp(knot)
		if positive {
			return sign >= 0
		}
		return sign <= 0
	}
	appendPoint := func(p massExactUV) {
		if len(out) == 0 || !massEqualUV(out[len(out)-1], p) {
			out = append(out, p)
		}
	}
	previous := poly[len(poly)-1]
	previousInside := inside(previous)
	for _, current := range poly {
		currentInside := inside(current)
		if currentInside != previousInside {
			t := new(big.Rat).Quo(new(big.Rat).Sub(knot, previous[axis]), new(big.Rat).Sub(current[axis], previous[axis]))
			var hit massExactUV
			hit[axis] = new(big.Rat).Set(knot)
			other := 1 - axis
			hit[other] = new(big.Rat).Add(previous[other], new(big.Rat).Mul(t, new(big.Rat).Sub(current[other], previous[other])))
			appendPoint(hit)
		}
		if currentInside {
			appendPoint(current)
		}
		runtime.KeepAlive(&previous) // golang/go#82026: keeps previous in the stack map. Remove once the Go fix is in use.
		previous, previousInside = current, currentInside
	}
	if len(out) > 1 && massEqualUV(out[0], out[len(out)-1]) {
		out = out[:len(out)-1]
	}
	return out
}
func massExactTriangleSign(a, b, c massExactUV) int {
	return new(big.Rat).Sub(new(big.Rat).Mul(new(big.Rat).Sub(b[0], a[0]), new(big.Rat).Sub(c[1], a[1])), new(big.Rat).Mul(new(big.Rat).Sub(b[1], a[1]), new(big.Rat).Sub(c[0], a[0]))).Sign()
}
func splitMassTriangleAtKnots(ctx context.Context, triangle [3]geom.Vec2, u, v []float64, limit int) ([][3]geom.Vec2, error) {
	if limit < 1 {
		return nil, parametric.ErrBudget
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	if len(u) == 0 && len(v) == 0 {
		return [][3]geom.Vec2{triangle}, nil
	}
	poly := make([]massExactUV, 3)
	for i, p := range triangle {
		poly[i] = massExactUV{new(big.Rat).SetFloat64(p.X), new(big.Rat).SetFloat64(p.Y)}
		if poly[i][0] == nil || poly[i][1] == nil {
			return nil, geom.ErrInvalid
		}
	}
	polygons := [][]massExactUV{poly}
	for axis, knots := range [2][]float64{u, v} {
		for _, k := range knots {
			if e := ctx.Err(); e != nil {
				return nil, e
			}
			knot := new(big.Rat).SetFloat64(k)
			if knot == nil {
				return nil, geom.ErrInvalid
			}
			var next [][]massExactUV
			for _, polygon := range polygons {
				below, above := false, false
				for _, p := range polygon {
					comparison := p[axis].Cmp(knot)
					below = below || comparison < 0
					above = above || comparison > 0
				}
				if below && above {
					for _, positive := range []bool{false, true} {
						piece := massClipHalfplane(polygon, axis, knot, positive)
						if len(piece) >= 3 {
							next = append(next, piece)
						}
					}
				} else {
					next = append(next, polygon)
				}
				if len(next) > limit {
					return nil, parametric.ErrBudget
				}
			}
			polygons = next
		}
	}
	var result [][3]geom.Vec2
	for _, polygon := range polygons {
		for i := 1; i+1 < len(polygon); i++ {
			exact := [3]massExactUV{polygon[0], polygon[i], polygon[i+1]}
			sign := massExactTriangleSign(exact[0], exact[1], exact[2])
			if sign == 0 {
				continue
			}
			if len(result) >= limit {
				return nil, parametric.ErrBudget
			}
			var t [3]geom.Vec2
			for j, p := range exact {
				t[j].X, _ = p[0].Float64()
				t[j].Y, _ = p[1].Float64()
			}
			represented, e := geom.Orient2D(t[0], t[1], t[2])
			if e != nil {
				return nil, e
			}
			if represented != sign {
				return nil, geom.ErrUnrepresentable
			}
			result = append(result, t)
		}
	}
	return result, nil
}
