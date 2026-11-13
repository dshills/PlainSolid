package geom

import (
	"context"
	"math/big"
)

// GraphPointContactOptions bounds exact contact proof. Zero selects depth12,
// 4096 cells, 8M operations and 16384 coefficient bits. The exact
// height-difference numerator is checked for exact C1 continuity on the union
// knot grid; genuine derivative jumps remain outside this convexity subdomain.
type GraphPointContactOptions struct{ MaxDepth, MaxCells, MaxOperations, MaxCoefficientBits int }

// SplineGraphPointContact proves one isolated contact of two same-XY injective
// graph patches, with Side the strict sign of A.Z-B.Z at every other parameter.
// Positive weights preserve numerator sign. The definite numerator Hessian plus
// exact zero and inward first-order KKT signs yield the global result along
// every feasible segment. At an interior witness both derivatives must be zero;
// at lower/upper boundaries Side*gradient must be nonnegative/nonpositive.
type SplineGraphPointContact struct {
	// BoundaryMask uses bits 0/1/2/3 for UMin/UMax/VMin/VMax. Zero is interior.
	BoundaryMask uint8
	// NumeratorGradient is exact in original UV units; at contact its sign
	// agrees with the height derivative because the denominator is positive.
	NumeratorGradient             [2]*big.Rat
	NumeratorC1                   bool
	ContinuityInterfaces          int
	Proven                        bool
	Side                          int
	Parameters                    [2]*big.Rat
	Point                         [3]*big.Rat
	PointBounds                   Box
	Graph                         SurfaceInjectivity
	Certified, Unresolved         []ParameterRectangle
	VisitedCells, ExactOperations int
}

