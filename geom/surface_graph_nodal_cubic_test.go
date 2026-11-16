package geom

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"testing"
)

func cubicGraphSourceFixture(t *testing.T, r, s, depth, scale *big.Rat, perturb bool) [2]*SurfacePatch {
	t.Helper()
	ctx := context.Background()
	turn := new(big.Rat).Sub(r, depth)
	power := make([]*big.Rat, 4)
	up := rootPower(t, r, 2)
	for i := range power {
		power[i] = new(big.Rat)
		if i < len(up) {
			power[i].Sub(power[i], new(big.Rat).Mul(up[i], turn))
		}
		if i > 0 {
			power[i].Add(power[i], up[i-1])
		}
	}
	vp := rootPower(t, s, 2)
	bernstein := func(power []*big.Rat, i, n int) *big.Rat {
		value := new(big.Rat)
		for j := 0; j <= i; j++ {
			weight := new(big.Rat).SetFrac(new(big.Int).Binomial(int64(i), int64(j)), new(big.Int).Binomial(int64(n), int64(j)))
			value.Add(value, new(big.Rat).Mul(power[j], weight))
		}
		return value
	}
	var result [2]*SurfacePatch
	for owner, degrees := range [2][2]int{{3, 2}, {1, 1}} {
		var net [][4]*big.Rat
		for i := 0; i <= degrees[0]; i++ {
			for j := 0; j <= degrees[1]; j++ {
				u, v := big.NewRat(int64(i), int64(degrees[0])), big.NewRat(int64(j), int64(degrees[1]))
				// Original B is bilinear. The residual on original A is
				// (1/16)*((v-s)²-k*(u-r)²*(u-r+depth)).
				z := new(big.Rat).Add(big.NewRat(2, 1), new(big.Rat).Mul(u, big.NewRat(1, 2)))
				z.Add(z, new(big.Rat).Mul(big.NewRat(1, 8), new(big.Rat).Mul(u, v)))
				if owner == 0 {
					gap := new(big.Rat).Sub(bernstein(vp, j, 2), new(big.Rat).Mul(scale, bernstein(power, i, 3)))
					if perturb && i == 1 && j == 1 {
						gap.Add(gap, big.NewRat(1, 1024))
					}
					z.Add(z, new(big.Rat).Mul(gap, big.NewRat(1, 16)))
				}
				net = append(net, [4]*big.Rat{u, v, z, big.NewRat(1, 1)})
			}
		}
		var err error
		result[owner], err = NewExactHomogeneousBezierSurface(ctx, degrees[0], degrees[1], net, SurfacePatchOptions{})
		if err != nil {
			t.Fatal(err)
		}
	}
	return result
}

func independentCubicGraphOwner(t *testing.T, p *SurfacePatch) *SurfacePatch {
	t.Helper()
	// The independently authored B chart is (2+2*y, 1-4*x) with
	// respect to the model XY coordinates, including a reversed free axis.
	out := *p
	out.net = []exactHomogeneous{p.net[2], p.net[0], p.net[3], p.net[1]}
	out.knotsU = []float64{2, 2, 4, 4}
	out.knotsV = []float64{-3, -3, 1, 1}
	b := &polynomialBudget{maxOperations: 8000000, maxBits: 16384}
	if err := out.computeBounds(b); err != nil {
		t.Fatal(err)
	}
	return &out
}

