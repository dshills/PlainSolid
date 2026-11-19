package trimmed

import (
	"math"
	"math/big"
	"runtime"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

type analyticUVLine struct {
	surface geom.Surface
	uv      [2][2]*big.Rat // endpoint, chart axis
}

func analyticUseLine(s geom.Surface, u Use) (analyticUVLine, bool) {
	if u.Curve != nil || !uvFinite(u.UVStart) || !uvFinite(u.UVEnd) || u.UVStart == u.UVEnd {
		return analyticUVLine{}, false
	}
	a, b := u.UVStart, u.UVEnd
	if u.Reversed {
		a, b = b, a
	}
	return analyticUVLine{s, [2][2]*big.Rat{{ratInset(a.X), ratInset(a.Y)}, {ratInset(b.X), ratInset(b.Y)}}}, true
}

func analyticModelLine(e parametric.Edge) (analyticUVLine, bool) {
	s, a, b, ok := parametric.IsoCurveParameters(e.Curve)
	if !ok {
		base, m := parametric.CurveMapping(e.Curve)
		c, isTrim := base.(surfaceTrimCurve)
		if !isTrim || c.curve != nil {
			return analyticUVLine{}, false
		}
		s, a, b = c.surface, c.a, c.b
		if m != geom.Identity() {
			s = parametric.MapSurface(s, m)
		}
	}
	if !finite(e.Range.Min) || !finite(e.Range.Max) || e.Range.Min < 0 || e.Range.Max > 1 || e.Range.Min >= e.Range.Max {
		return analyticUVLine{}, false
	}
	x := analyticUVLine{surface: s}
	for i, t := range [2]float64{e.Range.Min, e.Range.Max} {
		for axis, p := range [2][2]float64{{a.X, b.X}, {a.Y, b.Y}} {
			x.uv[i][axis] = new(big.Rat).Sub(ratInset(p[1]), ratInset(p[0]))
			x.uv[i][axis].Mul(x.uv[i][axis], ratInset(t))
			x.uv[i][axis].Add(x.uv[i][axis], ratInset(p[0]))
		}
	}
	return x, true
}

func sameAnalyticEdgeUse(e parametric.Edge, s geom.Surface, use Use) bool {
	u, ok := analyticUseLine(s, use)
	if !ok || e.Curve == nil {
		return false
	}
	a, ok := analyticModelLine(e)
	return ok && sameAnalyticUVLines(a, u)
}

func analyticPoleSquaredError(s geom.Surface, use Use, vertex geom.Vec3) *big.Rat {
	u, ok := analyticUseLine(s, use)
	if !ok || !vertex.IsFinite() {
		return nil
	}
	p, ok := analyticPolePoint(u)
	if !ok {
		return nil
	}
	distance := new(big.Rat)
	for i, x := range [3]float64{vertex.X, vertex.Y, vertex.Z} {
		d := new(big.Rat).Sub(p[i], ratInset(x))
		distance.Add(distance, d.Mul(d, d))
	}
	return distance
}

func isCanonicalAnalyticLoop(e parametric.Edge) bool {
	x, ok := analyticModelLine(e)
	if !ok {
		return false
	}
	s, _ := parametric.SurfaceMapping(x.surface)
	periodic, known := analyticPeriods(s)
	if !known {
		return false
	}
	for axis, wraps := range periodic {
		if !wraps || x.uv[0][axis^1].Cmp(x.uv[1][axis^1]) != 0 {
			continue
		}
		a, b := x.uv[0][axis], x.uv[1][axis]
		if a.Cmp(b) > 0 {
			a, b = b, a
		}
		if a.Sign() == 0 && b.Cmp(ratInset(2*math.Pi)) == 0 {
			_, collapsed := analyticPolePoint(x)
			return !collapsed
		}
	}
	return false
}

func sameAnalyticModelEdges(a, b parametric.Edge, reverse bool) bool {
	x, ok := analyticModelLine(a)
	y, supported := analyticModelLine(b)
	if !ok || !supported {
		return false
	}
	if reverse {
		y.uv[0], y.uv[1] = y.uv[1], y.uv[0]
	}
	return sameAnalyticUVLines(x, y)
}

func sameAnalyticUVLines(a, b analyticUVLine) bool {
	if sameWrappedUVLines(a, b) {
		return true
	}
	x, mx := parametric.SurfaceMapping(a.surface)
	y, my := parametric.SurfaceMapping(b.surface)
	periodic, known := analyticPeriods(x)
	if known && mx == my && sameAnalyticSupport(x, y) {
		same := true
		for i := range periodic {
			same = same && equalAnalyticParameterLine(a, b, i, periodic[i])
		}
		if same {
			return true
		}
	}
	// Different analytic charts can describe the same circle, notably
	// the lateral and polar cap charts of cylinders and cones.
	p, ok := analyticCircleLine(a)
	q, supported := analyticCircleLine(b)
	if !ok || !supported || !equalAnalyticParameterLine(a, b, 0, true) {
		return false
	}
	for i := range p {
		for j := range p[i] {
			if p[i][j].Cmp(q[i][j]) != 0 {
				return false
			}
		}
	}
	return true
}

func equalAnalyticParameterLine(a, b analyticUVLine, axis int, periodic bool) bool {
	x := new(big.Rat).Sub(a.uv[0][axis], b.uv[0][axis])
	y := new(big.Rat).Sub(a.uv[1][axis], b.uv[1][axis])
	if x.Cmp(y) != 0 {
		return false
	}
	if x.Sign() == 0 {
		return true
	}
	if !periodic || a.uv[0][axis].Cmp(a.uv[1][axis]) != 0 || b.uv[0][axis].Cmp(b.uv[1][axis]) != 0 {
		return false
	}
	// Treat the two canonical seam parameters as the same ideal angle.
	// A large multiple of represented 2π may accrue a real phase error.
	p, q := a.uv[0][axis], b.uv[0][axis]
	runtime.KeepAlive(&a) // Stack-map defect, js/wasm by-value parameter form (see docs/STATUS.md): keeps a in the stack map. Remove once Go is fixed.
	runtime.KeepAlive(&b)
	return p.Sign() == 0 && q.Cmp(ratInset(2*math.Pi)) == 0 || q.Sign() == 0 && p.Cmp(ratInset(2*math.Pi)) == 0
}

func analyticPeriods(s geom.Surface) ([2]bool, bool) {
	switch s.(type) {
	case geom.CylinderSurface, geom.ConeSurface, geom.SphereSurface:
		return [2]bool{true, false}, true
	case geom.TorusSurface:
		return [2]bool{true, true}, true
	case geom.PlaneSurface:
		return [2]bool{}, true
	}
	_, ok := parametric.PolarPlaneFrame(s)
	return [2]bool{true, false}, ok
}

func sameAnalyticSupport(a, b geom.Surface) bool {
	switch x := a.(type) {
	case geom.CylinderSurface:
		y, ok := b.(geom.CylinderSurface)
		return ok && x == y
	case geom.ConeSurface:
		y, ok := b.(geom.ConeSurface)
		return ok && x == y
	case geom.SphereSurface:
		y, ok := b.(geom.SphereSurface)
		return ok && x == y
	case geom.TorusSurface:
		y, ok := b.(geom.TorusSurface)
		return ok && x == y
	case geom.PlaneSurface:
		y, ok := b.(geom.PlaneSurface)
		return ok && x == y
	}
	x, ok := parametric.PolarPlaneFrame(a)
	y, supported := parametric.PolarPlaneFrame(b)
	return ok && supported && x == y
}

// Circle coefficients are center, cosine vector and sine vector in model
// space. All comparisons of represented frame data are exact rational ones.
func analyticCircleLine(x analyticUVLine) ([3][3]*big.Rat, bool) {
	var out [3][3]*big.Rat
	if x.uv[0][1].Cmp(x.uv[1][1]) != 0 || x.uv[0][0].Cmp(x.uv[1][0]) == 0 {
		return out, false
	}
	s, m := parametric.SurfaceMapping(x.surface)
	v := x.uv[0][1]
	z := v
	var f geom.Frame
	var radius *big.Rat
	switch a := s.(type) {
	case geom.CylinderSurface:
		frame, r := a.Parameters()
		f, radius = frame, ratInset(r)
	case geom.ConeSurface:
		frame, slope := a.Parameters()
		f, radius = frame, new(big.Rat).Mul(v, ratInset(slope))
	default:
		frame, ok := parametric.PolarPlaneFrame(s)
		if !ok {
			return out, false
		}
		f, radius, z = frame, v, new(big.Rat)
	}
	if radius.Sign() <= 0 {
		return out, false
	}
	out[0] = analyticModelVector(m, f.Origin(), f.ZAxis(), z, true)
	out[1] = analyticModelVector(m, geom.Vec3{}, f.XAxis(), radius, false)
	out[2] = analyticModelVector(m, geom.Vec3{}, f.YAxis(), radius, false)
	return out, true
}

func analyticPolePoint(x analyticUVLine) ([3]*big.Rat, bool) {
	if x.uv[0][1].Cmp(x.uv[1][1]) != 0 || x.uv[0][0].Cmp(x.uv[1][0]) == 0 {
		return [3]*big.Rat{}, false
	}
	s, m := parametric.SurfaceMapping(x.surface)
	v := x.uv[0][1]
	switch a := s.(type) {
	case geom.SphereSurface:
		f, r := a.Parameters()
		if v.Cmp(ratInset(math.Pi/2)) == 0 {
			return analyticModelVector(m, f.Origin(), f.ZAxis(), ratInset(r), true), true
		}
		if v.Cmp(ratInset(-math.Pi/2)) == 0 {
			return analyticModelVector(m, f.Origin(), f.ZAxis(), ratInset(-r), true), true
		}
	case geom.ConeSurface:
		f, _ := a.Parameters()
		if v.Sign() == 0 {
			return analyticModelVector(m, f.Origin(), geom.Vec3{}, v, true), true
		}
	default:
		if f, ok := parametric.PolarPlaneFrame(s); ok && v.Sign() == 0 {
			return analyticModelVector(m, f.Origin(), geom.Vec3{}, v, true), true
		}
	}
	return [3]*big.Rat{}, false
}

func analyticModelVector(m geom.Affine, origin, direction geom.Vec3, scale *big.Rat, point bool) [3]*big.Rat {
	p, d := [3]float64{origin.X, origin.Y, origin.Z}, [3]float64{direction.X, direction.Y, direction.Z}
	var x, out [3]*big.Rat
	for j := range x {
		x[j] = new(big.Rat).Add(ratInset(p[j]), new(big.Rat).Mul(ratInset(d[j]), scale))
	}
	for i := range out {
		out[i] = new(big.Rat)
		if point {
			out[i].SetFloat64(m[i][3])
		}
		for j, v := range x {
			out[i].Add(out[i], new(big.Rat).Mul(ratInset(m[i][j]), v))
		}
	}
	return out
}
