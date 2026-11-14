package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
	"sort"
)

// finishFilletedLineLedger orders immutable exact roots and classifies only
// proved open cells. It never infers equality from overlapping enclosures.
func finishFilletedLineLedger(ctx context.Context, out FilletedAxialLine, lo, hi *geom.ExactAlgebraicRoot, o geom.ExactCurvePlaneEventOptions, classify func(*big.Rat) (FilletedFrustumMaterialPoint, error)) (FilletedAxialLine, error) {
	compare := func(a, b *geom.ExactAlgebraicRoot) (int, error) {
		v, e := a.Compare(ctx, b, o)
		if e != nil {
			return 0, e
		}
		if !v.Proven {
			return 0, geom.ErrConvergence
		}
		return v.Order, nil
	}
	// Exact insertion ordering merges only proven equal algebraic parameters.
	for i := 1; i < len(out.Hits); i++ {
		for j := i; j > 0; j-- {
			order, e := compare(out.Hits[j-1].Parameter, out.Hits[j].Parameter)
			if e != nil {
				return out, e
			}
			if order <= 0 {
				break
			}
			out.Hits[j-1], out.Hits[j] = out.Hits[j], out.Hits[j-1]
		}
	}
	merged := []FilletedAxialHit{}
	for _, h := range out.Hits {
		if len(merged) > 0 {
			order, e := compare(merged[len(merged)-1].Parameter, h.Parameter)
			if e != nil {
				return out, e
			}
			if order == 0 {
				merged[len(merged)-1].Faces = append(merged[len(merged)-1].Faces, h.Faces...)
				continue
			}
		}
		merged = append(merged, h)
	}
	out.Hits = merged
	between := func(a, b *geom.ExactAlgebraicRoot) (geom.BorePointLocation, error) {
		r, e := a.Compare(ctx, b, o)
		if e != nil {
			return geom.BoreOutside, e
		}
		if !r.Proven || r.Order >= 0 {
			return geom.BoreOutside, geom.ErrConvergence
		}
		left, right := r.Intervals[0][1], r.Intervals[1][0]
		if left == nil || right == nil || left.Cmp(right) >= 0 {
			return geom.BoreOutside, geom.ErrConvergence
		}
		t := new(big.Rat).Quo(new(big.Rat).Add(left, right), big.NewRat(2, 1))
		p, e := classify(t)
		if e != nil {
			return geom.BoreOutside, e
		}
		if p.Location == geom.BoreBoundary {
			return geom.BoreOutside, geom.ErrConvergence
		}
		return p.Location, nil
	}
	for i := range out.Hits {
		h := &out.Hits[i]
		sort.Ints(h.Faces)
		a, b := lo, hi
		if i > 0 {
			a = out.Hits[i-1].Parameter
		}
		if i+1 < len(out.Hits) {
			b = out.Hits[i+1].Parameter
		}
		order, e := compare(a, h.Parameter)
		if e != nil {
			return out, e
		}
		if order < 0 {
			h.Before, e = between(a, h.Parameter)
			if e != nil {
				return out, e
			}
			h.BeforeKnown = true
		}
		order, e = compare(h.Parameter, b)
		if e != nil {
			return out, e
		}
		if order < 0 {
			h.After, e = between(h.Parameter, b)
			if e != nil {
				return out, e
			}
			h.AfterKnown = true
		}
		h.Kind = "endpoint"
		if h.BeforeKnown && h.AfterKnown {
			h.Kind = "tangent"
			if h.Before == geom.BoreOutside && h.After == geom.BoreInside {
				h.Kind = "entry"
			}
			if h.Before == geom.BoreInside && h.After == geom.BoreOutside {
				h.Kind = "exit"
			}
		}
	}
	for i := 1; i < len(out.TrimmedAway); i++ {
		for j := i; j > 0; j-- {
			order, e := compare(out.TrimmedAway[j-1].Parameter, out.TrimmedAway[j].Parameter)
			if e != nil {
				return out, e
			}
			if order < 0 || order == 0 && out.TrimmedAway[j-1].Face <= out.TrimmedAway[j].Face {
				break
			}
			out.TrimmedAway[j-1], out.TrimmedAway[j] = out.TrimmedAway[j], out.TrimmedAway[j-1]
		}
	}
	return out, nil
}
