package trimmed

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"testing"

	"plainsolid/geom"
)

func fourCrossingParabolaCircleStock(t *testing.T, axis int, reflected, perturb bool) [3]*geom.BSplineSurface {
	t.Helper()
	vertex, curvature := big.NewRat(327, 2048), big.NewRat(2, 1)
	if reflected {
		vertex.Sub(big.NewRat(1, 1), vertex)
		curvature.Neg(curvature)
	}
	return parabolaCircleCurvatureStock(t, axis, big.NewRat(1, 2), vertex, big.NewRat(7225, 65536), curvature, perturb)
}

func TestParabolaCircleFourCrossingsSource(t *testing.T) {
	ctx := context.Background()
	for layout := range 2 {
		reflected := layout == 1
		for axis := range 2 {
			stocks := fourCrossingParabolaCircleStock(t, axis, reflected, false)
			for order := range 2 {
				t.Run(fmt.Sprintf("%d/%d/%d", layout, axis, order), func(t *testing.T) {
					roofs := parabolaTwoCirclesPatches(t, stocks[1+order], stocks[2-order])
					p, err := geom.DiscoverSplineGraphParabolaCircleFourCrossings(ctx, roofs[0], roofs[1], axis, geom.GraphPointContactOptions{})
					if err != nil {
						t.Fatal(err)
					}
					side, sign := 2*layout-1, (1-2*order)*(1-2*layout)
					if p.FreeAxis != axis || p.CircleSide != side || p.RegionSides != [6]int{sign, -sign, -sign, -sign, -sign, sign} ||
						p.Circle.Center[0].Cmp(big.NewRat(1, 2)) != 0 || p.Circle.Center[1].Cmp(big.NewRat(1, 2)) != 0 ||
						p.Circle.RadiusSquared.Cmp(big.NewRat(7225, 65536)) != 0 || p.CircleRadius.Cmp(big.NewRat(85, 256)) != 0 ||
						p.CircleProjection[0].Cmp(big.NewRat(43, 256)) != 0 || p.CircleProjection[1].Cmp(big.NewRat(213, 256)) != 0 {
						t.Fatal("changed original circular factor or six signed regions", p)
					}
					checkFourCrossingCircleJunctions(t, p, reflected)
					for i, uv := range p.RegionWitnesses {
						if fourCrossingCircleRegion(uv, axis, reflected) != i {
							t.Fatal("wrong exact region witness", i, uv)
						}
						a, err := roofs[0].EvaluateRational(ctx, uv[0], uv[1], geom.SurfacePatchEvaluationOptions{})
						if err != nil {
							t.Fatal(err)
						}
						b, err := roofs[1].EvaluateRational(ctx, uv[0], uv[1], geom.SurfacePatchEvaluationOptions{})
						if err != nil || a.Point[2].Cmp(b.Point[2]) != p.RegionSides[i] {
							t.Fatal("wrong original roof order", i, err)
						}
					}
					checkFourCrossingCircleMeasures(t, stocks, p.RegionSides, sign, order)
					legacy, err := geom.DiscoverSplineGraphParabolaCircleCrossings(ctx, roofs[0], roofs[1], axis, geom.GraphPointContactOptions{})
					if legacy != nil || !errors.Is(err, geom.ErrConvergence) {
						t.Fatal("changed older two-crossing profile", legacy, err)
					}
				})
			}
		}
	}
}

func fourCrossingCircleRegion(uv [2]*big.Rat, axis int, reflected bool) int {
	x, y := uv[axis], new(big.Rat).Set(uv[1-axis])
	if reflected {
		y.Sub(big.NewRat(1, 1), y)
	}
	dx, dy := new(big.Rat).Sub(x, big.NewRat(1, 2)), new(big.Rat).Sub(y, big.NewRat(1, 2))
	x2, y2 := new(big.Rat).Mul(dx, dx), new(big.Rat).Mul(dy, dy)
	p := new(big.Rat).Add(big.NewRat(327, 2048), new(big.Rat).Mul(x2, big.NewRat(2, 1)))
	near := y.Cmp(p) < 0
	if new(big.Rat).Add(x2, y2).Cmp(big.NewRat(7225, 65536)) < 0 {
		if !near {
			return 5
		}
		if dx.Sign() < 0 {
			return 3
		}
		return 4
	}
	if near {
		return 0
	}
	if dy.Sign() < 0 && x.Cmp(big.NewRat(23, 64)) > 0 && x.Cmp(big.NewRat(41, 64)) < 0 {
		return 2
	}
	return 1
}

