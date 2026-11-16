package trimmed

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"testing"

	"plainsolid/geom"
)

func allCrossingParabolaCircles(reflected bool) [3]twoCircleStockProbe {
	if reflected {
		return [3]twoCircleStockProbe{twoCircleProbe(24, 87, 16, 128), twoCircleProbe(16, 23, 4, 32), twoCircleProbe(104, 87, 16, 128)}
	}
	return [3]twoCircleStockProbe{twoCircleProbe(24, 41, 16, 128), twoCircleProbe(16, 9, 4, 32), twoCircleProbe(104, 41, 16, 128)}
}

func allCrossingParabolaStocks(t *testing.T, axis int, circles [3]twoCircleStockProbe, reflected, perturb bool) [3]*geom.BSplineSurface {
	t.Helper()
	offset, orientation, divisor := int64(3), int64(1), int64(64)
	if reflected {
		offset, orientation, divisor = 1, -1, 128
	}
	return parabolaCircleProductStock(t, axis, circles[:],
		[3]*big.Rat{big.NewRat(offset, 4), big.NewRat(-2*orientation, 1), big.NewRat(2*orientation, 1)}, big.NewRat(1225, divisor), perturb)
}

func TestParabolaThreeCrossingCirclesSource(t *testing.T) {
	ctx := context.Background()
	for _, reflected := range []bool{false, true} {
		circles := allCrossingParabolaCircles(reflected)
		for axis := range 2 {
			stocks := allCrossingParabolaStocks(t, axis, circles, reflected, false)
			for order := range 2 {
				t.Run(fmt.Sprintf("%t/%d/%d", reflected, axis, order), func(t *testing.T) {
					src := parabolaTwoCirclesPatches(t, stocks[1+order], stocks[2-order])
					got, err := geom.DiscoverSplineGraphParabolaThreeCrossingCircles(ctx, src[0], src[1], axis, geom.GraphPointContactOptions{})
					if err != nil {
						t.Fatal(err)
					}
					sign := 1 - 2*order
					want := [8]int{sign, -sign, -sign, sign, -sign, sign, -sign, sign}
					if got.FreeAxis != axis || got.CircleOrder != [3]int{0, 1, 2} || got.RegionSides != want || got.ExactOperations < 1 {
						t.Fatal("changed exact signed region or original interval ordering", got)
					}
					checkAllCrossingParabolaNodes(t, src, got, circles, reflected)
					for i, p := range got.RegionWitnesses {
						if allCrossingCircleRegion(p, axis, reflected, circles) != i {
							t.Fatal("invalid exact source witness region", i, p)
						}
						a, err := src[0].EvaluateRational(ctx, p[0], p[1], geom.SurfacePatchEvaluationOptions{})
						if err != nil {
							t.Fatal(err)
						}
						b, err := src[1].EvaluateRational(ctx, p[0], p[1], geom.SurfacePatchEvaluationOptions{})
						if err != nil || a.Point[2].Cmp(b.Point[2]) != want[i] {
							t.Fatal("witness has wrong original roof order", i, err)
						}
					}
					checkAllCrossingParabolaMeasures(t, stocks, got.RegionSides, reflected, order)
				})
			}
		}
	}
}

func allCrossingCircleRegion(uv [2]*big.Rat, axis int, reflected bool, circles [3]twoCircleStockProbe) int {
	x, y := uv[axis], uv[1-axis]
	offset := new(big.Rat).Sub(x, big.NewRat(1, 2))
	height := new(big.Rat).Mul(offset, offset)
	height.Mul(height, big.NewRat(2, 1)).Add(height, big.NewRat(1, 4))
	if reflected {
		height.Sub(big.NewRat(1, 1), height)
	}
	region := 0
	if y.Cmp(height) > 0 {
		region = 1
	}
	for i, disc := range circles {
		dx, dy := new(big.Rat).Sub(x, disc.x), new(big.Rat).Sub(y, disc.y)
		if dx.Mul(dx, dx).Add(dx, dy.Mul(dy, dy)).Cmp(disc.radiusSquared) < 0 {
			return region + 2 + 2*i
		}
	}
	return region
}

