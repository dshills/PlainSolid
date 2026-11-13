package trimmed

import (
	"context"
	"math/big"
	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
)

func reversePolynomialPlaneCurve(c PCurve) PCurve {
	switch q := c.(type) {
	case ExactQuadraticUV:
		return reversePlaneQuadratic(q)
	case ExactBezierUV:
		return q.reversed()
	case SplineUV:
		q.Reversed = !q.Reversed
		return q
	}
	return c
}
func validatePolynomialPlanePart(ctx context.Context, p SewFace) error {
	r, e := ValidateFaceTrimLoops(ctx, p.Face, trimvalidate.Options{})
	if e != nil {
		return e
	}
	if r.Status != trimvalidate.Certified {
		return geom.ErrConvergence
	}
	return nil
}

// Both charts have exactly affine coordinates on the chosen projection on
// this section's original knot cell. Transport every UV Bernstein control
// through those affine maps. The caller independently proves the full model
// composition before accepting the edge.
func polynomialPlaneCapCurve(ctx context.Context, source *geom.SurfacePatch, c PCurve, cap *geom.SurfacePatch, axes [2]int, o SplineRectangleEmbeddingOptions) (PCurve, error) {
	h, e := polynomialPlaneControls(c)
	if e != nil {
		return nil, e
	}
	opt := geom.SurfacePatchEvaluationOptions{MaxOperations: o.Patches.MaxOperations, MaxCoefficientBits: o.Patches.MaxCoefficientBits}
	sourceOrigin := [2]*big.Rat{}
	// Use an interior anchor to select the actual knot side. A global chart
	// corner would use a different affine projection after a C0 crease.
	for axis := range sourceOrigin {
		last := h[len(h)-1]
		a := new(big.Rat).Quo(h[0][axis], h[0][2])
		b := new(big.Rat).Quo(last[axis], last[2])
		sourceOrigin[axis] = new(big.Rat).Quo(new(big.Rat).Add(a, b), big.NewRat(2, 1))
	}
	sj, e := source.EvaluateRational(ctx, sourceOrigin[0], sourceOrigin[1], opt)
	if e != nil {
		return nil, e
	}
	d := cap.Domain()
	j, e := cap.EvaluateRational(ctx, ratInset(d.UMin), ratInset(d.VMin), opt)
	if e != nil {
		return nil, e
	}
	a, b := axes[0], axes[1]
	det := new(big.Rat).Sub(new(big.Rat).Mul(j.Du[a], j.Dv[b]), new(big.Rat).Mul(j.Du[b], j.Dv[a]))
	if det.Sign() == 0 {
		return nil, geom.ErrConvergence
	}
	control := make([][3]*big.Rat, len(h))
	for i, p := range h {
		var xy [2]*big.Rat
		for k, axis := range axes {
			xy[k] = new(big.Rat).Mul(new(big.Rat).Sub(sj.Point[axis], j.Point[axis]), p[2])
			for a, derivative := range [2]*big.Rat{sj.Du[axis], sj.Dv[axis]} {
				delta := new(big.Rat).Sub(p[a], new(big.Rat).Mul(sourceOrigin[a], p[2]))
				xy[k].Add(xy[k], new(big.Rat).Mul(derivative, delta))
			}
		}
		u := new(big.Rat).Quo(new(big.Rat).Sub(new(big.Rat).Mul(xy[0], j.Dv[b]), new(big.Rat).Mul(xy[1], j.Dv[a])), det)
		v := new(big.Rat).Quo(new(big.Rat).Sub(new(big.Rat).Mul(j.Du[a], xy[1]), new(big.Rat).Mul(j.Du[b], xy[0])), det)
		control[i] = [3]*big.Rat{
			u.Add(u, new(big.Rat).Mul(ratInset(d.UMin), p[2])),
			v.Add(v, new(big.Rat).Mul(ratInset(d.VMin), p[2])), p[2],
		}
	}
	switch c.(type) {
	case ExactQuadraticUV:
		return NewExactQuadraticUV([3][3]*big.Rat{control[0], control[1], control[2]})
	case ExactBezierUV:
		return NewExactBezierUV(control)
	}
	return polynomialNativeTrim(control)
}

