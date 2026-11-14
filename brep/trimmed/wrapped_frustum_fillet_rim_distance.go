package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
	"sort"
)

// FilletedFrustumRimDistanceOptions preserves the original all-support budget
// and adds at most36 full native rim comparisons (4 fillet +2 per bore).
type FilletedFrustumRimDistanceOptions struct {
	Support      FilletedFrustumDistanceOptions
	MaxRimChecks int
}

// FilletedFrustumAlgebraicRimWitness is an exact point on a complete original
// circular rim: (CenterX+Radius*AzimuthX/sqrt(AzimuthSquared),
// CenterY+Radius*AzimuthY/sqrt(AzimuthSquared), Z). The square root is positive
// and its owned rational interval is enclosed by AzimuthRoot. No displayed
// or rounded point is substituted. Faces owns this whole circle; RimEdges
// identifies the entire native circle partition, not one chosen point edge.
type FilletedFrustumAlgebraicRimWitness struct {
	Center          [2]*big.Rat
	Radius, Z       *big.Rat
	Azimuth         [2]*big.Rat
	AzimuthSquared  *big.Rat
	AzimuthRoot     [2]*big.Rat
	Faces, RimEdges []int
}

// FilletedFrustumRimDistance refines the always-rational-witness baseline.
// If Rim is nonnil its exact algebraic point, not Baseline.Witness, supplies
// the new upper bound. NearestProven proves the selected witness is globally
// nearest: an exact baseline witness or a guarded entry-hole/blind-floor rim case. It
// never claims uniqueness. Otherwise the
// result is only a conservative interval. Baseline retains all source faces.
type FilletedFrustumRimDistance struct {
	Lower, Upper             *big.Rat
	Baseline                 FilletedFrustumDistance
	Rim                      *FilletedFrustumAlgebraicRimWitness
	NearestProven, Converged bool
	RimChecks                int
}

func (s Solid) filletedSharedRimEdges(a, b int) []int {
	collect := func(f Face) map[int]bool {
		m := map[int]bool{}
		for _, u := range f.Outer {
			m[u.Edge] = true
		}
		for _, h := range f.Holes {
			for _, u := range h {
				m[u.Edge] = true
			}
		}
		return m
	}
	first, second := collect(s.faces[a]), collect(s.faces[b])
	var edges []int
	for e := range first {
		if second[e] {
			edges = append(edges, e)
		}
	}
	sort.Ints(edges)
	return edges
}

// BoundBoundaryRimDistanceRational adds full-circle analytic witnesses, including
// irrational azimuths, without changing the rational-witness API. Complete rim
// retention follows from the private native fillet/bore preparation proof and
// its strict cutter clearance; no sampled trim or public history is trusted.
func (q FilletedFrustumMaterial) BoundBoundaryRimDistanceRational(ctx context.Context, p [3]*big.Rat, o FilletedFrustumRimDistanceOptions) (out FilletedFrustumRimDistance, err error) {
	defer func() {
		if err != nil {
			out = FilletedFrustumRimDistance{}
		}
	}()
	if o.MaxRimChecks == 0 {
		o.MaxRimChecks = 36
	}
	if o.MaxRimChecks < 1 || o.MaxRimChecks > 36 {
		return out, geom.ErrInvalid
	}
	base, e := q.BoundBoundaryDistanceRational(ctx, p, o.Support)
	if e != nil {
		return out, e
	}
	out.Baseline = base
	out.Lower = new(big.Rat).Set(base.Lower)
	out.Upper = new(big.Rat).Set(base.Upper)
	out.NearestProven = base.Exact
	bits := o.Support.SqrtBits
	if bits == 0 {
		bits = 64
	}
	candidate := func(center [2]*big.Rat, radius, z *big.Rat, faces [2]int, entryEnd int, floor bool) error {
		if e := ctx.Err(); e != nil {
			return e
		}
		if out.RimChecks >= o.MaxRimChecks {
			return geom.ErrConvergence
		}
		out.RimChecks++
		dx, dy := fdSub(p[0], center[0]), fdSub(p[1], center[1])
		rho2 := fdAdd(fdSquare(dx), fdSquare(dy))
		if rho2.Sign() == 0 {
			return nil
		} // rational +X witness already available
		lo, hi := fdSqrt(rho2, bits)
		_, upper := fdSquareRange(fdSub(lo, radius), fdSub(hi, radius))
		upper.Add(upper, fdSquare(fdSub(p[2], z)))
		if upper.Cmp(out.Upper) >= 0 {
			return nil
		}
		edges := q.solid.filletedSharedRimEdges(faces[0], faces[1])
		if len(edges) == 0 {
			return geom.ErrConvergence
		}
		w := &FilletedFrustumAlgebraicRimWitness{Radius: new(big.Rat).Set(radius), Z: new(big.Rat).Set(z), Azimuth: [2]*big.Rat{dx, dy}, AzimuthSquared: rho2, AzimuthRoot: [2]*big.Rat{lo, hi}, Faces: []int{faces[0], faces[1]}, RimEdges: edges}
		sort.Ints(w.Faces)
		for i := range center {
			w.Center[i] = new(big.Rat).Set(center[i])
		}
		out.Rim = w
		out.Upper = upper
		out.NearestProven = false
		// Inside an entry disk, on the external side of its original cap, both
		// the holed cap and finite cylinder attain their minimum on this circle.
		// At a blind end, outside the floor disk and beyond the wall end,
		// the retained disk and finite cylinder likewise share this minimum.
		// All other faces must independently be farther than this upper enclosure.
		radialSide := rho2.Cmp(fdSquare(radius))
		if entryEnd >= 0 && ((!floor && radialSide < 0) || (floor && radialSide > 0)) && ((entryEnd == 0 && p[2].Cmp(z) <= 0) || (entryEnd == 1 && p[2].Cmp(z) >= 0)) {
			proven := true
			for _, f := range base.PerFace {
				if f.Face != faces[0] && f.Face != faces[1] && f.SquaredLower.Cmp(upper) < 0 {
					proven = false
					break
				}
			}
			out.NearestProven = proven
		}
		return nil
	}
	for end, c := range q.fillets.Circles {
		if e := candidate(q.center, c.Center[0], q.z[end], [2]int{q.fillets.Caps[end], q.fillets.Blends[end]}, -1, false); e != nil {
			return out, e
		}
		if e := candidate(q.center, c.Tangency[0], c.Tangency[1], [2]int{q.fillets.Lateral, q.fillets.Blends[end]}, -1, false); e != nil {
			return out, e
		}
	}
	for _, c := range q.cuts {
		for end, z := range c.z {
			face, entry := c.floor, end
			if c.open[end] {
				face = q.fillets.Caps[end]
				entry = end
			}
			if e := candidate(c.center, c.radius, z, [2]int{face, c.wall}, entry, !c.open[end]); e != nil {
				return out, e
			}
		}
	}
	if out.Lower.Cmp(out.Upper) > 0 {
		return out, geom.ErrConvergence
	}
	tolerance := new(big.Rat)
	if o.Support.SquaredTolerance != nil {
		tolerance.Set(o.Support.SquaredTolerance)
	}
	out.Converged = fdSub(out.Upper, out.Lower).Cmp(tolerance) <= 0
	return out, nil
}
