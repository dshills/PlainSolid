package geom

import (
	"context"
	"errors"
	"math"
	"math/big"
	"plainsolid/internal/slowtest"
	"testing"
)

func TestVerifiedJoinedNormalOffsetSweepMomentsExactSlabs(t *testing.T) {
	ctx := context.Background()
	r := [3][3]*big.Rat{
		{big.NewRat(4, 5), big.NewRat(-3, 5), new(big.Rat)},
		{big.NewRat(3, 5), big.NewRat(4, 5), new(big.Rat)},
		{new(big.Rat), new(big.Rat), big.NewRat(1, 1)},
	}
	translation := [3]*big.Rat{big.NewRat(5, 4), big.NewRat(-7, 5), big.NewRat(11, 9)}
	world := func(x [3]*big.Rat) (p [3]*big.Rat) {
		for i := range p {
			p[i] = new(big.Rat).Set(translation[i])
			for j, q := range x {
				p[i].Add(p[i], new(big.Rat).Mul(q, r[i][j]))
			}
		}
		return p
	}
	x := [4]*big.Rat{big.NewRat(-1, 2), new(big.Rat), big.NewRat(1, 1), big.NewRat(3, 1)}
	lower, upper := big.NewRat(-1, 5), big.NewRat(3, 7)
	var sources [3]*VerifiedNormalOffsetSweep
	for i := range sources {
		var controls [][4]*big.Rat
		for _, px := range x[i : i+2] {
			for _, y := range []*big.Rat{big.NewRat(-1, 1), big.NewRat(2, 1)} {
				p := world([3]*big.Rat{px, y, new(big.Rat)})
				controls = append(controls, [4]*big.Rat{p[0], p[1], p[2], big.NewRat(1, 1)})
			}
		}
		p, err := NewExactHomogeneousBezierSurface(ctx, 1, 1, controls, SurfacePatchOptions{})
		if err != nil {
			t.Fatal(err)
		}
		sources[i], _, err = NewVerifiedNormalOffsetSweep(ctx, p, lower, upper,
			NormalOffsetSweepProjection{Axes: [3]int{0, 1, 2}}, NormalOffsetCoverageOptions{})
		if err != nil {
			t.Fatal(err)
		}
	}
	first := VerifiedNormalOffsetSweepJoin{
		Edges: [2]SurfaceEdge{SurfaceEdgeUMax, SurfaceEdgeUMin}, Axis: -1, AInteriorSign: -1,
		Covector: &[3]*big.Rat{big.NewRat(4, 1), big.NewRat(3, 1)},
	}
	second := first
	second.AInteriorSign = 1
	second.Covector = &[3]*big.Rat{big.NewRat(-20, 3), big.NewRat(-5, 1)}
	pair, chain := joinedSweepMomentFixture(t, sources, first, second)
	tol := big.NewRat(1, 100000)
	for i, query := range []func(context.Context, NormalOffsetSweepMomentOptions) (NormalOffsetSweepMassMoments, error){
		pair.MassMomentsRational, chain.MassMomentsRational,
	} {
		count := i + 2
		o := NormalOffsetSweepMomentOptions{VolumeTolerance: tol, CentroidTolerance: tol,
			SecondMomentTolerance: tol, InertiaTolerance: tol, MaxCells: count, Patch: SurfacePatchOptions{MaxCoefficientBits: 64}}
		got, err := query(ctx, o)
		if err != nil || !got.Complete || got.VisitedCells != count || got.ActiveCells != count {
			t.Fatal("expected one exact cell for each distinct original chart", count, got.Volume, got.VisitedCells, got.ExactOperations, err)
		}
		dx, dy, dz := new(big.Rat).Sub(x[count], x[0]), big.NewRat(3, 1), new(big.Rat).Sub(upper, lower)
		volume := new(big.Rat).Mul(dx, new(big.Rat).Mul(dy, dz))
		center := world([3]*big.Rat{new(big.Rat).Quo(new(big.Rat).Add(x[count], x[0]), big.NewRat(2, 1)), big.NewRat(1, 2), big.NewRat(4, 35)})
		verifyExactJoinedSweepSlab(t, got, volume, center, [3]*big.Rat{dx, dy, dz}, r)
		short := o
		short.Patch.MaxOperations = got.ExactOperations - 1
		refusal, err := query(ctx, short)
		if !errors.Is(err, ErrConvergence) || refusal.Complete || refusal.Volume.Lower == nil || refusal.Volume.Lower.Cmp(volume) != 0 {
			t.Fatal("one-short exact budget lost valid material bounds or published completion", count, refusal.Volume, err)
		}
		short.Patch.MaxOperations = 550
		refusal, err = query(ctx, short)
		if !errors.Is(err, ErrConvergence) || refusal.Complete || refusal.ActiveCells != 1 || refusal.Volume.Lower != nil {
			t.Fatal("an exact-work stop after one owner exposed a partial material sum", count, refusal.ActiveCells, refusal.Volume, err)
		}
		t.Logf("%d original slabs exact volume %s, centroid [%s %s %s], %d exact operations",
			count, volume, center[0], center[1], center[2], got.ExactOperations)
	}
}

