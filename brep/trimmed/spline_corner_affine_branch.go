package trimmed

import (
	"context"
	"fmt"
	"math/big"
	"plainsolid/geom"
)

// SplineCornerAffineBranch proves the entire intersection segment of two
// original affine face rectangles, bound to a verified local corner anchor.
// It is separate from a tube trace and does not authorize shell assembly.
type SplineCornerAffineBranch struct {
	anchor    *SplineCornerAnchor
	endpoints [2]geom.AffineChartIntersectionEndpoint
	corner    int
}

func (b *SplineCornerAffineBranch) FacePair() [2]int      { return b.anchor.pair }
func (b *SplineCornerAffineBranch) CornerEnd() int        { return b.corner }
func (b *SplineCornerAffineBranch) PendingAssembly() bool { return b != nil && b.anchor != nil }
func (b *SplineCornerAffineBranch) Endpoints() [2]geom.AffineChartIntersectionEndpoint {
	var out [2]geom.AffineChartIntersectionEndpoint
	for i, p := range b.endpoints {
		out[i].BoundaryA, out[i].BoundaryB = p.BoundaryA, p.BoundaryB
		for j := range p.Point {
			out[i].Point[j] = new(big.Rat).Set(p.Point[j])
		}
		for j := range p.A {
			out[i].A[j] = new(big.Rat).Set(p.A[j])
			out[i].B[j] = new(big.Rat).Set(p.B[j])
		}
	}
	return out
}

// CertifySplineCornerAffineBranch proves rational full-domain clipping and
// that the retained anchored tube continues into that unique segment. Exactly
// one endpoint must be the proved stock corner; the far endpoint must lie on
// one stock side, with the other face chart interior at both ends.
func CertifySplineCornerAffineBranch(ctx context.Context, anchor *SplineCornerAnchor, o SplineRectangleEmbeddingOptions) (*SplineCornerAffineBranch, error) {
	if ctx == nil || anchor == nil || anchor.chart != 0 {
		return nil, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	var charts [2]*geom.AffineSurfaceChart
	for i, s := range anchor.solids {
		f, _, e := s.splineEmbeddingFace(ctx, s.faces[anchor.pair[i]], o)
		if e != nil {
			return nil, e
		}
		charts[i], e = geom.NewAffineSurfaceChart(ctx, f.patch, geom.AffineSurfaceChartOptions{MaxOperations: o.Patches.MaxOperations, MaxCoefficientBits: o.Patches.MaxCoefficientBits})
		if e != nil {
			return nil, e
		}
	}
	exact, e := geom.IntersectAffineSurfaceCharts(ctx, charts[0], charts[1], geom.AffineChartIntersectionOptions{MaxOperations: o.Patches.MaxOperations, MaxCoefficientBits: o.Patches.MaxCoefficientBits})
	if e != nil {
		return nil, e
	}
	if !exact.Complete || exact.Kind != geom.AffineChartsSegment {
		return nil, geom.ErrConvergence
	}
	parameters := anchor.endpoint.Parameters()
	corner := -1
	for i, p := range exact.Endpoints {
		same := true
		for j, q := range [4]*big.Rat{p.A[0], p.A[1], p.B[0], p.B[1]} {
			same = same && q.Cmp(parameters[j]) == 0
		}
		if same {
			if corner >= 0 {
				return nil, geom.ErrConvergence
			}
			corner = i
		}
	}
	if corner < 0 {
		return nil, geom.ErrConvergence
	}
	mask := uint8(0)
	for _, side := range anchor.endpoint.Sides() {
		mask |= 1 << uint(side)
	}
	near, far := exact.Endpoints[corner], exact.Endpoints[corner^1]
	if near.BoundaryA != mask || near.BoundaryB != 0 || far.BoundaryB != 0 || far.BoundaryA == 0 || far.BoundaryA&(far.BoundaryA-1) != 0 {
		return nil, geom.ErrConvergence
	}
	axis, interval, e := anchor.tube.ExactFreeDomain()
	if e != nil {
		return nil, e
	}
	ends := [2][4]*big.Rat{{near.A[0], near.A[1], near.B[0], near.B[1]}, {far.A[0], far.A[1], far.B[0], far.B[1]}}
	lo, hi := ends[0][axis], ends[1][axis]
	if lo.Cmp(hi) > 0 {
		lo, hi = hi, lo
	}
	if lo.Cmp(hi) >= 0 || interval.Lower.Cmp(lo) < 0 || interval.Upper.Cmp(hi) > 0 {
		return nil, geom.ErrConvergence
	}
	if e = ctx.Err(); e != nil {
		return nil, e
	}
	return &SplineCornerAffineBranch{anchor: anchor, endpoints: exact.Endpoints, corner: corner}, nil
}

// ResolveSplineCornerAffineSections supplements a full diagnostic ledger with
// exact affine clipping for its two pending corner pairs. It never removes
// unknown entries on failure or grants assembly permission. Cancellation is
// atomic. Returned branch evidence is immutable and bound to original faces.
func ResolveSplineCornerAffineSections(ctx context.Context, source *SplineCornerSectionLedger, o SplineRectangleEmbeddingOptions) (*SplineCornerSectionLedger, error) {
	if ctx == nil || source == nil || source.fan == nil {
		return nil, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	out := &SplineCornerSectionLedger{fan: source.fan, pairs: source.Pairs(), affine: append([]*SplineCornerAffineBranch(nil), source.affine...)}
	for _, anchor := range source.fan.anchors {
		for i, p := range out.pairs {
			if p.Faces != anchor.pair || p.Kind != SplinePairUnknown {
				continue
			}
			proof, e := CertifySplineCornerAffineBranch(ctx, anchor, o)
			if e != nil {
				if err := ctx.Err(); err != nil {
					return nil, err
				}
				out.pairs[i].Reason += fmt.Sprintf("; affine corner: %v", e)
				continue
			}
			out.pairs[i].Kind = SplinePairAffineCornerBranchPendingAssembly
			out.pairs[i].Branches = 1
			out.pairs[i].Reason = ""
			out.affine = append(out.affine, proof)
		}
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	return out, nil
}
