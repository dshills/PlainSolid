package intersect

import (
	"context"
	"math/big"

	"plainsolid/geom"
)

// RetainedSplineTerminal retains exact discovery coordinates and all incident
// original domain sides. Bits 0..3 mean UMin, UMax, VMin, VMax respectively;
// both operand masks may be nonzero at a shared boundary or corner.
// Discovery.Contour indexes discovery contours, not Trace branch indices.
type RetainedSplineTerminal struct {
	Discovery     SplineOpenPlaneTerminal
	OriginalSides [2]uint8
}

// CertifiedOpenSplinePlanePair couples exact retained terminals with exhaustive
// coverage of the original (unclipped) input rectangles. It has no success state
// until discovery, terminal ownership and independent coverage all succeed.
type CertifiedOpenSplinePlanePair struct {
	A, B      Patch
	Trace     SplinePairTrace
	Terminals []RetainedSplineTerminal
	Complete  bool
}

// CertifyOpenSplinePlanePair qualifies open transverse branches against an
// affine planar spline patch, including clipped overlaps. It preserves original
// input-domain ownership rather than treating clipping edges as new source
// boundaries. Tangencies, mixed closed contours and unsupported endpoints fail
// atomically. Options retain their individual underlying operation limits.
func CertifyOpenSplinePlanePair(ctx context.Context, a, b Patch, o OverlappingGraphTraceOptions) (CertifiedOpenSplinePlanePair, error) {
	fail := func(e error) (CertifiedOpenSplinePlanePair, error) { return CertifiedOpenSplinePlanePair{}, e }
	if ctx == nil || !a.valid() || !b.valid() {
		return fail(geom.ErrInvalid)
	}
	discovered, e := DiscoverOpenSplinePlanePairTubes(ctx, a, b, o.Surface, o.Discovery)
	if e != nil {
		return fail(e)
	}
	terminals := make([]RetainedSplineTerminal, len(discovered.Terminals))
	for i, t := range discovered.Terminals {
		if !t.Certificate.Proven {
			return fail(ErrUnresolved)
		}
		terminals[i].Discovery = t
		for operand, p := range [2]Patch{a, b} {
			vals := [4]float64{p.Min.X, p.Max.X, p.Min.Y, p.Max.Y}
			for side, v := range vals {
				q := t.Parameters[2*operand+side/2]
				if q == nil {
					return fail(ErrUnresolved)
				}
				if q.Cmp(new(big.Rat).SetFloat64(v)) == 0 {
					terminals[i].OriginalSides[operand] |= 1 << side
				}
			}
		}
		if terminals[i].OriginalSides == ([2]uint8{}) {
			return fail(ErrUnresolved)
		}
	}
	trace, e := CertifySplinePairBranches(ctx, a, b, discovered.Seeds, o.Trace)
	if e != nil {
		return fail(e)
	}
	if !trace.Complete {
		return fail(ErrUnresolved)
	}
	return CertifiedOpenSplinePlanePair{A: a, B: b, Trace: trace, Terminals: terminals, Complete: true}, nil
}