func verifyExactJoinedSweepSlab(t *testing.T, got NormalOffsetSweepMassMoments, volume *big.Rat,
	center, lengths [3]*big.Rat, rotation [3][3]*big.Rat) {
	t.Helper()
	eq := func(q ExactParameterInterval, want *big.Rat) {
		t.Helper()
		if q.Lower.Cmp(want) != 0 || q.Upper.Cmp(want) != 0 {
			t.Fatal("joined slab differs from its independently known whole-body integral", q, want)
		}
	}
	eq(got.Volume, volume)
	var raw, central [3][3]*big.Rat
	for i := range center {
		eq(got.Centroid[i], center[i])
		eq(got.FirstMoment[i], new(big.Rat).Mul(center[i], volume))
		for j := range center {
			c := new(big.Rat)
			for k, l := range lengths {
				square := new(big.Rat).Mul(l, l)
				projection := new(big.Rat).Mul(rotation[i][k], rotation[j][k])
				c.Add(c, projection.Mul(projection, square))
			}
			central[i][j] = c.Mul(c, new(big.Rat).Quo(volume, big.NewRat(12, 1)))
			product := new(big.Rat).Mul(center[i], center[j])
			raw[i][j] = new(big.Rat).Add(central[i][j], product.Mul(product, volume))
			eq(got.SecondMoment[i][j], raw[i][j])
		}
	}
	for i := range central {
		for j := range central[i] {
			if i == j {
				k, l := (i+1)%3, (i+2)%3
				eq(got.InertiaOrigin[i][i], new(big.Rat).Add(raw[k][k], raw[l][l]))
				eq(got.InertiaCentroid[i][i], new(big.Rat).Add(central[k][k], central[l][l]))
			} else {
				eq(got.InertiaOrigin[i][j], new(big.Rat).Neg(raw[i][j]))
				eq(got.InertiaCentroid[i][j], new(big.Rat).Neg(central[i][j]))
			}
		}
	}
}

func TestVerifiedJoinedNormalOffsetSweepMomentsCurvedAndBudgets(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	sweeps := placedSweepChainFixture(t)
	first, second := sweepChainPlacedJoins()
	pair, chain := joinedSweepMomentFixture(t, sweeps, first, second)
	o := NormalOffsetSweepMomentOptions{
		VolumeTolerance: big.NewRat(1, 8), CentroidTolerance: big.NewRat(1, 4),
		SecondMomentTolerance: big.NewRat(7, 10), InertiaTolerance: big.NewRat(9, 10),
	}
	for i, query := range []func(context.Context, NormalOffsetSweepMomentOptions) (NormalOffsetSweepMassMoments, error){
		pair.MassMomentsRational, chain.MassMomentsRational,
	} {
		want := independentPlacedJoinedSweepMoments(float64(i + 1))
		limits := o
		if i == 1 {
			limits.VolumeTolerance = big.NewRat(1, 5)
			limits.SecondMomentTolerance = big.NewRat(1, 1)
			limits.InertiaTolerance = big.NewRat(5, 4)
		}
		got, err := query(ctx, limits)
		verifySweepMomentControl(t, got, err, limits, want)
		t.Logf("actual maximum interval widths of volume, centroid, second and inertia %v", joinedSweepMomentWidths(got))
		short := limits
		short.MaxCells = i + 2
		partial, err := query(ctx, short)
		if !errors.Is(err, ErrConvergence) || partial.Complete || partial.Volume.Lower == nil ||
			partial.Volume.Lower.Sign() <= 0 || partial.ActiveCells != i+2 || partial.VisitedCells != i+2 {
			t.Fatal("coarse result must cover every original owner before publication", i, partial.Volume, partial.ActiveCells, err)
		}
		if partial.Volume.Lower.Cmp(got.Volume.Lower) > 0 || partial.Volume.Upper.Cmp(got.Volume.Upper) < 0 {
			t.Fatal("coarse total excluded its fully refined result", i, partial.Volume, got.Volume)
		}
		for axis, q := range partial.Centroid {
			lo, _ := q.Lower.Float64()
			hi, _ := q.Upper.Float64()
			xc := want.first[axis] / want.volume
			if lo > xc || hi < xc {
				t.Fatal("coarse world centroid omitted independently integrated material", i, axis, q, xc)
			}
		}

		short.MaxCells--
		bad, err := query(ctx, short)
		if !errors.Is(err, ErrConvergence) || bad.Complete || bad.Volume.Lower != nil {
			t.Fatal("incomplete original coverage published a partial total", i, bad.Volume, err)
		}
		short = limits
		short.Patch.MaxOperations = 20000
		bad, err = query(ctx, short)
		if !errors.Is(err, ErrConvergence) || bad.Complete || bad.Volume.Lower == nil || bad.Volume.Lower.Sign() <= 0 {
			t.Fatal("short exact budget lost its valid coarse result or published completion", i, bad.Volume, err)
		}
		cancelled, cancel := context.WithCancel(ctx)
		cancel()
		bad, err = query(cancelled, o)
		if !errors.Is(err, context.Canceled) || bad.Complete || bad.Volume.Lower != nil {
			t.Fatal("cancelled composition published material bounds", i, bad.Volume, err)
		}
	}
}

