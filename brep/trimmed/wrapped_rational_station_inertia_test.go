//go:build research

package trimmed

import (
	"bytes"
	"context"
	"errors"
	"math/big"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// Use Green's theorem on the original unit profile independently of the
// certified polygon enclosures. The coordinates below use a single Bézier
// parameter per quadrant and unequal rational control weights.
func independentStationProfileSecond(weights [4]float64) (second [3]float64) {
	const n = 2048
	for quadrant, w := range weights {
		for i := 0; i <= n; i++ {
			t := float64(i) / n
			s := 1 - t
			q, dq := 2*w*t*s, 2*w*(1-2*t)
			den, dDen := s*s+q+t*t, -2*s+dq+2*t
			x, y := (s*s+q)/den, (q+t*t)/den
			dx, dy := (-2*s+dq-x*dDen)/den, (dq+2*t-y*dDen)/den
			for j := 0; j < quadrant; j++ {
				x, y, dx, dy = -y, x, -dy, dx
			}
			factor := stationInertiaSimpson(i, n) / (3 * n)
			second[0] += factor * x * x * x * dy / 3
			second[1] += factor * x * x * y * dy / 2
			second[2] -= factor * y * y * y * dx / 3
		}
	}
	return second
}

func stationInertiaSimpson(i, n int) float64 {
	if i == 0 || i == n {
		return 1
	}
	if i%2 == 0 {
		return 2
	}
	return 4
}

func independentStationInertia(weights [4]float64, families ...[][2]float64) (volume float64, first [3]float64, second, inertia [3][3]float64) {
	a, x, y := independentStationProfileMoments(weights, 2048)
	q := independentStationProfileSecond(weights)
	for owner, grid := range families {
		for i := 0; i+1 < len(grid); i++ {
			start, end := grid[i], grid[i+1]
			const n = 512
			for j := 0; j <= n; j++ {
				s := float64(j) / n
				z, r := start[0]+s*(end[0]-start[0]), start[1]+s*(end[1]-start[1])
				d := stationInertiaSimpson(j, n) * (end[0] - start[0]) / (3 * n)
				if owner == 1 {
					d = -d
				}
				v := d * r * r
				volume += v * a
				first[0] += v * r * x
				first[1] += v * r * y
				first[2] += v * z * a
				second[0][0] += v * r * r * q[0]
				second[0][1] += v * r * r * q[1]
				second[1][1] += v * r * r * q[2]
				second[0][2] += v * r * z * x
				second[1][2] += v * r * z * y
				second[2][2] += v * z * z * a
			}
		}
	}
	for i := range second {
		for j := i; j < 3; j++ {
			second[j][i] = second[i][j]
			inertia[i][j] = first[i]*first[j]/volume - second[i][j]
			inertia[j][i] = inertia[i][j]
		}
	}
	var trace float64
	for i := range inertia {
		trace -= inertia[i][i]
	}
	for i := range inertia {
		inertia[i][i] += trace
	}
	return
}

func checkStationInertia(t *testing.T, actual WrappedRationalStationInertiaBounds, tolerance *big.Rat, weights [4]float64, grids ...[][2]float64) {
	t.Helper()
	volume, first, q, inertia := independentStationInertia(weights, grids...)
	contains := func(entry WrappedStationMassInterval, want float64) {
		t.Helper()
		lo, _ := entry.Lower.Float64()
		hi, _ := entry.Upper.Float64()
		if want < lo-1e-9 || want > hi+1e-9 {
			t.Fatal("independent integral outside exact certificate", lo, want, hi)
		}
	}
	contains(actual.Volume, volume)
	for i := range inertia {
		contains(actual.FirstMoments[i], first[i])
		for j := range inertia[i] {
			contains(actual.SecondMoments[i][j], q[i][j])
			item := actual.CentroidalInertia[i][j]
			contains(item, inertia[i][j])
			opp := actual.CentroidalInertia[j][i]
			if item.Lower.Cmp(opp.Lower) != 0 || item.Upper.Cmp(opp.Upper) != 0 || new(big.Rat).Sub(item.Upper, item.Lower).Cmp(tolerance) > 0 {
				t.Fatal("invalid tensor symmetry or width", i, j)
			}
		}
	}
	t.Log("profile spans", actual.Segments, "exact operations", actual.ExactOperations, "independent inertia", inertia)
}

func TestCertifiedRationalStationInertia(t *testing.T) {
	ctx := context.Background()
	body := stationMassFixture(t, true, -4)
	options := WrappedRationalStationInertiaOptions{InertiaTolerance: big.NewRat(1, 20), MaxCoefficientBits: 65536}
	actual, err := body.BoundWrappedRationalStationInertia(ctx, options)
	if err != nil {
		t.Fatal(err)
	}
	if actual.Origin[2].Cmp(big.NewRat(-4, 1)) != 0 || len(actual.WallFaces) != 2 || actual.CentroidalInertia[0][2].Upper.Sign() >= 0 {
		t.Fatal("signed coupled inertia or original history lost", actual.Origin, actual.WallFaces, actual.CentroidalInertia[0][2].Upper.FloatString(9))
	}
	checkStationInertia(t, actual, options.InertiaTolerance, [4]float64{.5, .75, .75, .75}, [][2]float64{{0, 3}, {1, 1}, {3, 2}})
	moved, err := stationMassFixture(t, true, 1<<20).BoundWrappedRationalStationInertia(ctx, options)
	if err != nil {
		t.Fatal(err)
	}
	for i := range actual.CentroidalInertia {
		for j, entry := range actual.CentroidalInertia[i] {
			if entry.Lower.Cmp(moved.CentroidalInertia[i][j].Lower) != 0 || entry.Upper.Cmp(moved.CentroidalInertia[i][j].Upper) != 0 {
				t.Fatal("translation altered centroidal inertia bounds", i, j)
			}
		}
	}
	for _, deny := range []func(*WrappedRationalStationInertiaOptions){
		func(o *WrappedRationalStationInertiaOptions) { o.MaxOperations = actual.ExactOperations - 1 },
		func(o *WrappedRationalStationInertiaOptions) { o.MaxSegments = actual.Segments - 1 },
		func(o *WrappedRationalStationInertiaOptions) { o.Source.MaxOperations = 1 },
	} {
		limited := options
		deny(&limited)
		got, err := body.BoundWrappedRationalStationInertia(ctx, limited)
		if !errors.Is(err, geom.ErrConvergence) || got.Volume.Lower != nil || len(got.WallFaces) != 0 {
			t.Fatal("inertia failure published a partial result", err, got)
		}
	}
}

func TestCertifiedRationalStationBoreInertia(t *testing.T) {
	ctx := context.Background()
	o := geom.SurfacePatchOptions{}
	outer, _, err := NewWrappedRationalStationLoft(ctx, []*geom.WrappedSurfacePatch{
		rationalStationBand(t, 0, 1, 3, 2, .75), rationalStationBand(t, 1, 3, 2, 4, .75),
	}, 1e-8, o)
	if err != nil {
		t.Fatal(err)
	}
	inner, _, err := NewWrappedRationalStationLoft(ctx, []*geom.WrappedSurfacePatch{
		rationalStationBand(t, 0, 2, 1, 1, .75), rationalStationBand(t, 2, 3, 1, 2, .75),
	}, 1e-8, o)
	if err != nil {
		t.Fatal(err)
	}
	bore, history, err := CombineCoaxialWrappedRationalStationLofts(ctx, outer, inner, parametric.Difference, o)
	if err != nil {
		t.Fatal(err)
	}
	var native bytes.Buffer
	if err := WriteJSON(&native, bore); err != nil {
		t.Fatal(err)
	}
	bore, err = ReadJSONContext(ctx, &native, JSONLimits{})
	if err != nil {
		t.Fatal(err)
	}
	options := WrappedRationalStationInertiaOptions{InertiaTolerance: big.NewRat(1, 10), MaxCoefficientBits: 65536}
	actual, err := bore.BoundWrappedRationalStationBoreInertia(ctx, options)
	if err != nil {
		t.Fatal(err)
	}
	if len(actual.WallFaces) != 4 || actual.CapFaces != history.ResultCaps || actual.CentroidalInertia[1][2].Lower.Sign() != 0 || actual.CentroidalInertia[1][2].Upper.Sign() != 0 {
		t.Fatal("bore history or proved central symmetry lost", actual)
	}
	checkStationInertia(t, actual, options.InertiaTolerance, [4]float64{.75, .75, .75, .75},
		[][2]float64{{0, 3}, {1, 2}, {3, 4}}, [][2]float64{{0, 1}, {2, 1}, {3, 2}})
}
