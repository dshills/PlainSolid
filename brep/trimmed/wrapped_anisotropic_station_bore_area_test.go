//go:build research

package trimmed

import (
	"context"
	"math"
	"math/big"
	"plainsolid/internal/slowtest"
	"reflect"
	"testing"
)

func independentBoreWallArea(a, b [3]float64, n, m int) float64 {
	band := 0
	scales := [][2]float64{{a[1], a[2]}, {b[1], b[2]}}
	heights := []float64{a[0], b[0]}
	h := heights[band+1] - heights[band]
	dx := (scales[band+1][0] - scales[band][0]) / h
	dy := (scales[band+1][1] - scales[band][1]) / h
	total := 0.
	weight := func(i, n int) float64 {
		if i == 0 || i == n {
			return 1
		}
		if i%2 == 1 {
			return 4
		}
		return 2
	}
	for quadrant := 0; quadrant < 4; quadrant++ {
		sum := 0.
		for i := 0; i <= n; i++ {
			u := float64(i) / float64(n)
			w := .75
			if quadrant == 0 {
				w = .5
			}
			den := (1-u)*(1-u) + 2*w*u*(1-u) + u*u
			dd := -2*(1-u) + 2*w*(1-2*u) + 2*u
			nx := (1-u)*(1-u) + 2*w*u*(1-u)
			ny := 2*w*u*(1-u) + u*u
			nxp := -2*(1-u) + 2*w*(1-2*u)
			nyp := 2*w*(1-2*u) + 2*u
			x, y := nx/den, ny/den
			xp, yp := (nxp*den-nx*dd)/(den*den), (nyp*den-ny*dd)/(den*den)
			for k := 0; k < quadrant; k++ {
				x, y = -y, x
				xp, yp = -yp, xp
			}
			for j := 0; j <= m; j++ {
				v := float64(j) / float64(m)
				sx := scales[band][0] + v*(scales[band+1][0]-scales[band][0])
				sy := scales[band][1] + v*(scales[band+1][1]-scales[band][1])
				a, c, d := sy*yp, -sx*xp, sx*dy*xp*y-sy*dx*yp*x
				sum += weight(i, n) * weight(j, m) * math.Sqrt(a*a+c*c+d*d)
			}
		}
		total += sum * h / (9 * float64(n*m))
	}
	return total
}
func TestAnisotropicBoreBoundaryArea(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	s := anisotropicMeasuredBore(t)
	o := WrappedAnisotropicStationAreaOptions{AreaTolerance: big.NewRat(2, 1)}
	coarse, e := s.BoundWrappedAnisotropicStationBoreArea(ctx, o)
	if e != nil {
		t.Fatal(e)
	}
	o.AreaTolerance = big.NewRat(1, 2)
	q, e := s.BoundWrappedAnisotropicStationBoreArea(ctx, o)
	if e != nil {
		t.Fatal(e)
	}
	width := new(big.Rat).Sub(q.Total.Upper, q.Total.Lower)
	if width.Cmp(o.AreaTolerance) > 0 || width.Cmp(new(big.Rat).Sub(coarse.Total.Upper, coarse.Total.Lower)) >= 0 || q.Total.Lower.Cmp(coarse.Total.Lower) < 0 || q.Total.Upper.Cmp(coarse.Total.Upper) > 0 {
		t.Fatal("refinement")
	}
	var truth []float64
	for _, stations := range [][][3]float64{{{-4, 3, 1}, {-3, 1, 2}, {-1, 2, 1}}, {{-4, .5, .25}, {-2, .25, .5}, {-1, .375, .25}}} {
		for i := 0; i+1 < len(stations); i++ {
			a, b := independentBoreWallArea(stations[i], stations[i+1], 64, 32), independentBoreWallArea(stations[i], stations[i+1], 128, 64)
			if math.Abs(a-b) > 1e-5 {
				t.Fatal("independent convergence", a, b)
			}
			truth = append(truth, b)
		}
	}
	area, _, _ := independentStationProfileMoments([4]float64{.5, .75, .75, .75}, 8192)
	truth = append(truth, area*(3-.5*.25), area*(2-.375*.25))
	wantIDs := append(append(append([]int(nil), q.Source.OuterWalls...), q.Source.InnerWalls...), q.Source.Caps[:]...)
	if len(q.Faces) != len(s.faces) || len(q.Faces) != len(truth) {
		t.Fatal("boundary coverage")
	}
	total := 0.0
	sumLo, sumHi := new(big.Rat), new(big.Rat)
	for i, face := range q.Faces {
		lo, _ := face.Area.Lower.Float64()
		hi, _ := face.Area.Upper.Float64()
		if face.Face != wantIDs[i] || lo > truth[i] || hi < truth[i] {
			t.Fatal("original face enclosure", i, lo, truth[i], hi)
		}
		total += truth[i]
		sumLo.Add(sumLo, face.Area.Lower)
		sumHi.Add(sumHi, face.Area.Upper)
	}
	if sumLo.Cmp(q.Total.Lower) != 0 || sumHi.Cmp(q.Total.Upper) != 0 {
		t.Fatal("exact total")
	}
	lo, _ := q.Total.Lower.Float64()
	hi, _ := q.Total.Upper.Float64()
	t.Log("area interval truth cells segments operations", lo, hi, total, q.Cells, q.ProfileSegments, q.ExactOperations)
	exact := o
	exact.MaxOperations = q.ExactOperations
	repeat, e := s.BoundWrappedAnisotropicStationBoreArea(ctx, exact)
	if e != nil || !reflect.DeepEqual(q, repeat) {
		t.Fatal("exact cumulative budget", e)
	}
	for _, bad := range []WrappedAnisotropicStationAreaOptions{{AreaTolerance: o.AreaTolerance, MaxOperations: q.ExactOperations - 1}, {AreaTolerance: o.AreaTolerance, MaxCells: 16}, {AreaTolerance: o.AreaTolerance, MaxProfileSegments: 4}, {AreaTolerance: o.AreaTolerance, MaxDepth: 1}} {
		fail, e := s.BoundWrappedAnisotropicStationBoreArea(ctx, bad)
		if e == nil || fail.Total.Lower != nil || len(fail.Faces) != 0 || fail.Source.Complete {
			t.Fatal("atomic failure", e)
		}
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, e = s.BoundWrappedAnisotropicStationBoreArea(cancelled, o); e == nil {
		t.Fatal("cancel")
	}
}
