package trimmed

import (
	"context"
	"math"
	"math/big"
	"plainsolid/brep/edgecheck"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"plainsolid/intersect"
	"testing"
)

func nonunitRadialShells(t *testing.T) (Solid, Solid, *geom.BSplineSurface, *geom.BSplineSurface) {
	t.Helper()
	ctx := context.Background()
	build := func(stock bool) (Solid, *geom.BSplineSurface) {
		var surfaces []*geom.BSplineSurface
		var rev []bool
		add := func(nu, nv int, reversed bool, f func(int, int) geom.Vec3) {
			var poles []geom.Vec3
			for i := 0; i < nu; i++ {
				for j := 0; j < nv; j++ {
					poles = append(poles, f(i, j))
				}
			}
			knots := func(n int) []float64 {
				k := make([]float64, 2*n)
				for i := n; i < len(k); i++ {
					k[i] = 1
				}
				return k
			}
			s, e := geom.NewBSplineSurface(nu-1, nv-1, nu, nv, poles, knots(nu), knots(nv), nil)
			if e != nil {
				t.Fatal(e)
			}
			surfaces = append(surfaces, s)
			rev = append(rev, reversed)
		}
		n := 5
		x0, xw, y0, yw, floor := 2., 2., -1., 3., -256.
		if stock {
			n = 2
			x0, xw, y0, yw, floor = 1.5, 3, -1.5, 4, 0
		}
		var height [5][5]float64
		if !stock {
			x2 := [5]int64{3, 0, -1, 0, 3}
			x4 := [5]int64{1, -1, 1, -1, 1}
			sum := big.NewRat(125, 1024)
			product := big.NewRat(625, 262144)
			for i := 0; i < 5; i++ {
				for j := 0; j < 5; j++ {
					// -3*[X^4+Y^4+2X²Y²-(ri²+ro²)(X²+Y²)+ri²ro²].
					z := new(big.Rat).Add(big.NewRat(x4[i], 1), big.NewRat(81*x4[j], 16))
					z.Add(z, big.NewRat(x2[i]*x2[j], 2))
					q := new(big.Rat).Add(big.NewRat(x2[i], 3), big.NewRat(3*x2[j], 4))
					z.Sub(z, new(big.Rat).Mul(sum, q))
					z.Add(z, product)
					z.Mul(z, big.NewRat(-3, 1))
					var e error
					height[i][j], e = geom.RepresentedExactScalar(z)
					if e != nil {
						t.Fatal("source controls not exact", i, j, e)
					}
				}
			}
		}
		atX := func(i int) float64 { return x0 + xw*float64(i)/float64(n-1) }
		atY := func(i int) float64 { return y0 + yw*float64(i)/float64(n-1) }
		z := func(i, j int) float64 {
			if stock {
				return 1
			}
			return height[i][j]
		}
		add(2, 2, true, func(i, j int) geom.Vec3 { return geom.Vec3{X: x0 + xw*float64(i), Y: y0 + yw*float64(j), Z: floor} })
		add(n, n, false, func(i, j int) geom.Vec3 { return geom.Vec3{X: atX(i), Y: atY(j), Z: z(i, j)} })
		for _, side := range []int{0, n - 1} {
			add(n, 2, side == 0, func(i, j int) geom.Vec3 {
				return geom.Vec3{X: atX(side), Y: atY(i), Z: []float64{floor, z(side, i)}[j]}
			})
		}
		for _, side := range []int{0, n - 1} {
			add(n, 2, side != 0, func(i, j int) geom.Vec3 {
				return geom.Vec3{X: atX(i), Y: atY(side), Z: []float64{floor, z(i, side)}[j]}
			})
		}
		solid, _, e := SewSplineRectangles(ctx, surfaces, make([]geom.ParameterRectangle, len(surfaces)), rev, SewOptions{Tolerance: 1e-9})
		if e != nil {
			t.Fatal(e)
		}
		if stock {
			return solid, surfaces[0]
		}
		return solid, surfaces[1]
	}
	a, pa := build(true)
	b, pb := build(false)
	return a, b, pa, pb
}

