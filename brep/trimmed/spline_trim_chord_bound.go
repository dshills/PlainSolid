package trimmed

import (
	"context"
	"math"
	"math/big"

	"plainsolid/geom"
)

// SplineTrimChordBound encloses every point of an exact rational UV trim
// composed with a positive-weight spline patch. SquaredDistance is an exact
// rational upper bound on distance to the supplied closed model-space chord.
// Bounds describes a conservative model box, not a sampled curve envelope.
type SplineTrimChordBound struct {
	Bounds          geom.Box
	SquaredDistance *big.Rat
}

// BoundSplineTrimChord uses exact homogeneous restrictions and outward control
// hulls. The trim's Range is in original spline parameters; Reversed does not
// affect this undirected distance bound. The complete UV hull must lie inside
// the support domain. For wrapped supports pass FundamentalPatch and a trim
// confined to that fundamental rectangle. Chord endpoints may be rounded mesh
// vertices: their represented coordinates are used exactly in the bound.
//
// This box certificate is intentionally conservative and may converge only
// linearly under interval subdivision. It does not certify face-interior mesh
// error, embedding, or topology. Options bound the curve construction/restriction stages; the immutable
// support retains its own construction limits for Trim. No result is returned
// after any stage fails.
func BoundSplineTrimChord(ctx context.Context, patch *geom.SurfacePatch, trim SplineUV, start, end geom.Vec3, o geom.SurfacePatchOptions) (SplineTrimChordBound, error) {
	empty := SplineTrimChordBound{}
	if ctx == nil || patch == nil || trim.Curve == nil || !start.IsFinite() || !end.IsFinite() {
		return empty, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return empty, e
	}
	for _, p := range trim.Curve.Poles() {
		if p.Z != 0 {
			return empty, geom.ErrDomain
		}
	}
	c, e := geom.NewExactSplineCurve(trim.Curve, o)
	if e != nil {
		return empty, e
	}
	c, e = c.Restrict(trim.Range.Min, trim.Range.Max, o)
	if e != nil {
		return empty, e
	}
	uv, e := c.ProjectionBounds()
	if e != nil {
		return empty, e
	}
	d := patch.Domain()
	if uv.Min.X < d.UMin || uv.Max.X > d.UMax || uv.Min.Y < d.VMin || uv.Max.Y > d.VMax {
		return empty, geom.ErrDomain
	}
	// Exact constant UV coordinates still need a nonempty patch restriction.
	// One adjacent represented interval encloses that coordinate safely.
	widen := func(lo, hi, min, max float64) (float64, float64) {
		if lo == hi {
			lo = math.Max(min, math.Nextafter(lo, math.Inf(-1)))
			hi = math.Min(max, math.Nextafter(hi, math.Inf(1)))
		}
		return lo, hi
	}
	u0, u1 := widen(uv.Min.X, uv.Max.X, d.UMin, d.UMax)
	v0, v1 := widen(uv.Min.Y, uv.Max.Y, d.VMin, d.VMax)
	restricted, e := patch.Trim(geom.ParameterRectangle{UMin: u0, UMax: u1, VMin: v0, VMax: v1})
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
	return SplineTrimChordBound{Bounds: box, SquaredDistance: exactChordBoxDistance(box, start, end)}, nil
}

func exactChordBoxDistance(box geom.Box, start, end geom.Vec3) *big.Rat {
	a := [3]*big.Rat{new(big.Rat).SetFloat64(start.X), new(big.Rat).SetFloat64(start.Y), new(big.Rat).SetFloat64(start.Z)}
	b := [3]*big.Rat{new(big.Rat).SetFloat64(end.X), new(big.Rat).SetFloat64(end.Y), new(big.Rat).SetFloat64(end.Z)}
	var delta [3]*big.Rat
	length := new(big.Rat)
	for k := range delta {
		delta[k] = new(big.Rat).Sub(b[k], a[k])
		length.Add(length, new(big.Rat).Mul(delta[k], delta[k]))
	}
	maxDistance := new(big.Rat)
	for i := 0; i < 8; i++ {
		p := []float64{box.Min.X, box.Min.Y, box.Min.Z}
		hi := []float64{box.Max.X, box.Max.Y, box.Max.Z}
		var difference [3]*big.Rat
		dot := new(big.Rat)
		for k := range difference {
			if i&(1<<k) != 0 {
				p[k] = hi[k]
			}
			difference[k] = new(big.Rat).Sub(new(big.Rat).SetFloat64(p[k]), a[k])
			dot.Add(dot, new(big.Rat).Mul(difference[k], delta[k]))
		}
		t := new(big.Rat)
		if length.Sign() > 0 {
			t.Quo(dot, length)
			if t.Sign() < 0 {
				t.SetInt64(0)
			}
			if t.Cmp(big.NewRat(1, 1)) > 0 {
				t.SetInt64(1)
			}
		}
		distance := new(big.Rat)
		for k := range difference {
			x := new(big.Rat).Sub(difference[k], new(big.Rat).Mul(t, delta[k]))
			distance.Add(distance, new(big.Rat).Mul(x, x))
		}
		if distance.Cmp(maxDistance) > 0 {
			maxDistance.Set(distance)
		}
	}
	// Distance squared to a convex set is convex, hence the maximum over a
	// box is attained at a vertex. The positive hull enclosed the whole trim.
	return maxDistance
}

