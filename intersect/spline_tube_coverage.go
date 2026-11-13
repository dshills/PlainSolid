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
	// ProjectedAffineFallback retries unresolved coverage in ProjectionAxes,
	// ordered distinct model components (0=X,1=Y,2=Z). No default axes are inferred.
	ProjectedAffineFallback bool
	ProjectionAxes          [2]int
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
	exact := func(p Patch) (*geom.SurfacePatch, error) {
		s, ok := p.Surface.(*geom.BSplineSurface)
		if !ok {
			return nil, geom.ErrInvalid
		}
		x, e := geom.NewSurfacePatch(s, o.Patches)
		if e != nil {
			return nil, e
		}
		return x.Trim(geom.ParameterRectangle{UMin: p.Min.X, UMax: p.Max.X, VMin: p.Min.Y, VMax: p.Max.Y})
	}
	pa, e := exact(a)
	if e != nil {
		return r, e
	}
	pb, e := exact(b)
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
		p, e := geom.CertifySurfacePairCellInTubesContext(ctx, pa, pb, candidate, available, o.Cell)
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
	if !r.CoversAllContacts {
		return r, ErrUnresolved
	}
	return r, nil
}
