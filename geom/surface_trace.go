package geom

import (
	"context"
	"math/big"
	"sort"
)

type surfaceRootIdentity struct {
	axis          int // fixed parameter axis
	fixed, lo, hi *big.Rat
	start, end    float64
	polynomial    rationalPolynomial // in normalized edge parameter
}

// SurfaceTraceOptions bounds local arc certification and exact endpoint joins.
// Zero join limits select 1M comparisons, 2M operations and 16384 bits.
type SurfaceTraceOptions struct {
	Arcs                                                      SurfacePlaneArcOptions
	MaxJoinComparisons, MaxJoinOperations, MaxCoefficientBits int
	// AllowC0SeamContinuation permits a one-sided transverse join across
	// a position-closed wrapped seam, including a crease. It applies to
	// regular noncorner roots with strict derivative bounds on both cells.
	AllowC0SeamContinuation bool
}

// OrientedSurfaceArc references an arc in SurfacePlaneTrace.Isolation.Arcs.
// Reversed traverses endpoint 1 to endpoint 0.
type OrientedSurfaceArc struct {
	Index    int
	Reversed bool
}

// SurfaceContour is an ordered parameter-space chain. Closed chains repeat no
// arc. Spatial self-overlap between distinct surface parameters is not excluded.
type SurfaceContour struct {
	Arcs   []OrientedSurfaceArc
	Closed bool
}

// SurfaceJoinIssue identifies an endpoint whose unique continuation is unproved.
type SurfaceJoinIssue struct {
	Arc, Endpoint int
	Reason        string
}

// SurfacePlaneTrace preserves the complete isolation diagnostics. Contours are
// emitted only for connected components without ambiguous or missing joins.
type SurfacePlaneTrace struct {
	Isolation SurfacePlaneArcs
	Contours  []SurfaceContour
	// CanonicalArcs maps each isolation arc to its unique representative after
	// exact duplicate boundary-arc elimination. Contours reference representatives.
	CanonicalArcs                   []int
	Unresolved                      []SurfaceJoinIssue
	JoinComparisons, JoinOperations int
	Complete                        bool
}

