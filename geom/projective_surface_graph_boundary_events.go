package geom

import (
	"context"
	"errors"
	"math/big"
	"sort"
)

// ProjectiveGraphBoundaryEventOptions adds a shared algebraic subdivision
// limit to the original boundary extraction limits.
type ProjectiveGraphBoundaryEventOptions struct {
	SurfaceAffineCurveOptions
	MaxSubdivisions int
}

// ProjectiveGraphBoundaryRoot is an isolated contact of two original graph
// boundaries. Stations follow the directed owner traces in Contact. The
// orders are one-sided in the common increasing projected direction; zero
// means that the edge ends on that side of the event.
type ProjectiveGraphBoundaryRoot struct {
	Stations                [2]*ExactAlgebraicRoot
	OriginalUV              [2][2]*ExactAlgebraicRoot
	OrderBefore, OrderAfter int
}

// ProjectiveGraphBoundaryEvents separates isolated algebraic contacts and
// maximal coincident intervals. Intervals use A's normalized directed
// station, with the exact B station law in Contact. When the projected
// intersection is a point, Contact retains its classification instead.
// An unresolved result publishes no partial isolated or coincident claims.
type ProjectiveGraphBoundaryEvents struct {
	Contact                       ProjectiveGraphBoundaryContact
	Isolated                      []ProjectiveGraphBoundaryRoot
	Coincident                    []ExactParameterInterval
	Complete                      bool
	ExactOperations, Subdivisions int
}

// CertifyProjectiveGraphBoundaryEvents also classifies unequal recovered
// boundary traces. Every original knot is pulled back before isolating the
// remaining model coordinate's exact polynomial roots on each common span.
// The direct algebraic degree limit applies to each nonzero difference.
func CertifyProjectiveGraphBoundaryEvents(ctx context.Context, source, target *SurfacePatch, axes [2]int,
	o ProjectiveGraphBoundaryEventOptions) (r ProjectiveGraphBoundaryEvents, err error) {
	if o.MaxOperations == 0 {
		o.MaxOperations = 8_000_000
	}
	limits, err := graphRootOptions(ExactCurvePlaneEventOptions{MaxOperations: o.MaxOperations,
		MaxCoefficientBits: o.MaxCoefficientBits, MaxSubdivisions: o.MaxSubdivisions})
	if err != nil {
		return r, err
	}
	r.Contact, err = CertifyProjectiveGraphBoundaryContact(ctx, source, target, axes, o.SurfaceAffineCurveOptions)
	b := &polynomialBudget{operations: r.Contact.ExactOperations, maxOperations: limits.MaxOperations, maxBits: limits.MaxCoefficientBits}
	defer func() { r.ExactOperations = b.operations }()
	zero, one := new(big.Rat), big.NewRat(1, 1)
	if err == nil && r.Contact.Complete {
		r.Complete = true
		if r.Contact.Edge {
			r.Coincident = []ExactParameterInterval{{zero, one}}
		}
		return r, nil
	}
	if !errors.Is(err, ErrConvergence) || !r.Contact.Owners[0].Complete || !r.Contact.Owners[1].Complete || r.Contact.BClockRatio == nil {
		return r, err
	}
	if e := ctx.Err(); e != nil {
		return r, e
	}
	if b.operations >= b.maxOperations {
		return r, ErrConvergence
	}
	group := [2][]SurfaceAffineCurvePiece{r.Contact.Owners[0].Pieces, nil}
	group[1], err = pullProjectiveCurvePieces(ctx, r.Contact.Owners[1].Pieces, r.Contact.BClockRatio, false, b)
	if err != nil {
		return r, err
	}
	cuts := []*big.Rat{zero}
	for _, pieces := range group {
		for _, p := range pieces {
			cuts = append(cuts, p.End)
		}
	}
	sort.Slice(cuts, func(i, j int) bool { return cuts[i].Cmp(cuts[j]) < 0 })
	unique := cuts[:1]
	for _, x := range cuts[1:] {
		if x.Cmp(unique[len(unique)-1]) != 0 {
			unique = append(unique, x)
		}
	}
	maxSpans := o.MaxPieces
	if maxSpans == 0 {
		maxSpans = 4096
	}
	if len(unique)-1 > maxSpans {
		return r, ErrConvergence
	}
	type event struct {
		root          *ExactGraphLineRoot
		knot          *big.Rat
		before, after int
	}
	var found []event
	var runs []ExactParameterInterval
	var at [2]int
	residual := 3 - axes[0] - axes[1]
	scalars := boundedRatArithmetic{b: b}
	for i, end := range unique[1:] {
		if err := ctx.Err(); err != nil {
			return r, err
		}
		start := unique[i]
		var fields [2][2]rationalPolynomial
		for owner, pieces := range group {
			for pieces[at[owner]].End.Cmp(start) <= 0 {
				at[owner]++
			}
			piece := pieces[at[owner]]
			width := scalars.sub(piece.End, piece.Start)
			lo, hi := scalars.quo(scalars.sub(start, piece.Start), width), scalars.quo(scalars.sub(end, piece.Start), width)
			if scalars.err != nil {
				return r, scalars.err
			}
			p := piece.Curve.patch
			net := make([]exactHomogeneous, p.countU)
			for j := range net {
				net[j] = p.net[j*p.countV]
			}
			net, err = restrictHomogeneousBezier(net, lo, hi, b)
			if err != nil {
				return r, err
			}
			for k, axis := range [2]int{residual, 3} {
				controls := make([]*big.Rat, len(net))
				for j := range net {
					controls[j] = net[j][axis]
				}
				fields[owner][k], err = bernsteinToPower(controls, b)
				if err != nil {
					return r, err
				}
			}
		}
		p, err := offsetPolynomialDet(fields[0][0], fields[0][1], fields[1][0], fields[1][1], b)
		if err != nil {
			return r, err
		}
		if len(p) == 0 {
			if n := len(runs); n != 0 && runs[n-1].Upper.Cmp(start) == 0 {
				runs[n-1].Upper = new(big.Rat).Set(end)
			} else {
				runs = append(runs, ExactParameterInterval{new(big.Rat).Set(start), new(big.Rat).Set(end)})
			}
			continue
		}
		if len(p) == 1 {
			continue
		}
		if len(p) > ExactPolynomialMaxDegree+1 || b.operations >= b.maxOperations || r.Subdivisions >= limits.MaxSubdivisions {
			return r, ErrConvergence
		}
		isolated, err := IsolateExactPolynomialRoots(ctx, p, zero, one, ExactCurvePlaneEventOptions{
			MaxOperations: b.maxOperations - b.operations, MaxCoefficientBits: b.maxBits, MaxSubdivisions: limits.MaxSubdivisions - r.Subdivisions})
		b.operations += isolated.Operations
		r.Subdivisions += isolated.Subdivisions
		if err != nil {
			return r, err
		}
		for _, x := range isolated.Roots {
			record := event{before: x.Multiplicity, after: x.Multiplicity}
			at := x.Root.Interval()
			if at.Upper.Sign() == 0 {
				record.knot, record.before = start, 0
			} else if at.Lower.Cmp(one) == 0 {
				record.knot, record.after = end, 0
			}
			if n := len(found); n != 0 && found[n-1].knot != nil && record.knot != nil && found[n-1].knot.Cmp(record.knot) == 0 {
				found[n-1].after = record.after
				continue
			}
			record.root, err = affineMapRoot(x.Root.root, start, new(big.Rat).Sub(end, start), b)
			if err != nil {
				return r, err
			}
			found = append(found, record)
		}
	}
	var events []ProjectiveGraphBoundaryRoot
	for _, e := range found {
		if err := ctx.Err(); err != nil {
			return r, err
		}
		inRun := false
		if e.knot != nil {
			for _, span := range runs {
				inRun = inRun || e.knot.Cmp(span.Lower) >= 0 && e.knot.Cmp(span.Upper) <= 0
			}
		}
		if inRun {
			continue
		}
		other, err := projectiveBoundaryClockRoot(e.root, r.Contact.BClockRatio, b)
		if err != nil {
			return r, err
		}
		item := ProjectiveGraphBoundaryRoot{Stations: [2]*ExactAlgebraicRoot{{root: e.root}, {root: other}}, OrderBefore: e.before, OrderAfter: e.after}
		for owner, station := range item.Stations {
			for axis, first := range r.Contact.Endpoints[0].OriginalUV[owner] {
				last := r.Contact.Endpoints[1].OriginalUV[owner][axis]
				raw, err := affineMapRoot(station.root, first, new(big.Rat).Sub(last, first), b)
				if err != nil {
					return r, err
				}
				item.OriginalUV[owner][axis] = &ExactAlgebraicRoot{root: raw}
			}
		}
		events = append(events, item)
	}
	r.Isolated, r.Coincident, r.Complete = events, runs, true
	if len(events)+len(runs) == 0 {
		r.Contact.Complete, r.Contact.Disjoint = true, true
	}
	return r, nil
}

