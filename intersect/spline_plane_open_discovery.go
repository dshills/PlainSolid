package intersect

import (
	"context"
	"errors"
	"math/big"
	"plainsolid/geom"
)

// SplineOpenPlaneTerminal is an exact endpoint of a discovered tube. Attempt
// indexes Seeds.Attempts; Upper selects its increasing free-coordinate end.
// End follows traced contour orientation, independently of tube orientation.
type SplineOpenPlaneTerminal struct {
	Contour, End, Attempt int
	Upper                 bool
	Parameters            [4]*big.Rat
	Point                 [3]*big.Rat
	Certificate           geom.TubeEndpointCertificate
}

// SplineOpenPlaneDiscovery supplies locally proved tubes and exact terminals,
// not exhaustive pair coverage. CertifySplinePairBranches performs that separate
// validation. Every contour must be open, with terminals on either input boundary.
// Plane-side clipping currently requires an exact axis-aligned affine projection.
type SplineOpenPlaneDiscovery struct {
	Seeds          SplineSeedReport
	Terminals      []SplineOpenPlaneTerminal
	OpenContours   int
	ClosedContours int
}

// DiscoverOpenSplinePlanePairTubes discovers regular open plane sections. It
// rejects algebraic/unrepresentable free endpoints, tangent/coincident traces,
// nonaffine/skewed projections and endpoints not covered by a strict uniform tube. The
// exact raw plane retains arbitrary rational affine supports, including
// oblique supports. No rounded endpoints become exact hints.
func DiscoverOpenSplinePlanePairTubes(ctx context.Context, a, b Patch, p geom.SurfacePatchOptions, o SplinePlanePairDiscoveryOptions) (r SplineOpenPlaneDiscovery, err error) {
	r.Seeds, err = discoverSplinePlanePairTubes(ctx, a, b, p, o, &r, false)
	return r, err
}

// SplinePlanePairSections supplies both closed and open locally proved section
// proposals. Terminal certificates belong only to its open contours.
type SplinePlanePairSections = SplineOpenPlaneDiscovery

// DiscoverSplinePlanePairSections accepts mixed closed/open contour sets. Like
// the strict-open entry point, it requires independent full pair coverage before
// these local proposals may be treated as exhaustive intersection geometry.
func DiscoverSplinePlanePairSections(ctx context.Context, a, b Patch, p geom.SurfacePatchOptions, o SplinePlanePairDiscoveryOptions) (r SplinePlanePairSections, err error) {
	r.Seeds, err = discoverSplinePlanePairTubes(ctx, a, b, p, o, &r, true)
	return r, err
}