func physicalRootPolynomial(id *surfaceRootIdentity, b *polynomialBudget) (rationalPolynomial, error) {
	if len(id.polynomial) == 0 {
		return nil, ErrInvalid
	}
	width := new(big.Rat).Sub(new(big.Rat).SetFloat64(id.end), new(big.Rat).SetFloat64(id.start))
	linear := rationalPolynomial{new(big.Rat).Quo(new(big.Rat).Neg(new(big.Rat).SetFloat64(id.start)), width), new(big.Rat).Inv(width)}
	out := rationalPolynomial{new(big.Rat)}
	for i := len(id.polynomial) - 1; i >= 0; i-- {
		var e error
		out, e = polynomialProduct(out, linear, b)
		if e != nil {
			return nil, e
		}
		if len(out) == 0 {
			out = rationalPolynomial{new(big.Rat)}
		}
		out[0].Add(out[0], id.polynomial[i])
		if e = b.check(out[0]); e != nil {
			return nil, e
		}
	}
	return out, nil
}
func exactSurfaceRootMatch(a, c *surfaceRootIdentity, b *polynomialBudget) (bool, error) {
	if a.axis != c.axis {
		// Distinct supporting parameter lines can meet only at their exact crossing.
		x, y := c.fixed, a.fixed
		if x.Cmp(a.lo) < 0 || x.Cmp(a.hi) > 0 || y.Cmp(c.lo) < 0 || y.Cmp(c.hi) > 0 {
			return false, nil
		}
		for _, pair := range []struct {
			id *surfaceRootIdentity
			x  *big.Rat
		}{{a, x}, {c, y}} {
			if pair.id.lo.Cmp(pair.id.hi) == 0 {
				continue
			}
			p, e := physicalRootPolynomial(pair.id, b)
			if e != nil {
				return false, e
			}
			v, e := polynomialValue(p, pair.x, b)
			if e != nil {
				return false, e
			}
			if v.Sign() != 0 {
				return false, nil
			}
		}
		return true, nil
	}
	if a.fixed.Cmp(c.fixed) != 0 {
		return false, nil
	}
	lo, hi := a.lo, a.hi
	if c.lo.Cmp(lo) > 0 {
		lo = c.lo
	}
	if c.hi.Cmp(hi) < 0 {
		hi = c.hi
	}
	if lo.Cmp(hi) > 0 {
		return false, nil
	}
	if a.lo.Cmp(a.hi) == 0 && c.lo.Cmp(c.hi) == 0 {
		return true, nil
	}
	if a.lo.Cmp(a.hi) == 0 || c.lo.Cmp(c.hi) == 0 {
		point, other := a, c
		if c.lo.Cmp(c.hi) == 0 {
			point, other = c, a
		}
		poly, e := physicalRootPolynomial(other, b)
		if e != nil {
			return false, e
		}
		v, e := polynomialValue(poly, point.lo, b)
		return v != nil && v.Sign() == 0, e
	}
	ap, e := physicalRootPolynomial(a, b)
	if e != nil {
		return false, e
	}
	cp, e := physicalRootPolynomial(c, b)
	if e != nil {
		return false, e
	}
	gcd, e := polynomialGCD(ap, cp, b)
	if e != nil {
		return false, e
	}
	count := 0
	_, e = isolatePolynomialRoots(gcd, lo, hi, b, 128, func(_, _ *big.Rat, _ int) (bool, error) { count++; return true, nil })
	return count == 1, e
}
func rootOnOuterBoundary(id *surfaceRootIdentity, d ParameterRectangle) bool {
	limits := [2][2]float64{{d.UMin, d.UMax}, {d.VMin, d.VMax}}
	for _, v := range limits[id.axis] {
		if id.fixed.Cmp(new(big.Rat).SetFloat64(v)) == 0 {
			return true
		}
	}
	if id.lo.Cmp(id.hi) == 0 {
		for _, v := range limits[1-id.axis] {
			if id.lo.Cmp(new(big.Rat).SetFloat64(v)) == 0 {
				return true
			}
		}
	}
	return false
}

// TraceSurfacePlane stitches exact parameter-root identities, never rounded
// spatial coordinates. It supports open and closed chains on continuous clamped
// patches. Exactly matching internal boundary arcs are deduplicated; unmatched or
// ambiguous boundary arcs and branch/missing joins remain unresolved; no periodic seam identification is attempted. ErrConvergence
// preserves all certified arcs even when tracing cannot finish.
func TraceSurfacePlane(p *SurfacePatch, plane Plane, options SurfaceTraceOptions) (result SurfacePlaneTrace, err error) {
	return TraceSurfacePlaneContext(context.Background(), p, plane, options)
}