func checkAllCrossingParabolaNodes(t *testing.T, roofs [2]*geom.SurfacePatch, p *geom.SplineGraphParabolaThreeCrossingCircles,
	circles [3]twoCircleStockProbe, reflected bool) {
	t.Helper()
	axis := p.FreeAxis
	positions := [3][2][2]int64{{{3, 57}, {5, 41}}, {{6, 36}, {10, 36}}, {{11, 41}, {13, 57}}}
	for i, nodes := range p.Nodes {
		circle, authored := p.Circles[i], circles[i]
		left, right := new(big.Rat).Sub(authored.x, big.NewRat(1, 8)), new(big.Rat).Add(authored.x, big.NewRat(1, 8))
		if circle.Center[axis].Cmp(authored.x) != 0 || circle.Center[1-axis].Cmp(authored.y) != 0 ||
			circle.RadiusSquared.Cmp(authored.radiusSquared) != 0 || p.CircleRadii[i].Cmp(big.NewRat(1, 8)) != 0 ||
			p.CircleProjections[i][0].Cmp(left) != 0 || p.CircleProjections[i][1].Cmp(right) != 0 {
			t.Fatal("changed original circle factor", i, circle)
		}
		for j, node := range nodes {
			x, y := big.NewRat(positions[i][j][0], 16), big.NewRat(positions[i][j][1], 128)
			slope := new(big.Rat).Sub(new(big.Rat).Mul(big.NewRat(4, 1), x), big.NewRat(2, 1))
			if reflected {
				y.Sub(big.NewRat(1, 1), y)
				slope.Neg(slope)
			}
			if node.Parameters[axis].Cmp(x) != 0 || node.Parameters[1-axis].Cmp(y) != 0 ||
				node.Point[axis].Cmp(x) != 0 || node.Point[1-axis].Cmp(y) != 0 || node.Point[2].Cmp(big.NewRat(2, 1)) != 0 ||
				node.Rays[0][axis].Cmp(big.NewRat(1, 1)) != 0 || node.Rays[0][1-axis].Cmp(slope) != 0 {
				t.Fatal("changed original junction or parabola ray", i, j, node)
			}
			ccw := node.Rays[p.CircleCCWRays[i][j]]
			rx := new(big.Rat).Sub(node.Parameters[0], circle.Center[0])
			ry := new(big.Rat).Sub(node.Parameters[1], circle.Center[1])
			if ccw[0].Cmp(ry.Neg(ry)) != 0 || ccw[1].Cmp(rx) != 0 {
				t.Fatal("changed original circle ray", i, j, node)
			}
			for _, roof := range roofs {
				jet, err := roof.EvaluateRational(context.Background(), node.Parameters[0], node.Parameters[1], geom.SurfacePatchEvaluationOptions{})
				if err != nil || jet.Point[2].Cmp(big.NewRat(2, 1)) != 0 || jet.Du[2].Sign() != 0 || jet.Dv[2].Sign() != 0 {
					t.Fatal("lost original owner junction jet", i, j, err)
				}
			}
			var sectors [8]bool
			for k, ray := range node.Rays {
				next := node.Rays[(k+1)%4]
				det := new(big.Rat).Sub(new(big.Rat).Mul(ray[0], next[1]), new(big.Rat).Mul(ray[1], next[0]))
				opposite := node.Rays[(k+2)%4]
				if ray[2].Sign() != 0 || det.Sign() <= 0 ||
					new(big.Rat).Add(ray[0], opposite[0]).Sign() != 0 ||
					new(big.Rat).Add(ray[1], opposite[1]).Sign() != 0 {
					t.Fatal("lost cyclic model ray order", i, j, node)
				}
				var witness [2]*big.Rat
				for dim := range witness {
					delta := new(big.Rat).Add(ray[dim], next[dim])
					witness[dim] = new(big.Rat).Add(node.Parameters[dim], delta.Quo(delta, big.NewRat(100000, 1)))
				}
				region := allCrossingCircleRegion(witness, axis, reflected, circles)
				if region != node.SectorRegions[k] || sectors[region] ||
					p.RegionSides[region] == p.RegionSides[node.SectorRegions[(k+1)%4]] {
					t.Fatal("changed local owner sector", i, j, node)
				}
				sectors[region] = true
			}
			if !sectors[0] || !sectors[1] || !sectors[2+2*i] || !sectors[3+2*i] {
				t.Fatal("incomplete four-sector link", i, j, node)
			}
		}
	}
	dependent, middle := big.NewRat(3, 4), big.NewRat(-1, 4)
	if reflected {
		dependent.SetFrac64(1, 4)
		middle.SetFrac64(5, 4)
	}
	for i, uv := range p.ParabolaControls {
		ordinate := dependent
		if i == 1 {
			ordinate = middle
		}
		if uv[axis].Cmp(big.NewRat(int64(i), 2)) != 0 || uv[1-axis].Cmp(ordinate) != 0 {
			t.Fatal("changed original parabola controls", uv)
		}
	}
	for i, point := range p.ParabolaExits {
		if point[axis].Cmp(big.NewRat(int64(i), 1)) != 0 || point[1-axis].Cmp(dependent) != 0 || point[2].Cmp(big.NewRat(2, 1)) != 0 {
			t.Fatal("changed parabola stock exit", point)
		}
	}
}