func certifyOpenPlaneTerminals(ctx context.Context, trace geom.SurfacePlaneTrace, seeds []SplineSeedCell, a, b, plane *geom.SurfacePatch, swapped int, o SplinePlanePairDiscoveryOptions, allowClosed bool) ([]SplineOpenPlaneTerminal, int, error) {
	chart, e := geom.NewAffineSurfaceChart(ctx, plane, geom.AffineSurfaceChartOptions{MaxOperations: o.Tubes.MaxOperations, MaxCoefficientBits: o.Tubes.MaxCoefficientBits})
	if e != nil {
		return nil, 0, e
	}
	var result []SplineOpenPlaneTerminal
	if len(trace.Contours) == 0 {
		return nil, 0, ErrUnresolved
	}
	openCount := 0
	for ci, contour := range trace.Contours {
		if contour.Closed && allowClosed {
			continue
		}
		if contour.Closed || len(contour.Arcs) == 0 {
			return nil, 0, ErrUnresolved
		}
		openCount++
		for end := 0; end < 2; end++ {
			ref := contour.Arcs[0]
			index := 0
			if end == 1 {
				ref = contour.Arcs[len(contour.Arcs)-1]
				index = 1
			}
			if ref.Reversed {
				index = 1 - index
			}
			endpoint, e := trace.Isolation.Arcs[ref.Index].ExactEndpoint(index, geom.ArcExactEndpointOptions{MaxOperations: o.Tubes.MaxOperations, MaxCoefficientBits: o.Tubes.MaxCoefficientBits})
			if e != nil {
				return nil, 0, e
			}
			inv, e := chart.InvertExactContext(ctx, endpoint.Point, geom.AffineChartInverseOptions{MaxOperations: o.Tubes.MaxOperations, MaxCoefficientBits: o.Tubes.MaxCoefficientBits})
			if e != nil {
				return nil, 0, e
			}
			d := chart.Domain()
			if !inv.OnPlane || !inv.Inside {
				return nil, 0, ErrUnresolved
			}
			planeBoundary := inv.U.Cmp(new(big.Rat).SetFloat64(d.UMin)) == 0 || inv.U.Cmp(new(big.Rat).SetFloat64(d.UMax)) == 0 || inv.V.Cmp(new(big.Rat).SetFloat64(d.VMin)) == 0 || inv.V.Cmp(new(big.Rat).SetFloat64(d.VMax)) == 0

			curved := a
			if swapped == 1 {
				curved = b
			}
			cd := curved.Domain()
			u, v := endpoint.Parameters[0], endpoint.Parameters[1]
			boundary := u.Cmp(new(big.Rat).SetFloat64(cd.UMin)) == 0 || u.Cmp(new(big.Rat).SetFloat64(cd.UMax)) == 0 || v.Cmp(new(big.Rat).SetFloat64(cd.VMin)) == 0 || v.Cmp(new(big.Rat).SetFloat64(cd.VMax)) == 0
			if !boundary && !planeBoundary {
				return nil, 0, ErrUnresolved
			}
			hints := [4]*big.Rat{u, v, inv.U, inv.V}
			if swapped == 1 {
				hints = [4]*big.Rat{inv.U, inv.V, u, v}
			}
			found := false
			for si, seed := range seeds {
				if e := ctx.Err(); e != nil {
					return nil, 0, e
				}
				if seed.Tube == nil || !seed.Tube.ProvenBranch {
					continue
				}
				axis := seed.Tube.FreeAxis
				box := seed.Cell.DomainA
				if axis >= 2 {
					box = seed.Cell.DomainB
				}
				lo, hi := box.UMin, box.UMax
				if axis%2 == 1 {
					lo, hi = box.VMin, box.VMax
				}
				upper := hints[axis].Cmp(new(big.Rat).SetFloat64(hi)) == 0
				if !upper && hints[axis].Cmp(new(big.Rat).SetFloat64(lo)) != 0 {
					continue
				}
				qa, e := a.Trim(seed.Cell.DomainA)
				if e != nil {
					return nil, 0, e
				}
				qb, e := b.Trim(seed.Cell.DomainB)
				if e != nil {
					return nil, 0, e
				}
				tube, e := geom.NewVerifiedSurfaceTube(qa, qb, geom.SurfacePairTubeOptions{FreeAxis: axis, MaxOperations: o.Tubes.MaxOperations, MaxCoefficientBits: o.Tubes.MaxCoefficientBits})
				if e != nil {
					return nil, 0, e
				}
				cert, e := tube.CertifyEndpoint(ctx, upper, hints, geom.TubeEndpointOptions{MaxOperations: o.Tubes.MaxOperations, MaxCoefficientBits: o.Tubes.MaxCoefficientBits})
				if e != nil {
					if !errors.Is(e, geom.ErrConvergence) && !errors.Is(e, geom.ErrDomain) {
						return nil, 0, e
					}
					continue
				}
				terminal := SplineOpenPlaneTerminal{Contour: ci, End: end, Attempt: si, Upper: upper, Certificate: cert}
				for i, x := range cert.Parameters {
					terminal.Parameters[i] = new(big.Rat).Set(x)
				}
				for i, x := range cert.Point {
					terminal.Point[i] = new(big.Rat).Set(x)
				}
				result = append(result, terminal)
				found = true
				break
			}
			if !found {
				return nil, 0, ErrUnresolved
			}
		}
	}
	return result, openCount, nil
}
