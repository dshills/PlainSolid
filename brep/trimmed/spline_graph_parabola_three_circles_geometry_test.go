package trimmed

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"slices"
	"testing"

	"plainsolid/geom"
)

func parabolaThreeCircleStocks(t *testing.T, axis int, separated twoCircleStockProbe, perturb bool) [3]*geom.BSplineSurface {
	t.Helper()
	left, right := twoCircleProbe(24, 41, 16, 128), twoCircleProbe(104, 41, 16, 128)
	return parabolaCircleProductStock(t, axis, []twoCircleStockProbe{left, separated, right},
		[3]*big.Rat{big.NewRat(3, 4), big.NewRat(-2, 1), big.NewRat(2, 1)}, big.NewRat(1225, 64), perturb)
}

func TestParabolaThreeCirclesSource(t *testing.T) {
	ctx := context.Background()
	for layout, fixture := range []struct {
		disk          twoCircleStockProbe
		region, index int
	}{
		{twoCircleProbe(16, 12, 1, 32), 1, 1}, // local separation above
		{twoCircleProbe(16, 6, 1, 32), 0, 1},
		{twoCircleProbe(64, 41, 4, 128), 1, 1}, // repeated dependent coordinate
		{twoCircleProbe(6, 18, 1, 32), 1, 1},   // repeated free coordinate and nested disk projections
		{twoCircleProbe(2, 4, 1, 32), 0, 0},    // separated projection precedes both crossings
	} {
		for axis := range 2 {
			stocks := parabolaThreeCircleStocks(t, axis, fixture.disk, false)
			for order := range 2 {
				t.Run(fmt.Sprintf("%d/%d/%d", layout, axis, order), func(t *testing.T) {
					src := parabolaTwoCirclesPatches(t, stocks[1+order], stocks[2-order])
					got, err := geom.DiscoverSplineGraphParabolaThreeCircles(ctx, src[0], src[1], axis, geom.GraphPointContactOptions{})
					if err != nil {
						t.Fatal(err)
					}
					sign := 1 - 2*order
					want := [7]int{sign, -sign, -sign, sign, -sign, sign, sign * (2*fixture.region - 1)}
					if got.RegionSides != want || got.SeparatedRegion != fixture.region || got.SeparatedCircle != fixture.index ||
						slices.Contains(got.CrossingCircles[:], fixture.index) || got.ExactOperations <= got.Crossing.ExactOperations {
						t.Fatal("changed exact signed region or original factor ordering", got)
					}
					d := got.Circles[fixture.index]
					if d.Center[axis].Cmp(fixture.disk.x) != 0 || d.Center[1-axis].Cmp(fixture.disk.y) != 0 || d.RadiusSquared.Cmp(fixture.disk.radiusSquared) != 0 {
						t.Fatal("changed original isolated disk", d)
					}
					checkParabolaThreeCircleNodes(t, axis, got)
					checkParabolaThreeCircleExtrema(t, got, d)
					for i, uv := range got.RegionWitnesses {
						left, err := src[0].EvaluateRational(ctx, uv[0], uv[1], geom.SurfacePatchEvaluationOptions{})
						if err != nil {
							t.Fatal(err)
						}
						right, err := src[1].EvaluateRational(ctx, uv[0], uv[1], geom.SurfacePatchEvaluationOptions{})
						if err != nil || left.Point[2].Cmp(right.Point[2]) != want[i] ||
							parabolaThreeCircleRegion(uv, axis, fixture.disk) != i {
							t.Fatal("invalid original signed region witness", i, err)
						}
					}
				})
			}
		}
	}
}

func parabolaThreeCircleRegion(uv [2]*big.Rat, axis int, separated twoCircleStockProbe) int {
	x, y := uv[axis], uv[1-axis]
	offset := new(big.Rat).Sub(x, big.NewRat(1, 2))
	height := new(big.Rat).Mul(offset, offset)
	height.Mul(height, big.NewRat(2, 1)).Add(height, big.NewRat(1, 4))
	for i, disc := range []twoCircleStockProbe{twoCircleProbe(24, 41, 16, 128), twoCircleProbe(104, 41, 16, 128), separated} {
		dx, dy := new(big.Rat).Sub(x, disc.x), new(big.Rat).Sub(y, disc.y)
		if dx.Mul(dx, dx).Add(dx, dy.Mul(dy, dy)).Cmp(disc.radiusSquared) < 0 {
			if i == 2 {
				return 6
			}
			side := 2 + 2*i
			if y.Cmp(height) > 0 {
				side++
			}
			return side
		}
	}
	if y.Cmp(height) > 0 {
		return 1
	}
	return 0
}

