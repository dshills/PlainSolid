package trimmed

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"testing"

	"plainsolid/geom"
)

func parabolaCirclePocketDiskStocks(t *testing.T, axis int, reflected bool, disk twoCircleStockProbe, scale *big.Rat,
	perturb bool) [3]*geom.BSplineSurface {
	t.Helper()
	p := [3]*big.Rat{big.NewRat(1351, 2048), big.NewRat(-2, 1), big.NewRat(2, 1)}
	if reflected {
		p[0].Sub(big.NewRat(1, 1), p[0])
		p[1].Neg(p[1])
		p[2].Neg(p[2])
		disk.y = new(big.Rat).Sub(big.NewRat(1, 1), disk.y)
		scale = new(big.Rat).Quo(scale, big.NewRat(2, 1))
	}
	return parabolaCircleProductStock(t, axis, []twoCircleStockProbe{twoCircleProbe(128, 128, 85, 256), disk}, p, scale, perturb)
}

func TestParabolaCirclePocketDiskSource(t *testing.T) {
	ctx := context.Background()
	for layout := range 2 {
		reflected := layout != 0
		for axis := range 2 {
			stocks := parabolaCirclePocketDiskStocks(t, axis, reflected, twoCircleProbe(256, 84, 1, 512), big.NewRat(245, 16), false)
			for order := range 2 {
				t.Run(fmt.Sprintf("%d/%d/%d", layout, axis, order), func(t *testing.T) {
					src := parabolaTwoCirclesPatches(t, stocks[1+order], stocks[2-order])
					p, err := geom.DiscoverSplineGraphParabolaCirclePocketDiskWithVolume(ctx, src[0], src[1], axis, geom.GraphPointContactOptions{})
					if err != nil {
						t.Fatal(err)
					}
					sign := (1 - 2*layout) * (1 - 2*order)
					want := [7]int{sign, -sign, -sign, -sign, -sign, sign, sign}
					if p.RegionSides != want || p.PocketCircle != layout || p.CrossingCircle != 1-layout ||
						p.Crossing.CircleSide != 2*layout-1 || p.Crossing.FreeAxis != axis ||
						p.PocketProjection[0].Cmp(big.NewRat(255, 512)) != 0 || p.PocketProjection[1].Cmp(big.NewRat(257, 512)) != 0 ||
						p.ExactOperations <= p.Crossing.ExactOperations {
						t.Fatal("changed signed pocket coverage or original factor order", p)
					}
					if p.DiskSignedVolumePiCoefficient.Cmp(expectedPocketDiskVolumePiCoefficient(t, layout, order)) != 0 {
						t.Fatal("changed exact signed disk source volume", p.DiskSignedVolumePiCoefficient)
					}
					checkFourCrossingCircleJunctions(t, &p.Crossing, reflected)
					checkOriginalPocketDiskExtrema(t, p, axis, reflected)
					for i, uv := range p.RegionWitnesses {
						if parabolaCirclePocketRegion(uv, axis, reflected) != i {
							t.Fatal("source witness left its exact original pocket region", i, uv)
						}
						a, err := src[0].EvaluateRational(ctx, uv[0], uv[1], geom.SurfacePatchEvaluationOptions{})
						if err != nil {
							t.Fatal(err)
						}
						b, err := src[1].EvaluateRational(ctx, uv[0], uv[1], geom.SurfacePatchEvaluationOptions{})
						if err != nil || a.Point[2].Cmp(b.Point[2]) != want[i] {
							t.Fatal("source witness lost its original signed order", i, err)
						}
					}
					if parabolaCirclePocketRegion(p.Crossing.RegionWitnesses[2], axis, reflected) != 2 {
						t.Fatal("original pocket witness entered its removed disk", p.Crossing.RegionWitnesses[2])
					}
					checkPocketDiskMeasures(t, stocks, p.RegionSides, sign, layout, order)
				})
			}
		}
	}
}

func parabolaCirclePocketRegion(uv [2]*big.Rat, axis int, reflected bool) int {
	x, y := uv[axis], new(big.Rat).Set(uv[1-axis])
	if reflected {
		y.Sub(big.NewRat(1, 1), y)
	}
	dx, dy := new(big.Rat).Sub(x, big.NewRat(1, 2)), new(big.Rat).Sub(y, big.NewRat(21, 128))
	if dx.Mul(dx, dx).Add(dx, dy.Mul(dy, dy)).Cmp(big.NewRat(1, 262144)) < 0 {
		return 6
	}
	return fourCrossingCircleRegion(uv, axis, reflected)
}

