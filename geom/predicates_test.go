package geom

import (
	"math"
	"math/big"
	"math/rand"
	"testing"
)

// homogeneousOracle uses rational Gaussian elimination on homogeneous
// coordinates, independently of the translated Laplace determinant in code.
func homogeneousOracle(points []Vec3, lifted bool) int {
	n := len(points)
	a := make([][]*big.Rat, n)
	for i, p := range points {
		a[i] = make([]*big.Rat, n)
		a[i][0] = new(big.Rat).SetFloat64(p.X)
		a[i][1] = new(big.Rat).SetFloat64(p.Y)
		a[i][2] = new(big.Rat).SetFloat64(p.Z)
		if lifted {
			norm := new(big.Rat)
			for j := 0; j < 3; j++ {
				norm.Add(norm, new(big.Rat).Mul(a[i][j], a[i][j]))
			}
			a[i][3] = norm
		}
		a[i][n-1] = big.NewRat(1, 1)
	}
	sign := 1
	for k := 0; k < n; k++ {
		pivot := k
		for pivot < n && a[pivot][k].Sign() == 0 {
			pivot++
		}
		if pivot == n {
			return 0
		}
		if pivot != k {
			a[k], a[pivot] = a[pivot], a[k]
			sign = -sign
		}
		sign *= a[k][k].Sign()
		for i := k + 1; i < n; i++ {
			factor := new(big.Rat).Quo(a[i][k], a[k][k])
			for j := k + 1; j < n; j++ {
				a[i][j].Sub(a[i][j], new(big.Rat).Mul(factor, a[k][j]))
			}
			a[i][k] = new(big.Rat)
		}
	}
	return sign
}
func TestSpatialPredicates(t *testing.T) {
	a, b, c, d := Vec3{}, Vec3{1, 0, 0}, Vec3{0, 1, 0}, Vec3{0, 0, 1}
	orientation, e := Orient3D(a, b, c, d)
	if e != nil || orientation != -1 {
		t.Fatal(orientation, e)
	}
	for _, tt := range []struct {
		p    Vec3
		want int
	}{{Vec3{.5, .5, .5}, -1}, {Vec3{1, 1, 1}, 0}, {Vec3{2, 2, 2}, 1}, {Vec3{1, 1, math.Nextafter(1, 2)}, 1}, {Vec3{1, 1, math.Nextafter(1, 0)}, -1}} {
		got, e := InSphere(a, b, c, d, tt.p)
		if e != nil || got != tt.want {
			t.Fatal(tt, got, e)
		}
	}
	zero, e := Orient3D(a, b, c, Vec3{.1, .2, 0})
	if e != nil || zero != 0 {
		t.Fatal(zero, e)
	}
	for _, scale := range []float64{1e-300, 1e-150, 1, 1e150, 1e300} {
		points := []Vec3{a.Scale(scale), b.Scale(scale), c.Scale(scale), d.Scale(scale), Vec3{.5, .5, .5}.Scale(scale)}
		o, e := Orient3D(points[0], points[1], points[2], points[3])
		if e != nil || o != -1 {
			t.Fatal(scale, o, e)
		}
		s, e := InSphere(points[0], points[1], points[2], points[3], points[4])
		if e != nil || s != -1 {
			t.Fatal(scale, s, e)
		}
	}
	if _, e := Orient3D(Vec3{math.NaN(), 0, 0}, b, c, d); e == nil {
		t.Fatal("nan")
	}
	if _, e := InSphere(a, b, c, d, Vec3{math.Inf(1), 0, 0}); e == nil {
		t.Fatal("inf")
	}
}
func TestPredicateOracleRandom(t *testing.T) {
	r := rand.New(rand.NewSource(50129))
	for trial := 0; trial < 350; trial++ {
		points := make([]Vec3, 5)
		for i := range points {
			var v [3]float64
			for j := range v {
				exponent := r.Intn(2001) - 1000
				if trial%2 == 0 {
					exponent = trial%1200 - 600
				}
				v[j] = math.Ldexp(r.Float64()*2-1, exponent)
			}
			points[i] = Vec3{v[0], v[1], v[2]}
		}
		if trial%7 == 0 {
			points[4] = points[3]
		}
		o, e := Orient3D(points[0], points[1], points[2], points[3])
		if want := homogeneousOracle(points[:4], false); e != nil || o != want {
			t.Fatal("orientation", trial, o, want, e)
		}
		s, e := InSphere(points[0], points[1], points[2], points[3], points[4])
		if want := homogeneousOracle(points, true); e != nil || s != want {
			t.Fatal("sphere", trial, s, want, e)
		}
		swapped, e := InSphere(points[1], points[0], points[2], points[3], points[4])
		if e != nil || swapped != -s {
			t.Fatal("antisymmetry", trial, s, swapped, e)
		}
	}
}
func FuzzOrient3DAntisymmetry(f *testing.F) {
	f.Add(1.0, 2.0, 3.0, 4.0)
	f.Fuzz(func(t *testing.T, x, y, z, w float64) {
		a, b, c, d := Vec3{x, 0, w}, Vec3{0, y, 1}, Vec3{1, 0, z}, Vec3{0, 0, w}
		o, e := Orient3D(a, b, c, d)
		if e != nil {
			return
		}
		s, e := Orient3D(b, a, c, d)
		if e != nil || s != -o {
			t.Fatal(o, s, e)
		}
	})
}
