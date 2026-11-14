package intersect

import (
	"context"
	"errors"
	"fmt"
	"math/big"

	"plainsolid/geom"
)

// SplinePairTraceOptions controls automatic tube discovery and the independent
// coverage and endpoint graph checks. MaxTopologyPairs bounds the proof of
// non-overlapping interiors; zero selects 32768 comparisons.
type SplinePairTraceOptions struct {
	DyadicGraphFallback DyadicGraphDiscoveryOptions
	// UniformGraphFallback proposes exact dyadic cuts without sampled zero witnesses.
	UniformGraphFallback UniformGraphDiscoveryOptions
	// UniformOverlapGraphFallback retains original coverage after exact XY clipping.
	UniformOverlapGraphFallback UniformGraphDiscoveryOptions
	// ConcentricEllipseFallback recognizes an exact diagonal quartic factor pair.
	ConcentricEllipseFallback SplinePlanePairDiscoveryOptions
	// ProjectedGraphFallback discovers in an explicitly chosen model plane.
	ProjectedGraphFallback SplinePlanePairDiscoveryOptions
	GraphProjectionAxes    [2]int
	Seeds                  SplineSeedOptions
	Branches               TubeBranchOptions
	Coverage               SplineTubeCoverageOptions
	Coincidence            SplineCoincidenceOptions
	PlaneFallback          SplinePlanePairDiscoveryOptions
	// GraphFallback is opt-in exact common-XY height-difference discovery.
	// Its proposals still require independent coverage and branch topology.
	GraphFallback SplinePlanePairDiscoveryOptions
	// AffineGraphFallback extends graph discovery to independent signed-axis/scaled charts.
	AffineGraphFallback SplinePlanePairDiscoveryOptions
	// RadialGraphFallback proves a restricted exact concentric quartic factorization.
	RadialGraphFallback SplinePlanePairDiscoveryOptions
	MaxTopologyPairs    int
}

// SplinePairTrace separates local branch geometry from the certification
// that the branch set is complete. An incomplete result retains seed,
// coverage and junction diagnostics and any locally verified curves.
type SplinePairTrace struct {
	Seeds         SplineSeedReport
	BranchGraph   TubeBranchReport
	Coverage      SplineTubeCoverageReport
	Coincidence   SplineCoincidence
	Curves        [][]TubeSegment
	Issues        []string
	TopologyPairs int
	Complete      bool
}

