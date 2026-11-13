package trimmed

import (
	"context"
	"errors"
	"math/big"
	"plainsolid/geom"
	"plainsolid/intersect"
)

// AlgebraicSplineTerminal retains the verified four original chart roots and
// the physical boundary side of one operand. It is evidence for a terminal,
// not a rounded split proposal and not a complete branch-coverage proof.
type AlgebraicSplineTerminal struct {
	Operand     int
	Side        geom.SurfaceEdge
	Certificate geom.TubeAlgebraicGraphEndpointCertificate
}

// AlgebraicSplineBranchTerminal identifies an end in the oriented branch
// segment sequence. The exact proposal belongs to the source iso parameter.
type AlgebraicSplineBranchTerminal struct {
	Operand int
	Side    geom.SurfaceEdge
	Root    *geom.ExactAlgebraicRoot
}

type AlgebraicSplineBranchTerminals struct {
	Ends     [2]AlgebraicSplineTerminal
	Segments int
}

// AlgebraicSplineBranchEvidence revalidates every adjacent junction in
// addition to both original boundary terminals. Complete means the supplied
// sequence is a continuous exact path, not that it exhausts a face pair.
type AlgebraicSplineBranchEvidence struct {
	AlgebraicSplineBranchTerminals
	Junctions []intersect.TubeAlgebraicEndpointComparison
	Complete  bool
}

// AlgebraicSplinePairSection keeps independently proved face-pair coverage
// alongside exact evidence for each discovered open branch. The result is a
// section graph, not an edited B-rep or a material classification.
type AlgebraicSplinePairSection struct {
	Trace    intersect.SplinePairTrace
	Open     []AlgebraicSplineBranchEvidence
	Closed   int
	Complete bool
	recipes  []nativeTubeRecipe
}

// SectionSplinePairAlgebraic builds a complete section report on the subset
// where every regular open branch admits exact graph endpoint and junction
// proofs. Supplied seed cells are optional and are never accepted as proof
// without independent coverage and branch certification.
func SectionSplinePairAlgebraic(ctx context.Context, a, b intersect.Patch, seeds *intersect.SplineSeedReport,
	trace intersect.SplinePairTraceOptions, o geom.TubeAlgebraicGraphEndpointOptions) (AlgebraicSplinePairSection, error) {
	var out AlgebraicSplinePairSection
	if ctx == nil {
		return out, geom.ErrInvalid
	}
	var e error
	var recipes []nativeTubeRecipe
	if seeds == nil {
		out.Trace, e = intersect.TraceSplinePair(ctx, a, b, trace)
	} else {
		out.Trace, e = intersect.CertifySplinePairBranches(ctx, a, b, *seeds, trace)
	}
	if e != nil {
		return out, e
	}
	if !out.Trace.Complete || len(out.Trace.Curves) != len(out.Trace.BranchGraph.Branches) {
		return out, geom.ErrConvergence
	}
	if len(out.Trace.Curves) > 256 {
		return out, geom.ErrConvergence
	}
	for i, q := range out.Trace.BranchGraph.Branches {
		if e := ctx.Err(); e != nil {
			return out, e
		}
		if len(out.Trace.Curves[i]) == 0 {
			return out, geom.ErrConvergence
		}
		recipe, _, _ := out.Trace.Curves[i][0].TrimA().Recipe()
		native, e := encodeTubeRecipe(recipe.Data())
		if e != nil {
			return out, e
		}
		recipes = append(recipes, native)
		if q.Closed {
			out.Closed++
			continue
		}
		if q.StartUnresolved || q.EndUnresolved {
			return out, geom.ErrConvergence
		}
		x, e := DiscoverAlgebraicSplineBranch(ctx, out.Trace.Curves[i], a, b, o)
		if e != nil {
			return out, e
		}
		if !x.Complete {
			return out, geom.ErrConvergence
		}
		out.Open = append(out.Open, x)
	}
	out.Complete = true
	out.recipes = recipes
	return out, nil
}

