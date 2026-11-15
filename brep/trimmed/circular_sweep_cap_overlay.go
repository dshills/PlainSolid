package trimmed

import (
	"context"
	"fmt"
	"math/big"
	"sort"

	"plainsolid/geom"
)

// CircularSweepEdgeFragment maps one output edge to its original directed
// interval. Map coefficients are in constant/linear order:
// original parameter = (Map[0][0]+Map[0][1]*t)/(Map[1][0]+Map[1][1]*t).
// The rationals are caller-owned. Arc maps need not be affine.
type CircularSweepEdgeFragment struct {
	Edge             int
	Minimum, Maximum *big.Rat
	Map              [2][2]*big.Rat
}

// CircularSweepCapOverlay is the exact complete cap join after refining both
// original profiles to their common source vertex stations. A consumed cap
// has no surviving face. An original side may own multiple output faces.
type CircularSweepCapOverlay struct {
	Solid          Solid
	Caps           [2]int
	Faces          [2][][]int
	Edges          [2][][]CircularSweepEdgeFragment
	Vertices       [2][]int
	SharedRimEdges []int
}

type circularSweepOverlayChild struct {
	part        int
	first, last *big.Rat
	law         [2][2]*big.Rat
}

func circularSweepTangent(start, end, center sweepOffsetPoint, rad2 *big.Rat) (*big.Rat, bool) {
	a, b := sweepOffsetSub(start, center), sweepOffsetSub(end, center)
	if sweepOffsetDot(a, a).Cmp(rad2) != 0 || sweepOffsetDot(b, b).Cmp(rad2) != 0 {
		return nil, false
	}
	den := new(big.Rat).Add(rad2, sweepOffsetDot(a, b))
	if den.Sign() == 0 {
		return nil, false
	}
	return new(big.Rat).Quo(sweepOffsetCross(a, b), den), true
}

func circularSweepOverlayParameter(s ExactCircularLoopSegment, p geom.Vec2) (*big.Rat, bool) {
	a, b, q := sweepOffsetPointOf(s.Start), sweepOffsetPointOf(s.End), sweepOffsetPointOf(p)
	if s.Radius == 0 {
		v, w := sweepOffsetSub(b, a), sweepOffsetSub(q, a)
		if sweepOffsetCross(v, w).Sign() != 0 {
			return nil, false
		}
		axis := 0
		if v[0].Sign() == 0 {
			axis = 1
		}
		return new(big.Rat).Quo(w[axis], v[axis]), true
	}
	r, c := ratInset(s.Radius), sweepOffsetPointOf(s.Center)
	r2 := new(big.Rat).Mul(r, r)
	all, _ := circularSweepTangent(a, b, c, r2)
	part, ok := circularSweepTangent(a, q, c, r2)
	if !ok || all == nil || all.Sign() == 0 {
		return nil, false
	}
	return new(big.Rat).Quo(part, all), true
}

func circularSweepOverlayLaw(p, q ExactCircularLoopSegment, lo, hi *big.Rat) ([2][2]*big.Rat, error) {
	a := [2][2]*big.Rat{{new(big.Rat).Set(lo), new(big.Rat).Sub(hi, lo)}, {big.NewRat(1, 1), new(big.Rat)}}
	if p.Radius == 0 {
		return a, nil
	}
	c := sweepOffsetPointOf(p.Center)
	r2 := new(big.Rat).Mul(ratInset(p.Radius), ratInset(p.Radius))
	all, ok := circularSweepTangent(sweepOffsetPointOf(p.Start), sweepOffsetPointOf(p.End), c, r2)
	span, valid := circularSweepTangent(sweepOffsetPointOf(q.Start), sweepOffsetPointOf(q.End), c, r2)
	if !ok || !valid || all.Sign() == 0 || span.Sign() == 0 {
		return a, geom.ErrDomain
	}
	start := new(big.Rat).Mul(lo, all)
	a[0][1].Quo(span, all)
	a[1][1].Neg(new(big.Rat).Mul(start, span))
	top := new(big.Rat).Add(a[0][0], a[0][1])
	bottom := new(big.Rat).Add(a[1][0], a[1][1])
	if bottom.Sign() <= 0 || top.Quo(top, bottom).Cmp(hi) != 0 {
		return a, fmt.Errorf("trimmed: circular seam map does not restrict the original span: %w", geom.ErrDomain)
	}
	return a, nil
}