// TraceSplinePair discovers transverse spline intersection branches and
// attempts to certify completeness. It requires every contact to be covered,
// distinct tube interiors to be separated in parameter space, and each open
// endpoint to lie on the original boundary of its free coordinate. Adjacent
// mixed-axis tubes may have overlapping boxes if a common coordinate is
// certified to be monotone through their proved junction.
// Fully coincident charts have a separate mapping proof; since they do not
// form a finite curve set, they still return ErrUnresolved. Tangent, partially
// coincident, overlapping and ambiguous branches remain unresolved.
// Patch supports may be B-splines or exact positive homogeneous Bézier adapters.
func TraceSplinePair(ctx context.Context, a, b Patch, o SplinePairTraceOptions) (SplinePairTrace, error) {
	var r SplinePairTrace
	if o.MaxTopologyPairs == 0 {
		o.MaxTopologyPairs = 32768
	}
	if ctx == nil || !a.valid() || !b.valid() || o.MaxTopologyPairs < 1 {
		return r, geom.ErrInvalid
	}
	o.Seeds.BranchTubes, o.Seeds.Sections = true, true
	if o.Seeds.Covering.ParameterTolerance == 0 {
		o.Seeds.Covering.ParameterTolerance = max(a.Max.X-a.Min.X, a.Max.Y-a.Min.Y, b.Max.X-b.Min.X, b.Max.Y-b.Min.Y)
	}
	if o.Coverage.Isolation == (geom.SurfaceSurfaceIsolationOptions{}) {
		o.Coverage.Isolation = o.Seeds.Covering
	}
	if o.Coverage.Patches == (geom.SurfacePatchOptions{}) {
		o.Coverage.Patches = o.Seeds.Patches
	}
	if o.Branches.Curves.Patches == (geom.SurfacePatchOptions{}) {
		o.Branches.Curves.Patches = o.Seeds.Patches
	}
	if o.Coincidence.Patches == (geom.SurfacePatchOptions{}) {
		o.Coincidence.Patches = o.Seeds.Patches
	}
	var e error
	r.Coincidence, e = FindSplineCoincidence(ctx, a, b, o.Coincidence)
	if e != nil && !errors.Is(e, geom.ErrConvergence) {
		return r, e
	}
	if r.Coincidence.Found {
		m := r.Coincidence.Map
		c := geom.SurfaceSurfaceCell{DomainA: m.DomainA, DomainB: m.DomainB, Bounds: r.Coincidence.Bounds}
		// Preserve the existing covering diagnostics only for identity in
		// the original UV coordinates. Other maps have their own proof.
		if m.DomainA == m.DomainB && !m.Transposed && !m.ReverseU && !m.ReverseV {
			cover := geom.SurfaceSurfaceIsolation{Coincident: []geom.SurfaceSurfaceCell{c}, Complete: true}
			r.Seeds.Covering, r.Coverage.Isolation = cover, cover
		}
		r.Issues = append(r.Issues, "coincident surface regions are not finite branches")
		return r, ErrUnresolved
	}
	r.Seeds, e = FindSplineSeeds(ctx, a, b, o.Seeds)
	if e != nil && !errors.Is(e, geom.ErrConvergence) {
		return r, e
	}
	if len(r.Seeds.Attempts) == 1 && r.Seeds.Attempts[0].Tube == nil &&
		r.Seeds.Covering.Complete && o.Seeds.Covering.ParameterTolerance > 0 {
		refined := o.Seeds
		refined.Covering.ParameterTolerance /= 8
		if refined.Covering.ParameterTolerance > 0 {
			r.Seeds, e = FindSplineSeeds(ctx, a, b, refined)
			if e != nil && !errors.Is(e, geom.ErrConvergence) {
				return r, e
			}
		}
	}
	if o.Seeds.ExpandSectionTubes {
		r.Seeds = r.Seeds.WithExpandedTubes()
	}
	r, e = certifySuppliedSplinePairBranches(ctx, a, b, r, o)
	if e == nil || !errors.Is(e, ErrUnresolved) && !errors.Is(e, geom.ErrConvergence) {
		return r, e
	}
	for _, fallback := range []struct {
		options  SplinePlanePairDiscoveryOptions
		discover func(context.Context, Patch, Patch, geom.SurfacePatchOptions, SplinePlanePairDiscoveryOptions) (SplineSeedReport, error)
	}{{o.PlaneFallback, DiscoverSplinePlanePairTubes}, {o.GraphFallback, DiscoverCommonXYGraphPairTubes}, {o.AffineGraphFallback, DiscoverAffineXYGraphPairTubes}, {o.RadialGraphFallback, DiscoverRadialGraphPairTubes}, {o.ConcentricEllipseFallback, DiscoverConcentricEllipseGraphPairTubes}, {o.DyadicGraphFallback.SplinePlanePairDiscoveryOptions, func(ctx context.Context, a, b Patch, p geom.SurfacePatchOptions, d SplinePlanePairDiscoveryOptions) (SplineSeedReport, error) {
		q := o.DyadicGraphFallback
		q.SplinePlanePairDiscoveryOptions = d
		return DiscoverDyadicGraphPairTubes(ctx, a, b, p, q)
	}}, {o.UniformGraphFallback.SplinePlanePairDiscoveryOptions, func(ctx context.Context, a, b Patch, p geom.SurfacePatchOptions, d SplinePlanePairDiscoveryOptions) (SplineSeedReport, error) {
		q := o.UniformGraphFallback
		q.SplinePlanePairDiscoveryOptions = d
		return DiscoverUniformGraphPairTubes(ctx, a, b, p, q)
	}}, {o.UniformOverlapGraphFallback.SplinePlanePairDiscoveryOptions, func(ctx context.Context, a, b Patch, p geom.SurfacePatchOptions, d SplinePlanePairDiscoveryOptions) (SplineSeedReport, error) {
		q := o.UniformOverlapGraphFallback
		q.SplinePlanePairDiscoveryOptions = d
		result, e := DiscoverUniformOverlappingGraphPairTubes(ctx, a, b, p, q)
		return result.Seeds, e
	}}, {o.ProjectedGraphFallback, func(ctx context.Context, a, b Patch, p geom.SurfacePatchOptions, d SplinePlanePairDiscoveryOptions) (SplineSeedReport, error) {
		return DiscoverProjectedAffineGraphPairTubes(ctx, a, b, p, o.GraphProjectionAxes, d)
	}}} {
		if !fallback.options.Enabled {
			continue
		}
		hints, searchErr := fallback.discover(ctx, a, b, o.Seeds.Patches, fallback.options)
		if searchErr != nil {
			if errors.Is(searchErr, ErrUnresolved) || errors.Is(searchErr, geom.ErrConvergence) {
				continue
			}
			return r, searchErr
		}
		proved, fallbackErr := CertifySplinePairBranches(ctx, a, b, hints, o)
		if fallbackErr != nil && !errors.Is(fallbackErr, ErrUnresolved) && !errors.Is(fallbackErr, geom.ErrConvergence) {
			return r, fallbackErr
		}
		if fallbackErr == nil || len(proved.Curves) > len(r.Curves) {
			proved.Coincidence = r.Coincidence
			r, e = proved, fallbackErr
		}
		if e == nil {
			return r, nil
		}
	}
	return r, e
}

