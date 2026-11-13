package trimmed

import (
	"context"
	"math/big"

	"plainsolid/geom"
)

// SplineTriangleDeviation encloses the complete image of a represented UV
// triangle and bounds its squared distance to a closed model triangle.
// SquaredDistance is caller-owned exact rational arithmetic.
type SplineTriangleDeviation struct {
	Bounds          geom.Box
	SquaredDistance *big.Rat
	// BothDirections certifies a shared UV barycentric correspondence and
	// therefore both directed Hausdorff bounds (currently single-span biquadratic or lower).
	BothDirections bool
}

// BoundSplineTriangleDeviation certifies one-sided deviation from the spline
// image of the UV triangle to the supplied model triangle. The UV rectangle's
// positive control hull encloses the whole image. Distance squared to a filled
// triangle is convex, so its maximum over that box is attained at a corner.
// Model vertices may be rounded; their represented coordinates define the
// target triangle exactly. Both triangles must be nondegenerate.
//
// Single-span biquadratic or lower patches use an exact degree-at-most-five Bernstein
// correspondence bound and certify both directions; affine exact vertices give
// zero error. Other patches use the conservative box bound, which can converge
// linearly and does not prove reverse Hausdorff distance. Neither proves embedding,
// normals, mesh conformity, or topology. Wrapped supports can supply their
// FundamentalPatch for triangles inside one fundamental rectangle. Exact trim
// work uses the immutable patch's construction limits. Cancellation is checked
// before and after the bounded trim stage. No partial certificate is returned.
func BoundSplineTriangleDeviation(ctx context.Context, patch *geom.SurfacePatch, uv [3]geom.Vec2, model [3]geom.Vec3) (SplineTriangleDeviation, error) {
	empty := SplineTriangleDeviation{}
	if ctx == nil || patch == nil {
		return empty, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return empty, e
	}
	for _, p := range uv {
		if !finite(p.X) || !finite(p.Y) {
			return empty, geom.ErrInvalid
		}
	}
	for _, p := range model {
		if !p.IsFinite() {
			return empty, geom.ErrInvalid
		}
	}
	// Exact UV orientation avoids silently accepting rounded-away area.
	dx1 := new(big.Rat).Sub(ratInset(uv[1].X), ratInset(uv[0].X))
	dy1 := new(big.Rat).Sub(ratInset(uv[1].Y), ratInset(uv[0].Y))
	dx2 := new(big.Rat).Sub(ratInset(uv[2].X), ratInset(uv[0].X))
	dy2 := new(big.Rat).Sub(ratInset(uv[2].Y), ratInset(uv[0].Y))
	area := new(big.Rat).Sub(new(big.Rat).Mul(dx1, dy2), new(big.Rat).Mul(dy1, dx2))
	if area.Sign() == 0 {
		return empty, geom.ErrDegenerate
	}
	tri := newExactBoundTriangle(model)
	if tri.det.Sign() == 0 {
		return empty, geom.ErrDegenerate
	}
	d := geom.ParameterRectangle{UMin: uv[0].X, UMax: uv[0].X, VMin: uv[0].Y, VMax: uv[0].Y}
	for _, p := range uv[1:] {
		d.UMin = min(d.UMin, p.X)
		d.UMax = max(d.UMax, p.X)
		d.VMin = min(d.VMin, p.Y)
		d.VMax = max(d.VMax, p.Y)
	}
	restricted, e := patch.Trim(d)
	if e != nil {
		return empty, e
	}
	box, e := restricted.Bounds()
	if e != nil {
		return empty, e
	}
	if e := ctx.Err(); e != nil {
		return empty, e
	}
	if bound, ok := bilinearCorrespondenceBound(restricted, uv, model); ok {
		return SplineTriangleDeviation{Bounds: box, SquaredDistance: bound, BothDirections: true}, nil
	}
	if bound, ok := biquadraticCorrespondenceBound(restricted, uv, model); ok {
		return SplineTriangleDeviation{Bounds: box, SquaredDistance: bound, BothDirections: true}, nil
	}
	if bound, ok := rationalBiquadraticCorrespondenceBound(restricted, uv, model); ok {
		return SplineTriangleDeviation{Bounds: box, SquaredDistance: bound, BothDirections: true}, nil
	}
	bound := new(big.Rat)
	for i := 0; i < 8; i++ {
		p := geom.Vec3{X: box.Min.X, Y: box.Min.Y, Z: box.Min.Z}
		if i&1 != 0 {
			p.X = box.Max.X
		}
		if i&2 != 0 {
			p.Y = box.Max.Y
		}
		if i&4 != 0 {
			p.Z = box.Max.Z
		}
		distance := tri.distance(exactBoundPoint(p))
		if distance.Cmp(bound) > 0 {
			bound.Set(distance)
		}
	}
	return SplineTriangleDeviation{Bounds: box, SquaredDistance: bound}, nil
}

