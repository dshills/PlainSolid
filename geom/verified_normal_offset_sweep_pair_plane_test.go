package geom

import (
	"context"
	"encoding/json"
	"errors"
	"math/big"
	"plainsolid/internal/slowtest"
	"testing"
)

func TestVerifiedNormalOffsetSweepPairWithObliquePlane(t *testing.T) {
	ctx := context.Background()
	a, b := obliqueSweepPairSources(t, false)
	join := obliqueSweepPairJoin()
	pair, proof, err := NewVerifiedNormalOffsetSweepPair(ctx, a, b, join, NormalOffsetCoverageOptions{})
	expected := big.NewRat(477, 112)
	if err != nil || pair == nil || !proof.Complete || !proof.Seam.Complete || !proof.Seam.SharedOriginalEdge ||
		proof.Coordinate.Cmp(expected) != 0 || len(pair.Boundary()) != 10 {
		t.Fatal("exact rotated pair did not retain a complete swept wall and boundary", pair, proof, err)
	}
	wallA, _ := a.Boundary(NormalOffsetSweepUMax)
	wallB, _ := b.Boundary(NormalOffsetSweepVMin)
	for _, station := range []*big.Rat{new(big.Rat), big.NewRat(1, 5), big.NewRat(1, 2), big.NewRat(4, 5), big.NewRat(1, 1)} {
		y := new(big.Rat).Sub(new(big.Rat).Mul(station, big.NewRat(2, 1)), big.NewRat(1, 1))
		bRaw := new(big.Rat).Sub(big.NewRat(1, 1), station)
		den := new(big.Rat).Add(big.NewRat(1, 1), new(big.Rat).Quo(station, big.NewRat(2, 1)))
		bRaw.Quo(bRaw, den).Mul(bRaw, big.NewRat(2, 1)).Add(bRaw, big.NewRat(2, 1))
		for _, d := range []*big.Rat{big.NewRat(-1, 11), new(big.Rat), big.NewRat(1, 17), big.NewRat(2, 13)} {
			p, err := wallA.EvaluateRational(ctx, y, d, SurfacePatchEvaluationOptions{}, SurfaceNormalOffsetEvaluationOptions{})
			if err != nil {
				t.Fatal(err)
			}
			q, err := wallB.EvaluateRational(ctx, bRaw, d, SurfacePatchEvaluationOptions{}, SurfaceNormalOffsetEvaluationOptions{})
			if err != nil {
				t.Fatal(err)
			}
			for i := range p.Lower {
				if p.Lower[i].Cmp(q.Lower[i]) != 0 || p.Upper[i].Cmp(q.Upper[i]) != 0 {
					t.Fatal("paired original stations changed under rotation", station, bRaw, d, i)
				}
			}
			lo, hi := obliqueSweepTestProjection(p, *join.Covector)
			if lo.Cmp(expected) > 0 || hi.Cmp(expected) < 0 {
				t.Fatal("continuous swept wall left the oblique plane", station, bRaw, d, lo, hi)
			}
			if y.Sign() == 0 {
				// At the middle station the local source normal is +Z.
				// The rotated wall point is the exactly rational rigid
				// placement of (5/8,0,d).
				world := obliqueSweepTestTransform(big.NewRat(5, 8), new(big.Rat), d)
				for i := range world {
					if p.Lower[i].Cmp(world[i]) > 0 || p.Upper[i].Cmp(world[i]) < 0 {
						t.Fatal("rotated wall differs from the independent physical point", i, d, p, world)
					}
				}
			}
		}
	}
	pa, err := a.EvaluateRational(ctx, big.NewRat(5, 2), new(big.Rat), big.NewRat(2, 13),
		SurfacePatchEvaluationOptions{}, SurfaceNormalOffsetEvaluationOptions{})
	if err != nil {
		t.Fatal(err)
	}
	pb, err := b.EvaluateRational(ctx, big.NewRat(14, 5), big.NewRat(-1, 2), big.NewRat(-1, 11),
		SurfacePatchEvaluationOptions{}, SurfaceNormalOffsetEvaluationOptions{})
	if err != nil {
		t.Fatal(err)
	}
	_, ahi := obliqueSweepTestProjection(pa, *join.Covector)
	blo, _ := obliqueSweepTestProjection(pb, *join.Covector)
	if ahi.Cmp(expected) >= 0 || blo.Cmp(expected) <= 0 {
		t.Fatal("unequal displacement layers crossed their oblique halfspaces", ahi, blo)
	}
	// Negating the raw covector changes the original coordinate scale
	// and reverses the accepted open halfspace signs.
	reverse := obliqueSweepPairJoin()
	for _, q := range *reverse.Covector {
		q.Neg(q)
	}
	reverse.AInteriorSign = 1
	_, opposite, err := NewVerifiedNormalOffsetSweepPair(ctx, a, b, reverse, NormalOffsetCoverageOptions{})
	if err != nil || !opposite.Complete || opposite.Coordinate.Cmp(new(big.Rat).Neg(expected)) != 0 {
		t.Fatal("reversed raw covector convention changed the physical certificate", opposite, err)
	}

	(*join.Covector)[0].SetInt64(700)
	(*join.Covector)[1] = nil
	proof.Coordinate.SetInt64(700)
	recovered, plane := pair.Join()
	(*recovered.Covector)[2].SetInt64(700)
	plane.SetInt64(700)
	recovered, plane = pair.Join()
	for i, q := range *obliqueSweepPairJoin().Covector {
		if (*recovered.Covector)[i].Cmp(q) != 0 {
			t.Fatal("caller changed the retained original covector", recovered)
		}
	}
	if plane.Cmp(expected) != 0 {
		t.Fatal("caller changed the retained plane constant", plane)
	}
	t.Logf("oblique plane constant %s, %d exact operations including seam %d", plane, proof.ExactOperations, proof.Seam.ExactOperations)
}