// CertifyAlgebraicSplineBranch checks independently supplied algebraic root
// pairs at all consecutive junctions. A comparison proves both original
// charts agree at one exact model point, never merely that boxes overlap.
func CertifyAlgebraicSplineBranch(ctx context.Context, segments []intersect.TubeSegment, a, b intersect.Patch,
	ends [2]AlgebraicSplineBranchTerminal, junctions [][2]*geom.ExactAlgebraicRoot, o geom.TubeAlgebraicGraphEndpointOptions) (AlgebraicSplineBranchEvidence, error) {
	var empty AlgebraicSplineBranchEvidence
	if ctx == nil || len(segments) == 0 || len(junctions) != len(segments)-1 || len(segments) > 256 {
		return empty, geom.ErrInvalid
	}
	limit := o.MaxOperations
	if limit == 0 {
		limit = 8000000
	}
	if limit < 1 {
		return empty, geom.ErrInvalid
	}
	for _, s := range segments {
		for i, p := range [2]intersect.Patch{a, b} {
			model, ok := p.Surface.(*geom.BSplineSurface)
			trim := s.TrimA()
			if i == 1 {
				trim = s.TrimB()
			}
			if !ok || !geom.SameSplineSurface(model, trim.Support()) {
				return empty, geom.ErrInvalid
			}
		}
	}
	out := AlgebraicSplineBranchEvidence{}
	for i, s := range [2]intersect.TubeSegment{segments[0], segments[len(segments)-1]} {
		if limit < 1 {
			return empty, geom.ErrConvergence
		}
		child := o
		child.MaxOperations = limit
		p := ends[i]
		x, e := CertifyAlgebraicSplineTerminal(ctx, s, i, a, b, p.Operand, p.Side, p.Root, child)
		if e != nil {
			return empty, e
		}
		limit -= x.Certificate.ExactOperations
		out.Ends[i] = x
	}
	for i, p := range junctions {
		if limit < 1 {
			return empty, geom.ErrConvergence
		}
		child := o
		child.MaxOperations = limit
		r, e := intersect.CompareTubeAlgebraicGraphEndpoints(ctx, segments[i], segments[i+1], 1, 0, p[0], p[1], child)
		if e != nil {
			return empty, e
		}
		if !r.ProvenEqual {
			return empty, geom.ErrConvergence
		}
		limit -= r.ExactOperations
		out.Junctions = append(out.Junctions, r)
	}
	out.Segments = len(segments)
	out.Complete = true
	return out, nil
}

// CertifyAlgebraicSplineBranchTerminals validates both endpoints of a supplied
// oriented segment sequence. Intermediate junctions and complete coverage are
// separate requirements; this function does not assemble or classify material.
func CertifyAlgebraicSplineBranchTerminals(ctx context.Context, segments []intersect.TubeSegment, a, b intersect.Patch,
	ends [2]AlgebraicSplineBranchTerminal, o geom.TubeAlgebraicGraphEndpointOptions) (AlgebraicSplineBranchTerminals, error) {
	var empty AlgebraicSplineBranchTerminals
	if ctx == nil || len(segments) == 0 || len(segments) > 4096 {
		return empty, geom.ErrInvalid
	}
	var out AlgebraicSplineBranchTerminals
	for i, s := range [2]intersect.TubeSegment{segments[0], segments[len(segments)-1]} {
		p := ends[i]
		r, e := CertifyAlgebraicSplineTerminal(ctx, s, i, a, b, p.Operand, p.Side, p.Root, o)
		if e != nil {
			return empty, e
		}
		out.Ends[i] = r
	}
	out.Segments = len(segments)
	return out, nil
}

// CertifyAlgebraicSplineTerminal binds a proposed original isocurve root to
// a verified tube and an original chart boundary. The other operand must
// be strictly interior, excluding simultaneous unresolved corner events.
func CertifyAlgebraicSplineTerminal(ctx context.Context, segment intersect.TubeSegment, end int,
	a, b intersect.Patch, operand int, side geom.SurfaceEdge, root *geom.ExactAlgebraicRoot,
	o geom.TubeAlgebraicGraphEndpointOptions) (AlgebraicSplineTerminal, error) {
	var empty AlgebraicSplineTerminal
	if ctx == nil || end < 0 || end > 1 || operand < 0 || operand > 1 || side > geom.SurfaceEdgeVMax || root == nil {
		return empty, geom.ErrInvalid
	}
	if e := checkAlgebraicTerminalCharts(segment, a, b); e != nil {
		return empty, e
	}
	q, e := segment.CertifyAlgebraicGraphEndpoint(ctx, end, root, o)
	if e != nil {
		return empty, e
	}
	return verifyAlgebraicTerminalBoundary(ctx, q, a, b, operand, side, o)
}

func checkAlgebraicTerminalCharts(segment intersect.TubeSegment, a, b intersect.Patch) error {
	for i, p := range [2]intersect.Patch{a, b} {
		s, ok := p.Surface.(*geom.BSplineSurface)
		trim := segment.TrimA()
		if i == 1 {
			trim = segment.TrimB()
		}
		if !ok || !geom.SameSplineSurface(s, trim.Support()) || !uvFinite(p.Min) || !uvFinite(p.Max) || p.Min.X >= p.Max.X || p.Min.Y >= p.Max.Y {
			return geom.ErrInvalid
		}
	}
	return nil
}