func joinedSweepMomentWidths(r NormalOffsetSweepMassMoments) [4]float64 {
	n := func(q ExactParameterInterval) float64 {
		width, _ := new(big.Rat).Sub(q.Upper, q.Lower).Float64()
		return width
	}
	result := [4]float64{n(r.Volume)}
	for i := range r.Centroid {
		result[1] = max(result[1], n(r.Centroid[i]))
		for j := range r.SecondMoment[i] {
			result[2] = max(result[2], n(r.SecondMoment[i][j]))
			result[3] = max(result[3], n(r.InertiaOrigin[i][j]), n(r.InertiaCentroid[i][j]))
		}
	}
	return result
}

func joinedSweepMomentFixture(t *testing.T, sweeps [3]*VerifiedNormalOffsetSweep,
	first, second VerifiedNormalOffsetSweepJoin) (*VerifiedNormalOffsetSweepPair, *VerifiedNormalOffsetSweepChain) {
	t.Helper()
	ab := proveSweepChainPair(t, sweeps[0], sweeps[1], first)
	bc := proveSweepChainPair(t, sweeps[1], sweeps[2], second)
	chain, proof, err := NewVerifiedNormalOffsetSweepChain(context.Background(), ab, bc, SurfacePatchOptions{})
	if err != nil || !proof.Complete {
		t.Fatal("expected an analytic pair and chain on confined original material", proof, err)
	}
	return ab, chain
}

func independentPlacedJoinedSweepMoments(xMax float64) (result referenceSweepMoments) {
	lo, hi := -1.0/11, 2.0/13
	var displacement [5]float64
	for k := range displacement {
		degree := float64(k + 1)
		displacement[k] = (math.Pow(hi, degree) - math.Pow(lo, degree)) / degree
	}
	const n = 512
	dx, dy := (xMax+1)/n, 2.0/n
	r := [3][3]float64{{48, -39, 20}, {36, 52, 15}, {-25, 0, 60}}
	shift := [3]float64{1.0 / 8, -1.0 / 4, 5.0 / 16}
	for i := 0; i < n; i++ {
		// xi is the local x coordinate measured from the first wall.
		xi := -1 + dx*(float64(i)+.5)
		x := xi + 5.0/8
		fx := ((4*xi-6)*xi + 2) * xi / 512
		fxx := ((12*xi-12)*xi + 2) / 512
		for j := 0; j < n; j++ {
			y := -1 + dy*(float64(j)+.5)
			fy := y / 32
			g := 1 + fx*fx + fy*fy
			area := math.Sqrt(g)
			twoH := ((1+fy*fy)*fxx + (1+fx*fx)/32) / (g * area)
			k := fxx / (32 * g * g)
			f := xi * (xi - 1)
			local := [3]float64{x, y, f*f/512 + y*y/64}
			normal := [3]float64{-fx / area, -fy / area, 1 / area}
			world := shift
			var direction [3]float64
			for a, row := range r {
				for b, c := range row {
					world[a] += c * local[b] / 65
					direction[a] += c * normal[b] / 65
				}
			}
			var m [3]float64
			for p := range m {
				m[p] = area * (displacement[p] - twoH*displacement[p+1] + k*displacement[p+2]) * dx * dy
			}
			result.volume += m[0]
			for a := range world {
				result.first[a] += world[a]*m[0] + direction[a]*m[1]
				for b := range world {
					result.second[a][b] += world[a]*world[b]*m[0] +
						(world[a]*direction[b]+direction[a]*world[b])*m[1] + direction[a]*direction[b]*m[2]
				}
			}
		}
	}
	return result
}