func nonunitRadialTopologySeeds(t *testing.T, a, b *geom.BSplineSurface) intersect.SplineSeedReport {
	t.Helper()
	ctx := context.Background()
	pa, e := geom.NewSurfacePatch(a, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	pb, e := geom.NewSurfacePatch(b, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	center := [2]*big.Rat{big.NewRat(3, 1), big.NewRat(1, 2)}
	radii := []*big.Rat{big.NewRat(5, 32), big.NewRat(5, 16)}
	transport, e := geom.CertifyAffineRadialGraphContours(ctx, pa, pb, center, radii, geom.GraphPointContactOptions{})
	if e != nil {
		t.Fatal(e)
	}
	var seeds intersect.SplineSeedReport
	for _, radius := range []float64{5. / 32, 5. / 16} {
		for axis := 0; axis < 2; axis++ {
			for _, sign := range []float64{-1, 1} {
				limit := radius * .6
				if axis == 1 {
					limit = radius * .8
				}
				var add func(float64, float64, int)
				add = func(lo, hi float64, depth int) {
					near, far := math.Min(math.Abs(lo), math.Abs(hi)), math.Max(math.Abs(lo), math.Abs(hi))
					if lo <= 0 && hi >= 0 {
						near = 0
					}
					low, high := math.Sqrt(radius*radius-far*far), math.Sqrt(radius*radius-near*near)
					if sign < 0 {
						low, high = -high, -low
					}
					pad := radius / 64
					physical := geom.ParameterRectangle{UMin: 3 + lo, UMax: 3 + hi, VMin: .5 + low - pad, VMax: .5 + high + pad}
					if axis == 1 {
						physical = geom.ParameterRectangle{UMin: 3 + low - pad, UMax: 3 + high + pad, VMin: .5 + lo, VMax: .5 + hi}
					}
					freeOwner := 1
					if axis == 1 {
						freeOwner = 0
					}
					var boxes [2]geom.ParameterRectangle
					for owner := 0; owner < 2; owner++ {
						box := physical
						if owner != freeOwner {
							if axis == 0 {
								box.UMin -= pad
								box.UMax += pad
							} else {
								box.VMin -= pad
								box.VMax += pad
							}
						}
						var err error
						boxes[owner], _, _, err = transport.OriginalBox(ctx, owner, box, geom.SurfacePatchOptions{})
						if err != nil {
							t.Fatal(err)
						}
					}
					// Free physical X uses roof U=(X-2)/2; free Y uses stock V=(Y+3/2)/4.
					// Both exact dyadic endpoints are retained unchanged, not outward-rounded.
					expected := []float64{(3 + lo - 2) / 2, (3 + hi - 2) / 2}
					actual := []float64{boxes[1].UMin, boxes[1].UMax}
					freeAxis := 2
					if axis == 1 {
						expected = []float64{(.5 + lo + 1.5) / 4, (.5 + hi + 1.5) / 4}
						actual = []float64{boxes[0].VMin, boxes[0].VMax}
						freeAxis = 1
					}
					if actual[0] != expected[0] || actual[1] != expected[1] {
						t.Fatal("nonexact free chart station")
					}
					xa, err := pa.Trim(boxes[0])
					if err != nil {
						t.Fatal(err)
					}
					xb, err := pb.Trim(boxes[1])
					if err != nil {
						t.Fatal(err)
					}
					cert, err := geom.CertifySurfacePairTube(xa, xb, geom.SurfacePairTubeOptions{FreeAxis: freeAxis, MaxOperations: 8000000})
					if err != nil || !cert.ProvenBranch {
						if depth >= 8 {
							t.Fatal("original tube", axis, lo, hi, err)
						}
						mid := (lo + hi) / 2
						add(lo, mid, depth+1)
						add(mid, hi, depth+1)
						return
					}
					seeds.Attempts = append(seeds.Attempts, intersect.SplineSeedCell{Cell: geom.SurfaceSurfaceCell{DomainA: boxes[0], DomainB: boxes[1]}, Tube: &cert})
				}
				for part := 0; part < 8; part++ {
					add(-limit+2*limit*float64(part)/8, -limit+2*limit*float64(part+1)/8, 0)
				}
			}
		}
	}
	return seeds
}

func TestNativeNonunitRadialAnnulus(t *testing.T) {
	if testing.Short() {
		t.Skip("complete non-unit two-circle native annulus")
	}
	ctx := context.Background()
	a, b, pa, pb := nonunitRadialShells(t)
	roof, e := geom.NewSurfacePatch(pb, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	jet, e := roof.EvaluateRational(ctx, big.NewRat(79, 128), big.NewRat(1, 2), geom.SurfacePatchEvaluationOptions{})
	if e != nil || jet.Point[2].Cmp(big.NewRat(65625, 16777216)) != 0 {
		t.Fatal("independent physical annulus height", jet.Point, e)
	}
	for _, u := range []*big.Rat{big.NewRat(1, 2), big.NewRat(11, 16)} {
		jet, e := roof.EvaluateRational(ctx, u, big.NewRat(1, 2), geom.SurfacePatchEvaluationOptions{})
		if e != nil || jet.Point[2].Sign() >= 0 {
			t.Fatal("hole/exterior height", jet.Point, e)
		}
	}
	seeds := nonunitRadialTopologySeeds(t, pa, pb)
	t.Log("original ordered tubes", len(seeds.Attempts))
	o := AlgebraicShellSectionOptions{Seeds: []AlgebraicShellSectionSeed{{Faces: [2]int{0, 1}, Tubes: seeds}}}
	o.Trace.Coverage.RadialOverlap = &intersect.RadialOverlapCoverageOptions{AffineCharts: true, Center: [2]*big.Rat{big.NewRat(3, 1), big.NewRat(1, 2)}, Radii: []*big.Rat{big.NewRat(5, 32), big.NewRat(5, 16)}, Proof: geom.RadialTubeCoverageOptions{MaxOperations: 128000000, MaxTubes: 512}}
	o.Trace.Coverage.Cell.MaxOperations = 16000000
	o.Trace.Branches.PruneSeparatedEndpoints = true
	o.Trace.Branches.MaxComparisons = 4096
	o.Trace.MaxTopologyPairs = 1000000
	section, e := SectionSplineShellsAlgebraic(ctx, a, b, o)
	if e != nil {
		t.Fatal("section", e)
	}
	result, e := section.CombineAlgebraic(ctx, parametric.Intersection, AlgebraicShellBooleanOptions{})
	if e != nil {
		t.Fatal("material", e)
	}
	if len(result.faces) != 2 || result.Topology.InnerWires != 2 || result.Topology.EulerCharacteristic != 0 {
		t.Fatal(result.Topology)
	}
	native, e := result.NativeTubeRegionComponents(ctx, 1e-7, edgecheck.Options{})
	if e != nil || len(native) != 1 {
		t.Fatal("native", len(native), e)
	}
	n := native[0]
	if n.Topology().EulerCharacteristic != 0 || n.Topology().InnerWires != 2 || len(n.Edges()) != len(seeds.Attempts) {
		t.Fatal(n.Topology(), len(n.Edges()))
	}
	for _, f := range n.Faces() {
		if f.Surface != a.faces[0].Surface && f.Surface != b.faces[1].Surface {
			t.Fatal("original support owner")
		}
	}
	counts := make([]int, len(n.Edges()))
	directions := make([]int, len(n.Edges()))
	for _, f := range n.Faces() {
		for _, loop := range append([][]Use{f.Outer}, f.Holes...) {
			for _, u := range loop {
				counts[u.Edge]++
				sign := 1
				if u.Reversed != f.Reversed {
					sign = -1
				}
				directions[u.Edge] += sign
			}
		}
	}
	for i := range counts {
		if counts[i] != 2 || directions[i] != 0 {
			t.Fatal("original wire incidence", i, counts[i], directions[i])
		}
	}
	bad := result
	bad.faces = append([]AlgebraicShellBooleanFace(nil), result.faces...)
	bad.faces[0].Holes = nil
	if got, e := bad.NativeTubeRegionComponents(ctx, 1e-7, edgecheck.Options{}); e == nil || got != nil {
		t.Fatal("missing original inner wire", len(got), e)
	}
	t.Log("native annulus: Euler", n.Topology().EulerCharacteristic, "inner wires", n.Topology().InnerWires, "original edges", len(n.Edges()))
}

func TestNonunitRadialExactTransfer(t *testing.T) {
	if testing.Short() {
		t.Skip("two-circle original exact transfer")
	}
	ctx := context.Background()
	_, _, a, b := nonunitRadialShells(t)
	seeds := nonunitRadialTopologySeeds(t, a, b)
	pa, _ := geom.NewSurfacePatch(a, geom.SurfacePatchOptions{})
	pb, _ := geom.NewSurfacePatch(b, geom.SurfacePatchOptions{})
	proposals, e := intersect.DiscoverOverlappingAffineRadialGraphContourTubes(ctx, intersect.Patch{Surface: a, Max: geom.Vec2{X: 1, Y: 1}}, intersect.Patch{Surface: b, Max: geom.Vec2{X: 1, Y: 1}}, [2]*big.Rat{big.NewRat(3, 1), big.NewRat(1, 2)}, []*big.Rat{big.NewRat(5, 32), big.NewRat(5, 16)}, geom.SurfacePatchOptions{}, geom.RadialTubeCoverageOptions{MaxOperations: 128000000, MaxTubes: 512})
	if e != nil {
		t.Fatal(e)
	}
	transport, e := geom.CertifyAffineRadialGraphContours(ctx, pa, pb, [2]*big.Rat{big.NewRat(3, 1), big.NewRat(1, 2)}, []*big.Rat{big.NewRat(5, 32), big.NewRat(5, 16)}, geom.GraphPointContactOptions{})
	if e != nil {
		t.Fatal(e)
	}
	overlap := func(a, b geom.ParameterRectangle) bool {
		return a.UMin <= b.UMax && b.UMin <= a.UMax && a.VMin <= b.VMax && b.VMin <= a.VMax
	}
	for i, s := range proposals.Attempts {
		var targets []geom.SurfacePairTubeRegion
		for _, x := range seeds.Attempts {
			if overlap(s.Cell.DomainA, x.Cell.DomainA) && overlap(s.Cell.DomainB, x.Cell.DomainB) {
				targets = append(targets, geom.SurfacePairTubeRegion{Box: geom.SurfacePairParameterBox{A: x.Cell.DomainA, B: x.Cell.DomainB}, FreeAxis: x.Tube.FreeAxis})
			}
		}
		r, e := transport.CertifyCellInTubes(ctx, geom.SurfacePairParameterBox{A: s.Cell.DomainA, B: s.Cell.DomainB}, targets, geom.SurfaceTubeUnionCoverageOptions{MaxOperations: 16000000})
		if e != nil || !r.Covered {
			t.Fatalf("candidate %d/%d targets%d ops%d cells%d pieces%d unresolved%d pending%d: %v; box%+v", i, len(proposals.Attempts), len(targets), r.ExactOperations, r.VisitedCells, r.PartitionPieces, len(r.Unresolved), len(r.Pending), e, s.Cell)
		}
		if i == 0 {
			for _, options := range []geom.SurfaceTubeUnionCoverageOptions{{MaxOperations: r.ExactOperations - 1}, {MaxPieces: r.PartitionPieces - 1}, {MaxCells: 1}} {
				bad, e := transport.CertifyCellInTubes(ctx, geom.SurfacePairParameterBox{A: s.Cell.DomainA, B: s.Cell.DomainB}, targets, options)
				if e == nil || bad.Covered {
					t.Fatal("transfer quota", options, e)
				}
			}
		}
	}
}
