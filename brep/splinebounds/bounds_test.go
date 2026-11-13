package splinebounds

import (
	"context"
	"errors"
	"math"
	"math/big"
	"plainsolid/geom"
	"testing"
)

func patch(t *testing.T, p, q int, points []geom.Vec3, w []float64) *geom.BSplineSurface {
	t.Helper()
	ku, kv := make([]float64, 2*(p+1)), make([]float64, 2*(q+1))
	for i := p + 1; i < len(ku); i++ {
		ku[i] = 1
	}
	for i := q + 1; i < len(kv); i++ {
		kv[i] = 1
	}
	s, e := geom.NewBSplineSurface(p, q, p+1, q+1, points, ku, kv, w)
	if e != nil {
		t.Fatal(e)
	}
	return s
}
func TestPlaneAndBilinear(t *testing.T) {
	s := patch(t, 1, 1, []geom.Vec3{{}, {Y: 1}, {X: 1}, {X: 1, Y: 1}}, nil)
	r, e := Whole(context.Background(), s, Options{})
	if e != nil {
		t.Fatal(e)
	}
	if r.DU != 1 || r.DV != 1 || r.DUU != 0 || r.DUV != 0 || r.DVV != 0 || r.WeightLower != 1 {
		t.Fatal(r)
	}
	s = patch(t, 1, 1, []geom.Vec3{{}, {Y: 1}, {X: 1}, {X: 1, Y: 1, Z: 1}}, nil)
	r, e = Whole(context.Background(), s, Options{})
	if e != nil {
		t.Fatal(e)
	}
	if r.DUU != 0 || r.DVV != 0 || r.DUV != 1 || r.DU < math.Sqrt(2) || r.DV < math.Sqrt(2) {
		t.Fatal(r)
	}
}
func TestQuadraticPolynomialAndParameterScale(t *testing.T) {
	var points []geom.Vec3
	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			z := 3 * float64(i*j) / 4
			if i == 2 {
				z++
			}
			if j == 2 {
				z += 2
			}
			points = append(points, geom.Vec3{X: float64(i) / 2, Y: float64(j) / 2, Z: z})
		}
	}
	s := patch(t, 2, 2, points, nil)
	r, e := Whole(context.Background(), s, Options{})
	if e != nil {
		t.Fatal(e)
	}
	if r.DUU != 2 || r.DUV != 3 || r.DVV != 4 {
		t.Fatal(r)
	}
	d := s.Data()
	d.KnotsU = []float64{2, 2, 2, 4, 4, 4}
	d.KnotsV = []float64{5, 5, 5, 9, 9, 9}
	r, e = FromData(context.Background(), d, Domain{2, 4, 5, 9}, Options{})
	if e != nil || r.DUU != .5 || r.DUV != .375 || r.DVV != .25 {
		t.Fatal(r, e)
	}
}
func bernstein(n, i int, t float64) float64 {
	if i < 0 || i > n {
		return 0
	}
	c := 1.
	for j := 1; j <= i; j++ {
		c *= float64(n-j+1) / float64(j)
	}
	return c * math.Pow(t, float64(i)) * math.Pow(1-t, float64(n-i))
}
func basis(n, i, derivative int, t float64) float64 {
	if derivative == 0 {
		return bernstein(n, i, t)
	}
	if derivative > n {
		return 0
	}
	return float64(n) * (basis(n-1, i-1, derivative-1, t) - basis(n-1, i, derivative-1, t))
}