func checkFourCrossingCircleJunctions(t *testing.T, p *geom.SplineGraphParabolaCircleFourCrossings, reflected bool) {
	t.Helper()
	axis := p.FreeAxis
	for i, uv := range p.ParabolaControls {
		y := big.NewRat(1351, 2048)
		if i == 1 {
			y.Sub(y, big.NewRat(1, 1))
		}
		if reflected {
			y.Sub(big.NewRat(1, 1), y)
		}
		if uv[axis].Cmp(big.NewRat(int64(i), 2)) != 0 || uv[1-axis].Cmp(y) != 0 {
			t.Fatal("changed original parabola controls", i, uv)
		}
		if i != 1 {
			exit := p.ParabolaExits[i/2]
			if exit[axis].Cmp(uv[axis]) != 0 || exit[1-axis].Cmp(y) != 0 || exit[2].Cmp(big.NewRat(2, 1)) != 0 {
				t.Fatal("changed original stock exit", exit)
			}
		}
	}
	for i, node := range p.Nodes {
		x := big.NewRat([4]int64{15, 23, 41, 49}[i], 64)
		y := big.NewRat([4]int64{77, 51, 51, 77}[i], 256)
		slope := new(big.Rat).Sub(new(big.Rat).Mul(big.NewRat(4, 1), x), big.NewRat(2, 1))
		if reflected {
			y.Sub(big.NewRat(1, 1), y)
			slope.Neg(slope)
		}
		if node.Parameters[axis].Cmp(x) != 0 || node.Parameters[1-axis].Cmp(y) != 0 ||
			node.Point[axis].Cmp(x) != 0 || node.Point[1-axis].Cmp(y) != 0 || node.Point[2].Cmp(big.NewRat(2, 1)) != 0 ||
			node.Rays[0][axis].Cmp(big.NewRat(1, 1)) != 0 || node.Rays[0][1-axis].Cmp(slope) != 0 {
			t.Fatal("changed exact original junction or parabola ray", i, node)
		}
		dx := new(big.Rat).Sub(node.Parameters[0], big.NewRat(1, 2))
		dy := new(big.Rat).Sub(big.NewRat(1, 2), node.Parameters[1])
		circle := node.Rays[p.CircleCCWRays[i]]
		if circle[0].Cmp(dy) != 0 || circle[1].Cmp(dx) != 0 {
			t.Fatal("changed original circle tangent", i, node)
		}
		var sectors [6]bool
		for k, a := range node.Rays {
			b := node.Rays[(k+1)%4]
			cross := new(big.Rat).Sub(new(big.Rat).Mul(a[0], b[1]), new(big.Rat).Mul(a[1], b[0]))
			if cross.Sign() <= 0 || a[2].Sign() != 0 {
				t.Fatal("invalid physical ray cycle", i, k, node)
			}
			var witness [2]*big.Rat
			for j := range witness {
				step := new(big.Rat).Quo(new(big.Rat).Add(a[j], b[j]), big.NewRat(1_000_000, 1))
				witness[j] = new(big.Rat).Add(node.Parameters[j], step)
			}
			region := fourCrossingCircleRegion(witness, axis, reflected)
			if region != node.SectorRegions[k] || sectors[region] || p.RegionSides[region] == p.RegionSides[node.SectorRegions[(k+1)%4]] {
				t.Fatal("lost an original signed sector", i, k, node)
			}
			sectors[region] = true
		}
		far := 1
		if i == 1 || i == 2 {
			far = 2
		}
		if !sectors[0] || !sectors[far] || !sectors[3+i/2] || !sectors[5] {
			t.Fatal("wrong pocket incidence at original junction", i, node)
		}
	}
}

func TestParabolaCircleFourCrossingsSourceRefusals(t *testing.T) {
	ctx := context.Background()
	good := fourCrossingParabolaCircleStock(t, 0, false, false)
	for _, tc := range []struct {
		name    string
		stock   [3]*geom.BSplineSurface
		axis    int
		options geom.GraphPointContactOptions
		want    error
	}{
		{name: "two simple roots", stock: parabolaCircleCrossingsStock(t, 0), want: geom.ErrConvergence},
		{name: "double root with two simple roots", stock: parabolaCircleCurvatureStock(t, 0, big.NewRat(1, 2), big.NewRat(3, 16),
			big.NewRat(25, 256), big.NewRat(2, 1), false), want: geom.ErrDegenerate},
		{name: "four irrational roots", stock: parabolaCircleCurvatureStock(t, 0, big.NewRat(1, 2), big.NewRat(327, 2048),
			big.NewRat(2721*2721, 8192*8192), big.NewRat(2, 1), false), want: geom.ErrUnrepresentable},
		{name: "circle reaches stock walls", stock: parabolaCircleCurvatureStock(t, 0, big.NewRat(1, 2), big.NewRat(327, 2048),
			big.NewRat(1, 4), big.NewRat(2, 1), false), want: geom.ErrConvergence},
		{name: "changed original pole", stock: fourCrossingParabolaCircleStock(t, 0, false, true), want: geom.ErrConvergence},
		{name: "wrong chart", stock: good, axis: 1, want: geom.ErrConvergence},
		{name: "short node work", stock: good, options: geom.GraphPointContactOptions{MaxCells: 1}, want: geom.ErrConvergence},
		{name: "short exact work", stock: good, options: geom.GraphPointContactOptions{MaxOperations: 1}, want: geom.ErrConvergence},
	} {
		t.Run(tc.name, func(t *testing.T) {
			roofs := parabolaTwoCirclesPatches(t, tc.stock[1], tc.stock[2])
			p, err := geom.DiscoverSplineGraphParabolaCircleFourCrossings(ctx, roofs[0], roofs[1], tc.axis, tc.options)
			if p != nil || !errors.Is(err, tc.want) {
				t.Fatal("published unsupported original arrangement", p, err, tc.want)
			}
		})
	}
}