func checkOriginalPocketDiskExtrema(t *testing.T, p *geom.SplineGraphParabolaCirclePocketDisk, axis int, reflected bool) {
	t.Helper()
	inner, outer := p.Circles[p.PocketCircle], p.Circles[p.CrossingCircle]
	y := big.NewRat(21, 128)
	if reflected {
		y.Sub(big.NewRat(1, 1), y)
	}
	if inner.Center[axis].Cmp(big.NewRat(1, 2)) != 0 || inner.Center[1-axis].Cmp(y) != 0 ||
		inner.RadiusSquared.Cmp(big.NewRat(1, 262144)) != 0 || p.PocketRadius.Cmp(big.NewRat(1, 512)) != 0 ||
		outer.Center[0].Cmp(big.NewRat(1, 2)) != 0 || outer.Center[1].Cmp(big.NewRat(1, 2)) != 0 ||
		outer.RadiusSquared.Cmp(big.NewRat(7225, 65536)) != 0 {
		t.Fatal("changed original nested circular factor", inner, outer)
	}
	for i, radial := range [4][2]int64{{1, 0}, {0, 1}, {-1, 0}, {0, -1}} {
		ext := p.PocketExtrema[i]
		for j, d := range radial {
			want := new(big.Rat).Add(inner.Center[j], new(big.Rat).Mul(p.PocketRadius, big.NewRat(d, 1)))
			if ext.Parameters[j].Cmp(want) != 0 || ext.Point[j].Cmp(want) != 0 {
				t.Fatal("changed original nested rim cardinal endpoint", i, ext)
			}
		}
		for _, jet := range ext.Tangents {
			if ext.Point[2].Cmp(big.NewRat(2, 1)) != 0 || jet[0].Cmp(big.NewRat(-radial[1], 1)) != 0 ||
				jet[1].Cmp(big.NewRat(radial[0], 1)) != 0 || jet[2].Sign() != 0 {
				t.Fatal("nested rim lost original roof tangent", i, ext)
			}
		}
	}
}

func TestParabolaCirclePocketDiskSourceRefusals(t *testing.T) {
	ctx := context.Background()
	nested := twoCircleProbe(256, 84, 1, 512)
	for _, tc := range []struct {
		name    string
		disk    twoCircleStockProbe
		scale   *big.Rat
		edit    bool
		axis    int
		options geom.GraphPointContactOptions
	}{
		{name: "separate outside pocket", disk: twoCircleProbe(256, 20, 1, 512), scale: big.NewRat(245, 8)},
		{name: "shares disk interior", disk: twoCircleProbe(256, 192, 1, 512), scale: big.NewRat(245, 16)},
		{name: "touches outer disk", disk: twoCircleProbe(256, 85, 1, 512), scale: big.NewRat(245, 16)},
		{name: "touches stock wall", disk: twoCircleProbe(1, 84, 1, 512), scale: big.NewRat(245, 64)},
		{name: "changed original coefficient", disk: nested, scale: big.NewRat(245, 16), edit: true},
		{name: "wrong original chart", disk: nested, scale: big.NewRat(245, 16), axis: 1},
		{name: "short root isolation", disk: nested, scale: big.NewRat(245, 16), options: geom.GraphPointContactOptions{MaxCells: 1}},
		{name: "short factor proof", disk: nested, scale: big.NewRat(245, 16), options: geom.GraphPointContactOptions{MaxOperations: 1}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stock := parabolaCirclePocketDiskStocks(t, 0, false, tc.disk, tc.scale, tc.edit)
			patches := parabolaTwoCirclesPatches(t, stock[1], stock[2])
			out, err := geom.DiscoverSplineGraphParabolaCirclePocketDisk(ctx, patches[0], patches[1], tc.axis, tc.options)
			if out != nil || !errors.Is(err, geom.ErrConvergence) {
				t.Fatal("published unsupported original nested arrangement", out, err)
			}
		})
	}
}

