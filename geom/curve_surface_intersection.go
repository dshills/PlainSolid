package geom

import (
	"context"
	"math/big"
	"sort"
)

// CurveSurfaceIntersectionOptions bounds the isolation of all regular
// intersections between a clamped rational spline curve and a bounded spline
// surface. Zero values choose depth 24, 8192 cells and 8M exact operations.
type CurveSurfaceIntersectionOptions struct {
	MaxDepth, MaxCells, MaxOperations, MaxCoefficientBits int
}

type CurveSurfaceHit struct {
	SurfaceCell    ParameterRectangle
	CurveCell      ParameterSpan
	UV             UVBox
	CurveParameter CertifiedScalarRange
	Bounds         Box
	// Direction is sign((Su × Sv) · curve derivative) at the proved root.
	Direction int
}

type CurveSurfacePending struct {
	SurfaceCell ParameterRectangle
	CurveCell   ParameterSpan
	Depth       int
}

// CurveSurfaceIntersection is exhaustive only when Complete. Pending boxes
// retain possible contacts at tangencies, coincident curves, boundaries, or
// proof limits. Such contacts are never treated as misses.
type CurveSurfaceIntersection struct {
	Hits                          []CurveSurfaceHit
	Pending                       []CurveSurfacePending
	VisitedCells, ExactOperations int
	Complete                      bool
}