// CertifySplinePairBranches independently validates caller-supplied tube
// cells, assembles their branch graph and certifies coverage of all contacts.
// A claimed proof in supplied is only a hint; tube and junction proofs are
// recomputed from the two original patches. No numerical search is performed.
func CertifySplinePairBranches(ctx context.Context, a, b Patch, supplied SplineSeedReport, o SplinePairTraceOptions) (SplinePairTrace, error) {
	r := SplinePairTrace{Seeds: supplied}
	if o.MaxTopologyPairs == 0 {
		o.MaxTopologyPairs = 32768
	}
	if ctx == nil || !a.valid() || !b.valid() || o.MaxTopologyPairs < 1 {
		return r, geom.ErrInvalid
	}
	if o.Coverage.Isolation == (geom.SurfaceSurfaceIsolationOptions{}) {
		o.Coverage.Isolation = o.Seeds.Covering
		if o.Coverage.Isolation.ParameterTolerance == 0 {
			o.Coverage.Isolation.ParameterTolerance = max(a.Max.X-a.Min.X, a.Max.Y-a.Min.Y, b.Max.X-b.Min.X, b.Max.Y-b.Min.Y)
		}
	}
	if o.Coverage.Patches == (geom.SurfacePatchOptions{}) {
		o.Coverage.Patches = o.Seeds.Patches
	}
	if o.Branches.Curves.Patches == (geom.SurfacePatchOptions{}) {
		o.Branches.Curves.Patches = o.Seeds.Patches
	}
	return certifySuppliedSplinePairBranches(ctx, a, b, r, o)
}