// DiscoverAlgebraicSplineTerminal finds one exact algebraic graph contact
// at a verified tube end and classifies its unique original chart boundary.
// No rational guess or preselected face side is required.
func DiscoverAlgebraicSplineTerminal(ctx context.Context, segment intersect.TubeSegment, end int, a, b intersect.Patch,
	o geom.TubeAlgebraicGraphEndpointOptions) (AlgebraicSplineTerminal, error) {
	var empty AlgebraicSplineTerminal
	if ctx == nil || end < 0 || end > 1 {
		return empty, geom.ErrInvalid
	}
	if e := checkAlgebraicTerminalCharts(segment, a, b); e != nil {
		return empty, e
	}
	q, e := discoverExactTubeSectionEndpoint(ctx, segment, end, a, b, o)
	if e != nil {
		return empty, e
	}
	if !q.Proven || !q.SourceIsoTransverse {
		return empty, geom.ErrConvergence
	}
	owner, side, e := locateAlgebraicTerminalBoundary(q, a, b)
	if e != nil {
		return empty, e
	}
	return verifyAlgebraicTerminalBoundary(ctx, q, a, b, owner, side, o)
}

func locateAlgebraicTerminalBoundary(q geom.TubeAlgebraicGraphEndpointCertificate, a, b intersect.Patch) (int, geom.SurfaceEdge, error) {
	owner, side := -1, -1
	for i, p := range [2]intersect.Patch{a, b} {
		for j, v := range [4]float64{p.Min.X, p.Max.X, p.Min.Y, p.Max.Y} {
			r := q.Parameters[2*i+j/2]
			if r == nil {
				return -1, 0, geom.ErrConvergence
			}
			x := r.Interval()
			if x.Lower.Cmp(x.Upper) == 0 && x.Lower.Cmp(ratInset(v)) == 0 {
				if owner >= 0 {
					return -1, 0, geom.ErrConvergence
				}
				owner, side = i, j
			}
		}
	}
	if owner < 0 {
		return -1, 0, geom.ErrConvergence
	}
	return owner, geom.SurfaceEdge(side), nil
}

