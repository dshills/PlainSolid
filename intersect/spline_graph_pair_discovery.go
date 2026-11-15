package intersect

import (
	"context"
	"errors"
	"plainsolid/geom"
)

// DiscoverCommonXYGraphPairTubes proposes verified local tubes from an exact
// height-difference contour of two common-XY spline graphs. Degrees and weight
// fields may differ. The report is not a completeness proof: callers must run
// CertifySplinePairBranches to independently prove pair coverage and topology.
// Nonrepresented arc endpoints and unsupported boundary layouts stay unresolved.
func DiscoverCommonXYGraphPairTubes(ctx context.Context, a, b Patch, p geom.SurfacePatchOptions, o SplinePlanePairDiscoveryOptions) (out SplineSeedReport, err error) {
	return discoverGraphPairTubes(ctx, a, b, p, o, false)
}

// DiscoverAffineXYGraphPairTubes accepts independently scaled, reversed and
// permuted affine-XY graph charts with exactly matching projected rectangles.
// Returned local seeds still require independent pair coverage/topology proof.
func DiscoverAffineXYGraphPairTubes(ctx context.Context, a, b Patch, p geom.SurfacePatchOptions, o SplinePlanePairDiscoveryOptions) (SplineSeedReport, error) {
	return discoverGraphPairTubes(ctx, a, b, p, o, true)
}
func discoverGraphPairTubes(ctx context.Context, a, b Patch, p geom.SurfacePatchOptions, o SplinePlanePairDiscoveryOptions, affine bool) (out SplineSeedReport, err error) {
	if ctx == nil || !a.valid() || !b.valid() {
		return out, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return out, e
	}
	if o.MaxArcs == 0 {
		o.MaxArcs = 64
	}
	if o.MaxTubes == 0 {
		o.MaxTubes = 256
	}
	if o.MaxArcs < 1 || o.MaxArcs > 4096 || o.MaxTubes < 1 || o.MaxTubes > 4096 || len(o.UCuts)+len(o.VCuts) > 64 || o.Tubes.MaxOperations < 0 || (o.Tubes.MaxCoefficientBits != 0 && o.Tubes.MaxCoefficientBits < 64) {
		return out, geom.ErrInvalid
	}
	pa, e := exactSplinePatch(a, p)
	if e != nil {
		return out, e
	}
	pb, e := exactSplinePatch(b, p)
	if e != nil {
		return out, e
	}
	aligned := pb
	if affine {
		alignment, e := geom.AlignAffineXYCharts(ctx, pa, pb, geom.SurfacePatchOptions{MaxOperations: o.Tubes.MaxOperations, MaxCoefficientBits: o.Tubes.MaxCoefficientBits})
		if e != nil {
			return out, e
		}
		aligned = alignment.TargetOnSource()
	}
	difference, e := geom.NewGeneralSurfaceHeightDifferencePatch(ctx, pa, aligned, geom.GeneralSurfaceHeightOptions{MaxOperations: o.Tubes.MaxOperations, MaxCoefficientBits: o.Tubes.MaxCoefficientBits})
	if e != nil {
		if errors.Is(e, geom.ErrInvalid) || errors.Is(e, geom.ErrDomain) {
			return out, ErrUnresolved
		}
		return out, e
	}
	for axis, cuts := range [2][]float64{o.UCuts, o.VCuts} {
		for _, cut := range cuts {
			if axis == 0 {
				difference, e = difference.InsertUKnot(cut, 1, p)
			} else {
				difference, e = difference.InsertVKnot(cut, 1, p)
			}
			if e != nil {
				return out, e
			}
		}
	}
	plane, _ := geom.NewPlane(geom.Vec3{}, geom.Vec3{Z: 1})
	trace, e := geom.TraceSurfacePlaneContext(ctx, difference, plane, o.Plane)
	if e != nil && !errors.Is(e, geom.ErrConvergence) {
		return out, e
	}
	if !trace.Complete || len(trace.Isolation.Coplanar) > 0 || len(trace.Isolation.Tangencies) > 0 || len(trace.Isolation.Arcs) > o.MaxArcs {
		return out, ErrUnresolved
	}
	for _, contour := range trace.Contours {
		for _, ref := range contour.Arcs {
			arc := trace.Isolation.Arcs[ref.Index]
			x, e := arc.ExactFreeEndpoint(0, geom.ArcExactEndpointOptions{})
			if e != nil {
				return out, ErrUnresolved
			}
			y, e := arc.ExactFreeEndpoint(1, geom.ArcExactEndpointOptions{})
			if e != nil {
				return out, ErrUnresolved
			}
			start, e := geom.RepresentedExactScalar(x)
			if e != nil {
				return out, ErrUnresolved
			}
			end, e := geom.RepresentedExactScalar(y)
			if e != nil || start == end {
				return out, ErrUnresolved
			}
			discover := geom.DiscoverCommonXYGraphArcTubes
			if affine {
				discover = geom.DiscoverAffineXYGraphArcTubes
			}
			found, e := discover(ctx, arc, pa, pb, min(start, end), max(start, end), o.Tubes)
			if e != nil {
				return out, e
			}
			if !found.Complete || len(found.Excluded) > 0 || len(out.Attempts)+len(found.Tubes) > o.MaxTubes {
				return out, ErrUnresolved
			}
			for _, tube := range found.Tubes {
				qa, e := pa.Trim(tube.Box.A)
				if e != nil {
					return out, e
				}
				qb, e := pb.Trim(tube.Box.B)
				if e != nil {
					return out, e
				}
				proof, e := geom.CertifySurfacePairTube(qa, qb, geom.SurfacePairTubeOptions{FreeAxis: tube.FreeAxis, MaxOperations: o.Tubes.MaxOperations, MaxCoefficientBits: o.Tubes.MaxCoefficientBits})
				var anchor *TubeBoundaryAnchor
				if tube.Boundary != nil {
					proof, e = geom.CertifyBoundaryAnchoredSurfacePairTube(ctx, qa, qb, *tube.Boundary)
					if e != nil {
						return out, e
					}
					anchor, e = NewTubeBoundaryAnchor(tube.Boundary.AtUpper, tube.Boundary.Anchor)
					if e != nil {
						return out, e
					}
				}

				if e != nil {
					return out, e
				}
				if !proof.ProvenBranch {
					return out, ErrUnresolved
				}
				out.Attempts = append(out.Attempts, SplineSeedCell{Cell: geom.SurfaceSurfaceCell{DomainA: tube.Box.A, DomainB: tube.Box.B}, Tube: &proof, BoundaryAnchor: anchor})
			}
		}
	}
	return out, nil
}
