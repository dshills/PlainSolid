package intersect

import (
	"context"
	"errors"
	"fmt"

	"plainsolid/geom"
)

// SplineTubeCoverageOptions controls fresh candidate isolation and verification
// that every remaining contact belongs to at least one supplied local tube.
// MaxCandidateTests defaults to 512; each cell also has its own proof limits.
type SplineTubeCoverageOptions struct {
	// IsoPlaneFallback enables an independent full-domain scalar plane reduction.
	IsoPlaneFallback bool
	IsoPlane         IsoPlaneTubeCoverageOptions
	// OverlappingAffineXYFallback retries through exact projected overlap,
	// excluding omitted domains only by exact graph projection. Each clipping
	// and cell proof retains its own arithmetic limits.
	OverlappingAffineXYFallback bool
	// ProjectedAffineFallback retries unresolved coverage in ProjectionAxes,
	// ordered distinct model components (0=X,1=Y,2=Z). No default axes are inferred.
	ProjectedAffineFallback bool
	// OverlappingProjectedFallback clips unresolved candidates in ProjectionAxes
	// before checking the original tube cover. Only exact affine graph maps
	// with representable overlap intervals participate.
	OverlappingProjectedFallback bool
	ProjectionAxes               [2]int
	// AffineXYFallback retries unresolved cells through an exact represented rectangular chart map.
	AffineXYFallback  bool
	Patches           geom.SurfacePatchOptions
	Isolation         geom.SurfaceSurfaceIsolationOptions
	Cell              geom.SurfaceTubeUnionCoverageOptions
	MaxCandidateTests int
}

// SplineTubeCoverageReport distinguishes exhaustive set coverage from a
// topological branch enumeration. Even when CoversAllContacts is true,
// candidate tubes may overlap in their interiors, contain duplicate branch
// segments, or be arranged in an ambiguous endpoint graph.
type SplineTubeCoverageReport struct {
	// IsoPlane retains the independent full original-domain coverage proof.
	// When present, it establishes completeness independently of Isolation.Complete.
	IsoPlane *IsoPlaneTubeCoverage
	// TubeReuse reports successful local proof reuse; logical ExactOperations remain charged per candidate.
	TubeReuse                      geom.SurfaceTubeCoverageCacheStats
	Isolation                      geom.SurfaceSurfaceIsolation
	Covered, Unresolved, Unchecked []int
	// BoundaryCovered is a subset of Covered with leftover closed slabs
	// whose possible contacts were confined to verified tube boundaries.
	BoundaryCovered   []int
	CoversAllContacts bool
}