func TestVerifiedNormalOffsetSweepPairPreservesCartesianJoinJSON(t *testing.T) {
	join := VerifiedNormalOffsetSweepJoin{
		Edges: [2]SurfaceEdge{SurfaceEdgeUMax, SurfaceEdgeVMin}, ReverseB: true,
		BStationRatio: big.NewRat(3, 2), Axis: 0, AInteriorSign: -1,
	}
	stored, err := json.Marshal(join)
	want := `{"Edges":[1,2],"ReverseB":true,"BStationRatio":"3/2","Axis":0,"AInteriorSign":-1}`
	if err != nil || string(stored) != want {
		t.Fatal("default Cartesian join stored fields changed", string(stored), err)
	}
	a := sweepPairParaboloid(t, false, false, 0, big.NewRat(-1, 11), big.NewRat(2, 13))
	b := sweepPairParaboloid(t, true, true, 0, big.NewRat(-1, 11), big.NewRat(2, 13))
	join.Covector = &[3]*big.Rat{big.NewRat(2, 1)}
	_, sparse, err := NewVerifiedNormalOffsetSweepPair(context.Background(), a, b, join, NormalOffsetCoverageOptions{})
	if err != nil || !sparse.Complete || sparse.Coordinate.Cmp(big.NewRat(5, 4)) != 0 {
		t.Fatal("explicit rational projection lost its scale or zero components", sparse, err)
	}
}

func TestVerifiedNormalOffsetSweepPairRefusesBadObliquePlanes(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	a, b := obliqueSweepPairSources(t, false)
	tangentA, tangentB := obliqueSweepPairSources(t, true)
	for _, c := range []struct {
		name   string
		a, b   *VerifiedNormalOffsetSweep
		q      *[3]*big.Rat
		sign   int
		expect error
	}{
		{"false wall station projection", a, b, &[3]*big.Rat{big.NewRat(9, 1), big.NewRat(88, 1), big.NewRat(-25, 1)}, -1, ErrDomain},
		// The added rotated local Z component is orthogonal to the
		// straight original edge but projects the swept wall away.
		{"constant edge with nonzero normal projection", tangentA, tangentB,
			&[3]*big.Rat{big.NewRat(68, 1), big.NewRat(51, 1), big.NewRat(35, 1)}, -1, ErrDomain},
		{"incorrect halfspace sign", a, b, obliqueSweepPairJoin().Covector, 1, ErrDomain},
		{"explicit zero covector", a, b, &[3]*big.Rat{new(big.Rat)}, -1, ErrInvalid},
	} {
		t.Run(c.name, func(t *testing.T) {
			join := obliqueSweepPairJoin()
			join.Covector, join.AInteriorSign = c.q, c.sign
			pair, proof, err := NewVerifiedNormalOffsetSweepPair(ctx, c.a, c.b, join, NormalOffsetCoverageOptions{})
			if !errors.Is(err, c.expect) || pair != nil || proof.Complete || proof.Coordinate != nil {
				t.Fatal("bad covector published a joined sweep", pair, proof, err)
			}
			if c.expect == ErrDomain && !proof.Seam.Complete {
				t.Fatal("negative control must still prove original wall agreement", proof)
			}
		})
	}
	join := obliqueSweepPairJoin()
	_, full, err := NewVerifiedNormalOffsetSweepPair(ctx, a, b, join, NormalOffsetCoverageOptions{})
	if err != nil {
		t.Fatal(err)
	}
	short := NormalOffsetCoverageOptions{Patch: SurfacePatchOptions{MaxOperations: full.ExactOperations - 1}}
	pair, proof, err := NewVerifiedNormalOffsetSweepPair(ctx, a, b, join, short)
	if !errors.Is(err, ErrConvergence) || pair != nil || proof.Complete || proof.Coordinate != nil || !proof.Seam.Complete {
		t.Fatal("short exact budget published a rotated union", pair, proof, err)
	}
	scale := new(big.Rat).SetInt(new(big.Int).Lsh(big.NewInt(1), 17000))
	for _, q := range *join.Covector {
		q.Mul(q, scale)
	}
	pair, proof, err = NewVerifiedNormalOffsetSweepPair(ctx, a, b, join, NormalOffsetCoverageOptions{})
	if !errors.Is(err, ErrConvergence) || pair != nil || proof.Complete || proof.Coordinate != nil || !proof.Seam.Complete {
		t.Fatal("overlong exact covector bypassed the coefficient budget", pair, proof, err)
	}
}

