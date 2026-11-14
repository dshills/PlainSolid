package intersect

import (
	"context"
	"fmt"
	"math"
	"math/big"
	"plainsolid/geom"
)

// RadialOverlapCoverageOptions selects an independent radial cover followed by
// exact transfer into the supplied original tube union. Hints are re-certified
// against the original complete supports; they are not asserted coverage.
type RadialOverlapCoverageOptions struct {
	// AffineCharts proves exact signed-axis chart transport on a common positive physical rectangle.
	AffineCharts bool
	Center       [2]*big.Rat
	Radii        []*big.Rat
	Proof        geom.RadialTubeCoverageOptions
}

// RadialTransferMode identifies the independently checked transfer proof.
type RadialTransferMode string

const (
	RadialTransferGeneric  RadialTransferMode = "original-cell-union"
	RadialTransferPhysical RadialTransferMode = "exact-physical-circle-strata"
)

// RadialOverlapCoverage retains complete radial coverage and the separately
// proved transfer to the caller's original tubes. It makes no topology claim.
type RadialOverlapCoverage struct {
	TransferMode                  RadialTransferMode
	TransferStrata, TransferTests int
	Cover                         *geom.RadialTubeCoverage
	ExactOperations               int
	Transferred                   int
}

// DiscoverOverlappingRadialGraphContourTubes returns a separate proposal lane.
// Its arcs overlap deliberately and are not a native ordered branch graph.
// Every floating-point box proposal is independently proved on original supports.
func DiscoverOverlappingRadialGraphContourTubes(ctx context.Context, a, b Patch, center [2]*big.Rat, radii []*big.Rat, p geom.SurfacePatchOptions, o geom.RadialTubeCoverageOptions) (SplineSeedReport, error) {
	pa, e := exactSplinePatch(a, p)
	if e != nil {
		return SplineSeedReport{}, e
	}
	pb, e := exactSplinePatch(b, p)
	if e != nil {
		return SplineSeedReport{}, e
	}
	seeds, _, _, _, e := radialOverlapProposals(ctx, pa, pb, center, radii, o, nil)
	return seeds, e
}

// DiscoverOverlappingAffineRadialGraphContourTubes retains the original charts
// while proposing a cover of physical XY circles on their exact common rectangle.
// It publishes no branch order. All mapped boxes are independently re-proved.
func DiscoverOverlappingAffineRadialGraphContourTubes(ctx context.Context, a, b Patch, center [2]*big.Rat, radii []*big.Rat, p geom.SurfacePatchOptions, o geom.RadialTubeCoverageOptions) (SplineSeedReport, error) {
	if ctx == nil || !a.valid() || !b.valid() {
		return SplineSeedReport{}, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return SplineSeedReport{}, e
	}
	pa, e := exactSplinePatch(a, p)
	if e != nil {
		return SplineSeedReport{}, e
	}
	pb, e := exactSplinePatch(b, p)
	if e != nil {
		return SplineSeedReport{}, e
	}
	maximum := o.MaxOperations
	if maximum == 0 {
		maximum = 64000000
	}
	if maximum < 1 || maximum > 512000000 {
		return SplineSeedReport{}, geom.ErrInvalid
	}
	proofMax := maximum
	if proofMax > 128000000 {
		proofMax = 128000000
	}
	transport, e := geom.CertifyAffineRadialGraphContours(ctx, pa, pb, center, radii, geom.GraphPointContactOptions{MaxOperations: proofMax, MaxCoefficientBits: o.MaxCoefficientBits})
	if e != nil {
		return SplineSeedReport{}, e
	}
	if transport.ExactOperations() >= maximum {
		return SplineSeedReport{}, geom.ErrConvergence
	}
	o.MaxOperations = maximum - transport.ExactOperations()
	seeds, _, _, _, e := radialOverlapProposals(ctx, pa, pb, center, radii, o, transport)
	return seeds, e
}