type exactBoundVector [3]*big.Rat

func exactBoundPoint(p geom.Vec3) exactBoundVector {
	return exactBoundVector{ratInset(p.X), ratInset(p.Y), ratInset(p.Z)}
}
func boundSubtract(a, b exactBoundVector) (r exactBoundVector) {
	for k := range r {
		r[k] = new(big.Rat).Sub(a[k], b[k])
	}
	return
}
func boundDot(a, b exactBoundVector) *big.Rat {
	r := new(big.Rat)
	for k := range a {
		r.Add(r, new(big.Rat).Mul(a[k], b[k]))
	}
	return r
}

type exactBoundTriangle struct {
	p               [3]exactBoundVector
	u, v            exactBoundVector
	uu, uv, vv, det *big.Rat
}

func newExactBoundTriangle(p [3]geom.Vec3) exactBoundTriangle {
	t := exactBoundTriangle{}
	for k := range p {
		t.p[k] = exactBoundPoint(p[k])
	}
	t.u = boundSubtract(t.p[1], t.p[0])
	t.v = boundSubtract(t.p[2], t.p[0])
	t.uu = boundDot(t.u, t.u)
	t.uv = boundDot(t.u, t.v)
	t.vv = boundDot(t.v, t.v)
	t.det = new(big.Rat).Sub(new(big.Rat).Mul(t.uu, t.vv), new(big.Rat).Mul(t.uv, t.uv))
	return t
}
func (t exactBoundTriangle) distance(p exactBoundVector) *big.Rat {
	w := boundSubtract(p, t.p[0])
	wu, wv := boundDot(w, t.u), boundDot(w, t.v)
	a := new(big.Rat).Quo(new(big.Rat).Sub(new(big.Rat).Mul(wu, t.vv), new(big.Rat).Mul(wv, t.uv)), t.det)
	b := new(big.Rat).Quo(new(big.Rat).Sub(new(big.Rat).Mul(wv, t.uu), new(big.Rat).Mul(wu, t.uv)), t.det)
	if a.Sign() >= 0 && b.Sign() >= 0 && new(big.Rat).Add(a, b).Cmp(big.NewRat(1, 1)) <= 0 {
		var r exactBoundVector
		for k := range r {
			r[k] = new(big.Rat).Sub(w[k], new(big.Rat).Add(new(big.Rat).Mul(a, t.u[k]), new(big.Rat).Mul(b, t.v[k])))
		}
		return boundDot(r, r)
	}
	var best *big.Rat
	for i := 0; i < 3; i++ {
		d := boundSubtract(t.p[(i+1)%3], t.p[i])
		q := boundSubtract(p, t.p[i])
		length := boundDot(d, d)
		f := new(big.Rat)
		if length.Sign() > 0 {
			f.Quo(boundDot(q, d), length)
			if f.Sign() < 0 {
				f.SetInt64(0)
			}
			if f.Cmp(big.NewRat(1, 1)) > 0 {
				f.SetInt64(1)
			}
		}
		var r exactBoundVector
		for k := range r {
			r[k] = new(big.Rat).Sub(q[k], new(big.Rat).Mul(f, d[k]))
		}
		s := boundDot(r, r)
		if best == nil || s.Cmp(best) < 0 {
			best = s
		}
	}
	return best
}
