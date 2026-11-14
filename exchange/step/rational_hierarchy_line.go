package step

import (
	"context"
	"math/big"
	"plainsolid/geom"
)

// RationalHierarchyLineOptions bounds all root isolation/comparison work together.
// Defaults are 2,000,000 operations, 4096 coefficient bits, 4096 subdivisions.
// Source reproof is independently bounded. Only sphere and ring-torus leaves
// are clipped; all other occurrences are returned explicitly as unsupported.
type RationalHierarchyLineOptions struct {
	MaxOperations, MaxCoefficientBits, MaxSubdivisions int
	Source                                             Limits
}
type HierarchyLineOwner struct {
	ProductID, SourceProductID, SourceBodyID int64
	Path, SourceFaceIDs                      []int64
	Family                                   string
	Multiplicity                             int
}

// HierarchyLineHit retains every exactly simultaneous occurrence owner.
// Parameter is the caller's original t in origin+t*direction, never arc length.
type HierarchyLineHit struct {
	Parameter *geom.ExactAlgebraicRoot
	Owners    []HierarchyLineOwner
}

// HierarchyLineSpan has a strictly interior open parameter interval. Its closed
// endpoints are included by material continuity, including finite clip ends.
type HierarchyLineSpan struct{ Lower, Upper *geom.ExactAlgebraicRoot }
type HierarchyLineLeaf struct {
	Owner                HierarchyLineOwner
	Supported            bool
	LowerSign, UpperSign int
	Spans                []HierarchyLineSpan
}
type RationalHierarchyLine struct {
	Leaves                   []HierarchyLineLeaf
	Hits                     []HierarchyLineHit
	LengthUnit               float64
	Operations, Subdivisions int
}