// discoverExactTubeSectionEndpoint uses the graph solver first, then a
// small rational recovery path for independently proved non-graph tubes.
// Every candidate must pass the immutable tube's exact endpoint test.
func discoverExactTubeSectionEndpoint(ctx context.Context, s intersect.TubeSegment, end int, a, b intersect.Patch,
	o geom.TubeAlgebraicGraphEndpointOptions) (r geom.TubeAlgebraicGraphEndpointCertificate, err error) {
	r, err = s.DiscoverAlgebraicGraphEndpoint(ctx, end, o)
	if err == nil || !errors.Is(err, geom.ErrConvergence) && !errors.Is(err, geom.ErrDegenerate) {
		return r, err
	}
	limit := o.MaxOperations
	if limit == 0 {
		limit = 8000000
	}
	used := r.ExactOperations
	r = geom.TubeAlgebraicGraphEndpointCertificate{}
	defer func() { r.ExactOperations = used }()
	axis, domain, reverse, e := s.ExactFreeDomain()
	if e != nil {
		return r, e
	}
	fixed := domain.Lower
	if (end == 1) != reverse {
		fixed = domain.Upper
	}
	for _, axes := range [][2]int{{0, 2}, {1, 2}} {
		child := o
		child.MaxOperations = limit - used
		if child.MaxOperations < 1 {
			return r, geom.ErrConvergence
		}
		p, e := s.EvaluateRationalProjectedGraph(ctx, fixed, axes, child)
		used += p.ExactOperations
		if e == nil && p.Proven {
			return p, nil
		}
		if e != nil && !errors.Is(e, geom.ErrConvergence) && !errors.Is(e, geom.ErrDegenerate) &&
			!errors.Is(e, geom.ErrDomain) {
			return r, e
		}
	}
	p, e := s.Evaluate(float64(end))
	if e != nil {
		return r, e
	}
	lo := [4]float64{p.ParametersA.Min.X, p.ParametersA.Min.Y, p.ParametersB.Min.X, p.ParametersB.Min.Y}
	hi := [4]float64{p.ParametersA.Max.X, p.ParametersA.Max.Y, p.ParametersB.Max.X, p.ParametersB.Max.Y}
	var x [4]*big.Rat
	for i := range x {
		if i == axis {
			x[i] = fixed
			continue
		}
		q := terminalRationalCandidates(lo[i], hi[i])
		if len(q) == 0 {
			return r, geom.ErrConvergence
		}
		x[i] = q[0]
	}
	if used >= limit {
		return r, geom.ErrConvergence
	}
	q, e := s.CertifyEndpoint(ctx, end, x, geom.TubeEndpointOptions{MaxOperations: limit - used, MaxCoefficientBits: o.MaxCoefficientBits})
	used += q.ExactOperations
	if e != nil {
		return r, e
	}
	if !q.Proven {
		if used >= limit {
			return r, geom.ErrConvergence
		}
		p, e := intersect.DiscoverRationalSplineTubeTerminal(ctx, s, end, a, b, geom.TubeEndpointEdgeOptions{
			MaxOperations: limit - used, MaxCoefficientBits: o.MaxCoefficientBits})
		used += p.Proof.ExactOperations
		if e != nil {
			return r, e
		}
		q = geom.TubeEndpointCertificate{Proven: p.Proof.Proven, Parameters: p.Proof.Parameters, Point: p.Proof.Point,
			ParameterBoundsA: p.Proof.ParameterBoundsA, ParameterBoundsB: p.Proof.ParameterBoundsB, PointBounds: p.Proof.PointBounds}
	}
	if !q.Proven {
		return r, geom.ErrConvergence
	}
	opts := func() geom.ExactCurvePlaneEventOptions {
		return geom.ExactCurvePlaneEventOptions{MaxOperations: limit - used,
			MaxCoefficientBits: o.MaxCoefficientBits, MaxSubdivisions: o.MaxSubdivisions}
	}
	root := func(x *big.Rat) (*geom.ExactAlgebraicRoot, error) {
		p := opts()
		if p.MaxOperations < 1 {
			return nil, geom.ErrConvergence
		}
		r, n, e := geom.NewExactRationalRoot(ctx, x, p)
		used += n
		return r, e
	}
	for i, x := range q.Parameters {
		r.Parameters[i], e = root(x)
		if e != nil {
			return r, e
		}
	}
	// The exact model point is already proven by the tube. Bind it to
	// an immutable constant exact cell for cross-chart point comparison.
	pOpts := o.SurfacePatchOptions
	pOpts.MaxOperations = limit - used
	if pOpts.MaxOperations < 1 {
		return r, geom.ErrConvergence
	}
	h := [4]*big.Rat{q.Point[0], q.Point[1], q.Point[2], big.NewRat(1, 1)}
	c, e := geom.NewExactHomogeneousBezier(ctx, [][4]*big.Rat{h, h}, pOpts)
	if e != nil {
		return r, e
	}
	used += c.ExactOperations()
	pOpts.MaxOperations = limit - used
	if pOpts.MaxOperations < 1 {
		return r, geom.ErrConvergence
	}
	cell, e := geom.NewExactComposedBezierCell(ctx, c, pOpts)
	if e != nil {
		return r, e
	}
	t, e := root(new(big.Rat))
	if e != nil {
		return r, e
	}
	if opts().MaxOperations < 1 {
		return r, geom.ErrConvergence
	}
	r.ModelEvent, e = cell.BindRoot(ctx, t, opts())
	if e != nil {
		return r, e
	}
	r.Proven, r.SourceIsoTransverse, r.SourceIsoMultiplicity = true, true, 1
	r.ParameterBoundsA, r.ParameterBoundsB, r.PointBounds = q.ParameterBoundsA, q.ParameterBoundsB, q.PointBounds
	return r, nil
}