// Endpoint incidence selects a single cycle; exact polynomial Green integration
// fixes its orientation. The subsequent trim validator independently proves
// Jordan topology, so nonzero signed area alone never certifies embeddedness.
func polynomialPlaneCycle(ctx context.Context, arcs []PCurve) ([]PCurve, error) {
	if len(arcs) == 0 {
		return nil, geom.ErrConvergence
	}
	degree := map[geom.Vec2]int{}
	ends := make([][2]geom.Vec2, len(arcs))
	for i, q := range arcs {
		_, p, e := polynomialPlaneEndpoints(q)
		if e != nil {
			return nil, e
		}
		ends[i] = p
		degree[p[0]]++
		degree[p[1]]++
	}
	for _, n := range degree {
		if n != 2 {
			return nil, geom.ErrConvergence
		}
	}
	loop := []PCurve{arcs[0]}
	used := map[int]bool{0: true}
	end := ends[0][1]
	for len(loop) < len(arcs) {
		found := -1
		var next PCurve
		for i, q := range arcs {
			if used[i] {
				continue
			}
			p := ends[i]
			if p[0] == end || p[1] == end {
				if found >= 0 {
					return nil, geom.ErrConvergence
				}
				found = i
				next = q
				if p[1] == end {
					next = reversePolynomialPlaneCurve(q)
				}
			}
		}
		if found < 0 {
			return nil, geom.ErrConvergence
		}
		used[found] = true
		loop = append(loop, next)
		_, p, e := polynomialPlaneEndpoints(next)
		if e != nil {
			return nil, e
		}
		end = p[1]
	}
	if end != ends[0][0] {
		return nil, geom.ErrConvergence
	}
	reverse := func() {
		for i, j := 0, len(loop)-1; i <= j; i, j = i+1, j-1 {
			a, b := loop[i], loop[j]
			loop[i], loop[j] = reversePolynomialPlaneCurve(b), reversePolynomialPlaneCurve(a)
		}
	}
	for _, q := range loop {
		h, e := polynomialPlaneControls(q)
		if e != nil {
			return nil, e
		}
		for _, c := range h {
			if c[2].Cmp(big.NewRat(1, 1)) != 0 {
				// The exact trim validator encloses rational signed area and
				// proves a simple loop. Endpoint polygon area is not sufficient.
				for range 2 {
					var face Face
					for _, curve := range loop {
						_, p, e := polynomialPlaneEndpoints(curve)
						if e != nil {
							return nil, e
						}
						face.Outer = append(face.Outer, Use{UVStart: p[0], UVEnd: p[1], Curve: curve})
					}
					report, e := ValidateFaceTrimLoops(ctx, face, trimvalidate.Options{})
					if e != nil {
						return nil, e
					}
					if report.Status == trimvalidate.Certified {
						return loop, nil
					}
					reverse()
				}
				return nil, geom.ErrConvergence
			}
		}
	}
	area := new(big.Rat)
	for _, q := range loop {
		h, e := polynomialPlaneControls(q)
		if e != nil {
			return nil, e
		}
		n := len(h) - 1
		power := make([][2]*big.Rat, n+1)
		for k := 0; k <= n; k++ {
			for a := 0; a < 2; a++ {
				power[k][a] = new(big.Rat)
				for i := 0; i <= k; i++ {
					v := polynomialChoose(n, i) * polynomialChoose(n-i, k-i)
					if (k-i)%2 != 0 {
						v = -v
					}
					power[k][a].Add(power[k][a], new(big.Rat).Mul(big.NewRat(v, 1), h[i][a]))
				}
			}
		}
		for i := 0; i <= n; i++ {
			for j := 1; j <= n; j++ {
				v := new(big.Rat).Sub(new(big.Rat).Mul(power[i][0], power[j][1]), new(big.Rat).Mul(power[i][1], power[j][0]))
				area.Add(area, v.Mul(v, big.NewRat(int64(j), int64(i+j))))
			}
		}
	}
	if area.Sign() == 0 {
		return nil, geom.ErrConvergence
	}
	if area.Sign() < 0 {
		reverse()
	}
	return loop, nil
}
