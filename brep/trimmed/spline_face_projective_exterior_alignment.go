//go:build research

package trimmed

import (
	"context"
	"math/big"
	"sort"

	"plainsolid/geom"
)

// The sealed graph gives both original UV identities for every chosen
// common station. Projected coordinates are separable, hence either
// transverse boundary station yields the same aligned cut.
func projectiveImprintExteriorStations(ctx context.Context, g geom.ProjectiveSurfaceContactGraph,
	o geom.SurfacePatchOptions) (out [2][2][]*big.Rat, work int, err error) {
	var result [2][2][]*big.Rat
	zero, one := new(big.Rat), big.NewRat(1, 1)
	for axis := range 2 {
		byValue := make(map[string]*big.Rat)
		for _, cell := range g.Cover.Atlas.Cells {
			if err := ctx.Err(); err != nil {
				return out, work, err
			}
			d := [2]geom.ExactParameterInterval{cell.Common.U, cell.Common.V}[axis]
			for _, x := range [2]*big.Rat{d.Lower, d.Upper} {
				if x.Sign() > 0 && x.Cmp(one) < 0 {
					byValue[x.RatString()] = x
				}
			}
		}
		cuts := make([]*big.Rat, 0, len(byValue))
		for _, x := range byValue {
			cuts = append(cuts, x)
		}
		sort.Slice(cuts, func(i, j int) bool { return cuts[i].Cmp(cuts[j]) < 0 })
		rawB := axis
		if g.Cover.Atlas.Overlap.SwapB {
			rawB = 1 - axis
		}
		for _, x := range cuts {
			left := o.MaxOperations - work
			if left < 1 {
				return out, work, geom.ErrConvergence
			}
			station := [2]*big.Rat{zero, zero}
			station[axis] = x
			q, err := g.ClassifyCommonHeightRational(ctx, station,
				geom.SurfacePatchEvaluationOptions{MaxOperations: left, MaxCoefficientBits: o.MaxCoefficientBits})
			work += q.ExactOperations
			if err != nil {
				return out, work, err
			}
			result[0][axis] = append(result[0][axis], q.OriginalUV[0][axis])
			result[1][rawB] = append(result[1][rawB], q.OriginalUV[1][rawB])
		}
	}
	return result, work, ctx.Err()
}

// Refine the original directed use, keeping the native UV endpoints and
// original normalized use clock exact at each added station.
func projectiveImprintAlignOriginalUse(ctx context.Context, p []geom.Vec2, t []*big.Rat, cuts [2][]*big.Rat,
	o geom.SurfacePatchOptions) ([]geom.Vec2, []*big.Rat, int, error) {
	a, b := p[0], p[len(p)-1]
	axis := 0
	first, last := a.X, b.X
	if first == last {
		axis, first, last = 1, a.Y, b.Y
	}
	if first == last || len(cuts[axis]) == 0 {
		return p, t, 0, nil
	}
	type station struct {
		uv geom.Vec2
		t  *big.Rat
	}
	positions := make([]station, len(p), len(p)+len(cuts[axis]))
	for i := range p {
		positions[i] = station{p[i], t[i]}
	}
	start, end := new(big.Rat).SetFloat64(first), new(big.Rat).SetFloat64(last)
	width := new(big.Rat).Sub(end, start)
	lo, hi := start, end
	if lo.Cmp(hi) > 0 {
		lo, hi = hi, lo
	}
	bits := o.MaxCoefficientBits
	if bits == 0 {
		bits = 16384
	}
	work := 1
	for _, q := range cuts[axis] {
		if err := ctx.Err(); err != nil {
			return nil, nil, work, err
		}
		if q.Cmp(lo) <= 0 || q.Cmp(hi) >= 0 {
			continue
		}
		if work > o.MaxOperations-2 {
			return nil, nil, work, geom.ErrConvergence
		}
		work += 2
		uv := a
		x, err := geom.RepresentedExactScalar(q)
		if err != nil {
			return nil, nil, work, err
		}
		if axis == 0 {
			uv.X = x
		} else {
			uv.Y = x
		}
		clock := new(big.Rat).Quo(new(big.Rat).Sub(q, start), width)
		if clock.Num().BitLen() > bits || clock.Denom().BitLen() > bits {
			return nil, nil, work, geom.ErrConvergence
		}
		positions = append(positions, station{uv, clock})
	}
	sort.Slice(positions, func(i, j int) bool { return positions[i].t.Cmp(positions[j].t) < 0 })
	p, t = p[:0], t[:0]
	for _, q := range positions {
		if len(t) > 0 && q.t.Cmp(t[len(t)-1]) == 0 {
			if q.uv != p[len(p)-1] {
				return nil, nil, work, geom.ErrConvergence
			}
		} else {
			p, t = append(p, q.uv), append(t, q.t)
		}
	}
	return p, t, work, nil
}