func radialOverlapProposals(ctx context.Context, pa, pb *geom.SurfacePatch, center [2]*big.Rat, radii []*big.Rat, o geom.RadialTubeCoverageOptions, transport *geom.AffineRadialContours) (SplineSeedReport, []geom.VerifiedSurfaceTube, *geom.SplineGraphRadialContours, int, error) {
	fail := func(e error) (SplineSeedReport, []geom.VerifiedSurfaceTube, *geom.SplineGraphRadialContours, int, error) {
		return SplineSeedReport{}, nil, nil, 0, e
	}
	if ctx == nil {
		return fail(geom.ErrInvalid)
	}
	if e := ctx.Err(); e != nil {
		return fail(e)
	}
	maximum := o.MaxOperations
	if maximum == 0 {
		maximum = 64000000
	}
	if maximum < 1 || maximum > 512000000 {
		return fail(geom.ErrInvalid)
	}
	maxTubes := o.MaxTubes
	if maxTubes == 0 {
		maxTubes = 256
	}
	if maxTubes < 1 || maxTubes > 512 {
		return fail(geom.ErrInvalid)
	}
	proofMax := maximum
	if proofMax > 128000000 {
		proofMax = 128000000
	}
	var radial *geom.SplineGraphRadialContours
	var e error
	used := 0
	if transport != nil {
		center = transport.Center()
		radii = transport.Radii()
	} else {
		radial, e = geom.CertifySplineGraphRadialContours(ctx, pa, pb, center, radii, geom.GraphPointContactOptions{MaxOperations: proofMax, MaxCoefficientBits: o.MaxCoefficientBits})
		if e != nil {
			return fail(e)
		}
		used = radial.ExactOperations()
		center = radial.Center()
		radii = radial.Radii()
	}
	var c [2]float64
	for i, q := range center {
		c[i], e = geom.RepresentedExactScalar(q)
		if e != nil {
			return fail(e)
		}
	}
	var seeds SplineSeedReport
	var tubes []geom.VerifiedSurfaceTube
	for _, q := range radii {
		r, e := geom.RepresentedExactScalar(q)
		if e != nil {
			return fail(e)
		}
		for axis := 0; axis < 2; axis++ {
			for _, sign := range []float64{-1, 1} {
				// Adaptive base intervals span +/-3r/4. An eighth-cell extension
				// provides true overlap; proposals alone never establish containment.
				for part, step := 0, 8; part < 64; {
					if e := ctx.Err(); e != nil {
						return fail(e)
					}
					if step > 64-part {
						step = 64 - part
					}
					lo := -.75*r + 1.5*r*float64(part)/64
					hi := -.75*r + 1.5*r*float64(part+step)/64
					pad := (hi - lo) / 8
					lo -= pad
					hi += pad
					near, far := math.Min(math.Abs(lo), math.Abs(hi)), math.Max(math.Abs(lo), math.Abs(hi))
					if lo <= 0 && hi >= 0 {
						near = 0
					}
					low, high := math.Sqrt(r*r-far*far), math.Sqrt(r*r-near*near)
					if sign < 0 {
						low, high = -high, -low
					}
					depPad := r / 32
					da := geom.ParameterRectangle{UMin: c[0] + lo - depPad, UMax: c[0] + hi + depPad, VMin: c[1] + low - depPad, VMax: c[1] + high + depPad}
					db := da
					db.UMin = c[0] + lo
					db.UMax = c[0] + hi
					if axis == 1 {
						da = geom.ParameterRectangle{UMin: c[0] + low - depPad, UMax: c[0] + high + depPad, VMin: c[1] + lo - depPad, VMax: c[1] + hi + depPad}
						db = da
						db.VMin = c[1] + lo
						db.VMax = c[1] + hi
					}
					freeAxis := axis + 2
					if transport != nil {
						for owner := 0; owner < 2; owner++ {
							box := da
							if owner == 1 {
								box = db
							}
							remaining := maximum - used
							if remaining < 1 {
								return fail(geom.ErrConvergence)
							}
							mapped, _, work, err := transport.OriginalBox(ctx, owner, box, geom.SurfacePatchOptions{MaxOperations: remaining, MaxCoefficientBits: o.MaxCoefficientBits})
							used += work
							if err != nil {
								return fail(err)
							}
							if owner == 0 {
								da = mapped
							} else {
								db = mapped
							}
						}
						originalAxis, err := transport.OriginalAxis(1, axis)
						if err != nil {
							return fail(err)
						}
						freeAxis = originalAxis + 2
					}
					xa, e := pa.Trim(da)
					if e != nil {
						return fail(e)
					}
					xb, e := pb.Trim(db)
					if e != nil {
						return fail(e)
					}
					remaining := (maximum - used) / 2
					if remaining < 1 {
						return fail(geom.ErrConvergence)
					}
					opts := geom.SurfacePairTubeOptions{FreeAxis: freeAxis, MaxOperations: remaining, MaxCoefficientBits: o.MaxCoefficientBits}
					cert, e := geom.CertifySurfacePairTube(xa, xb, opts)
					used += cert.Test.ExactOperations
					if e != nil || !cert.ProvenBranch {
						if step > 1 {
							step /= 2
							continue
						}
						if e == nil {
							e = geom.ErrConvergence
						}
						return fail(e)
					}
					// The immutable constructor re-proves the same inputs; charge that exact work.
					tube, e := geom.NewVerifiedSurfaceTube(xa, xb, opts)
					used += cert.Test.ExactOperations
					if e != nil {
						return fail(e)
					}
					if len(tubes) >= maxTubes {
						return fail(geom.ErrConvergence)
					}
					tubes = append(tubes, tube)
					seeds.Attempts = append(seeds.Attempts, SplineSeedCell{Cell: geom.SurfaceSurfaceCell{DomainA: da, DomainB: db}, Tube: &cert})
					part += step
					step = 8
				}
			}
		}
	}
	return seeds, tubes, radial, used, nil
}

