package geom

import (
	"context"
	"math/big"
	"slices"
)

// SurfaceNormalOffsetSeamOptions describes normalized edge correspondence
// and the original face orientations. ReverseB aligns B's decreasing free
// parameter with A's increasing one. FlipNormal selects the outward
// orientation when an original face chart has the opposite winding.
type SurfaceNormalOffsetSeamOptions struct {
	ReverseB, FlipNormalA, FlipNormalB bool
	MaxOperations, MaxCoefficientBits  int
}

// SurfaceNormalOffsetSeam is an exact certificate that coincident original
// edges have identical oriented unit normals at every matched station.
// Complete then implies identical boundary locations after any common signed
// normal displacement. It does not establish adjacent interior disjointness,
// a smooth derivative across the offset seam, or a valid final shell.
type SurfaceNormalOffsetSeam struct {
	Complete, SharedOriginalEdge bool
	ExactOperations              int
}

// CertifySurfaceNormalOffsetSeam proves correspondence on two single
// rational Bezier patch edges. It compares rational point functions and
// proves that the original normal numerators are everywhere parallel and
// agree in sign, including endpoints and any interior zeros. A mismatched
// edge, crease, reversed normal or singular station leaves Complete false.
func CertifySurfaceNormalOffsetSeam(ctx context.Context, a, b *SurfacePatch, edgeA, edgeB SurfaceEdge,
	o SurfaceNormalOffsetSeamOptions) (r SurfaceNormalOffsetSeam, err error) {
	if ctx == nil || a == nil || b == nil || len(a.net) == 0 || len(b.net) == 0 ||
		edgeA > SurfaceEdgeVMax || edgeB > SurfaceEdgeVMax {
		return r, ErrInvalid
	}
	if a.countU != a.degreeU+1 || a.countV != a.degreeV+1 || b.countU != b.degreeU+1 || b.countV != b.degreeV+1 {
		return r, ErrDomain
	}
	limits, err := patchOptions(SurfacePatchOptions{MaxOperations: o.MaxOperations, MaxCoefficientBits: o.MaxCoefficientBits})
	if err != nil {
		return r, err
	}
	budget := &polynomialBudget{maxOperations: limits.MaxOperations, maxBits: limits.MaxCoefficientBits}
	defer func() { r.ExactOperations = budget.operations }()
	x, err := offsetEdgePowers(ctx, a, edgeA, false, budget)
	if err != nil {
		return r, err
	}
	y, err := offsetEdgePowers(ctx, b, edgeB, o.ReverseB, budget)
	if err != nil {
		return r, err
	}
	for axis := range 3 {
		d, err := offsetPolynomialDet(x[0][axis], x[0][3], y[0][axis], y[0][3], budget)
		if err != nil || len(d) != 0 {
			return r, err
		}
	}
	r.SharedOriginalEdge = true
	nx, err := offsetNormalPowers(ctx, x, budget)
	if err != nil {
		return r, err
	}
	ny, err := offsetNormalPowers(ctx, y, budget)
	if err != nil {
		return r, err
	}
	var dot rationalPolynomial
	for i := range 3 {
		j, k := (i+1)%3, (i+2)%3
		d, err := offsetPolynomialDet(nx[j], nx[k], ny[j], ny[k], budget)
		if err != nil || len(d) != 0 {
			return r, err
		}
		p, err := polynomialProduct(nx[i], ny[i], budget)
		if err != nil {
			return r, err
		}
		dot, err = graphPolynomialSum(dot, p, 1, budget)
		if err != nil {
			return r, err
		}
	}
	sign := 1
	if o.FlipNormalA != o.FlipNormalB {
		sign = -1
	}
	start, end := new(big.Rat), big.NewRat(1, 1)
	for _, t := range []*big.Rat{start, end} {
		d, err := polynomialValue(dot, t, budget)
		if err != nil || d.Sign() != sign {
			return r, err
		}
	}
	if err := ctx.Err(); err != nil {
		return r, err
	}
	sequence, err := polynomialSturm(dot, budget)
	if err != nil {
		return r, err
	}
	roots, err := sturmCount(sequence, start, end, budget)
	if err != nil {
		return r, err
	}
	if err := ctx.Err(); err != nil {
		return r, err
	}
	r.Complete = roots == 0
	return r, nil
}

// offsetEdgePowers returns homogeneous H, H_u and H_v along the increasing
// station of the requested correspondence. Each partial omits one positive
// constant scale (degree divided by physical width). This does not change
// the normal direction. Reversing the station never flips a source partial.
func offsetEdgePowers(ctx context.Context, p *SurfacePatch, edge SurfaceEdge, reverse bool,
	b *polynomialBudget) (jet [3][4]rationalPolynomial, err error) {
	indices := p.edgeIndices(edge)
	a := boundedRatArithmetic{b: b}
	stride, fixed, free := p.countV, 1, 2
	if edge >= SurfaceEdgeVMin {
		stride, fixed, free = 1, 2, 1
	}
	high := edge == SurfaceEdgeUMax || edge == SurfaceEdgeVMax
	for axis := range 4 {
		if err := ctx.Err(); err != nil {
			return jet, err
		}
		h := make([]*big.Rat, len(indices))
		fixedPartial := make([]*big.Rat, len(indices))
		freePartial := make([]*big.Rat, len(indices)-1)
		for i, pole := range indices {
			h[i] = p.net[pole][axis]
			if err := b.check(h[i]); err != nil {
				return jet, err
			}
			if high {
				fixedPartial[i] = a.sub(h[i], p.net[pole-stride][axis])
			} else {
				fixedPartial[i] = a.sub(p.net[pole+stride][axis], h[i])
			}
			if i > 0 {
				freePartial[i-1] = a.sub(h[i], h[i-1])
			}
		}
		if a.err != nil {
			return jet, a.err
		}
		var byDirection [3][]*big.Rat
		byDirection[0], byDirection[fixed], byDirection[free] = h, fixedPartial, freePartial
		for dir, controls := range byDirection {
			if reverse {
				slices.Reverse(controls)
			}
			jet[dir][axis], err = bernsteinToPower(controls, b)
			if err != nil {
				return jet, err
			}
		}
	}
	return jet, nil
}

func offsetNormalPowers(ctx context.Context, jet [3][4]rationalPolynomial, b *polynomialBudget) (n [3]rationalPolynomial, err error) {
	var d [2][3]rationalPolynomial
	for dir := range 2 {
		if err := ctx.Err(); err != nil {
			return n, err
		}
		for i := range 3 {
			// Differentiate H/W without division; positive W² does not
			// change the tangent vector's orientation.
			d[dir][i], err = offsetPolynomialDet(jet[dir+1][i], jet[dir+1][3], jet[0][i], jet[0][3], b)
			if err != nil {
				return n, err
			}
		}
	}
	for i := range 3 {
		j, k := (i+1)%3, (i+2)%3
		n[i], err = offsetPolynomialDet(d[0][j], d[0][k], d[1][j], d[1][k], b)
		if err != nil {
			return n, err
		}
	}
	return n, nil
}

func offsetPolynomialDet(a, b, c, d rationalPolynomial, budget *polynomialBudget) (rationalPolynomial, error) {
	x, err := polynomialProduct(a, d, budget)
	if err != nil {
		return nil, err
	}
	y, err := polynomialProduct(b, c, budget)
	if err != nil {
		return nil, err
	}
	return graphPolynomialSum(x, y, -1, budget)
}