// CertifySplineTubeCoverage checks a supplied collection of certified tube
// candidates against an independent covering of the full patch pair. It never
// trusts a caller-owned covering or accepts a missed/tangent/coincident cell on
// the strength of an intersection sample. Indices refer to Isolation.Candidates.
func CertifySplineTubeCoverage(ctx context.Context, a, b Patch, supplied SplineSeedReport, o SplineTubeCoverageOptions) (SplineTubeCoverageReport, error) {
	var r SplineTubeCoverageReport
	if o.MaxCandidateTests == 0 {
		o.MaxCandidateTests = 512
	}
	if ctx == nil || !a.valid() || !b.valid() || o.MaxCandidateTests < 1 || len(supplied.Attempts) > 4096 {
		return r, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return r, e
	}
	if o.IsoPlaneFallback {
		var e error
		o.IsoPlane, e = normalizedIsoPlaneCoverageOptions(o.IsoPlane)
		if e != nil {
			return r, e
		}
	}
	pa, e := exactSplinePatch(a, o.Patches)
	if e != nil {
		return r, e
	}
	pb, e := exactSplinePatch(b, o.Patches)
	if e != nil {
		return r, e
	}
	r.Isolation, e = geom.BoundSurfaceSurfaceContext(ctx, pa, pb, o.Isolation)
	if e != nil && !errors.Is(e, geom.ErrConvergence) {
		return r, e
	}
	var tubes []geom.SurfacePairTubeRegion
	for i, c := range supplied.Attempts {
		if c.Tube == nil || !c.Tube.ProvenBranch {
			continue
		}
		axis := c.Tube.FreeAxis
		if axis < 0 || axis > 3 || !c.Cell.DomainA.Valid() || !c.Cell.DomainB.Valid() {
			return r, fmt.Errorf("%w: invalid tube cell %d", geom.ErrInvalid, i)
		}
		t := geom.SurfacePairTubeRegion{Box: geom.SurfacePairParameterBox{A: c.Cell.DomainA, B: c.Cell.DomainB}, FreeAxis: axis}
		if c.BoundaryAnchor != nil {
			x, e := c.BoundaryAnchor.exact(geom.SurfacePairTubeOptions{FreeAxis: axis})
			if e != nil {
				return r, e
			}
			t.Boundary = &x
		}
		tubes = append(tubes, t)
	}
	overlap := func(a, b geom.ParameterRectangle) bool {
		// A tube endpoint can lie on the boundary of a candidate even
		// when the two closed boxes share no positive-area rectangle.
		return a.UMin <= b.UMax && b.UMin <= a.UMax && a.VMin <= b.VMax && b.VMin <= a.VMax
	}
	cache, e := geom.NewSurfacePairTubeCoverageCache(pa, pb, 0)
	if e != nil {
		return r, e
	}
	for i, c := range r.Isolation.Candidates {
		if e := ctx.Err(); e != nil {
			for j := i; j < len(r.Isolation.Candidates); j++ {
				r.Unchecked = append(r.Unchecked, j)
			}
			return r, e
		}
		if i >= o.MaxCandidateTests {
			r.Unchecked = append(r.Unchecked, i)
			continue
		}
		var available []geom.SurfacePairTubeRegion
		for _, t := range tubes {
			if overlap(t.Box.A, c.DomainA) && overlap(t.Box.B, c.DomainB) {
				available = append(available, t)
			}
		}
		candidate := geom.SurfacePairParameterBox{A: c.DomainA, B: c.DomainB}
		p, e := cache.CertifyCell(ctx, candidate, available, o.Cell)
		r.TubeReuse = cache.Stats()
		if e != nil && !errors.Is(e, geom.ErrConvergence) {
			for j := i; j < len(r.Isolation.Candidates); j++ {
				r.Unchecked = append(r.Unchecked, j)
			}
			return r, e
		}
		if !p.Covered && o.AffineXYFallback {
			mapped, me := geom.CertifyAffineXYSurfacePairCellInTubes(ctx, pa, pb, candidate, available, o.Cell)
			if me == nil && mapped.Covered {
				p = mapped
			}
			if me != nil && !errors.Is(me, geom.ErrConvergence) && !errors.Is(me, geom.ErrUnrepresentable) && !errors.Is(me, geom.ErrDomain) && !errors.Is(me, geom.ErrInvalid) {
				return r, me
			}
		}
		if !p.Covered && o.ProjectedAffineFallback {
			mapped, me := geom.CertifyProjectedAffineSurfacePairCellInTubes(ctx, pa, pb, candidate, available, o.ProjectionAxes, o.Cell)
			if me == nil && mapped.Covered {
				p = mapped
			}
			if me != nil && !errors.Is(me, geom.ErrConvergence) && !errors.Is(me, geom.ErrUnrepresentable) && !errors.Is(me, geom.ErrDomain) && !errors.Is(me, geom.ErrInvalid) {
				return r, me
			}
		}

		if !p.Covered && o.OverlappingAffineXYFallback {
			mapped, me := certifyOverlapCell(ctx, pa, pb, candidate, available, o.Cell)
			if me == nil && mapped.Covered {
				p = mapped
			}
			if me != nil && !errors.Is(me, geom.ErrConvergence) && !errors.Is(me, geom.ErrUnrepresentable) && !errors.Is(me, geom.ErrDomain) && !errors.Is(me, geom.ErrInvalid) {
				return r, me
			}
		}
		if !p.Covered && o.OverlappingProjectedFallback {
			mapped, me := certifyProjectedOverlapCell(ctx, pa, pb, candidate, available, o.ProjectionAxes, o.Cell)
			if me == nil && mapped.Covered {
				p = mapped
			}
			if me != nil && !errors.Is(me, geom.ErrConvergence) && !errors.Is(me, geom.ErrUnrepresentable) && !errors.Is(me, geom.ErrDomain) && !errors.Is(me, geom.ErrInvalid) {
				return r, me
			}
		}
		if p.Covered {
			r.Covered = append(r.Covered, i)
			if len(p.BoundaryCovered) != 0 {
				r.BoundaryCovered = append(r.BoundaryCovered, i)
			}
		} else {
			r.Unresolved = append(r.Unresolved, i)
		}
	}
	r.CoversAllContacts = r.Isolation.Complete && len(r.Isolation.Coincident) == 0 &&
		len(r.Unresolved) == 0 && len(r.Unchecked) == 0
	if !r.CoversAllContacts && o.IsoPlaneFallback {
		p, pe := CertifyIsoPlaneSplineTubeCoverage(ctx, a, b, supplied, o.IsoPlane)
		if pe == nil && p.Covered {
			r.IsoPlane = &p
			r.CoversAllContacts = true
			r.Covered = nil
			for i := range r.Isolation.Candidates {
				r.Covered = append(r.Covered, i)
			}
			r.Unresolved, r.Unchecked = nil, nil
		} else if ctx.Err() != nil {
			return r, ctx.Err()
		} else if pe != nil && !errors.Is(pe, ErrUnresolved) && !errors.Is(pe, geom.ErrConvergence) && !errors.Is(pe, geom.ErrDegenerate) && !errors.Is(pe, geom.ErrUnrepresentable) {
			return r, pe
		}
	}
	if !r.CoversAllContacts {
		return r, ErrUnresolved
	}
	return r, nil
}