// Independent direct Bernstein-basis differentiation, without control-net
// hulls, samples the real quotient identities using ordinary float arithmetic.
func sampled(d geom.BSplineSurfaceData, u, v float64) [5]geom.Vec3 {
	orders := [][2]int{{0, 0}, {1, 0}, {0, 1}, {2, 0}, {1, 1}, {0, 2}}
	var h [6][4]float64
	for k, o := range orders {
		for i := 0; i < d.CountU; i++ {
			for j := 0; j < d.CountV; j++ {
				p := d.Poles[i*d.CountV+j]
				w := d.Weights[i*d.CountV+j]
				a := w * basis(d.DegreeU, i, o[0], u) * basis(d.DegreeV, j, o[1], v)
				for c, x := range []float64{p.X, p.Y, p.Z, 1} {
					h[k][c] += a * x
				}
			}
		}
	}
	var r [5]geom.Vec3
	for c := 0; c < 3; c++ {
		s := h[0][c] / h[0][3]
		du := (h[1][c] - s*h[1][3]) / h[0][3]
		dv := (h[2][c] - s*h[2][3]) / h[0][3]
		values := []float64{du, dv, (h[3][c] - 2*du*h[1][3] - s*h[3][3]) / h[0][3], (h[4][c] - du*h[2][3] - dv*h[1][3] - s*h[4][3]) / h[0][3], (h[5][c] - 2*dv*h[2][3] - s*h[5][3]) / h[0][3]}
		for i, x := range values {
			switch c {
			case 0:
				r[i].X = x
			case 1:
				r[i].Y = x
			case 2:
				r[i].Z = x
			}
		}
	}
	return r
}
func TestRationalPatchSamplesAndTranslation(t *testing.T) {
	points := []geom.Vec3{{X: 1}, {X: 1, Z: 2}, {X: 1, Y: 1}, {X: 1, Y: 1, Z: 2}, {Y: 1}, {Y: 1, Z: 2}}
	w := []float64{1, 1, math.Sqrt(.5), math.Sqrt(.5), 1, 1}
	s := patch(t, 2, 1, points, w)
	r, e := Whole(context.Background(), s, Options{})
	if e != nil {
		t.Fatal(e)
	}
	bounds := []float64{r.DU, r.DV, r.DUU, r.DUV, r.DVV}
	if r.WeightLower != math.Sqrt(.5) || r.WeightUpper != 1 {
		t.Fatal("weight certificate")
	}
	for i := 0; i <= 30; i++ {
		for j := 0; j <= 30; j++ {
			values := sampled(s.Data(), float64(i)/30, float64(j)/30)
			for k, v := range values {
				if v.Length() > bounds[k]*(1+1e-13)+1e-12 {
					t.Fatal("sample escaped bound", k, v, bounds[k])
				}
			}
		}
	}
	for i := range points {
		points[i] = points[i].Add(geom.Vec3{X: 1048576, Y: -1048576, Z: 1048576})
	}
	translated := patch(t, 2, 1, points, w)
	rr, e := Whole(context.Background(), translated, Options{})
	if e != nil || rr.DU != r.DU || rr.DV != r.DV || rr.DUU != r.DUU || rr.DUV != r.DUV || rr.DVV != r.DVV {
		t.Fatal("translation-dependent bounds", e)
	}
}
func TestConstantRationalPatch(t *testing.T) {
	s := patch(t, 1, 1, []geom.Vec3{{X: 7, Y: 8, Z: 9}, {X: 7, Y: 8, Z: 9}, {X: 7, Y: 8, Z: 9}, {X: 7, Y: 8, Z: 9}}, []float64{1, 2, 3, 4})
	r, e := Whole(context.Background(), s, Options{})
	if e != nil || r.DU != 0 || r.DV != 0 || r.DUU != 0 || r.DUV != 0 || r.DVV != 0 {
		t.Fatal(r, e)
	}
}
func TestKnotsContinuityAndTrim(t *testing.T) {
	d := geom.BSplineSurfaceData{DegreeU: 2, DegreeV: 1, CountU: 4, CountV: 2, KnotsU: []float64{0, 0, 0, .5, 1, 1, 1}, KnotsV: []float64{0, 0, 1, 1}, Poles: make([]geom.Vec3, 8)}
	for i := range d.Poles {
		d.Poles[i] = geom.Vec3{X: float64(i / 2), Y: float64(i % 2)}
	}
	if _, e := FromData(context.Background(), d, Domain{0, 1, 0, 1}, Options{}); !errors.Is(e, ErrUnsupported) {
		t.Fatal(e)
	}
	r, e := FromData(context.Background(), d, Domain{0, .5, .2, .8}, Options{})
	if e != nil || r.WeightLower != 1 || r.ActiveControls != 6 {
		t.Fatal(r, e)
	}
	if _, e = FromData(context.Background(), d, Domain{.1, .9, 0, 1}, Options{}); !errors.Is(e, ErrUnsupported) {
		t.Fatal(e)
	}
	d.KnotsU = []float64{-1, 0, 0, .5, 1, 1, 2}
	if _, e = FromData(context.Background(), d, Domain{0, 1, 0, 1}, Options{}); !errors.Is(e, ErrUnsupported) {
		t.Fatal(e)
	}
}
func TestInvalidAndLimits(t *testing.T) {
	s := patch(t, 1, 1, []geom.Vec3{{}, {Y: 1}, {X: 2}, {X: 2, Y: 1}}, nil)
	for _, o := range []Options{{MaxControls: 1}, {MaxWork: 1}, {MaxBits: 1}} {
		if _, e := Whole(context.Background(), s, o); !errors.Is(e, ErrLimit) {
			t.Fatal(e)
		}
	}
	for _, o := range []Options{{MaxWork: -1}, {MaxDegree: 65}} {
		if _, e := Whole(context.Background(), s, o); !errors.Is(e, ErrInvalid) {
			t.Fatal(e)
		}
	}
	d := s.Data()
	d.Weights[0] = 0
	if _, e := FromData(context.Background(), d, Domain{0, 1, 0, 1}, Options{}); !errors.Is(e, ErrInvalid) {
		t.Fatal(e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e := Whole(ctx, s, Options{}); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
	if _, e := Bound(context.Background(), s, Domain{0, 2, 0, 1}, Options{}); !errors.Is(e, ErrInvalid) {
		t.Fatal(e)
	}
}

func TestCubicInteriorKnotAndActiveWeightHull(t *testing.T) {
	d := geom.BSplineSurfaceData{DegreeU: 3, DegreeV: 1, CountU: 5, CountV: 2, KnotsU: []float64{0, 0, 0, 0, .5, 1, 1, 1, 1}, KnotsV: []float64{0, 0, 1, 1}}
	for i := 0; i < 5; i++ {
		for j := 0; j < 2; j++ {
			d.Poles = append(d.Poles, geom.Vec3{X: float64(i), Y: float64(j), Z: float64(i*i) / 4})
			w := 1.
			if i == 0 {
				w = 1e-8
			}
			d.Weights = append(d.Weights, w)
		}
	}
	whole, e := FromData(context.Background(), d, Domain{0, 1, 0, 1}, Options{})
	if e != nil {
		t.Fatal(e)
	}
	trim, e := FromData(context.Background(), d, Domain{.5, 1, .25, .75}, Options{})
	if e != nil {
		t.Fatal(e)
	}
	if whole.WeightLower != 1e-8 || trim.WeightLower != 1 || trim.ActiveControls != 8 || !(trim.DUU < whole.DUU) {
		t.Fatal("active support restriction", whole, trim)
	}
	s, e := geom.NewBSplineSurface(d.DegreeU, d.DegreeV, d.CountU, d.CountV, d.Poles, d.KnotsU, d.KnotsV, d.Weights)
	if e != nil {
		t.Fatal(e)
	}
	for i := 0; i < 30; i++ {
		u := .5 + (float64(i)+.5)/60
		du, dv, e := s.Partials(u, .5)
		if e != nil || du.Length() > trim.DU*(1+1e-13) || dv.Length() > trim.DV*(1+1e-13) {
			t.Fatal("first derivative exceeded hull", e)
		}
		h := 1e-6
		before, _, _ := s.Partials(u-h, .5)
		after, _, _ := s.Partials(u+h, .5)
		if after.Sub(before).Scale(1/(2*h)).Length() > trim.DUU*(1+1e-7) {
			t.Fatal("sampled second derivative exceeded hull")
		}
	}
}
func TestOutwardNormRoundingAndRange(t *testing.T) {
	x := big.NewRat(2, 1)
	v, e := sqrtUpper(x)
	if e != nil || mul(number(v), number(v)).Cmp(x) < 0 {
		t.Fatal(v, e)
	}
	tiny := div(number(math.SmallestNonzeroFloat64), big.NewRat(2, 1))
	v, e = sqrtUpper(mul(tiny, tiny))
	if e != nil || v != math.SmallestNonzeroFloat64 {
		t.Fatal(v, e)
	}
	huge := mul(number(math.MaxFloat64), big.NewRat(2, 1))
	if _, e = sqrtUpper(mul(huge, huge)); !errors.Is(e, ErrRange) {
		t.Fatal(e)
	}
	d := geom.BSplineSurfaceData{DegreeU: 1, DegreeV: 1, CountU: 2, CountV: 2, KnotsU: []float64{0, 0, 1, 1}, KnotsV: []float64{0, 0, 1, 1}, Poles: []geom.Vec3{{X: -math.MaxFloat64}, {X: -math.MaxFloat64, Y: 1}, {X: math.MaxFloat64}, {X: math.MaxFloat64, Y: 1}}}
	if _, e = FromData(context.Background(), d, Domain{0, 1, 0, 1}, Options{}); !errors.Is(e, ErrRange) {
		t.Fatal(e)
	}
}
func FuzzRationalBezierBounds(f *testing.F) {
	f.Add([]byte{1, 2, 3, 4, 5, 6, 7, 8, 9})
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) < 9 {
			return
		}
		var poles []geom.Vec3
		var weights []float64
		for i := 0; i < 3; i++ {
			for j := 0; j < 3; j++ {
				x := data[3*i+j]
				poles = append(poles, geom.Vec3{X: float64(i), Y: float64(j), Z: float64(int8(x)) / 16})
				weights = append(weights, float64(x%16+1)/8)
			}
		}
		s := patch(t, 2, 2, poles, weights)
		r, e := Whole(context.Background(), s, Options{})
		if e != nil {
			t.Fatal(e)
		}
		bounds := []float64{r.DU, r.DV, r.DUU, r.DUV, r.DVV}
		for _, u := range []float64{0, .25, .5, .75, 1} {
			for _, v := range []float64{0, .25, .5, .75, 1} {
				for k, d := range sampled(s.Data(), u, v) {
					if d.Length() > bounds[k]*(1+1e-12)+1e-10 {
						t.Fatal("sample escaped certificate", k, d, bounds[k])
					}
				}
			}
		}
	})
}