// CertifiedSplineTrimPolyline carries original-parameter intervals and a
// conservative squared model error for each chord. These are boundary-only
// certificates, not a face or solid mesh certificate.
type CertifiedSplineTrimPolyline struct {
	Parameters    []float64
	Points        []geom.Vec3
	SquaredErrors []*big.Rat
}

// TessellateSplineTrimCertified subdivides until every chord satisfies the
// represented deflection. maxSegments is a hard leaf/work cap (1..65536).
// Failure returns no partial polyline. Reversed traverses the resulting points
// and original parameters backwards. Support trimming keeps the patch's own
// limits; options controls exact UV restrictions. Endpoint rounding is included
// in the chord certificate, rather than assumed exact.
func TessellateSplineTrimCertified(ctx context.Context, patch *geom.SurfacePatch, trim SplineUV, deflection float64, maxSegments int, options geom.SurfacePatchOptions) (CertifiedSplineTrimPolyline, error) {
	empty := CertifiedSplineTrimPolyline{}
	if ctx == nil || patch == nil || trim.Curve == nil || !finite(deflection) || deflection <= 0 || maxSegments < 1 || maxSegments > 65536 || !finite(trim.Range.Min) || !finite(trim.Range.Max) || trim.Range.Min >= trim.Range.Max {
		return empty, geom.ErrInvalid
	}
	target := new(big.Rat).SetFloat64(deflection)
	target.Mul(target, target)
	point := func(t float64) (geom.Vec3, error) {
		uv, e := trim.Curve.Eval(t)
		if e != nil {
			return geom.Vec3{}, e
		}
		jet, e := patch.EvaluateRational(ctx, new(big.Rat).SetFloat64(uv.X), new(big.Rat).SetFloat64(uv.Y), geom.SurfacePatchEvaluationOptions{MaxOperations: options.MaxOperations, MaxCoefficientBits: options.MaxCoefficientBits})
		if e != nil {
			return geom.Vec3{}, e
		}
		x, _ := jet.Point[0].Float64()
		y, _ := jet.Point[1].Float64()
		z, _ := jet.Point[2].Float64()
		p := geom.Vec3{X: x, Y: y, Z: z}
		if !p.IsFinite() {
			return geom.Vec3{}, geom.ErrConvergence
		}
		return p, nil
	}
	a, e := point(trim.Range.Min)
	if e != nil {
		return empty, e
	}
	b, e := point(trim.Range.Max)
	if e != nil {
		return empty, e
	}
	type interval struct {
		lo, hi float64
		a, b   geom.Vec3
	}
	stack := []interval{{trim.Range.Min, trim.Range.Max, a, b}}
	result := CertifiedSplineTrimPolyline{Parameters: []float64{trim.Range.Min}, Points: []geom.Vec3{a}}
	leaves := 1
	for len(stack) > 0 {
		if e := ctx.Err(); e != nil {
			return empty, e
		}
		node := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		part := trim
		part.Range = UVInterval{Min: node.lo, Max: node.hi}
		bound, e := BoundSplineTrimChord(ctx, patch, part, node.a, node.b, options)
		if e != nil {
			return empty, e
		}
		if bound.SquaredDistance.Cmp(target) <= 0 {
			result.Parameters = append(result.Parameters, node.hi)
			result.Points = append(result.Points, node.b)
			result.SquaredErrors = append(result.SquaredErrors, bound.SquaredDistance)
			continue
		}
		mid := node.lo + (node.hi-node.lo)/2
		if leaves >= maxSegments || !finite(mid) || mid <= node.lo || mid >= node.hi {
			return empty, geom.ErrConvergence
		}
		p, e := point(mid)
		if e != nil {
			return empty, e
		}
		leaves++
		stack = append(stack, interval{mid, node.hi, p, node.b}, interval{node.lo, mid, node.a, p})
	}
	if trim.Reversed {
		for i, j := 0, len(result.Points)-1; i < j; i, j = i+1, j-1 {
			result.Points[i], result.Points[j] = result.Points[j], result.Points[i]
			result.Parameters[i], result.Parameters[j] = result.Parameters[j], result.Parameters[i]
		}
		for i, j := 0, len(result.SquaredErrors)-1; i < j; i, j = i+1, j-1 {
			result.SquaredErrors[i], result.SquaredErrors[j] = result.SquaredErrors[j], result.SquaredErrors[i]
		}
	}
	return result, nil
}
