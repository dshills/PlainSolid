package intersect

import (
	"context"
	"errors"
	"math"

	"plainsolid/geom"
)

// SplineSeedOptions bounds exact candidate discovery and numerical correction.
// Zero values choose 1024 corrections, 512 seed records and 1e-8 residual.
type SplineSeedOptions struct {
	Patches        geom.SurfacePatchOptions
	Covering       geom.SurfaceSurfaceIsolationOptions
	Transversality geom.SurfaceTransversalityOptions
	// Sections tries exact fixed-coordinate contact certificates per cell.
	// MaxOperations and MaxCoefficientBits apply independently to each try.
	Sections       bool
	SectionOptions geom.SurfacePairIntervalOptions
	// BranchTubes attempts uniform branch certificates within each cell.
	// TubeOptions arithmetic limits apply independently to each try.
	BranchTubes bool
	TubeOptions geom.SurfacePairTubeOptions
	// ExpandSectionTubes grows local tubes around verified fixed-section
	// contacts when the isolation cell is not itself a certified tube.
	// Expanded boxes may overlap or extend outside their isolation cell.
	ExpandSectionTubes   bool
	Expansion            geom.SurfaceTubeDiscoveryOptions
	MaxCells, MaxSamples int
	ResidualTolerance    float64
}

// SplineSeedCell ties a conservative possible-contact cell to a local
// transversality certificate and a numerical seed, if one was found.
// A positive certificate proves regularity of any contact in this cell;
// it does not prove that contact exists.
type SplineSeedCell struct {
	Cell           geom.SurfaceSurfaceCell
	Transversality geom.SurfaceTransversality
	Sample         *Sample
	Section        *SplineSectionWitness
	Tube           *geom.SurfaceTubeCertificate
	// BoundaryAnchor permits re-proving a branch starting at two chart
	// boundaries. Like Tube, it is a caller proposal, not trusted evidence.
	BoundaryAnchor *TubeBoundaryAnchor
	// ExpandedTube contains an optional certified box distinct from Cell.
	ExpandedTube *geom.SurfaceTubeDiscovery
}

// SplineSectionWitness proves one actual contact on a fixed-coordinate
// section of a possible-contact cell. It makes no claim about other slices.
type SplineSectionWitness struct {
	FixedAxis      int
	FixedParameter float64
	Certificate    geom.SurfaceSectionCertificate
}

// SplineSeedReport keeps the complete covering independently of correction
// limits. Even when all attempted cells produce seeds, this is not a complete
// branch enumeration; a cell may contain more than one branch.
type SplineSeedReport struct {
	Covering    geom.SurfaceSurfaceIsolation
	Attempts    []SplineSeedCell
	Unattempted int
	Truncated   bool
}

// FindSplineSeeds finds numerical starting points on bounded rational
// spline patch pairs. Only control-hull separation removes possible
// contact. Cells that cannot be corrected or certified remain in the report.
// Coincidence and tangency are not treated as regular seeds.
func FindSplineSeeds(ctx context.Context, a, b Patch, o SplineSeedOptions) (SplineSeedReport, error) {
	var r SplineSeedReport
	if o.MaxCells == 0 {
		o.MaxCells = 1024
	}
	if o.MaxSamples == 0 {
		o.MaxSamples = 512
	}
	if o.ResidualTolerance == 0 {
		o.ResidualTolerance = 1e-8
	}
	if ctx == nil || !a.valid() || !b.valid() || o.MaxCells < 1 || o.MaxCells > 32768 || o.MaxSamples < 1 || o.MaxSamples > 32768 ||
		!finite(o.ResidualTolerance) || o.ResidualTolerance <= 0 {
		return r, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return r, e
	}
	ap, e := exactSplinePatch(a, o.Patches)
	if e != nil {
		return r, e
	}
	bp, e := exactSplinePatch(b, o.Patches)
	if e != nil {
		return r, e
	}
	r.Covering, e = geom.BoundSurfaceSurfaceContext(ctx, ap, bp, o.Covering)
	if e != nil && !errors.Is(e, geom.ErrConvergence) {
		return r, e
	}
	coverErr := e
	r.Truncated = coverErr != nil
	r.Unattempted = len(r.Covering.Candidates)
	samples := 0
	for _, c := range r.Covering.Candidates {
		if e = ctx.Err(); e != nil {
			return r, e
		}
		if len(r.Attempts) >= o.MaxCells || samples >= o.MaxSamples {
			r.Truncated = true
			break
		}
		attempt := SplineSeedCell{Cell: c}
		r.Unattempted--
		pa, e := ap.Trim(c.DomainA)
		if e != nil {
			if !errors.Is(e, geom.ErrConvergence) {
				return r, e
			}
			r.Attempts = append(r.Attempts, attempt)
			continue
		}
		pb, e := bp.Trim(c.DomainB)
		if e != nil {
			if !errors.Is(e, geom.ErrConvergence) {
				return r, e
			}
			r.Attempts = append(r.Attempts, attempt)
			continue
		}
		attempt.Transversality, e = geom.CertifySurfaceTransversality(pa, pb, o.Transversality)
		if e != nil && !errors.Is(e, geom.ErrDomain) && !errors.Is(e, geom.ErrConvergence) {
			return r, e
		}
		if attempt.Transversality.ProvenTransverse {
			attempt.Sample = correctSplineCell(ctx, a.Surface, b.Surface, c, o.ResidualTolerance)
			if o.Sections || o.ExpandSectionTubes {
				attempt.Section, e = certifySplineSection(ctx, pa, pb, attempt.Sample, o.SectionOptions)
				if e != nil {
					return r, e
				}
			}
			if o.BranchTubes {
				attempt.Tube, e = certifySplineTube(ctx, pa, pb, attempt.Section, o.TubeOptions)
				if e != nil {
					return r, e
				}
			}
			if o.ExpandSectionTubes && attempt.Section != nil && attempt.Tube == nil {
				w := attempt.Section
				opts := o.Expansion
				opts.FreeAxis, opts.Parameter = w.FixedAxis, w.FixedParameter
				opts.DependentBox = geom.SurfacePairParameterBox{A: c.DomainA, B: c.DomainB}
				d, e := geom.DiscoverSurfacePairTube(ap, bp, opts)
				if e != nil && !errors.Is(e, geom.ErrDomain) && !errors.Is(e, geom.ErrConvergence) {
					return r, e
				}
				attempt.ExpandedTube = &d
			}
		}
		if attempt.Sample != nil {
			samples++
		}
		r.Attempts = append(r.Attempts, attempt)
	}
	return r, coverErr
}

