package fillet

import (
	"context"
	"fmt"
	"math/big"

	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// CircularSweepCorner selects the start of one original directed segment.
// Ring zero is the exterior; subsequent indices are source holes.
type CircularSweepCorner struct {
	Ring, Corner int
	Radius       float64
}

// CircularSweepCornerEdge retains the original directed parameter range and
// identity of one surviving source span. Map evaluates the original
// parameter from [0,1] on the surviving edge as a fractional-linear function,
// with constant/linear coefficients in the first row for the numerator and
// the second row for the denominator.
type CircularSweepCornerEdge struct {
	Wall, LowerRim, UpperRim int
	Minimum, Maximum         *big.Rat
	Map                      [2][2]*big.Rat
}

// CircularSweepCornerRound identifies the newly added exact circular support.
type CircularSweepCornerRound struct {
	Selection                CircularSweepCorner
	Wall, LowerRim, UpperRim int
}

// CircularSweepCornerResult retains all unselected source arcs and original
// ring identities, and maps each shortened original span to its exact range.
type CircularSweepCornerResult struct {
	Profile  trimmed.ExactCircularSweep
	Solid    trimmed.Solid
	Original [][]CircularSweepCornerEdge
	Rounds   []CircularSweepCornerRound
}

type mixedCornerRound struct {
	incoming, outgoing, center geom.Vec2
	radius                     float64
	reach2                     q
}

func mixedFilletAxis(curve trimmed.ExactCircularLoopSegment) (orthogonalDirection, bool) {
	if curve.Radius != 0 {
		return orthogonalDirection{}, false
	}
	x, y := qs(num(curve.End.X), num(curve.Start.X)), qs(num(curve.End.Y), num(curve.Start.Y))
	if (x.Sign() == 0) == (y.Sign() == 0) {
		return orthogonalDirection{}, false
	}
	return orthogonalDirection{int64(x.Sign()), int64(y.Sign())}, true
}

func mixedFilletPoint(a, b geom.Vec2) [2]q {
	return [2]q{qs(num(a.X), num(b.X)), qs(num(a.Y), num(b.Y))}
}
func mixedFilletDot(a, b [2]q) q   { return qa(qm(a[0], b[0]), qm(a[1], b[1])) }
func mixedFilletCross(a, b [2]q) q { return qs(qm(a[0], b[1]), qm(a[1], b[0])) }

func mixedFilletDiskClear(center geom.Vec2, r2 q, s trimmed.ExactCircularLoopSegment) bool {
	if squaredDistance(center, s.Start).Cmp(r2) <= 0 || squaredDistance(center, s.End).Cmp(r2) <= 0 {
		return false
	}
	if s.Radius == 0 {
		d, v := mixedFilletPoint(s.End, s.Start), mixedFilletPoint(center, s.Start)
		t, n := mixedFilletDot(d, v), mixedFilletDot(d, d)
		if t.Sign() <= 0 || t.Cmp(n) >= 0 {
			return true
		}
		// Compare the exact squared perpendicular distance without division.
		cross := mixedFilletCross(d, v)
		return qm(cross, cross).Cmp(qm(r2, n)) > 0
	}
	r0 := num(s.Radius)
	d := mixedFilletPoint(center, s.Center)
	D := mixedFilletDot(d, d)
	r02 := qm(r0, r0)
	// |sqrt(D)-r0|² > r2, compared exactly without extracting roots.
	gap := qs(qa(D, r02), r2)
	if gap.Sign() > 0 && qm(gap, gap).Cmp(qm(big.NewRat(4, 1), qm(D, r02))) > 0 {
		return true
	}
	if D.Sign() == 0 {
		return r02.Cmp(r2) > 0
	}
	// Endpoints are already clear. The only possible interior minimum
	// is the support-circle point on the ray towards the disk center.
	a, b := mixedFilletPoint(s.Start, s.Center), mixedFilletPoint(s.End, s.Center)
	orientation := mixedFilletCross(a, b).Sign()
	return mixedFilletCross(a, d).Sign()*orientation < 0 || mixedFilletCross(d, b).Sign()*orientation < 0
}

func mixedFilletDisksSeparated(distance2, r2, s2 q) bool {
	gap := qs(qs(distance2, r2), s2)
	return gap.Sign() > 0 && qm(gap, gap).Cmp(qm(big.NewRat(4, 1), qm(r2, s2))) > 0
}

func mixedFilletTangent(c geom.Vec2, prev, next orthogonalDirection, r float64) (mixedCornerRound, error) {
	turn := prev[0]*next[1] - prev[1]*next[0]
	if turn == 0 {
		return mixedCornerRound{}, ErrInvalid
	}
	rad := num(r)
	shift := func(x float64, v int64) q { return qa(num(x), qm(rad, big.NewRat(v, 1))) }
	u, v := shift(c.X, -prev[0]), shift(c.Y, -prev[1])
	in, okA := exactOrthogonalPoint(u, v)
	out, okB := exactOrthogonalPoint(shift(c.X, next[0]), shift(c.Y, next[1]))
	center, okC := exactOrthogonalPoint(qa(u, qm(rad, big.NewRat(-turn*prev[1], 1))), qa(v, qm(rad, big.NewRat(turn*prev[0], 1))))
	if !okA || !okB || !okC {
		return mixedCornerRound{}, geom.ErrUnrepresentable
	}
	// The disk with twice the radius also contains the original sharp corner.
	return mixedCornerRound{incoming: in, outgoing: out, center: center, radius: r, reach2: qm(qm(rad, rad), big.NewRat(4, 1))}, nil
}

// CircularSweepCorners rounds selected junctions of lines and circular arcs
// on a model-Z sweep. Tangent points and the center must be exactly
// representable, including on oblique lines with rational unit directions.
// All arcs and holes retain their original supports. Every corner
// influence region is separated from all nonincident spans and other rounds.
func CircularSweepCorners(ctx context.Context, source trimmed.ExactCircularSweep, selection []CircularSweepCorner, o OrthogonalPrismOptions) (CircularSweepCornerResult, error) {
	var zero CircularSweepCornerResult
	if ctx == nil || len(selection) == 0 || len(selection) > 64 || o.MaxWork < 0 {
		return zero, ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	if _, err := trimmed.ExtrudeExactDirectedCircularLoops(ctx, source.Outer, source.Holes, source.Bottom, source.Top, o.Circular); err != nil {
		return zero, err
	}
	rings := append([][]trimmed.ExactCircularLoopSegment{source.Outer}, source.Holes...)
	limit := o.MaxWork
	if limit == 0 {
		limit = 100000
	}
	tick := func() error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if limit == 0 {
			return ErrLimit
		}
		limit--
		return nil
	}
	n := 0
	for _, r := range rings {
		n += len(r)
	}
	if len(selection) > 64-n {
		return zero, ErrLimit
	}
	sites := map[[2]int]mixedCornerRound{}
	for _, s := range selection {
		if s.Ring < 0 || s.Ring >= len(rings) || s.Corner < 0 || s.Corner >= len(rings[s.Ring]) || !finite(s.Radius) || s.Radius <= 0 {
			return zero, ErrInvalid
		}
		key := [2]int{s.Ring, s.Corner}
		if _, exists := sites[key]; exists {
			return zero, ErrInvalid
		}
		ring := rings[s.Ring]
		prev := (s.Corner + len(ring) - 1) % len(ring)
		in, out := ring[prev], ring[s.Corner]
		da, a := mixedFilletAxis(in)
		db, b := mixedFilletAxis(out)
		var round mixedCornerRound
		var err error
		switch {
		case a && b:
			round, err = mixedFilletTangent(out.Start, da, db, s.Radius)
		case in.Radius == 0 && out.Radius == 0:
			round, err = mixedFilletObliqueLines(in, out, s.Radius)
		case in.Radius > 0 && out.Radius > 0:
			round, err = mixedFilletArcPair(in, out, s.Radius)
		default:
			round, err = mixedFilletLineArc(in, out, s.Radius)
		}
		if err != nil {
			return zero, err
		}
		for i, wire := range rings {
			for j, edge := range wire {
				if i == s.Ring && (j == prev || j == s.Corner) {
					continue
				}
				if err := tick(); err != nil {
					return zero, err
				}
				if !mixedFilletDiskClear(round.center, round.reach2, edge) {
					return zero, fmt.Errorf("%w: fillet influence at ring %d corner %d reaches original ring %d span %d", ErrRadius, s.Ring, s.Corner, i, j)
				}
			}
		}
		for _, other := range sites {
			if err := tick(); err != nil {
				return zero, err
			}
			if !mixedFilletDisksSeparated(squaredDistance(round.center, other.center), round.reach2, other.reach2) {
				return zero, fmt.Errorf("%w: selected fillet influence regions meet", ErrRadius)
			}
		}
		sites[key] = round
	}
	return assembleMixedCircularSweepCorners(ctx, source, rings, sites, n+len(sites), o.Circular)
}

func assembleMixedCircularSweepCorners(ctx context.Context, source trimmed.ExactCircularSweep, rings [][]trimmed.ExactCircularLoopSegment,
	sites map[[2]int]mixedCornerRound, total int, o trimmed.ExactCircularLoopOptions) (CircularSweepCornerResult, error) {
	var zero CircularSweepCornerResult
	out := CircularSweepCornerResult{
		Profile:  trimmed.ExactCircularSweep{Bottom: source.Bottom, Top: source.Top},
		Original: make([][]CircularSweepCornerEdge, len(rings)),
	}
	offset := 0
	for i, wire := range rings {
		var rebuilt []trimmed.ExactCircularLoopSegment
		out.Original[i] = make([]CircularSweepCornerEdge, len(wire))
		for j, s := range wire {
			if err := ctx.Err(); err != nil {
				return zero, err
			}
			here, selected := sites[[2]int{i, j}]
			next := sites[[2]int{i, (j + 1) % len(wire)}]
			parent := s
			if selected {
				k := offset + len(rebuilt)
				out.Rounds = append(out.Rounds, CircularSweepCornerRound{
					Selection: CircularSweepCorner{Ring: i, Corner: j, Radius: here.radius}, Wall: 2 + k, LowerRim: k, UpperRim: total + k,
				})
				rebuilt = append(rebuilt, trimmed.ExactCircularLoopSegment{Start: here.incoming, End: here.outgoing, Center: here.center, Radius: here.radius})
				s.Start = here.outgoing
			}
			if next.radius > 0 {
				s.End = next.incoming
			}
			lo, hi := new(big.Rat), big.NewRat(1, 1)
			law := [2][2]*big.Rat{{new(big.Rat), big.NewRat(1, 1)}, {big.NewRat(1, 1), new(big.Rat)}}
			if selected || next.radius > 0 {
				var err error
				lo, hi, law, err = trimmed.CircularLoopRestriction(parent, s)
				if err != nil {
					return zero, fmt.Errorf("%w: fillets consume or reverse original ring %d span %d: %v", ErrRadius, i, j, err)
				}
			}
			k := offset + len(rebuilt)
			out.Original[i][j] = CircularSweepCornerEdge{Wall: 2 + k, LowerRim: k, UpperRim: total + k, Minimum: lo, Maximum: hi, Map: law}
			rebuilt = append(rebuilt, s)
		}
		if i == 0 {
			out.Profile.Outer = rebuilt
		} else {
			out.Profile.Holes = append(out.Profile.Holes, rebuilt)
		}
		offset += len(rebuilt)
	}
	p, err := trimmed.ExtrudeExactDirectedCircularLoops(ctx, out.Profile.Outer, out.Profile.Holes, out.Profile.Bottom, out.Profile.Top, o)
	if err != nil {
		return zero, err
	}
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	out.Solid = p
	return out, nil
}
