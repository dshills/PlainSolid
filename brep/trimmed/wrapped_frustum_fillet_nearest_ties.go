package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
	"sort"
)

type FilletedFrustumTieOptions struct {
	Support FilletedFrustumDistanceOptions
	// Maximum retained rational candidates observed in the single query.
	// Zero selects128; range1..128. Exhaustion is atomic.
	MaxCandidates int
}

// FilletedFrustumTiedFace has an exact retained witness attaining the global
// squared minimum. Different faces may share a point or have distinct points.
type FilletedFrustumTiedFace struct {
	Face    int
	Witness [3]*big.Rat
}

// FilletedFrustumNearestTies partitions original face IDs only when Exact.
// Tied requires an attained retained witness, Excluded a strictly larger
// lower bound. Equal lower bounds alone yield Unresolved. Complete means all
// original faces classified, not that all closest POINTS have been listed.
// If the baseline value is not exact, every original face remains Unresolved.
type FilletedFrustumNearestTies struct {
	Baseline             FilletedFrustumDistance
	Tied                 []FilletedFrustumTiedFace
	Excluded, Unresolved []int
	Exact, Complete      bool
	RetainedCandidates   int
}

// ClassifyNearestFacesRational observes existing retained rational candidates
// during ONE bounded support query. It never promotes an unattained support
// lower bound to a nearest contact. Lists are deterministic by original ID;
// unknown algebraic or continuous minimizers remain explicit unless a rational
// witness already proves that face tied. Errors return no partial ledger.
func (q FilletedFrustumMaterial) ClassifyNearestFacesRational(ctx context.Context, p [3]*big.Rat, o FilletedFrustumTieOptions) (out FilletedFrustumNearestTies, err error) {
	defer func() {
		if err != nil {
			out = FilletedFrustumNearestTies{}
		}
	}()
	if o.MaxCandidates == 0 {
		o.MaxCandidates = 128
	}
	if o.MaxCandidates < 1 || o.MaxCandidates > 128 {
		return out, geom.ErrInvalid
	}
	type candidate struct {
		d *big.Rat
		p [3]*big.Rat
	}
	seen := map[int]candidate{}
	observe := func(point [3]*big.Rat, faces []int, d *big.Rat) error {
		if out.RetainedCandidates >= o.MaxCandidates {
			return geom.ErrConvergence
		}
		out.RetainedCandidates++
		for _, face := range faces {
			old, ok := seen[face]
			if ok && old.d.Cmp(d) <= 0 {
				continue
			}
			c := candidate{d: new(big.Rat).Set(d)}
			for i := range point {
				c.p[i] = new(big.Rat).Set(point[i])
			}
			seen[face] = c
		}
		return nil
	}
	base, e := q.boundBoundaryDistanceRational(ctx, p, o.Support, observe)
	if e != nil {
		return out, e
	}
	out.Baseline = base
	out.Exact = base.Exact
	for _, f := range base.PerFace {
		if e := ctx.Err(); e != nil {
			return out, e
		}
		if !base.Exact {
			out.Unresolved = append(out.Unresolved, f.Face)
			continue
		}
		if f.SquaredLower.Cmp(base.Upper) > 0 {
			out.Excluded = append(out.Excluded, f.Face)
			continue
		}
		c, ok := seen[f.Face]
		if ok && c.d.Cmp(base.Upper) == 0 {
			out.Tied = append(out.Tied, FilletedFrustumTiedFace{Face: f.Face, Witness: c.p})
		} else {
			out.Unresolved = append(out.Unresolved, f.Face)
		}
	}
	sort.Slice(out.Tied, func(i, j int) bool { return out.Tied[i].Face < out.Tied[j].Face })
	sort.Ints(out.Excluded)
	sort.Ints(out.Unresolved)
	out.Complete = out.Exact && len(out.Unresolved) == 0
	return out, nil
}