func TestDiscoverProjectedGraphNodalCubic(t *testing.T) {
	ctx := context.Background()
	for _, axes := range [][2]int{{0, 1}, {0, 2}, {2, 0}} {
		src := cubicGraphSourceFixture(t, big.NewRat(13, 16), big.NewRat(1, 2), big.NewRat(1, 16), big.NewRat(1, 1), false)
		src[1] = independentCubicGraphOwner(t, src[1])
		elevation, dependent := 2, 1
		if axes != ([2]int{0, 1}) {
			elevation, dependent = 1, 2
			for i := range src {
				var err error
				src[i], err = permuteGraphPatch(src[i], [3]int{0, 2, 1}, &polynomialBudget{maxOperations: 8000000, maxBits: 16384})
				if err != nil {
					t.Fatal(err)
				}
			}
		}
		for order := range 2 {
			t.Run(fmt.Sprintf("axes%d%d/order%d", axes[0], axes[1], order), func(t *testing.T) {
				proof, err := DiscoverProjectedGraphNodalCubic(ctx, src[order], src[1-order], axes, order, GraphPointContactOptions{})
				if err != nil {
					t.Fatal(err)
				}
				orientation := 1
				if axes[0] == 2 {
					orientation = -1
				}
				if proof.ProjectionAxes != axes || proof.ProjectionOrientation != orientation ||
					proof.FreeAxis != order || proof.SourceDirection != 1-2*order ||
					proof.Scale.RatString() != "1" || proof.LoopDepth.RatString() != "1/16" ||
					proof.Sectors != ([4]int{0, 1, 0, 2}) {
					t.Fatal("canonical equation and section incidence", proof)
				}
				wantA := [4][2]string{{"13/16", "1/2"}, {"3/4", "1/2"}, {"1", "13/32"}, {"1", "19/32"}}
				wantB := [4][2]string{{"3", "-9/4"}, {"3", "-2"}, {"45/16", "-3"}, {"51/16", "-3"}}
				wantMasks := [4][2]uint8{{}, {}, {2, 4}, {2, 4}}
				for i, v := range proof.Vertices {
					for axis := range 2 {
						if v.UV[order][axis].RatString() != wantA[i][axis] || v.UV[1-order][axis].RatString() != wantB[i][axis] ||
							v.Boundary[axis^order] != wantMasks[i][axis] {
							t.Fatal("original owner endpoint and side", i, v)
						}
					}
					if v.Point[0].RatString() != wantA[i][0] || v.Point[dependent].RatString() != wantA[i][1] {
						t.Fatal("world coordinates at original node or exit", i, v.Point)
					}
				}
				if proof.Vertices[0].Point[elevation].RatString() != "629/256" ||
					proof.Rays[0].Model[elevation].RatString() != "301/512" {
					t.Fatal("physical original contact height and tangent", proof.Vertices[0].Point, proof.Rays[0].Model)
				}
				for i, id := range [4]int{3, 1, 2, 0} {
					ray := proof.Rays[i]
					edge := proof.Edges[id]
					if ray.Edge != id || ray.Reversed != (i >= 2) || (ray.Reversed && edge.End != 0) || (!ray.Reversed && edge.Start != 0) {
						t.Fatal("unbound original ray", ray, edge)
					}
				}
				for i, want := range [3]int{1 - 2*order, 2*order - 1, 2*order - 1} {
					if proof.Regions[i].HeightSide != want {
						t.Fatal("wrong signed complementary region", i, proof.Regions)
					}
				}
				checkNodalCubicRestrictions(t, proof, [2]*SurfacePatch{src[order], src[1-order]})
			})
		}
	}
}

func checkNodalCubicRestrictions(t *testing.T, proof *NodalCubicSection, src [2]*SurfacePatch) {
	t.Helper()
	ctx := context.Background()
	times := [5]string{"-1/2", "-1/4", "0", "1/4", "1/2"}
	ends := [5]int{2, 0, 1, 0, 3}
	for i, edge := range proof.Edges {
		if edge.Start != ends[i] || edge.End != ends[i+1] || edge.Range[0].RatString() != times[i] || edge.Range[1].RatString() != times[i+1] {
			t.Fatal("loop or open arm restriction is missing", edge)
		}
		middle := new(big.Rat).Mul(big.NewRat(1, 2), new(big.Rat).Add(edge.Range[0], edge.Range[1]))
		for _, q := range []*big.Rat{edge.Range[0], middle, edge.Range[1]} {
			x := new(big.Rat).Sub(new(big.Rat).Mul(q, q), big.NewRat(1, 16))
			y := new(big.Rat).Mul(q, x)
			var world [2][3]*big.Rat
			for owner, source := range src {
				var uv [2]*big.Rat
				for j, row := range proof.ChartMaps[owner] {
					uv[j] = new(big.Rat).Add(row[2], new(big.Rat).Add(new(big.Rat).Mul(row[0], x), new(big.Rat).Mul(row[1], y)))
				}
				jet, err := source.EvaluateRational(ctx, uv[0], uv[1], SurfacePatchEvaluationOptions{})
				if err != nil {
					t.Fatal(err)
				}
				world[owner] = jet.Point
			}
			for j, p := range world[0] {
				if p.Cmp(world[1][j]) != 0 {
					t.Fatal("original pair does not share the exact restriction", i, q, world)
				}
			}
		}
	}
	// Independent area check by integrating y dx along the rail
	// x=t²-1/16, y=t*x. The closing side for the outer arms has dx=0.
	primitive := func(t *big.Rat) *big.Rat {
		t3 := new(big.Rat).Mul(t, new(big.Rat).Mul(t, t))
		t5 := new(big.Rat).Mul(t3, new(big.Rat).Mul(t, t))
		return new(big.Rat).Sub(new(big.Rat).Mul(big.NewRat(2, 5), t5), new(big.Rat).Mul(big.NewRat(1, 24), t3))
	}
	areas := [2]*big.Rat{new(big.Rat), new(big.Rat)}
	for i, edge := range proof.Edges {
		area := new(big.Rat).Sub(primitive(edge.Range[1]), primitive(edge.Range[0]))
		region := [4]int{0, 1, 1, 0}[i]
		areas[region].Add(areas[region], area)
	}
	if areas[0].RatString() != "29/1920" || areas[1].RatString() != "-1/1920" {
		t.Fatal("exact complementary projected areas", areas)
	}
}