func ClipRationalHierarchyAnalyticLine(ctx context.Context, m RationalSphericalHierarchy, origin, direction [3]*big.Rat, lower, upper *big.Rat, o RationalHierarchyLineOptions) (RationalHierarchyLine, error) {
	return clipRationalHierarchyLine(ctx, m, origin, direction, lower, upper, o, false)
}
func ClipOCCTRationalHierarchyAnalyticLine(ctx context.Context, m OCCTRationalSphericalHierarchy, origin, direction [3]*big.Rat, lower, upper *big.Rat, o RationalHierarchyLineOptions) (RationalHierarchyLine, error) {
	return clipRationalHierarchyLine(ctx, RationalSphericalHierarchy(m), origin, direction, lower, upper, o, true)
}
func clipRationalHierarchyLine(ctx context.Context, m RationalSphericalHierarchy, origin, direction [3]*big.Rat, lower, upper *big.Rat, o RationalHierarchyLineOptions, vendor bool) (RationalHierarchyLine, error) {
	bad := func(e error) (RationalHierarchyLine, error) { return RationalHierarchyLine{}, e }
	if ctx == nil || lower == nil || upper == nil || lower.Cmp(upper) >= 0 {
		return bad(geom.ErrInvalid)
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 2000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 4096
	}
	if o.MaxSubdivisions == 0 {
		o.MaxSubdivisions = 4096
	}
	if o.MaxOperations < 1 || o.MaxOperations > 20000000 || o.MaxCoefficientBits < 256 || o.MaxCoefficientBits > 8192 || o.MaxSubdivisions < 1 || o.MaxSubdivisions > 65536 {
		return bad(geom.ErrInvalid)
	}
	a := hierarchyPointArithmetic{bits: o.MaxCoefficientBits}
	nonzero := false
	for i := 0; i < 3; i++ {
		if origin[i] == nil || direction[i] == nil {
			return bad(geom.ErrInvalid)
		}
		a.check(origin[i])
		a.check(direction[i])
		nonzero = nonzero || direction[i].Sign() != 0
	}
	a.check(lower)
	a.check(upper)
	if !nonzero {
		return bad(geom.ErrInvalid)
	}
	if a.err != nil {
		return bad(a.err)
	}
	origin = copyHierarchyRats(origin)
	direction = copyHierarchyRats(direction)
	lower = new(big.Rat).Set(lower)
	upper = new(big.Rat).Set(upper)
	// The point query authenticates the whole graph and returns exact local origins.
	locations, e := locateHierarchyAnalyticPoint(ctx, m, origin, RationalHierarchyPointOptions{MaxCoefficientBits: o.MaxCoefficientBits, Source: o.Source}, vendor)
	if e != nil {
		return bad(e)
	}
	out := RationalHierarchyLine{LengthUnit: locations.LengthUnit}
	opts := func() (geom.ExactCurvePlaneEventOptions, error) {
		if out.Operations >= o.MaxOperations || out.Subdivisions >= o.MaxSubdivisions {
			return geom.ExactCurvePlaneEventOptions{}, geom.ErrConvergence
		}
		return geom.ExactCurvePlaneEventOptions{MaxOperations: o.MaxOperations - out.Operations, MaxCoefficientBits: o.MaxCoefficientBits, MaxSubdivisions: o.MaxSubdivisions - out.Subdivisions}, nil
	}
	compare := func(x, y *geom.ExactAlgebraicRoot) (int, error) {
		op, e := opts()
		if e != nil {
			return 0, e
		}
		r, e := x.Compare(ctx, y, op)
		out.Operations += r.Operations
		out.Subdivisions += r.Subdivisions
		if e != nil {
			return 0, e
		}
		if !r.Proven {
			return 0, geom.ErrConvergence
		}
		return r.Order, nil
	}
	op, e := opts()
	if e != nil {
		return bad(e)
	}
	lo, n, e := geom.NewExactRationalRoot(ctx, lower, op)
	out.Operations += n
	if e != nil {
		return bad(e)
	}
	op, e = opts()
	if e != nil {
		return bad(e)
	}
	hi, n, e := geom.NewExactRationalRoot(ctx, upper, op)
	out.Operations += n
	if e != nil {
		return bad(e)
	}
	for k, location := range locations.Leaves {
		if e = ctx.Err(); e != nil {
			return bad(e)
		}
		owner := HierarchyLineOwner{ProductID: location.ProductID, SourceProductID: location.SourceProductID, SourceBodyID: location.SourceBodyID, Path: append([]int64(nil), location.Path...), SourceFaceIDs: append([]int64(nil), location.SourceFaceIDs...), Family: location.Family}
		leaf := HierarchyLineLeaf{Owner: owner}
		if location.Family != "sphere" && location.Family != "torus" {
			out.Leaves = append(out.Leaves, leaf)
			continue
		}
		leaf.Supported = true
		p := m.Products[location.ProductID]
		u := m.Instances[k]
		var frame geom.Frame
		var radius, major *big.Rat
		if p.Sphere != nil {
			frame = p.Sphere.Frame
			radius = new(big.Rat).SetFloat64(p.Sphere.Radius)
		} else {
			frame = p.Torus.Frame
			radius = new(big.Rat).SetFloat64(p.Torus.MinorRadius)
			major = new(big.Rat).SetFloat64(p.Torus.MajorRadius)
		}
		zero := [3]*big.Rat{new(big.Rat), new(big.Rat), new(big.Rat)}
		d := a.inverse(direction, zero, [3][3]*big.Rat{u.pose.X, u.pose.Y, u.pose.Z})
		d = a.inverse(d, zero, [3][3]*big.Rat{mappedRatVec(frame.XAxis()), mappedRatVec(frame.YAxis()), mappedRatVec(frame.ZAxis())})
		x := location.AnalyticCoordinates()
		var squares [3][3]*big.Rat
		for i := 0; i < 3; i++ {
			squares[i] = [3]*big.Rat{a.mul(x[i], x[i]), a.mul(big.NewRat(2, 1), a.mul(x[i], d[i])), a.mul(d[i], d[i])}
		}
		poly := make([]*big.Rat, 3)
		for i := range poly {
			poly[i] = a.add(a.add(squares[0][i], squares[1][i]), squares[2][i])
		}
		poly[0] = a.sub(poly[0], a.mul(radius, radius))
		if major != nil {
			R2 := a.mul(major, major)
			poly[0] = a.add(poly[0], R2)
			q := make([]*big.Rat, 5)
			for i := range q {
				q[i] = new(big.Rat)
			}
			for i := range poly {
				for j := range poly {
					q[i+j] = a.add(q[i+j], a.mul(poly[i], poly[j]))
				}
			}
			for i := 0; i < 3; i++ {
				q[i] = a.sub(q[i], a.mul(big.NewRat(4, 1), a.mul(R2, a.add(squares[0][i], squares[1][i]))))
			}
			poly = q
		}
		if a.err != nil {
			return bad(a.err)
		}
		op, e = opts()
		if e != nil {
			return bad(e)
		}
		roots, e := geom.IsolateExactPolynomialRoots(ctx, poly, lower, upper, op)
		out.Operations += roots.Operations
		out.Subdivisions += roots.Subdivisions
		if e != nil {
			return bad(e)
		}
		leaf.LowerSign = roots.LowerSign
		leaf.UpperSign = roots.UpperSign
		// Exact right-hand sign at lower, even when it is a repeated endpoint root.
		derivative := append([]*big.Rat(nil), poly...)
		sign := 0
		for sign == 0 && len(derivative) > 0 {
			q := new(big.Rat)
			for i := len(derivative) - 1; i >= 0; i-- {
				q = a.add(a.mul(q, lower), derivative[i])
			}
			sign = q.Sign()
			next := make([]*big.Rat, len(derivative)-1)
			for i := range next {
				next[i] = a.mul(derivative[i+1], big.NewRat(int64(i+1), 1))
			}
			derivative = next
		}
		if a.err != nil {
			return bad(a.err)
		}
		previous := lo
		for _, r := range roots.Roots {
			order, e := compare(r.Root, lo)
			if e != nil {
				return bad(e)
			}
			if order > 0 {
				if sign < 0 {
					leaf.Spans = append(leaf.Spans, HierarchyLineSpan{previous, r.Root})
				}
				if r.Multiplicity%2 == 1 {
					sign = -sign
				}
				previous = r.Root
			}
			own := owner
			own.Multiplicity = r.Multiplicity
			inserted := false
			for i := range out.Hits {
				order, e := compare(r.Root, out.Hits[i].Parameter)
				if e != nil {
					return bad(e)
				}
				if order == 0 {
					out.Hits[i].Owners = append(out.Hits[i].Owners, own)
					inserted = true
					break
				}
				if order < 0 {
					out.Hits = append(out.Hits, HierarchyLineHit{})
					copy(out.Hits[i+1:], out.Hits[i:])
					out.Hits[i] = HierarchyLineHit{r.Root, []HierarchyLineOwner{own}}
					inserted = true
					break
				}
			}
			if !inserted {
				out.Hits = append(out.Hits, HierarchyLineHit{r.Root, []HierarchyLineOwner{own}})
			}
		}
		order, e := compare(previous, hi)
		if e != nil {
			return bad(e)
		}
		if sign < 0 && order < 0 {
			leaf.Spans = append(leaf.Spans, HierarchyLineSpan{previous, hi})
		}
		out.Leaves = append(out.Leaves, leaf)
	}
	return out, nil
}
