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

// Both charts have exactly affine model XY coordinates. Transport every UV
// Bernstein control through those exact affine maps; the caller independently
// proves the full model composition, including Z, before accepting the edge.
func polynomialPlaneCapCurve(ctx context.Context, source *geom.SurfacePatch, c PCurve, cap *geom.SurfacePatch, o SplineRectangleEmbeddingOptions) (PCurve, error) {
	h, e := polynomialPlaneControls(c)
	if e != nil {
		return nil, e
	}
	sd, d := source.Domain(), cap.Domain()
	opt := geom.SurfacePatchEvaluationOptions{MaxOperations: o.Patches.MaxOperations, MaxCoefficientBits: o.Patches.MaxCoefficientBits}
	sj, e := source.EvaluateRational(ctx, ratInset(sd.UMin), ratInset(sd.VMin), opt)
	if e != nil {
		return nil, e
	}
	j, e := cap.EvaluateRational(ctx, ratInset(d.UMin), ratInset(d.VMin), opt)
	if e != nil {
		return nil, e
	}
	det := new(big.Rat).Sub(new(big.Rat).Mul(j.Du[0], j.Dv[1]), new(big.Rat).Mul(j.Du[1], j.Dv[0]))
	if det.Sign() == 0 {
		return nil, geom.ErrConvergence
	}
	control := make([][3]*big.Rat, len(h))
	for i, p := range h {
		var xy [2]*big.Rat
		for a := range xy {
			xy[a] = new(big.Rat).Add(sj.Point[a], new(big.Rat).Add(new(big.Rat).Mul(sj.Du[a], new(big.Rat).Sub(p[0], ratInset(sd.UMin))), new(big.Rat).Mul(sj.Dv[a], new(big.Rat).Sub(p[1], ratInset(sd.VMin)))))
			xy[a].Sub(xy[a], j.Point[a])
		}
		u := new(big.Rat).Quo(new(big.Rat).Sub(new(big.Rat).Mul(xy[0], j.Dv[1]), new(big.Rat).Mul(xy[1], j.Dv[0])), det)
		v := new(big.Rat).Quo(new(big.Rat).Sub(new(big.Rat).Mul(j.Du[0], xy[1]), new(big.Rat).Mul(j.Du[1], xy[0])), det)
		control[i] = [3]*big.Rat{u.Add(u, ratInset(d.UMin)), v.Add(v, ratInset(d.VMin)), big.NewRat(1, 1)}
	}
	return polynomialNativeTrim(control)
}

// Endpoint incidence selects a single cycle; exact polynomial Green integration
// fixes its orientation. The subsequent trim validator independently proves
// Jordan topology, so nonzero signed area alone never certifies embeddedness.
func polynomialPlaneCycle(arcs []PCurve) ([]PCurve, error) {
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
		for i, j := 0, len(loop)-1; i <= j; i, j = i+1, j-1 {
			a, b := loop[i], loop[j]
			loop[i], loop[j] = reversePolynomialPlaneCurve(b), reversePolynomialPlaneCurve(a)
		}
	}
	return loop, nil
}