// TraceSurfacePlaneContext retains the isolation report if cancellation
// interrupts subdivision or endpoint stitching.
func TraceSurfacePlaneContext(ctx context.Context, p *SurfacePatch, plane Plane, options SurfaceTraceOptions) (result SurfacePlaneTrace, err error) {
	if ctx == nil {
		return result, ErrInvalid
	}
	o := options
	if o.MaxJoinComparisons == 0 {
		o.MaxJoinComparisons = 1000000
	}
	if o.MaxJoinOperations == 0 {
		o.MaxJoinOperations = 2000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	if o.MaxJoinComparisons < 1 || o.MaxJoinOperations < 1 || o.MaxCoefficientBits < 64 {
		return result, ErrInvalid
	}
	result.Isolation, err = CertifySurfacePlaneArcsContext(ctx, p, plane, o.Arcs)
	if err != nil && err != ErrConvergence {
		return result, err
	}
	isolationErr := err
	b := &polynomialBudget{maxOperations: o.MaxJoinOperations, maxBits: o.MaxCoefficientBits}
	defer func() { result.JoinOperations = b.operations }()
	arcs, e := alignBoundaryArcs(ctx, result.Isolation.Arcs, o, b, &result.JoinComparisons)
	if e != nil {
		return result, e
	}
	result.Isolation.Arcs = arcs
	n := len(arcs) * 2
	matches := make([][]int, n)
	bad := make([]bool, n)
	issue := func(i int, reason string) {
		bad[i] = true
		result.Unresolved = append(result.Unresolved, SurfaceJoinIssue{i / 2, i % 2, reason})
	}
	suppressed := make([]bool, len(arcs))
	result.CanonicalArcs = make([]int, len(arcs))
	for i := range arcs {
		result.CanonicalArcs[i] = i
	}
	partners := make([][]int, len(arcs))
	for i, a := range arcs {
		if !a.OnBoundary || boundaryArcIsOuter(a, p.Domain()) {
			continue
		}
		for j := i + 1; j < len(arcs); j++ {
			if e := ctx.Err(); e != nil {
				return result, e
			}
			if !arcs[j].OnBoundary {
				continue
			}
			result.JoinComparisons++
			if result.JoinComparisons > o.MaxJoinComparisons {
				issue(2*i, "boundary comparison limit")
				return result, ErrConvergence
			}
			same, e := sameBoundaryArc(a, arcs[j], b)
			if e != nil {
				issue(2*i, "boundary identity budget")
				return result, e
			}
			if same {
				partners[i] = append(partners[i], j)
				partners[j] = append(partners[j], i)
			}
		}
	}
	for i, a := range arcs {
		if !a.OnBoundary || boundaryArcIsOuter(a, p.Domain()) {
			continue
		}
		if len(partners[i]) != 1 || len(partners[partners[i][0]]) != 1 {
			issue(2*i, "unproved or ambiguous internal boundary arc")
			issue(2*i+1, "unproved or ambiguous internal boundary arc")
			continue
		}
		j := partners[i][0]
		if j < i {
			suppressed[i] = true
			result.CanonicalArcs[i] = j
		}
	}
	for i := 0; i < n; i++ {
		for j := i + 1; j < n; j++ {
			if e := ctx.Err(); e != nil {
				return result, e
			}
			if i/2 == j/2 || suppressed[i/2] || suppressed[j/2] {
				continue
			}
			result.JoinComparisons++
			if result.JoinComparisons > o.MaxJoinComparisons {
				issue(i, "join comparison limit")
				return result, ErrConvergence
			}
			same, e := exactSurfaceRootMatch(arcs[i/2].Endpoints[i%2].identity, arcs[j/2].Endpoints[j%2].identity, b)
			if e != nil {
				issue(i, "exact join budget or identity unresolved")
				return result, e
			}
			if same {
				matches[i] = append(matches[i], j)
				matches[j] = append(matches[j], i)
			}
		}
	}
	for i, m := range matches {
		if suppressed[i/2] {
			continue
		}
		if len(m) > 1 {
			issue(i, "multiple continuations")
		} else if len(m) == 0 && !rootOnOuterBoundary(arcs[i/2].Endpoints[i%2].identity, p.Domain()) {
			issue(i, "missing interior continuation")
		}
	}
	// Intersect certified enclosures of the same exact root. Both sides then
	// expose the same representative without choosing a rounded root parameter.
	for i, m := range matches {
		if len(m) != 1 || m[0] < i || len(matches[m[0]]) != 1 {
			continue
		}
		j := m[0]
		a, c := &arcs[i/2].Endpoints[i%2], &arcs[j/2].Endpoints[j%2]
		uv := UVBox{Vec2{max(a.Parameters.Min.X, c.Parameters.Min.X), max(a.Parameters.Min.Y, c.Parameters.Min.Y)}, Vec2{min(a.Parameters.Max.X, c.Parameters.Max.X), min(a.Parameters.Max.Y, c.Parameters.Max.Y)}}
		bounds := Box{Min: Vec3{max(a.Bounds.Min.X, c.Bounds.Min.X), max(a.Bounds.Min.Y, c.Bounds.Min.Y), max(a.Bounds.Min.Z, c.Bounds.Min.Z)}, Max: Vec3{min(a.Bounds.Max.X, c.Bounds.Max.X), min(a.Bounds.Max.Y, c.Bounds.Max.Y), min(a.Bounds.Max.Z, c.Bounds.Max.Z)}}
		a.Parameters, c.Parameters = uv, uv
		a.Bounds, c.Bounds = bounds, bounds
	}
	visited := make([]bool, len(arcs))
	for seed := range arcs {
		if e := ctx.Err(); e != nil {
			return result, e
		}
		if visited[seed] || suppressed[seed] {
			continue
		}
		component := []int{}
		todo := []int{seed}
		seen := map[int]bool{}
		valid := true
		start := 2 * seed
		for len(todo) > 0 {
			a := todo[len(todo)-1]
			todo = todo[:len(todo)-1]
			if seen[a] {
				continue
			}
			seen[a] = true
			component = append(component, a)
			for ep := 0; ep < 2; ep++ {
				i := 2*a + ep
				if bad[i] {
					valid = false
				}
				if len(matches[i]) == 0 {
					start = i
				}
				for _, j := range matches[i] {
					todo = append(todo, j/2)
				}
			}
		}
		for _, a := range component {
			visited[a] = true
		}
		if !valid {
			continue
		}
		contour := SurfaceContour{}
		entry := start
		for {
			contour.Arcs = append(contour.Arcs, OrientedSurfaceArc{entry / 2, entry%2 == 1})
			exit := entry ^ 1
			if len(matches[exit]) == 0 {
				break
			}
			entry = matches[exit][0]
			if entry == start {
				contour.Closed = true
				break
			}
			if len(contour.Arcs) > len(component) {
				return result, ErrConvergence
			}
		}
		result.Contours = append(result.Contours, contour)
	}
	for _, p := range result.Isolation.CornerContacts {
		found := false
		for i, a := range result.Isolation.Arcs {
			if suppressed[i] {
				continue
			}
			for _, x := range a.Endpoints {
				if e := ctx.Err(); e != nil {
					return result, e
				}
				result.JoinComparisons++
				if result.JoinComparisons > o.MaxJoinComparisons {
					return result, ErrConvergence
				}
				same, e := exactSurfaceRootMatch(p.identity, x.identity, b)
				if e != nil {
					return result, e
				}
				found = found || same
			}
			if found {
				break
			}
		}
		if !found {
			result.Unresolved = append(result.Unresolved, SurfaceJoinIssue{Arc: -1, Endpoint: -1, Reason: "corner-only contact has no certified incident arc"})
		}
	}
	if isolationErr != nil || len(result.Unresolved) > 0 {
		return result, ErrConvergence
	}
	result.Complete = true
	return result, nil
}

func boundaryArcIsOuter(a SurfacePlaneArc, d ParameterRectangle) bool {
	id := a.Endpoints[0].identity
	limits := [2][2]float64{{d.UMin, d.UMax}, {d.VMin, d.VMax}}
	for _, v := range limits[id.axis] {
		if id.fixed.Cmp(new(big.Rat).SetFloat64(v)) == 0 {
			return true
		}
	}
	return false
}

// sameBoundaryArc proves adjacent opposite boundaries have the same complete
// rational parameter curve, not merely the same endpoints. Exact homogeneous
// control vectors must agree up to one positive global scale.
func sameBoundaryArc(a, c SurfacePlaneArc, b *polynomialBudget) (bool, error) {
	if !a.OnBoundary || !c.OnBoundary || a.source == nil || c.source == nil || a.BoundaryEdge^1 != c.BoundaryEdge {
		return false, nil
	}
	ai, ci := a.Endpoints[0].identity, c.Endpoints[0].identity
	if ai.axis != ci.axis || ai.fixed.Cmp(ci.fixed) != 0 {
		return false, nil
	}
	for endpoint := 0; endpoint < 2; endpoint++ {
		x, y := a.Endpoints[endpoint].identity, c.Endpoints[endpoint].identity
		if x.lo.Cmp(y.lo) != 0 || x.hi.Cmp(y.hi) != 0 {
			return false, nil
		}
	}
	an, cn := a.source.edgeIndices(a.BoundaryEdge), c.source.edgeIndices(c.BoundaryEdge)
	if len(an) != len(cn) {
		return false, nil
	}
	aw, cw := a.source.net[an[0]][3], c.source.net[cn[0]][3]
	for i, index := range an {
		for axis := 0; axis < 4; axis++ {
			left := new(big.Rat).Mul(a.source.net[index][axis], cw)
			right := new(big.Rat).Mul(c.source.net[cn[i]][axis], aw)
			if e := b.check(left); e != nil {
				return false, e
			}
			if e := b.check(right); e != nil {
				return false, e
			}
			if left.Cmp(right) != 0 {
				return false, nil
			}
		}
	}
	return true, nil
}

// Align partial opposite-edge overlaps before comparing exact curve controls.
func alignBoundaryArcs(ctx context.Context, arcs []SurfacePlaneArc, o SurfaceTraceOptions, b *polynomialBudget, comparisons *int) ([]SurfacePlaneArc, error) {
	out := []SurfacePlaneArc{}
	parameterTolerance, boxTolerance := o.Arcs.EndpointParameterTolerance, o.Arcs.EndpointBoxTolerance
	if parameterTolerance == 0 {
		parameterTolerance = 1e-8
	}
	if boxTolerance == 0 {
		boxTolerance = 1e-8
	}
	for _, a := range arcs {
		if e := ctx.Err(); e != nil {
			return nil, e
		}
		if !a.OnBoundary {
			out = append(out, a)
			continue
		}
		id := a.Endpoints[0].identity
		lo, hi := a.Endpoints[0].identity.lo, a.Endpoints[1].identity.lo
		cuts := []float64{}
		for _, c := range arcs {
			if e := ctx.Err(); e != nil {
				return nil, e
			}
			if !c.OnBoundary {
				continue
			}
			*comparisons++
			if *comparisons > o.MaxJoinComparisons {
				return nil, ErrConvergence
			}
			ci := c.Endpoints[0].identity
			if id.axis != ci.axis || id.fixed.Cmp(ci.fixed) != 0 {
				continue
			}
			for _, ep := range c.Endpoints {
				x := ep.identity.lo
				if x.Cmp(lo) > 0 && x.Cmp(hi) < 0 {
					v, e := exactRepresentableFloat(x)
					if e != nil {
						return nil, ErrConvergence
					}
					cuts = append(cuts, v)
				}
			}
		}
		sort.Float64s(cuts)
		patch := a.source
		previous := 0.0
		havePrevious := false
		appendPart := func(p *SurfacePatch) error {
			part := a
			part.source = p
			part.Domain = p.Domain()
			part.Bounds = p.bounds
			for endpoint := 0; endpoint < 2; endpoint++ {
				x := big.NewRat(int64(endpoint), 1)
				ep, ok, e := p.endpoint(a.BoundaryEdge, x, x, 1, parameterTolerance, boxTolerance, b)
				if e != nil {
					return e
				}
				if !ok {
					return ErrConvergence
				}
				part.Endpoints[endpoint] = ep
			}
			out = append(out, part)
			return nil
		}
		for _, cut := range cuts {
			if havePrevious && cut == previous {
				continue
			}
			left, right, e := patch.split(cut, id.axis == 1, b)
			if e != nil {
				return nil, e
			}
			if e = appendPart(left); e != nil {
				return nil, e
			}
			patch = right
			previous, havePrevious = cut, true
		}
		if e := appendPart(patch); e != nil {
			return nil, e
		}
	}
	return out, nil
}