func certifyRadialOverlapCoverage(ctx context.Context, pa, pb *geom.SurfacePatch, supplied SplineSeedReport, o SplineTubeCoverageOptions) (*RadialOverlapCoverage, error) {
	q := *o.RadialOverlap
	maximum := q.Proof.MaxOperations
	if maximum == 0 {
		maximum = 64000000
	}
	var transport *geom.AffineRadialContours
	transportWork := 0
	proposalOptions := q.Proof
	if q.AffineCharts {
		cap := maximum
		if cap > 128000000 {
			cap = 128000000
		}
		var err error
		transport, err = geom.CertifyAffineRadialGraphContours(ctx, pa, pb, q.Center, q.Radii, geom.GraphPointContactOptions{MaxOperations: cap, MaxCoefficientBits: q.Proof.MaxCoefficientBits})
		if err != nil {
			return nil, fmt.Errorf("affine radial transport: %w", err)
		}
		transportWork = transport.ExactOperations()
		if transportWork >= maximum {
			return nil, geom.ErrConvergence
		}
		proposalOptions.MaxOperations = maximum - transportWork
	}
	seeds, tubes, radial, used, e := radialOverlapProposals(ctx, pa, pb, q.Center, q.Radii, proposalOptions, transport)
	used += transportWork
	if e != nil {
		return nil, fmt.Errorf("radial proposals: %w", e)
	}
	if used >= maximum {
		return nil, geom.ErrConvergence
	}
	proofOptions := q.Proof
	proofOptions.MaxOperations = maximum - used
	var cover *geom.RadialTubeCoverage
	if transport != nil {
		cover, e = transport.CertifyTubeCoverage(ctx, tubes, proofOptions)
	} else {
		cover, e = radial.CertifyTubeCoverage(ctx, tubes, proofOptions)
	}
	if e != nil {
		return nil, fmt.Errorf("radial endpoint cover: %w", e)
	}
	used += cover.ExactOperations()
	var targets []geom.SurfacePairTubeRegion
	for _, s := range supplied.Attempts {
		if s.Tube == nil || !s.Tube.ProvenBranch {
			continue
		}
		if s.Tube.FreeAxis < 0 || s.Tube.FreeAxis > 3 || !s.Cell.DomainA.Valid() || !s.Cell.DomainB.Valid() {
			return nil, geom.ErrInvalid
		}
		if s.BoundaryAnchor != nil {
			return nil, geom.ErrConvergence
		}
		targets = append(targets, geom.SurfacePairTubeRegion{Box: geom.SurfacePairParameterBox{A: s.Cell.DomainA, B: s.Cell.DomainB}, FreeAxis: s.Tube.FreeAxis})
	}
	cache, e := geom.NewSurfacePairTubeCoverageCache(pa, pb, 0)
	if e != nil {
		return nil, e
	}
	if len(seeds.Attempts) > o.MaxCandidateTests {
		return nil, geom.ErrConvergence
	}
	mode := RadialTransferGeneric
	strata, tests := 0, 0
	if transport != nil {
		mode = RadialTransferPhysical
	}
	for index, s := range seeds.Attempts {
		if used >= maximum {
			return nil, fmt.Errorf("radial transfer cell %d/%d: cumulative operation cap %d: %w", index+1, len(seeds.Attempts), maximum, geom.ErrConvergence)
		}
		opts := o.Cell
		bits := q.Proof.MaxCoefficientBits
		if bits == 0 {
			bits = 4096
		}
		if opts.MaxCoefficientBits == 0 || opts.MaxCoefficientBits > bits {
			opts.MaxCoefficientBits = bits
		}
		if opts.MaxOperations == 0 || opts.MaxOperations > maximum-used {
			opts.MaxOperations = maximum - used
		}
		box := geom.SurfacePairParameterBox{A: s.Cell.DomainA, B: s.Cell.DomainB}
		var available []geom.SurfacePairTubeRegion
		overlap := func(a, b geom.ParameterRectangle) bool {
			return a.UMin <= b.UMax && b.UMin <= a.UMax && a.VMin <= b.VMax && b.VMin <= a.VMax
		}
		for _, target := range targets {
			if overlap(box.A, target.Box.A) && overlap(box.B, target.Box.B) {
				available = append(available, target)
			}
		}
		var result geom.SurfaceTubeUnionCoverage
		if transport != nil {
			result, e = transport.CertifyCellInTubes(ctx, box, available, opts)
		} else {
			result, e = cache.CertifyCell(ctx, box, available, opts)
		}
		strata += result.PartitionPieces
		tests += result.VisitedCells
		used += result.ExactOperations
		if e != nil {
			return nil, fmt.Errorf("radial transfer cell %d/%d: %d original tubes, %d visited cells, %d partition pieces, %d/%d cell operations and %d/%d total: %w",
				index+1, len(seeds.Attempts), len(available), result.VisitedCells, result.PartitionPieces, result.ExactOperations, opts.MaxOperations, used, maximum, e)
		}
		if !result.Covered {
			return nil, fmt.Errorf("radial transfer cell %d/%d: %d original tubes, %d unresolved pieces: %w", index+1, len(seeds.Attempts), len(available), len(result.Unresolved)+len(result.Pending), ErrUnresolved)
		}
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	if used > maximum {
		return nil, geom.ErrConvergence
	}
	return &RadialOverlapCoverage{Cover: cover, ExactOperations: used, Transferred: len(seeds.Attempts), TransferMode: mode, TransferStrata: strata, TransferTests: tests}, nil
}