func refineCircularSweepOverlay(ctx context.Context, a, b ExactCircularSweep) (ExactCircularSweep, []circularSweepOverlayChild, int, error) {
	var zero ExactCircularSweep
	p := func(s ExactCircularSweep) [][]ExactCircularLoopSegment {
		return append([][]ExactCircularLoopSegment{s.Outer}, s.Holes...)
	}
	var indexed []geom.Vec2
	for _, ring := range p(b) {
		for _, s := range ring {
			indexed = append(indexed, s.Start)
		}
	}
	out := ExactCircularSweep{Bottom: a.Bottom, Top: a.Top}
	var children []circularSweepOverlayChild
	part := 0
	one := big.NewRat(1, 1)
	type event struct {
		p geom.Vec2
		t *big.Rat
	}
	for wire, ring := range p(a) {
		var refined []ExactCircularLoopSegment
		for _, s := range ring {
			if err := ctx.Err(); err != nil {
				return zero, nil, 0, err
			}
			events := []event{{s.Start, new(big.Rat)}, {s.End, one}}
			for _, v := range indexed {
				if t, ok := circularSweepOverlayParameter(s, v); ok && t.Sign() > 0 && t.Cmp(one) < 0 {
					events = append(events, event{v, t})
				}
			}
			sort.Slice(events, func(i, j int) bool { return events[i].t.Cmp(events[j].t) < 0 })
			for j := 0; j+1 < len(events); j++ {
				u, v := events[j], events[j+1]
				if u.t.Cmp(v.t) >= 0 {
					return zero, nil, 0, geom.ErrDomain
				}
				if len(children) >= 64 {
					return zero, nil, 0, geom.ErrConvergence
				}
				q := s
				q.Start, q.End = u.p, v.p
				law, err := circularSweepOverlayLaw(s, q, u.t, v.t)
				if err != nil {
					return zero, nil, 0, err
				}
				children = append(children, circularSweepOverlayChild{part: part, first: u.t, last: v.t, law: law})
				refined = append(refined, q)
			}
			part++
		}
		if wire == 0 {
			out.Outer = refined
		} else {
			out.Holes = append(out.Holes, refined)
		}
	}
	return out, children, part, nil
}

// JoinExactCircularSweepCapOverlay joins complete curved caps whose original
// rim subdivisions differ. The common segmentation uses only original exact
// vertices. Each child carries its original line parameter or the exact
// fractional-linear parameter transfer on its original circular arc.
func JoinExactCircularSweepCapOverlay(ctx context.Context, a, b ExactCircularSweep, o ExactCircularLoopOptions) (CircularSweepCapOverlay, error) {
	var zero CircularSweepCapOverlay
	if ctx == nil {
		return zero, geom.ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	if a.Top != b.Bottom && a.Bottom != b.Top {
		return zero, fmt.Errorf("trimmed: circular cap overlay needs abutting sweeps: %w", geom.ErrDomain)
	}
	for _, r := range []ExactCircularSweep{a, b} {
		if _, err := ExtrudeExactDirectedCircularLoops(ctx, r.Outer, r.Holes, r.Bottom, r.Top, o); err != nil {
			return zero, err
		}
	}
	x, xr, xn, err := refineCircularSweepOverlay(ctx, a, b)
	if err != nil {
		return zero, err
	}
	y, yr, yn, err := refineCircularSweepOverlay(ctx, b, a)
	if err != nil {
		return zero, err
	}
	joined, err := JoinExactCircularSweepCaps(ctx, x, y, o)
	if err != nil {
		return zero, err
	}
	out := CircularSweepCapOverlay{Solid: joined.Solid, Caps: joined.Caps, SharedRimEdges: append([]int(nil), joined.SharedRimEdges...)}
	makeFragment := func(edge int, u, v *big.Rat, law [2][2]*big.Rat) CircularSweepEdgeFragment {
		f := CircularSweepEdgeFragment{Edge: edge, Minimum: new(big.Rat).Set(u), Maximum: new(big.Rat).Set(v)}
		for i := range law {
			for j := range law[i] {
				f.Map[i][j] = new(big.Rat).Set(law[i][j])
			}
		}
		return f
	}
	z, one := new(big.Rat), big.NewRat(1, 1)
	identity := [2][2]*big.Rat{{z, one}, {one, z}}
	for operand, ch := range [2][]circularSweepOverlayChild{xr, yr} {
		n, m := []int{xn, yn}[operand], len(ch)
		out.Faces[operand] = make([][]int, n+2)
		out.Edges[operand] = make([][]CircularSweepEdgeFragment, 3*n)
		out.Vertices[operand] = make([]int, 2*n)
		for i := 0; i < 2; i++ {
			if f := joined.Faces[operand][i]; f >= 0 {
				out.Faces[operand][i] = []int{f}
			}
		}
		for i, c := range ch {
			out.Faces[operand][2+c.part] = append(out.Faces[operand][2+c.part], joined.Faces[operand][2+i])
			for cap := 0; cap < 2; cap++ {
				out.Edges[operand][cap*n+c.part] = append(out.Edges[operand][cap*n+c.part], makeFragment(joined.Edges[operand][cap*m+i], c.first, c.last, c.law))
				if c.first.Sign() == 0 {
					out.Vertices[operand][cap*n+c.part] = joined.Vertices[operand][cap*m+i]
				}
			}
			if c.first.Sign() == 0 {
				out.Edges[operand][2*n+c.part] = []CircularSweepEdgeFragment{makeFragment(joined.Edges[operand][2*m+i], z, one, identity)}
			}
		}
	}
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	return out, nil
}