func checkParabolaThreeCircleNodes(t *testing.T, axis int, proof *geom.SplineGraphParabolaThreeCircles) {
	t.Helper()
	expectX := [2][2]int64{{3, 5}, {11, 13}}
	expectY := [2][2]int64{{57, 41}, {41, 57}}
	separated := proof.Circles[proof.SeparatedCircle]
	disco := twoCircleStockProbe{x: separated.Center[axis], y: separated.Center[1-axis], radiusSquared: separated.RadiusSquared}
	for i, nodes := range proof.Crossing.Nodes {
		circle := proof.Circles[proof.CrossingCircles[i]]
		if circle.Center[axis].Cmp(big.NewRat(expectX[i][i], 16)) != 0 ||
			circle.Center[1-axis].Cmp(big.NewRat(41, 128)) != 0 || circle.RadiusSquared.Cmp(big.NewRat(1, 64)) != 0 {
			t.Fatal("changed crossing factor or its center", i, circle)
		}
		for j, node := range nodes {
			x, y := big.NewRat(expectX[i][j], 16), big.NewRat(expectY[i][j], 128)
			if node.Parameters[axis].Cmp(x) != 0 || node.Parameters[1-axis].Cmp(y) != 0 ||
				node.Point[axis].Cmp(x) != 0 || node.Point[1-axis].Cmp(y) != 0 || node.Point[2].Cmp(big.NewRat(2, 1)) != 0 {
				t.Fatal("changed an original transverse endpoint", node)
			}
			slope := new(big.Rat).Sub(new(big.Rat).Mul(big.NewRat(4, 1), x), big.NewRat(2, 1))
			if node.Rays[0][axis].Cmp(big.NewRat(1, 1)) != 0 || node.Rays[0][1-axis].Cmp(slope) != 0 {
				t.Fatal("changed original parabola ray", node)
			}
			r := [2]*big.Rat{new(big.Rat).Sub(node.Parameters[0], circle.Center[0]), new(big.Rat).Sub(node.Parameters[1], circle.Center[1])}
			ccw := node.Rays[proof.Crossing.CircleCCWRays[i][j]]
			if ccw[0].Cmp(new(big.Rat).Neg(r[1])) != 0 || ccw[1].Cmp(r[0]) != 0 {
				t.Fatal("changed original outgoing circle ray", node)
			}
			for k, a := range node.Rays {
				b := node.Rays[(k+1)%4]
				det := new(big.Rat).Sub(new(big.Rat).Mul(a[0], b[1]), new(big.Rat).Mul(a[1], b[0]))
				if a[2].Sign() != 0 || det.Sign() <= 0 || proof.RegionSides[node.SectorRegions[k]] == proof.RegionSides[node.SectorRegions[(k+1)%4]] {
					t.Fatal("invalid physical transverse sector", node)
				}
				var q [2]*big.Rat
				for dim := range q {
					q[dim] = new(big.Rat).Add(node.Parameters[dim], new(big.Rat).Mul(big.NewRat(1, 100000), new(big.Rat).Add(a[dim], b[dim])))
				}
				if parabolaThreeCircleRegion(q, axis, disco) != node.SectorRegions[k] {
					t.Fatal("node sector lost an original owner", node)
				}
			}
		}
	}
}

func checkParabolaThreeCircleExtrema(t *testing.T, proof *geom.SplineGraphParabolaThreeCircles, circle geom.GraphContactCircle) {
	t.Helper()
	for i, radial := range [4][2]int64{{1, 0}, {0, 1}, {-1, 0}, {0, -1}} {
		v := proof.SeparatedExtrema[i]
		for dim := range radial {
			want := new(big.Rat).Add(circle.Center[dim], new(big.Rat).Mul(big.NewRat(radial[dim], 1), proof.SeparatedRadius))
			if v.Parameters[dim].Cmp(want) != 0 || v.Point[dim].Cmp(want) != 0 {
				t.Fatal("changed isolated original circle cardinal vertex", v)
			}
		}
		for _, jet := range v.Tangents {
			if jet[0].Cmp(big.NewRat(-radial[1], 1)) != 0 || jet[1].Cmp(big.NewRat(radial[0], 1)) != 0 ||
				jet[2].Sign() != 0 || v.Point[2].Cmp(big.NewRat(2, 1)) != 0 {
				t.Fatal("changed isolated original circle owner jet", v)
			}
		}
	}
}

func TestParabolaThreeCirclesSourceRefusals(t *testing.T) {
	ctx := context.Background()
	normal := twoCircleProbe(16, 12, 1, 32)
	for _, tc := range []struct {
		name    string
		disk    twoCircleStockProbe
		perturb bool
		axis    int
		options geom.GraphPointContactOptions
		want    error
	}{
		{"unmatched original coefficient", normal, true, 0, geom.GraphPointContactOptions{}, geom.ErrConvergence},
		{"wrong original chart", normal, false, 1, geom.GraphPointContactOptions{}, geom.ErrConvergence},
		{"overlapping disks", twoCircleProbe(6, 12, 1, 32), false, 0, geom.GraphPointContactOptions{}, geom.ErrConvergence},
		{"stock wall contact", twoCircleProbe(1, 4, 1, 32), false, 0, geom.GraphPointContactOptions{}, geom.ErrConvergence},
		{"third circle tangent", twoCircleProbe(16, 9, 1, 32), false, 0, geom.GraphPointContactOptions{}, geom.ErrDegenerate},
		{"three crossing circles", twoCircleProbe(16, 9, 4, 32), false, 0, geom.GraphPointContactOptions{}, geom.ErrConvergence},
		{"short exact work", normal, false, 0, geom.GraphPointContactOptions{MaxOperations: 1}, geom.ErrConvergence},
		{"short isolation budget", normal, false, 0, geom.GraphPointContactOptions{MaxCells: 1}, geom.ErrConvergence},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stocks := parabolaThreeCircleStocks(t, 0, tc.disk, tc.perturb)
			src := parabolaTwoCirclesPatches(t, stocks[1], stocks[2])
			out, err := geom.DiscoverSplineGraphParabolaThreeCircles(ctx, src[0], src[1], tc.axis, tc.options)
			if out != nil || !errors.Is(err, tc.want) {
				t.Fatal("published unresolved three-circle arrangement", out, err, tc.want)
			}
		})
	}
}