// WithExpandedTubes returns a copy with successful expansions appended as
// local tube cells. Covering and Unattempted still describe the original
// isolation; the extra cells are candidate branch geometry only.
func (r SplineSeedReport) WithExpandedTubes() SplineSeedReport {
	cells := append([]SplineSeedCell(nil), r.Attempts...)
	for _, x := range r.Attempts {
		d := x.ExpandedTube
		if d == nil || !d.Found {
			continue
		}
		c := d.Tube
		cells = append(cells, SplineSeedCell{Cell: geom.SurfaceSurfaceCell{DomainA: d.Box.A, DomainB: d.Box.B}, Tube: &c})
	}
	r.Attempts = cells
	return r
}

func certifySplineTube(ctx context.Context, a, b *geom.SurfacePatch, w *SplineSectionWitness, o geom.SurfacePairTubeOptions) (*geom.SurfaceTubeCertificate, error) {
	axes := [4]int{0, 1, 2, 3}
	if w != nil {
		axes[0], axes[w.FixedAxis] = axes[w.FixedAxis], axes[0]
	}
	for _, axis := range axes {
		if e := ctx.Err(); e != nil {
			return nil, e
		}
		o.FreeAxis = axis
		c, e := geom.CertifySurfacePairTube(a, b, o)
		if e != nil {
			if errors.Is(e, geom.ErrDomain) || errors.Is(e, geom.ErrConvergence) {
				continue
			}
			return nil, e
		}
		if c.ProvenBranch || c.ExcludesContact {
			return &c, nil
		}
	}
	return nil, nil
}

func certifySplineSection(ctx context.Context, a, b *geom.SurfacePatch, p *Sample, o geom.SurfacePairIntervalOptions) (*SplineSectionWitness, error) {
	da, db := a.Domain(), b.Domain()
	lo := [4]float64{da.UMin, da.VMin, db.UMin, db.VMin}
	hi := [4]float64{da.UMax, da.VMax, db.UMax, db.VMax}
	center := [4]float64{}
	for i := range 4 {
		center[i] = lo[i]/2 + hi[i]/2
	}
	point := center
	if p != nil {
		point = [4]float64{p.Parameters.A.X, p.Parameters.A.Y, p.Parameters.B.X, p.Parameters.B.Y}
	}
	for axis := range 4 {
		if e := ctx.Err(); e != nil {
			return nil, e
		}
		q := point[axis]
		if q <= lo[axis] || q >= hi[axis] {
			q = center[axis]
		}
		o.UseFixedParameter, o.FixedAxis, o.FixedParameter = true, axis, q
		c, e := geom.CertifySurfacePairSection(a, b, o)
		if e != nil {
			if errors.Is(e, geom.ErrDomain) || errors.Is(e, geom.ErrConvergence) {
				continue
			}
			return nil, e
		}
		if c.ProvenUnique {
			return &SplineSectionWitness{axis, q, c}, nil
		}
	}
	return nil, nil
}

func correctSplineCell(ctx context.Context, a, b geom.Surface, c geom.SurfaceSurfaceCell, tol float64) *Sample {
	patch := func(s geom.Surface, d geom.ParameterRectangle) Patch {
		return Patch{s, geom.Vec2{X: d.UMin, Y: d.VMin}, geom.Vec2{X: d.UMax, Y: d.VMax}}
	}
	pa, pb := patch(a, c.DomainA), patch(b, c.DomainB)
	center := func(p Patch) geom.Vec2 {
		return geom.Vec2{X: p.Min.X/2 + p.Max.X/2, Y: p.Min.Y/2 + p.Max.Y/2}
	}
	x := Parameters{center(pa), center(pb)}
	f, e := evaluate(pa, pb, x)
	if e != nil {
		return nil
	}
	target := c.Bounds.Min.Scale(.5).Add(c.Bounds.Max.Scale(.5))
	if !target.IsFinite() {
		return nil
	}
	x, f, e = correct(ctx, pa, pb, x, target, f.t, Options{Tolerance: tol, MaxNewton: 20})
	if e != nil {
		return nil
	}
	res := f.p.Distance(f.q)
	if math.IsNaN(res) || res > tol {
		return nil
	}
	return &Sample{x, f.p.Scale(.5).Add(f.q.Scale(.5)), f.t, res}
}
