package trimmed

import (
	"context"
	"math"
	"math/big"
	"plainsolid/geom"
	"plainsolid/internal/slowtest"
	"reflect"
	"testing"
)

// Independent tensor Simpson integration of the explicitly authored original
// quadratic quotient and its quotient-rule derivatives. No certified-area or
// surface derivative helper is used in this numerical comparison.
func independentAnisotropicWallArea(band, n, m int) float64 {
	scales := [][2]float64{{3, 1}, {1, 2}, {2, 1}}
	heights := []float64{0, 1, 3}
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
func TestAnisotropicStationArea(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	s := anisotropicMassFixture(t, false, geom.Vec3{})
	o := WrappedAnisotropicStationAreaOptions{AreaTolerance: big.NewRat(2, 1)}
	coarse, e := s.BoundWrappedAnisotropicStationArea(ctx, o)
	if e != nil {
		t.Fatal(e)
	}
	o.AreaTolerance = big.NewRat(1, 2)
	q, e := s.BoundWrappedAnisotropicStationArea(ctx, o)
	if e != nil {
		t.Fatal(e)
	}
	width := new(big.Rat).Sub(q.Total.Upper, q.Total.Lower)
	if width.Cmp(o.AreaTolerance) > 0 || width.Cmp(new(big.Rat).Sub(coarse.Total.Upper, coarse.Total.Lower)) >= 0 {
		t.Fatal("width convergence")
	}
	if q.Total.Lower.Cmp(coarse.Total.Lower) < 0 || q.Total.Upper.Cmp(coarse.Total.Upper) > 0 {
		t.Fatal("nested fixture enclosures")
	}
	truth := []float64{}
	for band := 0; band < 2; band++ {
		a, c := independentAnisotropicWallArea(band, 64, 32), independentAnisotropicWallArea(band, 128, 64)
		if math.Abs(a-c) > 1e-5 {
			t.Fatal("independent integration unresolved", a, c)
		}
		truth = append(truth, c)
	}
	profile := rationalQuarterProfileArea(.75, 8192)
	truth = append(truth, profile*3, profile*2)
	if len(q.Faces) != 4 {
		t.Fatal("boundary omissions")
	}
	sumLo, sumHi := new(big.Rat), new(big.Rat)
	total := 0.
	for i, face := range q.Faces {
		want := q.Source.History.CapFaces[max(0, i-2)]
		if i < 2 {
			want = q.Source.History.WallFaces[i]
		}
		if face.Face != want {
			t.Fatal("original face owner")
		}
		lo, _ := face.Area.Lower.Float64()
		hi, _ := face.Area.Upper.Float64()
		if !(lo <= truth[i] && truth[i] <= hi) {
			t.Fatal("independent face", i, lo, truth[i], hi)
		}
		total += truth[i]
		sumLo.Add(sumLo, face.Area.Lower)
		sumHi.Add(sumHi, face.Area.Upper)
	}
	if sumLo.Cmp(q.Total.Lower) != 0 || sumHi.Cmp(q.Total.Upper) != 0 {
		t.Fatal("sum")
	}
	lo, _ := q.Total.Lower.Float64()
	hi, _ := q.Total.Upper.Float64()
	t.Logf("[%g,%g] independent=%g cells=%d segments=%d operations=%d", lo, hi, total, q.Cells, q.ProfileSegments, q.ExactOperations)
	exact := o
	exact.MaxOperations = q.ExactOperations
	v, e := s.BoundWrappedAnisotropicStationArea(ctx, exact)
	if e != nil || !reflect.DeepEqual(q, v) {
		t.Fatal("exact work", e)
	}
	for _, bad := range []WrappedAnisotropicStationAreaOptions{{AreaTolerance: o.AreaTolerance, MaxOperations: q.ExactOperations - 1}, {AreaTolerance: o.AreaTolerance, MaxCells: 8}, {AreaTolerance: o.AreaTolerance, MaxProfileSegments: 4}, {AreaTolerance: o.AreaTolerance, MaxDepth: 1}} {
		v, e = s.BoundWrappedAnisotropicStationArea(ctx, bad)
		if e == nil || v.Total.Lower != nil || len(v.Faces) != 0 {
			t.Fatal("partial limit")
		}
	}
	q.Faces[0].Area.Lower.SetInt64(-1)
	if coarse.Faces[0].Area.Lower.Sign() < 0 {
		t.Fatal("aliased result")
	}
	c, cancel := context.WithCancel(ctx)
	cancel()
	if _, e = s.BoundWrappedAnisotropicStationArea(c, o); e == nil {
		t.Fatal("cancel")
	}
}

func TestAnisotropicAreaRectangleCell(t *testing.T) {
	b := &stationMassBudget{ctx: context.Background(), limit: 100000, bits: 16384}
	r := big.NewRat
	q := stationMassSegment{{r(0, 1), r(0, 1), r(1, 1)}, {r(1, 2), r(0, 1), r(1, 1)}, {r(1, 1), r(0, 1), r(1, 1)}}
	p := anisotropicAreaProfileFrom(q, b)
	got := p.enclose([2]*big.Rat{r(1, 1), r(1, 1)}, [2]*big.Rat{r(1, 1), r(1, 1)}, r(2, 1), b)
	if b.err != nil || got.Lower.Cmp(r(2, 1)) != 0 || got.Upper.Cmp(r(2, 1)) != 0 {
		t.Fatalf("exact rectangle %+v %v", got, b.err)
	}
}