func checkPocketDiskMeasures(t *testing.T, stocks [3]*geom.BSplineSurface, signs [7]int, sign, layout, order int) {
	t.Helper()
	// Coefficients of 1, Pi, atan(77/36), atan(3/4) from independent
	// Green integrals along the two original circle laws and parabola.
	areas := [7][4]string{
		{"3109/8192", "0", "-7225/65536", "7225/65536"},
		{"18703/32768", "-7225/131072", "0", "-7225/65536"},
		{"1629/32768", "-14451/262144", "7225/65536", "0"},
		{"-1307/49152", "0", "7225/131072", "-7225/131072"},
		{"-1307/49152", "0", "7225/131072", "-7225/131072"},
		{"1307/24576", "7225/65536", "-7225/65536", "7225/65536"},
		{"0", "1/262144", "0", "0"},
	}
	volume := [7][4]string{
		{"-101735706652906906641341/3038842831726616695013376", "0", "37627827748182371875/7083549724304467820544", "-37627827748182371875/7083549724304467820544"},
		{"448406331183903175966057/3038842831726616695013376", "37627827748182371875/14167099448608935641088", "0", "37627827748182371875/7083549724304467820544"},
		{"-196101538945131177939/84412300881294908194816", "802726991961595730225/302231454903657293676544", "-37627827748182371875/7083549724304467820544", "0"},
		{"2965897459904911289723/2279132123794962521260032", "0", "-37627827748182371875/14167099448608935641088", "37627827748182371875/14167099448608935641088"},
		{"2965897459904911289723/2279132123794962521260032", "0", "-37627827748182371875/14167099448608935641088", "37627827748182371875/14167099448608935641088"},
		{"-2965897459904911289723/1139566061897481260630016", "-37627827748182371875/7083549724304467820544", "37627827748182371875/7083549724304467820544", "-37627827748182371875/7083549724304467820544"},
		{"0", "-1115390675/906694364710971881029632", "0", "0"},
	}
	checkParabolaCircleGreenMeasures(t, stocks, signs[:], areas[:], volume[:], big.NewRat(int64(-sign), int64(1+layout)), order)
}

func expectedPocketDiskVolumePiCoefficient(t *testing.T, layout, order int) *big.Rat {
	t.Helper()
	r, ok := new(big.Rat).SetString("1115390675/906694364710971881029632")
	if !ok {
		t.Fatal("invalid independent disk volume coefficient")
	}
	return r.Mul(r, big.NewRat(int64((1-2*layout)*(1-2*order)), int64(1+layout)))
}

func TestParabolaCirclePocketDiskVariableWeightVolumeRefusal(t *testing.T) {
	ctx := context.Background()
	stock := parabolaCirclePocketDiskStocks(t, 0, false, twoCircleProbe(256, 84, 1, 512), big.NewRat(735, 32), false)
	// Degree-seven denominator poles [3,3,3,3,1,1,1,1] elevate to
	// degree-eight weights [3,3,3,3,2,1,1,1,1]. Their product with u has
	// the authored Cartesian X poles i/8 except for i=4 at 3/4.
	// Both original roofs retain physical identity XY and the same
	// homogeneous factored numerator with a variable denominator.
	for i := 1; i <= 2; i++ {
		data := stock[i].Data()
		weights := make([]float64, len(data.Poles))
		for j, pole := range data.Poles {
			row := j / data.CountV
			w := [9]int64{3, 3, 3, 3, 2, 1, 1, 1, 1}[row]
			weights[j] = float64(w)
			if row == 4 {
				pole.X = .75
			}
			two := big.NewRat(2, 1)
			z := new(big.Rat).Sub(new(big.Rat).SetFloat64(pole.Z), two)
			z.Quo(z, big.NewRat(w, 1)).Add(z, two)
			var exact bool
			pole.Z, exact = z.Float64()
			if !exact {
				t.Fatal("weighted original fixture lost an exact source pole", z)
			}
			data.Poles[j] = pole
		}
		var err error
		stock[i], err = geom.NewBSplineSurfaceWithExactWeights(data.DegreeU, data.DegreeV, data.CountU, data.CountV,
			data.Poles, data.KnotsU, data.KnotsV, weights)
		if err != nil {
			t.Fatal(err)
		}
	}
	patches := parabolaTwoCirclesPatches(t, stock[1], stock[2])
	proof, err := geom.DiscoverSplineGraphParabolaCirclePocketDisk(ctx, patches[0], patches[1], 0, geom.GraphPointContactOptions{})
	if err != nil || proof.RegionSides[6] != 1 {
		t.Fatal("weighted source geometry should retain its exact disk sign", proof, err)
	}
	out, err := geom.DiscoverSplineGraphParabolaCirclePocketDiskWithVolume(ctx, patches[0], patches[1], 0, geom.GraphPointContactOptions{})
	if out != nil || !errors.Is(err, geom.ErrUnrepresentable) {
		t.Fatal("published a polynomial volume for a variable original denominator", out, err)
	}
}