func obliqueSweepTestProjection(p RationalSurfaceOffsetPoint, covector [3]*big.Rat) (lo, hi *big.Rat) {
	lo, hi = new(big.Rat), new(big.Rat)
	for i, q := range covector {
		a, b := p.Lower[i], p.Upper[i]
		if q.Sign() < 0 {
			a, b = b, a
		}
		lo.Add(lo, new(big.Rat).Mul(q, a))
		hi.Add(hi, new(big.Rat).Mul(q, b))
	}
	return lo, hi
}

func obliqueSweepPairJoin() VerifiedNormalOffsetSweepJoin {
	return VerifiedNormalOffsetSweepJoin{
		Edges: [2]SurfaceEdge{SurfaceEdgeUMax, SurfaceEdgeVMin}, ReverseB: true, BStationRatio: big.NewRat(3, 2),
		Axis: -1, AInteriorSign: -1, Covector: &[3]*big.Rat{big.NewRat(48, 7), big.NewRat(36, 7), big.NewRat(-25, 7)},
	}
}

// The author rotates exact bivariate paraboloids about two world axes. The
// original B chart retains its independent reversed projective station law.
func obliqueSweepPairSources(t *testing.T, acrossOnly bool) (*VerifiedNormalOffsetSweep, *VerifiedNormalOffsetSweep) {
	t.Helper()
	ctx := context.Background()
	lower, upper := big.NewRat(-1, 11), big.NewRat(2, 13)
	var sweeps [2]*VerifiedNormalOffsetSweep
	for owner := range sweeps {
		recipe := ExactBezierSurfaceRecipe{
			Version: 1, DegreeU: 2, DegreeV: 2, Domain: ParameterRectangle{2, 3, -1, 1},
		}
		view := NormalOffsetSweepProjection{Axes: [3]int{0, 1, 2}}
		if owner == 1 {
			recipe.Domain = ParameterRectangle{2, 4, -1, 0}
			view = NormalOffsetSweepProjection{Axes: [3]int{1, 0, 2}, Reversed: [3]bool{true}}
		}
		for i := int64(0); i <= 2; i++ {
			for j := int64(0); j <= 2; j++ {
				xi, yi, weight := i, j, big.NewRat(1, 1)
				square := [3]int64{1, 0, 0}[xi]
				if owner == 1 {
					xi, yi = j, 2-i
					square = [3]int64{0, 0, 1}[xi]
					weight = [3]*big.Rat{big.NewRat(1, 1), big.NewRat(3, 2), big.NewRat(9, 4)}[i]
				}
				x := big.NewRat(5+4*(xi+int64(owner)*2-2), 8)
				y := big.NewRat(yi-1, 1)
				z := big.NewRat(square, 32)
				if !acrossOnly {
					z.Add(z, big.NewRat([3]int64{1, -1, 1}[yi], 64))
				}
				world := obliqueSweepTestTransform(x, y, z)
				recipe.Controls = append(recipe.Controls, [4]string{
					world[0].Mul(world[0], weight).RatString(), world[1].Mul(world[1], weight).RatString(),
					world[2].Mul(world[2], weight).RatString(), weight.RatString(),
				})
			}
		}
		source, err := ImportExactBezierSurfaceRecipe(ctx, recipe, SurfacePatchOptions{})
		if err != nil {
			t.Fatal(err)
		}
		sweep, proof, err := NewVerifiedNormalOffsetSweep(ctx, source, lower, upper, view, NormalOffsetCoverageOptions{})
		if err != nil || !proof.Complete {
			t.Fatal("independent oblique source sweep must be embedded", owner, acrossOnly, proof, err)
		}
		sweeps[owner] = sweep
	}
	return sweeps[0], sweeps[1]
}

func obliqueSweepTestTransform(x, y, z *big.Rat) (world [3]*big.Rat) {
	matrix := [3][3]int64{{48, -39, 20}, {36, 52, 15}, {-25, 0, 60}}
	world = [3]*big.Rat{big.NewRat(1, 8), big.NewRat(-1, 4), big.NewRat(5, 16)}
	for i, row := range matrix {
		for j, q := range []*big.Rat{x, y, z} {
			world[i].Add(world[i], new(big.Rat).Mul(big.NewRat(row[j], 65), q))
		}
	}
	return world
}
