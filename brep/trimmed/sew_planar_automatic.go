package trimmed

import (
	"context"
	"fmt"
	"math/big"
	"plainsolid/geom"
)

// AutomaticPlanarSewOptions bounds seam discovery and fixed-support repair.
// MaxPairTests defaults to 131072 comparisons between directed borders on
// different faces.
type AutomaticPlanarSewOptions struct {
	PlanarToleranceSewOptions
	MaxPairTests int
}

// AutomaticPlanarSewReport records the exact repair and the inferred pairings.
// Seams are ordered by the original flat face, loop and edge positions.
type AutomaticPlanarSewReport struct {
	PlanarToleranceSewReport
	Seams     []PlanarSewSeam
	PairTests int
}

// SewPlanarAutomaticallyWithinTolerance finds a unique partner and direction
// for each original straight border. Each endpoint pair must lie within twice
// the requested displacement, which is necessary for a common repaired point.
// The exact fixed-support repair then proves every displacement against the
// original tolerance. Missing or ambiguous correspondence, inconsistent
// supports, and invalid or unembedded output fail atomically.
func SewPlanarAutomaticallyWithinTolerance(ctx context.Context, input []PlanarSewFace, o AutomaticPlanarSewOptions) (Solid, AutomaticPlanarSewReport, error) {
	var empty AutomaticPlanarSewReport
	if o.MaxPairTests == 0 {
		o.MaxPairTests = 131072
	}
	if o.MaxPairTests < 1 || o.MaxPairTests > 1048576 {
		return Solid{}, empty, geom.ErrInvalid
	}
	settings, err := planarToleranceSewOptions(ctx, len(input), o.PlanarToleranceSewOptions)
	if err != nil {
		return Solid{}, empty, err
	}
	charts, rings, vertices, err := planarSewOriginal(ctx, input, settings)
	if err != nil {
		return Solid{}, empty, err
	}
	distance := new(big.Rat).Mul(ratInset(settings.Tolerance), big.NewRat(2, 1))
	seams, pairs, err := discoverPlanarBorders(ctx, rings, vertices, distance.Mul(distance, distance), o.MaxPairTests)
	if err != nil {
		return Solid{}, empty, err
	}
	result, proof, err := sewPreparedPlanarFaces(ctx, input, seams, charts, rings, vertices, settings)
	if err != nil {
		return Solid{}, empty, err
	}
	return result, AutomaticPlanarSewReport{proof, seams, pairs}, nil
}

func discoverPlanarBorders(ctx context.Context, rings [][][]int, vertices []planarSewSample, squaredTolerance *big.Rat, limit int) ([]PlanarSewSeam, int, error) {
	type border struct {
		ref      PlanarSewBorder
		ends     [2][3]*big.Rat
		matched  int
		reversed bool
	}
	borders := make([]border, len(vertices))
	for face, wires := range rings {
		for loop, wire := range wires {
			for j, id := range wire {
				a, b := vertices[id].point, vertices[wire[(j+1)%len(wire)]].point
				borders[id] = border{ref: PlanarSewBorder{face, loop, j}, ends: [2][3]*big.Rat{a, b}, matched: -1}
			}
		}
	}
	near := func(a, b [3]*big.Rat) bool {
		delta := ddSub(a, b)
		return ddDot(delta, delta).Cmp(squaredTolerance) <= 0
	}
	work := 0
	for i := range borders {
		a := &borders[i]
		for j := i + 1; j < len(borders); j++ {
			if err := ctx.Err(); err != nil {
				return nil, 0, err
			}
			b := &borders[j]
			if a.ref.Face == b.ref.Face {
				continue
			}
			if work == limit {
				return nil, 0, geom.ErrConvergence
			}
			work++
			forward := near(a.ends[0], b.ends[0]) && near(a.ends[1], b.ends[1])
			reverse := near(a.ends[0], b.ends[1]) && near(a.ends[1], b.ends[0])
			if !forward && !reverse {
				continue
			}
			if forward && reverse || a.matched >= 0 || b.matched >= 0 {
				return nil, 0, fmt.Errorf("trimmed: ambiguous planar border pair %v and %v: %w", a.ref, b.ref, geom.ErrConvergence)
			}
			a.matched, b.matched, a.reversed = j, i, reverse
		}
	}
	var seams []PlanarSewSeam
	for i, b := range borders {
		if b.matched < 0 {
			return nil, 0, fmt.Errorf("trimmed: no planar border match for %v: %w", b.ref, geom.ErrConvergence)
		}
		if b.matched > i {
			seams = append(seams, PlanarSewSeam{First: b.ref, Second: borders[b.matched].ref, Reversed: b.reversed})
		}
	}
	return seams, work, nil
}