// DiscoverAlgebraicSplineBranch verifies an entire supplied segment sequence
// without rational hints. It discovers both boundary terminals and all
// intermediate junctions from the original charts. Completeness here concerns
// this path's continuity; exhaustive face-pair coverage is separate.
func DiscoverAlgebraicSplineBranch(ctx context.Context, segments []intersect.TubeSegment, a, b intersect.Patch,
	o geom.TubeAlgebraicGraphEndpointOptions) (AlgebraicSplineBranchEvidence, error) {
	var empty AlgebraicSplineBranchEvidence
	if ctx == nil || len(segments) == 0 || len(segments) > 256 {
		return empty, geom.ErrInvalid
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 8000000
	}
	if o.MaxOperations < 1 {
		return empty, geom.ErrInvalid
	}
	remaining := o.MaxOperations
	var endpoint [][2]geom.TubeAlgebraicGraphEndpointCertificate
	for _, s := range segments {
		if e := checkAlgebraicTerminalCharts(s, a, b); e != nil {
			return empty, e
		}
		var x [2]geom.TubeAlgebraicGraphEndpointCertificate
		for i := range x {
			if remaining < 1 {
				return empty, geom.ErrConvergence
			}
			child := o
			child.MaxOperations = remaining
			q, e := discoverExactTubeSectionEndpoint(ctx, s, i, a, b, child)
			if e != nil {
				return empty, e
			}
			if !q.Proven {
				return empty, geom.ErrConvergence
			}
			remaining -= q.ExactOperations
			x[i] = q
		}
		endpoint = append(endpoint, x)
	}
	var out AlgebraicSplineBranchEvidence
	for i, q := range []geom.TubeAlgebraicGraphEndpointCertificate{endpoint[0][0], endpoint[len(endpoint)-1][1]} {
		owner, side, e := locateAlgebraicTerminalBoundary(q, a, b)
		if e != nil {
			return empty, e
		}
		if remaining < 1 {
			return empty, geom.ErrConvergence
		}
		child := o
		child.MaxOperations = remaining
		p, e := verifyAlgebraicTerminalBoundary(ctx, q, a, b, owner, side, child)
		if e != nil {
			return empty, e
		}
		out.Ends[i] = p
	}
	for i := 1; i < len(endpoint); i++ {
		x, y := endpoint[i-1][1], endpoint[i][0]
		j := intersect.TubeAlgebraicEndpointComparison{SupportsEqual: true, Left: x, Right: y}
		for k := range x.Parameters {
			if remaining < 1 {
				return empty, geom.ErrConvergence
			}
			r, e := x.Parameters[k].Compare(ctx, y.Parameters[k], geom.ExactCurvePlaneEventOptions{MaxOperations: remaining, MaxSubdivisions: o.MaxSubdivisions, MaxCoefficientBits: o.MaxCoefficientBits})
			remaining -= r.Operations
			j.ExactOperations += r.Operations
			if e != nil {
				return empty, e
			}
			if !r.Proven || r.Order != 0 {
				return empty, geom.ErrConvergence
			}
		}
		j.ProvenEqual = true
		out.Junctions = append(out.Junctions, j)
	}
	out.Segments = len(segments)
	out.Complete = true
	return out, nil
}

func verifyAlgebraicTerminalBoundary(ctx context.Context, q geom.TubeAlgebraicGraphEndpointCertificate,
	a, b intersect.Patch, operand int, side geom.SurfaceEdge, o geom.TubeAlgebraicGraphEndpointOptions) (AlgebraicSplineTerminal, error) {
	var empty AlgebraicSplineTerminal
	if !q.Proven || !q.SourceIsoTransverse {
		return empty, geom.ErrConvergence
	}
	axis := int(side) / 2
	p := [2]intersect.Patch{a, b}[operand]
	boundary := [4]float64{p.Min.X, p.Max.X, p.Min.Y, p.Max.Y}[side]
	r := q.Parameters[2*operand+axis]
	if r == nil {
		return empty, geom.ErrConvergence
	}
	x := r.Interval()
	if x.Lower.Cmp(x.Upper) != 0 || x.Lower.Cmp(ratInset(boundary)) != 0 {
		return empty, geom.ErrDomain
	}
	options := geom.ExactCurvePlaneEventOptions{MaxOperations: o.MaxOperations, MaxCoefficientBits: o.MaxCoefficientBits, MaxSubdivisions: o.MaxSubdivisions}
	check := func(root *geom.ExactAlgebraicRoot, lo, hi float64) error {
		for i, b := range [2]float64{lo, hi} {
			v := ratInset(b)
			bound, e := geom.ImportExactRoot(ctx, geom.ExactRootRecipe{Version: 1, Coefficients: []string{new(big.Rat).Neg(v).RatString(), "1"}, Lower: v.RatString(), Upper: v.RatString()},
				geom.ExactRootCodecOptions{ExactCurvePlaneEventOptions: options})
			if e != nil {
				return e
			}
			x, e := root.Compare(ctx, bound, options)
			if e != nil {
				return e
			}
			if !x.Proven || x.Order != 1-2*i {
				return geom.ErrConvergence
			}
		}
		return nil
	}
	for i, p := range [2]intersect.Patch{a, b} {
		lo, hi := [2]float64{p.Min.X, p.Min.Y}, [2]float64{p.Max.X, p.Max.Y}
		for j := 0; j < 2; j++ {
			if i == operand && j == axis {
				continue
			}
			if e := check(q.Parameters[2*i+j], lo[j], hi[j]); e != nil {
				return empty, e
			}
		}
	}
	return AlgebraicSplineTerminal{Operand: operand, Side: side, Certificate: q}, nil
}