func certifySuppliedSplinePairBranches(ctx context.Context, a, b Patch, r SplinePairTrace, o SplinePairTraceOptions) (SplinePairTrace, error) {
	var e error
	r.BranchGraph, e = JoinSplineTubeBranches(ctx, a, b, r.Seeds, o.Branches)
	if e != nil {
		return r, e
	}
	coverageOptions := o.Coverage
	if o.UniformOverlapGraphFallback.Enabled {
		o.Coverage.OverlappingProjectedFallback = true
		o.Coverage.ProjectionAxes = [2]int{0, 1}
	}
	if o.AffineGraphFallback.Enabled {
		coverageOptions.AffineXYFallback = true
	}
	if o.ProjectedGraphFallback.Enabled {
		coverageOptions.ProjectedAffineFallback = true
		coverageOptions.ProjectionAxes = o.GraphProjectionAxes
	}
	r.Coverage, e = CertifySplineTubeCoverage(ctx, a, b, r.Seeds, coverageOptions)
	if e != nil && !errors.Is(e, ErrUnresolved) {
		return r, e
	}
	if !r.Coverage.CoversAllContacts {
		r.Issues = append(r.Issues, "some contact cells are not covered by verified tubes")
	}
	if len(r.Coverage.Isolation.Coincident) != 0 {
		r.Issues = append(r.Issues, "coincident surface regions are not finite branches")
	}
	if r.BranchGraph.Truncated {
		r.Issues = append(r.Issues, "branch graph comparison limit")
	}
	boundary := func(c TubeSegment, end int) bool {
		lo := [4]float64{a.Min.X, a.Min.Y, b.Min.X, b.Min.Y}
		hi := [4]float64{a.Max.X, a.Max.Y, b.Max.X, b.Max.Y}
		if c.reversed {
			end ^= 1
		}
		anchor, _, _, atUpper, hasAnchor := c.proof.ExactBoundaryAnchor()
		if hasAnchor && (end == 1) == atUpper {
			for axis, q := range anchor {
				if q.Cmp(new(big.Rat).SetFloat64(lo[axis])) == 0 || q.Cmp(new(big.Rat).SetFloat64(hi[axis])) == 0 {
					return true
				}
			}
		}
		return end == 0 && c.lo == lo[c.axis] || end == 1 && c.hi == hi[c.axis]
	}
	for i, x := range r.BranchGraph.Branches {
		c, e := SplineBranchCurves(ctx, a, b, r.Seeds, x, o.Branches.Curves)
		if e != nil {
			return r, e
		}
		r.Curves = append(r.Curves, c)
		if x.StartUnresolved || x.EndUnresolved {
			r.Issues = append(r.Issues, fmt.Sprintf("branch %d has an ambiguous continuation", i))
		} else if !x.Closed && (!boundary(c[0], 0) || !boundary(c[len(c)-1], 1)) {
			r.Issues = append(r.Issues, fmt.Sprintf("branch %d has an endpoint not proved to be on the input boundary", i))
		}
	}
	if e = certifyTubeInteriorDisjointness(ctx, &r, o); e != nil {
		return r, e
	}
	r.Complete = len(r.Issues) == 0
	if !r.Complete {
		return r, ErrUnresolved
	}
	return r, nil
}

func certifyTubeInteriorDisjointness(ctx context.Context, r *SplinePairTrace, o SplinePairTraceOptions) error {
	type segment struct {
		s                TubeSegment
		branch, index, n int
		closed           bool
	}
	var segments []segment
	for i, branch := range r.Curves {
		for j, s := range branch {
			segments = append(segments, segment{s, i, j, len(branch), r.BranchGraph.Branches[i].Closed})
		}
	}
	for i, x := range segments {
		for _, y := range segments[i+1:] {
			if e := ctx.Err(); e != nil {
				return e
			}
			if r.TopologyPairs == o.MaxTopologyPairs {
				r.Issues = append(r.Issues, "tube interior comparison limit")
				return nil
			}
			r.TopologyPairs++
			joined := x.branch == y.branch && (y.index == x.index+1 || x.closed && x.index == 0 && y.index == x.n-1)
			left, right := x.s, y.s
			if joined && y.index != x.index+1 {
				left, right = right, left
			}
			proved, e := tubePairInteriorsSeparated(ctx, left, right, joined, o.Seeds.Transversality)
			if e != nil {
				return e
			}
			if !proved {
				r.Issues = append(r.Issues, "tube branches are not proved to have disjoint interiors")
				return nil
			}
		}
	}
	return nil
}