func projectiveBoundaryClockRoot(root *ExactGraphLineRoot, ratio *big.Rat, b *polynomialBudget) (*ExactGraphLineRoot, error) {
	if ratio.Cmp(big.NewRat(1, 1)) == 0 {
		return root, nil
	}
	a := boundedRatArithmetic{b: b}
	one := big.NewRat(1, 1)
	n := len(root.polynomial) - 1
	d := rationalPolynomial{ratio, a.sub(one, ratio)}
	if a.err != nil {
		return nil, a.err
	}
	// Invert the positive station law to t=s/(r+(1-r)*s), then
	// multiply by the strictly positive denominator to degree n.
	powers := make([]rationalPolynomial, n+1)
	powers[0] = rationalPolynomial{one}
	for i := 1; i <= n; i++ {
		var err error
		powers[i], err = polynomialProduct(powers[i-1], d, b)
		if err != nil {
			return nil, err
		}
	}
	p := make(rationalPolynomial, n+1)
	for i := range p {
		p[i] = new(big.Rat)
	}
	for i, x := range root.polynomial {
		for j, y := range powers[n-i] {
			p[i+j] = a.add(p[i+j], a.mul(x, y))
		}
	}
	var bounds [2]*big.Rat
	k := a.sub(ratio, one)
	for i, x := range root.bounds {
		bounds[i] = a.quo(a.mul(ratio, x), a.add(one, a.mul(k, x)))
	}
	if a.err != nil {
		return nil, a.err
	}
	return &ExactGraphLineRoot{polynomial: trimPolynomial(p), bounds: bounds}, nil
}