func CertifySplineGraphPointContact(ctx context.Context, a, c *SurfacePatch, uv [2]*big.Rat, o GraphPointContactOptions) (r SplineGraphPointContact, err error) {
	if ctx == nil || a == nil || c == nil || len(a.net) == 0 || len(c.net) == 0 || uv[0] == nil || uv[1] == nil {
		return r, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return r, e
	}
	if o.MaxDepth == 0 {
		o.MaxDepth = 12
	}
	if o.MaxCells == 0 {
		o.MaxCells = 4096
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 8000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	if o.MaxDepth < 0 || o.MaxCells < 1 || o.MaxOperations < 1 || o.MaxCoefficientBits < 64 {
		return r, ErrInvalid
	}
	b := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	defer func() { r.ExactOperations = b.operations }()
	if a.Domain() != c.Domain() {
		return r, ErrDomain
	}
	d := a.Domain()
	bounds := [2][2]float64{{d.UMin, d.UMax}, {d.VMin, d.VMax}}
	normalized := [2]*big.Rat{}
	for axis, x := range uv {
		if e := b.check(x); e != nil {
			return r, e
		}
		lo, hi := new(big.Rat).SetFloat64(bounds[axis][0]), new(big.Rat).SetFloat64(bounds[axis][1])
		if x.Cmp(lo) < 0 || x.Cmp(hi) > 0 {
			return r, ErrDomain
		}
		if x.Cmp(lo) == 0 {
			r.BoundaryMask |= 1 << uint(2*axis)
		}
		if x.Cmp(hi) == 0 {
			r.BoundaryMask |= 1 << uint(2*axis+1)
		}
		normalized[axis] = new(big.Rat).Quo(new(big.Rat).Sub(x, lo), new(big.Rat).Sub(hi, lo))
	}
	p, ok, e := commonXYDifference(a, c, b)
	if e != nil {
		return r, e
	}
	if !ok {
		remaining := o.MaxOperations - b.operations
		if remaining < 1 {
			return r, ErrConvergence
		}
		p, e = NewGeneralSurfaceHeightDifferencePatch(ctx, a, c, GeneralSurfaceHeightOptions{MaxCells: o.MaxCells, MaxOperations: remaining, MaxCoefficientBits: o.MaxCoefficientBits})
		if e != nil {
			return r, e
		}
		b.operations += p.operations
	}
	if p.degreeU < 2 || p.degreeV < 2 {
		return r, ErrConvergence
	}
	cells, e := graphContactBezierCells(ctx, p, o.MaxCells, b)
	if e != nil {
		return r, e
	}
	r.ContinuityInterfaces, e = graphContactC1(ctx, cells, b)
	if e != nil {
		return r, e
	}
	r.NumeratorC1 = true
	// Select one closed incident cell; C1 proof makes stationarity independent
	// of this choice, including contact exactly on a knot or grid vertex.
	var rootCell *SurfacePatch
	for _, cell := range cells {
		cd := cell.Domain()
		if uv[0].Cmp(new(big.Rat).SetFloat64(cd.UMin)) >= 0 && uv[0].Cmp(new(big.Rat).SetFloat64(cd.UMax)) <= 0 && uv[1].Cmp(new(big.Rat).SetFloat64(cd.VMin)) >= 0 && uv[1].Cmp(new(big.Rat).SetFloat64(cd.VMax)) <= 0 {
			rootCell = cell
			break
		}
	}
	if rootCell == nil {
		return r, ErrDomain
	}
	p = rootCell
	d = p.Domain()
	normalized[0] = new(big.Rat).Quo(new(big.Rat).Sub(uv[0], new(big.Rat).SetFloat64(d.UMin)), new(big.Rat).Sub(new(big.Rat).SetFloat64(d.UMax), new(big.Rat).SetFloat64(d.UMin)))
	normalized[1] = new(big.Rat).Quo(new(big.Rat).Sub(uv[1], new(big.Rat).SetFloat64(d.VMin)), new(big.Rat).Sub(new(big.Rat).SetFloat64(d.VMax), new(big.Rat).SetFloat64(d.VMin)))
	fields := make([]*big.Rat, len(p.net))
	for i, h := range p.net {
		fields[i] = h[2]
	}
	du, e := scalarPartial(fields, p.countU, p.countV, true, d, b)
	if e != nil {
		return r, e
	}
	dv, e := scalarPartial(fields, p.countU, p.countV, false, d, b)
	if e != nil {
		return r, e
	}
	for fieldIndex, field := range []struct {
		v      []*big.Rat
		nu, nv int
	}{{fields, p.countU, p.countV}, {du, p.countU - 1, p.countV}, {dv, p.countU, p.countV - 1}} {
		value, e := tensorScalarValue(field.v, field.nu, field.nv, normalized[0], normalized[1], b)
		if e != nil {
			return r, e
		}
		if fieldIndex == 0 {
			if value.Sign() != 0 {
				return r, ErrConvergence
			}
		} else {
			r.NumeratorGradient[fieldIndex-1] = new(big.Rat).Set(value)
		}
	}
	remaining := o.MaxOperations - b.operations
	if remaining < 1 {
		return r, ErrConvergence
	}
	r.Graph, e = CertifySurfaceInjectivity(a, SurfaceInjectivityOptions{ProjectionAxes: [2]int{0, 1}, MaxDepth: o.MaxDepth, MaxCells: o.MaxCells, MaxOperations: remaining, MaxCoefficientBits: o.MaxCoefficientBits})
	b.operations += r.Graph.ExactOperations
	if e != nil {
		return r, e
	}
	if !r.Graph.ProvenInjective {
		return r, ErrConvergence
	}
	type cell struct {
		p     *SurfacePatch
		depth int
	}
	stack := make([]cell, 0, len(cells))
	for _, p := range cells {
		stack = append(stack, cell{p, 0})
	}
	side := 0
	for len(stack) > 0 {
		if e := ctx.Err(); e != nil {
			return r, e
		}
		item := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		r.VisitedCells++
		if r.VisitedCells > o.MaxCells {
			r.Unresolved = append(r.Unresolved, item.p.Domain())
			for _, pending := range stack {
				r.Unresolved = append(r.Unresolved, pending.p.Domain())
			}
			return r, ErrConvergence
		}
		sign, e := graphNumeratorHessianSign(item.p, b)
		if e != nil {
			return r, e
		}
		if sign != 0 && (side == 0 || side == sign) {
			side = sign
			r.Certified = append(r.Certified, item.p.Domain())
			continue
		}
		if sign != 0 && side != 0 && sign != side {
			r.Unresolved = append(r.Unresolved, item.p.Domain())
			return r, ErrConvergence
		}
		if item.depth >= o.MaxDepth {
			r.Unresolved = append(r.Unresolved, item.p.Domain())
			continue
		}
		dom := item.p.Domain()
		axisU := (dom.UMax - dom.UMin) >= (dom.VMax - dom.VMin)
		lo, hi := dom.UMin, dom.UMax
		if !axisU {
			lo, hi = dom.VMin, dom.VMax
		}
		mid := lo + (hi-lo)/2
		if mid <= lo || mid >= hi {
			r.Unresolved = append(r.Unresolved, dom)
			continue
		}
		left, right, e := item.p.split(mid, axisU, b)
		if e != nil {
			return r, e
		}
		stack = append(stack, cell{right, item.depth + 1}, cell{left, item.depth + 1})
	}
	if len(r.Unresolved) > 0 || side == 0 {
		return r, ErrConvergence
	}
	// KKT signs orient the gradient into the feasible rectangle for a minimum
	// of Side*F. Strict Hessian definiteness supplies strict growth elsewhere,
	// even when a boundary derivative is exactly zero.
	for axis, g := range r.NumeratorGradient {
		sign := side * g.Sign()
		lower := r.BoundaryMask&(1<<uint(2*axis)) != 0
		upper := r.BoundaryMask&(1<<uint(2*axis+1)) != 0
		if (lower && sign < 0) || (upper && sign > 0) || (!lower && !upper && sign != 0) {
			return r, ErrConvergence
		}
	}
	remaining = o.MaxOperations - b.operations
	if remaining < 1 {
		return r, ErrConvergence
	}
	jet, e := a.EvaluateRational(ctx, uv[0], uv[1], SurfacePatchEvaluationOptions{MaxOperations: remaining, MaxCoefficientBits: o.MaxCoefficientBits})
	b.operations += jet.ExactOperations
	point := jet.Point
	if e != nil {
		return r, e
	}
	r.Point = point
	r.Parameters = [2]*big.Rat{new(big.Rat).Set(uv[0]), new(big.Rat).Set(uv[1])}
	r.PointBounds = Box{Vec3{rationalFloatBound(point[0], true), rationalFloatBound(point[1], true), rationalFloatBound(point[2], true)}, Vec3{rationalFloatBound(point[0], false), rationalFloatBound(point[1], false), rationalFloatBound(point[2], false)}}
	r.Side = side
	r.Proven = true
	return r, nil
}
func graphNumeratorHessianSign(p *SurfacePatch, b *polynomialBudget) (int, error) {
	f := make([]*big.Rat, len(p.net))
	for i, h := range p.net {
		f[i] = h[2]
	}
	d := p.Domain()
	du, e := scalarPartial(f, p.countU, p.countV, true, d, b)
	if e != nil {
		return 0, e
	}
	dv, e := scalarPartial(f, p.countU, p.countV, false, d, b)
	if e != nil {
		return 0, e
	}
	uu, e := scalarPartial(du, p.countU-1, p.countV, true, d, b)
	if e != nil {
		return 0, e
	}
	uv, e := scalarPartial(du, p.countU-1, p.countV, false, d, b)
	if e != nil {
		return 0, e
	}
	vv, e := scalarPartial(dv, p.countU, p.countV-1, false, d, b)
	if e != nil {
		return 0, e
	}
	a, c, cross := coefficientRange(uu), coefficientRange(vv), coefficientRange(uv)
	if a.sign() == 0 || a.sign() != c.sign() {
		return 0, nil
	}
	diagonal, e := intervalMultiply(a, c, b)
	if e != nil {
		return 0, e
	}
	square, e := intervalMultiply(cross, cross, b)
	if e != nil {
		return 0, e
	}
	det, e := intervalSubtract(diagonal, square, b)
	if e != nil {
		return 0, e
	}
	if det.sign() > 0 {
		return a.sign(), nil
	}
	return 0, nil
}
