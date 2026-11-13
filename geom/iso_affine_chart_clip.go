package geom

import (
	"context"
	"math/big"
	"sort"
)

// AffineChartCurveEndpoint owns exact curve, world and original chart values.
// BoundaryMask uses UMin,UMax,VMin,VMax bits 1,2,4,8.
type AffineChartCurveEndpoint struct {
	Parameter       *big.Rat
	Point           [3]*big.Rat
	UV              [2]*big.Rat
	BoundaryMask    uint8
	TangentBoundary bool
}
type AffineChartCurveSpan struct{ Start, End AffineChartCurveEndpoint }

// AffineChartCurveClip describes the preimage of the closed chart rectangle.
// Contacts are isolated retained points; Spans have positive parameter length.
// CoincidentBoundary records side-coincident spans and makes this operation
// unresolved. Algebraic endpoints without rational recovery also remain unresolved.
type AffineChartCurveClip struct {
	Spans              []AffineChartCurveSpan
	Contacts           []AffineChartCurveEndpoint
	CoincidentBoundary []ParameterSpan
	Complete           bool
	Operations         int
}

// ClipIsoCurveToAffineChart first proves every homogeneous curve control lies
// exactly in the chart plane, then isolates roots of all four projected sides.
// Rational root recovery and exact midpoint classification certify retained
// parameter intervals; no sampled sign test replaces exhaustive root isolation.
// Options follow IntersectRawPlane, with default total work of 8M operations.
func ClipIsoCurveToAffineChart(ctx context.Context, c *SurfaceIsoCurve, chart *AffineSurfaceChart, o SplinePlaneOptions) (r AffineChartCurveClip, err error) {
	if ctx == nil || c == nil || c.patch == nil || chart == nil || chart.determinant == nil {
		return r, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return r, e
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 8000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	if o.MaxOperations < 1 || o.MaxCoefficientBits < 64 {
		return r, ErrInvalid
	}
	budget := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	defer func() { r.Operations = budget.operations }()
	remaining := func() (int, error) {
		n := o.MaxOperations - budget.operations
		if n < 1 {
			return 0, ErrConvergence
		}
		return n, nil
	}
	curve, e := projectIsoCurveChart(ctx, c, chart, budget)
	if e != nil {
		return r, e
	}

	lo, hi := c.Domain()
	events := []*big.Rat{new(big.Rat).SetFloat64(lo), new(big.Rat).SetFloat64(hi)}
	tangent := map[string]bool{}
	d := chart.Domain()
	bounds := []float64{d.UMin, d.UMax, d.VMin, d.VMax}
	for side, bound := range bounds {
		left, e := remaining()
		if e != nil {
			return r, e
		}
		local := o
		local.MaxOperations = left
		a, b, z := Vec3{X: bound}, Vec3{X: bound, Y: 1}, Vec3{X: bound, Z: 1}
		if side >= 2 {
			a, b, z = Vec3{Y: bound}, Vec3{X: 1, Y: bound}, Vec3{Y: bound, Z: 1}
		}
		roots, e := curve.IntersectRawPlane(ctx, a, b, z, local)
		budget.operations += roots.Operations
		if e != nil {
			return r, e
		}
		if len(roots.Coincident) > 0 {
			r.CoincidentBoundary = append(r.CoincidentBoundary, roots.Coincident...)
			return r, ErrConvergence
		}
		for _, hit := range roots.Hits {
			if hit.Parameter == nil {
				return r, ErrConvergence
			}
			events = append(events, new(big.Rat).Set(hit.Parameter))
			if hit.Tangent {
				tangent[hit.Parameter.RatString()] = true
			}
		}
	}
	sort.Slice(events, func(i, j int) bool { return events[i].Cmp(events[j]) < 0 })
	unique := events[:0]
	for _, x := range events {
		if len(unique) == 0 || unique[len(unique)-1].Cmp(x) != 0 {
			unique = append(unique, x)
		}
	}
	events = unique
	evaluate := func(t *big.Rat) (AffineChartCurveEndpoint, bool, error) {
		point, e := exactCurveRationalPoint(c.patch, t, budget)
		if e != nil {
			return AffineChartCurveEndpoint{}, false, e
		}
		left, e := remaining()
		if e != nil {
			return AffineChartCurveEndpoint{}, false, e
		}
		inv, e := chart.InvertExactContext(ctx, point, AffineChartInverseOptions{MaxOperations: left, MaxCoefficientBits: o.MaxCoefficientBits})
		budget.operations += inv.ExactOperations
		if e != nil {
			return AffineChartCurveEndpoint{}, false, e
		}
		ep := AffineChartCurveEndpoint{Parameter: new(big.Rat).Set(t), Point: point, UV: [2]*big.Rat{inv.U, inv.V}, TangentBoundary: tangent[t.RatString()]}
		for side, bound := range bounds {
			if ep.UV[side/2].Cmp(new(big.Rat).SetFloat64(bound)) == 0 {
				ep.BoundaryMask |= 1 << uint(side)
			}
		}
		return ep, inv.Inside, nil
	}
	endpoints := make([]AffineChartCurveEndpoint, len(events))
	inside := make([]bool, len(events))
	for i, t := range events {
		var e error
		endpoints[i], inside[i], e = evaluate(t)
		if e != nil {
			return r, e
		}
	}
	included := make([]bool, len(events))
	for i := 0; i+1 < len(events); i++ {
		mid := new(big.Rat).Quo(new(big.Rat).Add(events[i], events[i+1]), big.NewRat(2, 1))
		_, in, e := evaluate(mid)
		if e != nil {
			return r, e
		}
		if in {
			if !inside[i] || !inside[i+1] {
				return r, ErrConvergence
			}
			r.Spans = append(r.Spans, AffineChartCurveSpan{endpoints[i], endpoints[i+1]})
			included[i], included[i+1] = true, true
		}
	}
	for i, in := range inside {
		if in && !included[i] {
			r.Contacts = append(r.Contacts, endpoints[i])
		}
	}
	r.Complete = true
	return r, nil
}

func exactCurveRationalPoint(p *SurfacePatch, t *big.Rat, b *polynomialBudget) ([3]*big.Rat, error) {
	for {
		k, ok := internalPatchKnot(p.knotsU, p.degreeU, p.countU)
		if !ok {
			break
		}
		a, c, e := p.split(k, true, b)
		if e != nil {
			return [3]*big.Rat{}, e
		}
		if t.Cmp(new(big.Rat).SetFloat64(k)) < 0 {
			p = a
		} else {
			p = c
		}
	}
	point, _, e := exactSurfaceJet(p, t, new(big.Rat), b)
	return point, e
}

func projectIsoCurveChart(ctx context.Context, c *SurfaceIsoCurve, chart *AffineSurfaceChart, budget *polynomialBudget) (*SurfaceIsoCurve, error) {
	projected := *c.patch
	projected.net = make([]exactHomogeneous, len(c.patch.net))
	for i, h := range c.patch.net {
		var point [3]*big.Rat
		for axis := 0; axis < 3; axis++ {
			point[axis] = new(big.Rat).Quo(h[axis], h[3])
			if e := budget.check(point[axis]); e != nil {
				return nil, e
			}
		}
		left := budget.maxOperations - budget.operations
		var e error
		if left < 1 {
			e = ErrConvergence
		}
		if e != nil {
			return nil, e
		}
		inv, e := chart.InvertExactContext(ctx, point, AffineChartInverseOptions{MaxOperations: left, MaxCoefficientBits: budget.maxBits})
		budget.operations += inv.ExactOperations
		if e != nil {
			return nil, e
		}
		if !inv.OnPlane {
			return nil, ErrDomain
		}
		projected.net[i] = exactHomogeneous{new(big.Rat).Mul(inv.U, h[3]), new(big.Rat).Mul(inv.V, h[3]), new(big.Rat), new(big.Rat).Set(h[3])}
		for _, x := range projected.net[i] {
			if e := budget.check(x); e != nil {
				return nil, e
			}
		}
	}
	if e := projected.computeBounds(budget); e != nil {
		return nil, e
	}
	return &SurfaceIsoCurve{patch: &projected}, nil
}