// IntersectExactSplineCurveSurface isolates a curve against an independent
// spline support using exact homogeneous hulls and a three-variable Krawczyk
// proof. The curve is embedded as a constant auxiliary chart with no rounded
// 3D coefficients. No sign change or proximity test can certify a root.
func IntersectExactSplineCurveSurface(ctx context.Context, curve *ExactSplineCurve, surface *SurfacePatch,
	o CurveSurfaceIntersectionOptions) (out CurveSurfaceIntersection, err error) {
	if ctx == nil || curve == nil || curve.patch == nil || surface == nil || len(surface.net) == 0 {
		return out, ErrInvalid
	}
	if o.MaxDepth == 0 {
		o.MaxDepth = 24
	}
	if o.MaxCells == 0 {
		o.MaxCells = 8192
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 8000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	if o.MaxDepth < 1 || o.MaxDepth > 128 || o.MaxCells < 1 || o.MaxOperations < 1 || o.MaxCoefficientBits < 64 {
		return out, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return out, e
	}
	c := curve.patch
	if c.degreeV != 1 || c.countV != 2 || c.Domain().VMin != 0 || c.Domain().VMax != 1 {
		return out, ErrInvalid
	}
	for _, d := range []ParameterRectangle{surface.Domain(), c.Domain()} {
		if !d.Valid() || !finite(d.UMax-d.UMin) || !finite(d.VMax-d.VMin) {
			return out, ErrInvalid
		}
	}
	// ExactSplineCurve constructors retain two identical homogeneous rows.
	// Recheck this invariant before discarding the auxiliary parameter.
	budget := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	for i := 0; i < len(c.net); i += 2 {
		if e := ctx.Err(); e != nil {
			return out, e
		}
		for axis := range 4 {
			if c.net[i][axis].Cmp(c.net[i+1][axis]) != 0 {
				return out, ErrInvalid
			}
			if e := budget.check(c.net[i][axis]); e != nil {
				return out, e
			}
		}
	}
	for _, h := range surface.net {
		if e := ctx.Err(); e != nil {
			return out, e
		}
		for _, x := range h {
			if e := budget.check(x); e != nil {
				return out, e
			}
		}
	}
	out.ExactOperations = budget.operations
	type node struct {
		s, c  *SurfacePatch
		depth int
	}
	stack := []node{{surface, c, 0}}
	pending := func(n node) {
		d := n.c.Domain()
		out.Pending = append(out.Pending, CurveSurfacePending{n.s.Domain(), ParameterSpan{Start: d.UMin, End: d.UMax}, n.depth})
	}
	fail := func(n node, err error) (CurveSurfaceIntersection, error) {
		pending(n)
		for _, p := range stack {
			pending(p)
		}
		return out, err
	}
	for len(stack) > 0 {
		n := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if e := ctx.Err(); e != nil {
			return fail(n, e)
		}
		if out.VisitedCells >= o.MaxCells || out.ExactOperations >= o.MaxOperations {
			return fail(n, ErrConvergence)
		}
		out.VisitedCells++
		space, overlap := n.s.bounds.Intersect(n.c.bounds)
		if !overlap {
			continue
		}
		u, uk := internalPatchKnot(n.s.knotsU, n.s.degreeU, n.s.countU)
		v, vk := internalPatchKnot(n.s.knotsV, n.s.degreeV, n.s.countV)
		t, tk := internalPatchKnot(n.c.knotsU, n.c.degreeU, n.c.countU)
		d, cd := n.s.Domain(), n.c.Domain()
		if !uk && !vk && !tk {
			section := SurfacePairIntervalOptions{UseFixedParameter: true, FixedAxis: 3, FixedParameter: 0,
				MaxOperations: o.MaxOperations - out.ExactOperations, MaxCoefficientBits: o.MaxCoefficientBits}
			q, e := CertifySurfacePairSection(n.s, n.c, section)
			out.ExactOperations += q.ExactOperations
			if e != nil {
				return fail(n, e)
			}
			if q.ExcludesContact {
				continue
			}
			if q.ProvenUnique {
				z := q.KrawczykBounds
				out.Hits = append(out.Hits, CurveSurfaceHit{SurfaceCell: d, CurveCell: ParameterSpan{Start: cd.UMin, End: cd.UMax},
					UV:             UVBox{Min: Vec2{X: max(d.UMin, z[0].Min), Y: max(d.VMin, z[1].Min)}, Max: Vec2{X: min(d.UMax, z[0].Max), Y: min(d.VMax, z[1].Max)}},
					CurveParameter: z[2], Bounds: space, Direction: -q.JacobianDeterminantSign})
				continue
			}
		}
		if n.depth >= o.MaxDepth {
			pending(n)
			continue
		}
		axis, cut := 0, u
		if !uk && vk {
			axis, cut = 1, v
		} else if !uk && !vk && tk {
			axis, cut = 2, t
		} else if !uk && !vk && !tk {
			// Compare normalized widths in the original domains. Units of
			// the three independent parameters need not be comparable.
			sd, sc := surface.Domain(), c.Domain()
			wu := (d.UMax - d.UMin) / (sd.UMax - sd.UMin)
			wv := (d.VMax - d.VMin) / (sd.VMax - sd.VMin)
			wt := (cd.UMax - cd.UMin) / (sc.UMax - sc.UMin)
			axis, cut = 0, d.UMin/2+d.UMax/2
			if wv > wu {
				axis, cut = 1, d.VMin/2+d.VMax/2
			}
			if wt > max(wu, wv) {
				axis, cut = 2, cd.UMin/2+cd.UMax/2
			}
		}
		patch, alongU := n.s, axis == 0
		if axis == 2 {
			patch, alongU = n.c, true
		}
		p := patch.Domain()
		lo, hi := p.VMin, p.VMax
		if alongU {
			lo, hi = p.UMin, p.UMax
		}
		if cut <= lo || cut >= hi {
			pending(n)
			continue
		}
		if out.ExactOperations >= o.MaxOperations {
			return fail(n, ErrConvergence)
		}
		b := &polynomialBudget{maxOperations: o.MaxOperations - out.ExactOperations, maxBits: o.MaxCoefficientBits}
		l, r, e := patch.split(cut, alongU, b)
		out.ExactOperations += b.operations
		if e != nil {
			return fail(n, e)
		}
		if axis == 2 {
			stack = append(stack, node{n.s, r, n.depth + 1}, node{n.s, l, n.depth + 1})
		} else {
			stack = append(stack, node{r, n.c, n.depth + 1}, node{l, n.c, n.depth + 1})
		}
	}
	if len(out.Pending) > 0 {
		if e := rescueCurveSurfaceContacts(ctx, &out, surface, c, o); e != nil {
			return out, e
		}
	}
	sort.SliceStable(out.Hits, func(i, j int) bool { return out.Hits[i].CurveParameter.Min < out.Hits[j].CurveParameter.Min })
	out.Complete = len(out.Pending) == 0
	if !out.Complete {
		return out, ErrConvergence
	}
	return out, nil
}

func rescueCurveSurfaceContacts(ctx context.Context, out *CurveSurfaceIntersection, surface, curve *SurfacePatch,
	o CurveSurfaceIntersectionOptions) error {
	pending := out.Pending
	out.Pending = nil
	d, cd := surface.Domain(), curve.Domain()
	covered := func(a, b CurveSurfaceHit) bool {
		return a.UV.Min.X >= b.SurfaceCell.UMin && a.UV.Max.X <= b.SurfaceCell.UMax &&
			a.UV.Min.Y >= b.SurfaceCell.VMin && a.UV.Max.Y <= b.SurfaceCell.VMax &&
			a.CurveParameter.Min >= b.CurveCell.Start && a.CurveParameter.Max <= b.CurveCell.End
	}
	overlap := func(a, b CurveSurfaceHit) bool {
		return a.UV.Min.X <= b.UV.Max.X && b.UV.Min.X <= a.UV.Max.X &&
			a.UV.Min.Y <= b.UV.Max.Y && b.UV.Min.Y <= a.UV.Max.Y &&
			a.CurveParameter.Min <= b.CurveParameter.Max && b.CurveParameter.Min <= a.CurveParameter.Max
	}
	for i, p := range pending {
		if e := ctx.Err(); e != nil {
			out.Pending = append(out.Pending, pending[i:]...)
			return e
		}
		if out.ExactOperations >= o.MaxOperations {
			out.Pending = append(out.Pending, pending[i:]...)
			return ErrConvergence
		}
		expand := func(lo, hi, minValue, maxValue float64) (float64, float64) {
			w := hi - lo
			return max(minValue, lo-w), min(maxValue, hi+w)
		}
		u0, u1 := expand(p.SurfaceCell.UMin, p.SurfaceCell.UMax, d.UMin, d.UMax)
		v0, v1 := expand(p.SurfaceCell.VMin, p.SurfaceCell.VMax, d.VMin, d.VMax)
		t0, t1 := expand(p.CurveCell.Start, p.CurveCell.End, cd.UMin, cd.UMax)
		sr, cr := ParameterRectangle{u0, u1, v0, v1}, ParameterRectangle{t0, t1, 0, 1}
		b := &polynomialBudget{maxOperations: o.MaxOperations - out.ExactOperations, maxBits: o.MaxCoefficientBits}
		s, e := trimPatchBudget(surface, sr, b)
		var c *SurfacePatch
		if e == nil {
			c, e = trimPatchBudget(curve, cr, b)
		}
		out.ExactOperations += b.operations
		if e != nil {
			out.Pending = append(out.Pending, pending[i:]...)
			return e
		}
		if s.countU != s.degreeU+1 || s.countV != s.degreeV+1 || c.countU != c.degreeU+1 {
			out.Pending = append(out.Pending, p)
			continue
		}
		remaining := o.MaxOperations - out.ExactOperations
		if remaining < 1 {
			out.Pending = append(out.Pending, pending[i:]...)
			return ErrConvergence
		}
		q, e := CertifySurfacePairSection(s, c, SurfacePairIntervalOptions{UseFixedParameter: true, FixedAxis: 3, FixedParameter: 0,
			MaxOperations: remaining, MaxCoefficientBits: o.MaxCoefficientBits})
		out.ExactOperations += q.ExactOperations
		if e != nil {
			out.Pending = append(out.Pending, pending[i:]...)
			return e
		}
		if q.ExcludesContact {
			continue
		}
		if !q.ProvenUnique {
			out.Pending = append(out.Pending, p)
			continue
		}
		z := q.KrawczykBounds
		space, ok := s.bounds.Intersect(c.bounds)
		if !ok {
			out.Pending = append(out.Pending, pending[i:]...)
			return ErrInvalid
		}
		h := CurveSurfaceHit{SurfaceCell: sr, CurveCell: ParameterSpan{Start: t0, End: t1},
			UV: UVBox{Min: Vec2{X: max(u0, z[0].Min), Y: max(v0, z[1].Min)}, Max: Vec2{X: min(u1, z[0].Max), Y: min(v1, z[1].Max)}}, CurveParameter: z[2], Bounds: space, Direction: -q.JacobianDeterminantSign}
		same, ambiguous := false, false
		for _, old := range out.Hits {
			if covered(h, old) || covered(old, h) {
				same = true
				break
			}
			ambiguous = ambiguous || overlap(h, old)
		}
		if same {
			continue
		}
		if ambiguous {
			out.Pending = append(out.Pending, p)
			continue
		}
		out.Hits = append(out.Hits, h)
	}
	return nil
}

// IntersectSplineSurface searches an outward represented continuation of this
// cell. A root is retained only if its whole parameter enclosure lies strictly
// inside the true algebraic domain. Roots separated from the domain are
// discarded; roots touching an exact cut stay pending for a seam proof.
func (c *ExactComposedCurveCell) IntersectSplineSurface(ctx context.Context, surface *SurfacePatch,
	o CurveSurfaceIntersectionOptions) (CurveSurfaceIntersection, error) {
	var empty CurveSurfaceIntersection
	if ctx == nil || c == nil || c.endpoints[0] == nil || c.endpoints[1] == nil || surface == nil || len(surface.net) == 0 {
		return empty, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return empty, e
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 8000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	if o.MaxOperations < 1 || o.MaxCoefficientBits < 64 || o.MaxCells < 0 || o.MaxDepth < 0 || o.MaxDepth > 128 {
		return empty, ErrInvalid
	}
	b := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	var ends [2][2]*big.Rat
	// Avoid searching a wide isolating bracket as part of the curve. In
	// particular a nearby continuation root on the loose bracket's bound
	// could otherwise look like an unresolved original-domain root.
	threshold := new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Lsh(big.NewInt(1), 60))
	for i, root := range c.endpoints {
		_, e := isolatePolynomialRoots(root.polynomial, root.bounds[0], root.bounds[1], b, 8192, func(lo, hi *big.Rat, _ int) (bool, error) {
			if e := ctx.Err(); e != nil {
				return false, e
			}
			if new(big.Rat).Sub(hi, lo).Cmp(threshold) > 0 {
				return false, nil
			}
			ends[i] = [2]*big.Rat{new(big.Rat).Set(lo), new(big.Rat).Set(hi)}
			return true, nil
		})
		if e != nil {
			return empty, e
		}
		if ends[i][0] == nil {
			return empty, ErrConvergence
		}
	}
	lo, hi := ends[0][0], ends[1][1]
	a, z := rationalFloatBound(lo, true), rationalFloatBound(hi, false)
	p, e := exactCurveCutPatch(c, a, z, b)
	if e != nil {
		empty.ExactOperations = b.operations
		return empty, e
	}
	o.MaxOperations -= b.operations
	if o.MaxOperations < 1 {
		empty.ExactOperations = b.operations
		return empty, ErrConvergence
	}
	r, e := IntersectExactSplineCurveSurface(ctx, &ExactSplineCurve{patch: p}, surface, o)
	r.ExactOperations += b.operations
	keep := r.Hits[:0]
	for _, h := range r.Hits {
		hlo, hhi := new(big.Rat).SetFloat64(h.CurveParameter.Min), new(big.Rat).SetFloat64(h.CurveParameter.Max)
		if hhi.Cmp(lo) < 0 || hlo.Cmp(hi) > 0 {
			continue
		}
		if hlo.Cmp(ends[0][1]) > 0 && hhi.Cmp(ends[1][0]) < 0 {
			keep = append(keep, h)
			continue
		}
		r.Pending = append(r.Pending, CurveSurfacePending{SurfaceCell: h.SurfaceCell, CurveCell: h.CurveCell})
	}
	r.Hits = keep
	if len(r.Pending) > 0 {
		r.Complete = false
		if e == nil {
			e = ErrConvergence
		}
	}
	return r, e
}

// exactCurveCutPatch converts an algebraic cell's continuation over a
// represented outward interval into exact homogeneous Bernstein controls.
// The caller must separately prove each reported root belongs to the true
// algebraic domain. A nonpositive denominator hull remains unresolved.
func exactCurveCutPatch(c *ExactComposedCurveCell, lo, hi float64, b *polynomialBudget) (*SurfacePatch, error) {
	if c == nil || !finite(lo) || !finite(hi) || lo >= hi {
		return nil, ErrInvalid
	}
	degree := 1
	for _, f := range c.power {
		degree = max(degree, len(f)-1)
	}
	if degree > 128 {
		return nil, ErrConvergence
	}
	net := make([]exactHomogeneous, 2*(degree+1))
	start := new(big.Rat).SetFloat64(lo)
	width := new(big.Rat).Sub(new(big.Rat).SetFloat64(hi), start)
	for k, f := range c.power {
		var p rationalPolynomial
		for i := len(f) - 1; i >= 0; i-- {
			q := make(rationalPolynomial, len(p)+1)
			for j := range q {
				q[j] = new(big.Rat)
			}
			for j, x := range p {
				q[j].Add(q[j], new(big.Rat).Mul(x, start))
				q[j+1].Add(q[j+1], new(big.Rat).Mul(x, width))
			}
			q[0].Add(q[0], f[i])
			for _, x := range q {
				if e := b.check(x); e != nil {
					return nil, e
				}
			}
			p = q
		}
		for i := 0; i <= degree; i++ {
			x := new(big.Rat)
			for j := 0; j <= i && j < len(p); j++ {
				ratio := new(big.Rat).SetFrac(new(big.Int).Binomial(int64(i), int64(j)), new(big.Int).Binomial(int64(degree), int64(j)))
				x.Add(x, new(big.Rat).Mul(p[j], ratio))
				if e := b.check(x); e != nil {
					return nil, e
				}
			}
			if k == 3 && x.Sign() <= 0 {
				return nil, ErrConvergence
			}
			net[2*i][k], net[2*i+1][k] = x, x
		}
	}
	knots := make([]float64, 2*(degree+1))
	for i := 0; i <= degree; i++ {
		knots[i], knots[i+degree+1] = lo, hi
	}
	o, _ := patchOptions(SurfacePatchOptions{})
	p := &SurfacePatch{degreeU: degree, degreeV: 1, countU: degree + 1, countV: 2, knotsU: knots, knotsV: []float64{0, 0, 1, 1}, net: net, options: o}
	if e := p.computeBounds(b); e != nil {
		return nil, e
	}
	return p, nil
}