func TestDiscoverProjectedGraphNodalCubicChangedCoefficients(t *testing.T) {
	ctx := context.Background()
	src := cubicGraphSourceFixture(t, big.NewRat(3, 4), big.NewRat(3, 8), big.NewRat(9, 64), big.NewRat(4, 1), false)
	axes := [2]int{0, 1}
	proof, err := DiscoverProjectedGraphNodalCubic(ctx, src[0], src[1], axes, 0, GraphPointContactOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if proof.Scale.RatString() != "4" || proof.LoopDepth.RatString() != "9/64" ||
		proof.Edges[0].Range[0].RatString() != "-5/4" || proof.Edges[0].Range[1].RatString() != "-3/4" ||
		proof.Vertices[2].UV[0][1].RatString() != "1/16" || proof.Vertices[3].UV[0][1].RatString() != "11/16" {
		t.Fatal("discovery used the wrong cubic, slope, or exact exits", proof)
	}
	if out, err := DiscoverProjectedGraphNodalCubic(ctx, src[0], src[1], axes, 0,
		GraphPointContactOptions{MaxOperations: proof.ExactOperations}); err != nil || out.ExactOperations != proof.ExactOperations {
		t.Fatal("recorded proof budget does not reproduce", err)
	}
	if out, err := DiscoverProjectedGraphNodalCubic(ctx, src[0], src[1], axes, 0,
		GraphPointContactOptions{MaxOperations: proof.ExactOperations - 1}); err == nil || out != nil {
		t.Fatal("truncated proof published a curve", err)
	}
}

func TestDiscoverProjectedGraphNodalCubicObliqueFootprint(t *testing.T) {
	ctx := context.Background()
	src := cubicGraphSourceFixture(t, big.NewRat(13, 16), big.NewRat(1, 2), big.NewRat(1, 16), big.NewRat(1, 1), false)
	src[1] = independentCubicGraphOwner(t, src[1])
	for _, patch := range src {
		for i, h := range patch.net {
			x := new(big.Rat).Add(new(big.Rat).Mul(big.NewRat(2, 1), h[0]), h[1])
			patch.net[i][0] = x.Add(x, h[3])
			y := new(big.Rat).Sub(h[0], h[1])
			patch.net[i][1] = y.Sub(y, h[3])
		}
		if err := patch.computeBounds(&polynomialBudget{maxOperations: 8000000, maxBits: 16384}); err != nil {
			t.Fatal(err)
		}
	}
	proof, err := DiscoverProjectedGraphNodalCubic(ctx, src[0], src[1], [2]int{0, 1}, 0, GraphPointContactOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if proof.ProjectionOrientation != -1 || proof.Vertices[0].Point[0].RatString() != "25/8" ||
		proof.Vertices[0].Point[1].RatString() != "-11/16" ||
		proof.Rays[0].Model[0].RatString() != "9/4" || proof.Rays[0].Model[1].RatString() != "3/4" {
		t.Fatal("negative determinant changes the original model node or projected orientation", proof.Vertices[0], proof.Rays[0])
	}
	checkNodalCubicRestrictions(t, proof, src)
}

func TestDiscoverProjectedGraphNodalCubicExactNonbinaryVertices(t *testing.T) {
	src := cubicGraphSourceFixture(t, big.NewRat(2, 3), big.NewRat(1, 2), big.NewRat(1, 9), big.NewRat(1, 1), false)
	proof, err := DiscoverProjectedGraphNodalCubic(context.Background(), src[0], src[1], [2]int{0, 1}, 0, GraphPointContactOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if proof.Vertices[0].Point[0].RatString() != "2/3" || proof.Vertices[1].Point[0].RatString() != "5/9" ||
		proof.Vertices[2].Point[1].RatString() != "5/18" || proof.Vertices[3].Point[1].RatString() != "13/18" ||
		proof.Edges[0].Range[0].RatString() != "-2/3" || proof.Edges[0].Range[1].RatString() != "-1/3" {
		t.Fatal("exact nonbinary events were rounded or discarded", proof.Vertices, proof.Edges)
	}
	if _, err := RepresentedExactPoint(proof.Vertices[0].Point); !errors.Is(err, ErrUnrepresentable) {
		t.Fatal("exact chart event falsely claims a represented native vertex", err)
	}
}

func TestDiscoverProjectedGraphNodalCubicCoefficientBudget(t *testing.T) {
	ctx := context.Background()
	depth := new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Lsh(big.NewInt(1), 160))
	center := new(big.Rat).Add(big.NewRat(3, 4), depth)
	src := cubicGraphSourceFixture(t, center, big.NewRat(1, 2), depth, big.NewRat(1, 1), false)
	axes := [2]int{0, 1}
	proof, err := DiscoverProjectedGraphNodalCubic(ctx, src[0], src[1], axes, 0, GraphPointContactOptions{})
	if err != nil || proof.LoopDepth.Cmp(depth) != 0 || proof.Vertices[0].Point[0].Cmp(proof.Vertices[1].Point[0]) == 0 {
		t.Fatal("exact small loop lost distinct node and turning point", err)
	}
	if out, err := DiscoverProjectedGraphNodalCubic(ctx, src[0], src[1], axes, 0,
		GraphPointContactOptions{MaxCoefficientBits: 64}); err == nil || out != nil {
		t.Fatal("coefficient budget publishes a partial curve", err)
	}
}

func TestDiscoverProjectedGraphNodalCubicRefusals(t *testing.T) {
	ctx := context.Background()
	axes := [2]int{0, 1}
	for _, tc := range []struct {
		name    string
		r, s, a *big.Rat
		perturb bool
		err     error
	}{
		{"true cusp", big.NewRat(13, 16), big.NewRat(1, 2), new(big.Rat), false, ErrDomain},
		{"isolated real node", big.NewRat(13, 16), big.NewRat(1, 2), big.NewRat(-1, 16), false, ErrDomain},
		{"loop meets near side", big.NewRat(1, 16), big.NewRat(1, 2), big.NewRat(1, 16), false, ErrDomain},
		{"loop clipped by transverse side", big.NewRat(13, 16), big.NewRat(1, 1024), big.NewRat(1, 16), false, ErrDomain},
		{"exit on transverse side", big.NewRat(13, 16), big.NewRat(1, 16), big.NewRat(1, 16), false, ErrDomain},
		{"exit at corner", big.NewRat(13, 16), big.NewRat(3, 32), big.NewRat(1, 16), false, ErrDomain},
		{"irrational node slope", big.NewRat(13, 16), big.NewRat(1, 2), big.NewRat(1, 32), false, ErrUnrepresentable},
		{"irrational terminal", big.NewRat(3, 4), big.NewRat(1, 2), big.NewRat(1, 16), false, ErrUnrepresentable},
		{"coupled height perturbation", big.NewRat(13, 16), big.NewRat(1, 2), big.NewRat(1, 16), true, ErrConvergence},
	} {
		t.Run(tc.name, func(t *testing.T) {
			src := cubicGraphSourceFixture(t, tc.r, tc.s, tc.a, big.NewRat(1, 1), tc.perturb)
			out, err := DiscoverProjectedGraphNodalCubic(ctx, src[0], src[1], axes, 0, GraphPointContactOptions{})
			if !errors.Is(err, tc.err) || out != nil {
				t.Fatal("unsupported source publishes an arrangement", out, err, tc.err)
			}
		})
	}

	src := cubicGraphSourceFixture(t, big.NewRat(13, 16), big.NewRat(1, 2), big.NewRat(1, 16), big.NewRat(1, 1), false)
	for _, tc := range []struct {
		axes [2]int
		free int
	}{{[2]int{0, 0}, 0}, {axes, -1}, {axes, 1}, {[2]int{0, 2}, 0}} {
		if out, err := DiscoverProjectedGraphNodalCubic(ctx, src[0], src[1], tc.axes, tc.free, GraphPointContactOptions{}); err == nil || out != nil {
			t.Fatal("wrong chart publishes an arrangement", tc, err)
		}
	}
	shifted := *src[1]
	shifted.net = append([]exactHomogeneous(nil), shifted.net...)
	for i, p := range shifted.net {
		shifted.net[i][0] = new(big.Rat).Add(p[0], big.NewRat(1, 16))
	}
	if out, err := DiscoverProjectedGraphNodalCubic(ctx, src[0], &shifted, axes, 0, GraphPointContactOptions{}); !errors.Is(err, ErrDomain) || out != nil {
		t.Fatal("different footprints publish an arrangement", err)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if out, err := DiscoverProjectedGraphNodalCubic(cancelled, src[0], src[1], axes, 0, GraphPointContactOptions{}); !errors.Is(err, context.Canceled) || out != nil {
		t.Fatal("cancelled source publishes an arrangement", err)
	}
}