// Independent Green integrals along the original quadratic parabola and
// circular arcs. Each row gives coefficients of 1, Pi, atan(77/36),
// and atan(3/4). The angle intervals follow from Machin's formula and the
// alternating rational arctangent series.
func checkFourCrossingCircleMeasures(t *testing.T, stocks [3]*geom.BSplineSurface, sides [6]int, sign, order int) {
	t.Helper()
	areas := [6][4]string{
		{"3109/8192", "0", "-7225/65536", "7225/65536"},
		{"18703/32768", "-7225/131072", "0", "-7225/65536"},
		{"1629/32768", "-7225/131072", "7225/65536", "0"},
		{"-1307/49152", "0", "7225/131072", "-7225/131072"},
		{"-1307/49152", "0", "7225/131072", "-7225/131072"},
		{"1307/24576", "7225/65536", "-7225/65536", "7225/65536"},
	}
	volumes := [6][4]string{
		{"-26804408946209/3940649673949184", "0", "9347096113125/9007199254740992", "-9347096113125/9007199254740992"},
		{"513666796345967/78812993478983680", "9347096113125/18014398509481984", "0", "9347096113125/9007199254740992"},
		{"-7153651033695/15762598695796736", "9347096113125/18014398509481984", "-9347096113125/9007199254740992", "0"},
		{"12021671951207/47287796087390208", "0", "-9347096113125/18014398509481984", "9347096113125/18014398509481984"},
		{"12021671951207/47287796087390208", "0", "-9347096113125/18014398509481984", "9347096113125/18014398509481984"},
		{"-12021671951207/23643898043695104", "-9347096113125/9007199254740992", "9347096113125/9007199254740992", "-9347096113125/9007199254740992"},
	}
	checkParabolaCircleGreenMeasures(t, stocks, sides[:], areas[:], volumes[:], big.NewRat(int64(-sign), 1), order)
}

func checkParabolaCircleGreenMeasures(t *testing.T, stocks [3]*geom.BSplineSurface, sides []int,
	areas, volumes [][4]string, orientation *big.Rat, order int) {
	t.Helper()
	parse := func(r string) *big.Rat {
		q, ok := new(big.Rat).SetString(r)
		if !ok {
			t.Fatal("bad independent exact coefficient", r)
		}
		return q
	}
	angles := [4][2]*big.Rat{
		{big.NewRat(1, 1), big.NewRat(1, 1)},
		{parse("3.141592653589793238462643383279"), parse("3.141592653589793238462643383280")},
		{parse("1.133458435047012695146974191139"), parse("1.133458435047012695146974191140")},
		{parse("0.643501108793284386802809228717"), parse("0.643501108793284386802809228718")},
	}
	var areaSum, volSum [4]big.Rat
	for i, side := range sides {
		areaBound, signedBound := new(big.Rat), new(big.Rat)
		for j, angle := range angles {
			a := parse(areas[i][j])
			v := new(big.Rat).Mul(parse(volumes[i][j]), orientation)
			areaSum[j].Add(&areaSum[j], a)
			volSum[j].Add(&volSum[j], v)
			v.Mul(v, big.NewRat(int64(side), 1))
			for _, term := range [2]struct {
				coefficient *big.Rat
				bound       *big.Rat
			}{{a, areaBound}, {v, signedBound}} {
				end := 0
				if term.coefficient.Sign() < 0 {
					end = 1
				}
				term.bound.Add(term.bound, new(big.Rat).Mul(term.coefficient, angle[end]))
			}
		}
		if areaBound.Sign() <= 0 || signedBound.Sign() <= 0 {
			t.Fatal("independent region integral disagrees with exact source sign", i, areaBound, signedBound)
		}
	}
	for j := 1; j < len(angles); j++ {
		if areaSum[j].Sign() != 0 || volSum[j].Sign() != 0 {
			t.Fatal("coverage should cancel every circular angle", j)
		}
	}
	// Integrating either unweighted tensor stock independently gives
	// the mean of its original binary64-exact Bernstein poles.
	actual := new(big.Rat)
	other := stocks[2-order].Data().Poles
	for i, pole := range stocks[1+order].Data().Poles {
		a, b := new(big.Rat).SetFloat64(pole.Z), new(big.Rat).SetFloat64(other[i].Z)
		actual.Add(actual, a.Sub(a, b))
	}
	actual.Quo(actual, big.NewRat(int64(len(other)), 1))
	if areaSum[0].Cmp(big.NewRat(1, 1)) != 0 || volSum[0].Cmp(actual) != 0 {
		t.Fatal("independent integrals leave a coverage gap", &areaSum[0], &volSum[0], actual)
	}
}