func TestParabolaThreeCrossingCirclesSourceRefusals(t *testing.T) {
	ctx := context.Background()
	for _, tc := range []struct {
		name    string
		circle  int
		value   twoCircleStockProbe
		perturb bool
		axis    int
		options geom.GraphPointContactOptions
		want    error
	}{
		{name: "no crossing", circle: 1, value: twoCircleProbe(16, 12, 1, 32), want: geom.ErrConvergence},
		{name: "double root", circle: 1, value: twoCircleProbe(16, 9, 1, 32), want: geom.ErrDegenerate},
		{name: "irrational roots", circle: 1, value: twoCircleProbe(64, 37, 16, 128), want: geom.ErrUnrepresentable},
		{name: "disk overlap", circle: 1, value: twoCircleProbe(40, 36, 16, 128), want: geom.ErrConvergence},
		{name: "stock contact", circle: 2, value: twoCircleProbe(112, 41, 16, 128), want: geom.ErrConvergence},
		{name: "changed original coefficient", perturb: true, want: geom.ErrConvergence},
		{name: "wrong original axis", axis: 1, want: geom.ErrConvergence},
		{name: "short exact work", options: geom.GraphPointContactOptions{MaxOperations: 1}, want: geom.ErrConvergence},
		{name: "short isolation work", options: geom.GraphPointContactOptions{MaxCells: 1}, want: geom.ErrConvergence},
	} {
		t.Run(tc.name, func(t *testing.T) {
			circles := allCrossingParabolaCircles(false)
			if tc.value.x != nil {
				circles[tc.circle] = tc.value
			}
			stocks := allCrossingParabolaStocks(t, 0, circles, false, tc.perturb)
			src := parabolaTwoCirclesPatches(t, stocks[1], stocks[2])
			got, err := geom.DiscoverSplineGraphParabolaThreeCrossingCircles(ctx, src[0], src[1], tc.axis, tc.options)
			if got != nil || !errors.Is(err, tc.want) {
				t.Fatal("published unresolved original arrangement", got, err, tc.want)
			}
		})
	}
}

// These reference integrals come from the polynomial antiderivative along
// each original parabola interval and exact disk sector moments. The left
// and right caps use quarter-circle moments and the middle caps use
// semicircle moments. Each coefficient pair represents a + b*Pi.
func checkAllCrossingParabolaMeasures(t *testing.T, stocks [3]*geom.BSplineSurface, signs [8]int, reflected bool, order int) {
	t.Helper()
	areas := [8][2]string{
		{"313/768", "-1/32"}, {"455/768", "-1/64"},
		{"11/1536", "3/256"}, {"-11/1536", "1/256"},
		{"-1/192", "1/128"}, {"1/192", "1/128"},
		{"11/1536", "3/256"}, {"-11/1536", "1/256"},
	}
	volumes := [8][2]string{
		{"-16493411484214369271/2364822149933737967616", "-1080508954595/54043195528445952"},
		{"510766466154070014455/2364822149933737967616", "-118001187745/18014398509481984"},
		{"16814697493128941/1576548099955825311744", "2837911685/281474976710656"},
		{"-16814697493128941/1576548099955825311744", "2837911685/844424930131968"},
		{"58738310281301/49267128123619540992", "-9249132445/54043195528445952"},
		{"-58738310281301/49267128123619540992", "-9249132445/54043195528445952"},
		{"16814697493128941/1576548099955825311744", "2837911685/281474976710656"},
		{"-16814697493128941/1576548099955825311744", "2837911685/844424930131968"},
	}
	ref := func(s string) *big.Rat {
		r, ok := new(big.Rat).SetString(s)
		if !ok {
			t.Fatal("bad independent reference coefficient", s)
		}
		return r
	}
	pi := [2]*big.Rat{ref("3.14159265358979323846264338327950288"), ref("3.14159265358979323846264338327950289")}
	areaSum := [2]*big.Rat{new(big.Rat), new(big.Rat)}
	volumeSum := [2]*big.Rat{new(big.Rat), new(big.Rat)}
	orientation := big.NewRat(int64(2*order-1), 1)
	if reflected {
		orientation.Mul(orientation, big.NewRat(-1, 2))
	}
	for i, sign := range signs {
		j := i
		if reflected {
			j ^= 1
		}
		a := [2]*big.Rat{ref(areas[j][0]), ref(areas[j][1])}
		v := [2]*big.Rat{ref(volumes[j][0]), ref(volumes[j][1])}
		for k := range a {
			v[k].Mul(v[k], orientation)
			areaSum[k].Add(areaSum[k], a[k])
			volumeSum[k].Add(volumeSum[k], v[k])
		}
		for _, bound := range pi {
			area := new(big.Rat).Add(a[0], new(big.Rat).Mul(a[1], bound))
			volume := new(big.Rat).Add(v[0], new(big.Rat).Mul(v[1], bound))
			if area.Sign() <= 0 || volume.Sign() != sign {
				t.Fatal("signed source region disagrees with exact independent measures", i, area, volume)
			}
		}
	}
	if areaSum[0].Cmp(big.NewRat(1, 1)) != 0 || areaSum[1].Sign() != 0 || volumeSum[1].Sign() != 0 {
		t.Fatal("independent region integrals do not cover whole chart", areaSum, volumeSum)
	}
	// On the original unweighted degree-eight tensor stock, the integral of
	// every Bernstein basis function is exactly 1/81.
	source := new(big.Rat)
	other := stocks[2-order].Data().Poles
	for k, pole := range stocks[1+order].Data().Poles {
		a := new(big.Rat).SetFloat64(pole.Z)
		c := new(big.Rat).SetFloat64(other[k].Z)
		source.Add(source, a.Sub(a, c))
	}
	source.Quo(source, big.NewRat(81, 1))
	if source.Cmp(volumeSum[0]) != 0 {
		t.Fatal("independent region volumes miss original source measure", source, volumeSum)
	}
}
