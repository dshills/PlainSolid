package geom

import (
	"context"
	"math/big"
	"testing"
)

func TestHelicalFiniteBandPointClassification(t *testing.T) {
	ctx := context.Background()
	f, _ := NewFrame(Vec3{}, Vec3{1, 0, 0}, Vec3{0, 0, 1})
	p, _ := NewHelicalPolygon(f, []Vec2{{2, 0}, {3, .5}, {2, 1}}, 2, RightHanded, -1, 2)
	origin := [3]*big.Rat{big.NewRat(0, 1), big.NewRat(5, 2), big.NewRat(0, 1)}
	band := [2]*big.Rat{big.NewRat(-2, 1), big.NewRat(4, 1)}
	for _, test := range []struct {
		x                 *big.Rat
		band, material    HelicalLineLocation
		retained, removed bool
	}{{big.NewRat(-3, 1), HelicalLineOutside, HelicalLineOutside, false, false}, {big.NewRat(-2, 1), HelicalLineBoundary, HelicalLineOutside, false, false}, {big.NewRat(0, 1), HelicalLineInterior, HelicalLineOutside, true, false}, {big.NewRat(1, 1), HelicalLineInterior, HelicalLineInterior, false, true}, {big.NewRat(3, 4), HelicalLineInterior, HelicalLineBoundary, false, true}} {
		r, e := p.ClassifyBandPoint(ctx, big.NewRat(0, 1), big.NewRat(1, 1), origin, big.NewRat(1, 1), band, test.x, HelicalPolygonLineOptions{})
		if e != nil || !r.Complete || r.BandLocation != test.band || r.MaterialLocation != test.material || r.RetainedInterior != test.retained || r.Removed != test.removed {
			t.Fatal(test, r, e)
		}
	}
	// A rational query just outside a cardinal endpoint stays outside, even below
	// float64 underflow scale; no display tolerance determines groove membership.
	tiny := new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Lsh(big.NewInt(1), 1100))
	q := new(big.Rat).Sub(big.NewRat(3, 4), tiny)
	r, e := p.ClassifyBandPoint(ctx, big.NewRat(0, 1), big.NewRat(1, 1), origin, big.NewRat(1, 1), band, q, HelicalPolygonLineOptions{})
	if e != nil || !r.RetainedInterior {
		t.Fatal(r, e)
	}
	rayOrigin := [3]*big.Rat{big.NewRat(3, 2), big.NewRat(2, 1), big.NewRat(0, 1)}
	coarse, e := p.IntersectAxisLine(ctx, big.NewRat(3, 5), big.NewRat(4, 5), rayOrigin, big.NewRat(1, 1), HelicalPolygonLineOptions{PhaseBits: 16})
	if e != nil {
		t.Fatal(e)
	}
	endpoint := coarse.Intervals[1].Bounds[0]
	mid := new(big.Rat).Quo(new(big.Rat).Add(endpoint[0], endpoint[1]), big.NewRat(2, 1))
	refined, e := p.ClassifyBandPoint(ctx, big.NewRat(3, 5), big.NewRat(4, 5), rayOrigin, big.NewRat(1, 1), band, mid, HelicalPolygonLineOptions{PhaseBits: 16})
	if e != nil || !refined.Complete || refined.Refinements < 1 || refined.MaterialLocation == HelicalLineBoundary {
		t.Fatal(refined, e)
	}

	if bad, e := p.ClassifyBandPoint(ctx, big.NewRat(3, 5), big.NewRat(4, 5), rayOrigin, big.NewRat(1, 1), band, mid, HelicalPolygonLineOptions{PhaseBits: 16, MaxOperations: refined.ExactOperations - 1}); e == nil || bad.Complete || bad.RetainedInterior || bad.Removed {
		t.Fatal("cumulative refinement cap", bad, e)
	}
	if bad, e := p.ClassifyBandPoint(ctx, big.NewRat(0, 1), big.NewRat(1, 1), origin, big.NewRat(1, 1), band, big.NewRat(0, 1), HelicalPolygonLineOptions{MaxOperations: 1}); e == nil || bad.Complete {
		t.Fatal("budget", bad, e)
	}
}
