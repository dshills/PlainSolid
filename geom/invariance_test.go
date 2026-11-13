package geom

import (
	"math"
	"math/rand"
	"testing"
)

// bernsteinPoint is an independent direct Bernstein-polynomial oracle for
// rational Bezier curves; production evaluation uses homogeneous de Boor.
func bernsteinPoint(poles []Vec3, weights []float64, t float64) Vec3 {
	n := len(poles) - 1
	sum := Vec3{}
	den := 0.0
	choose := 1.0
	for i, p := range poles {
		basis := choose * math.Pow(t, float64(i)) * math.Pow(1-t, float64(n-i)) * weights[i]
		sum = sum.Add(p.Scale(basis))
		den += basis
		if i < n {
			choose *= float64(n-i) / float64(i+1)
		}
	}
	return sum.Scale(1 / den)
}
func TestRandomSplineEditAndAffineInvariance(t *testing.T) {
	r := rand.New(rand.NewSource(31951))
	for trial := 0; trial < 100; trial++ {
		degree := 1 + r.Intn(6)
		poles := make([]Vec3, degree+1)
		weights := make([]float64, len(poles))
		for i := range poles {
			poles[i] = Vec3{20*r.Float64() - 10, 20*r.Float64() - 10, 20*r.Float64() - 10}
			weights[i] = math.Pow(10, 4*r.Float64()-2)
		}
		c, e := NewBezier(poles, weights)
		if e != nil {
			t.Fatal(e)
		}
		split := .1 + .8*r.Float64()
		refined, e := c.InsertKnot(split, degree+1)
		if e != nil {
			t.Fatal(e)
		}
		left, right, e := c.Split(split)
		if e != nil {
			t.Fatal(e)
		}
		transform := Affine{{2, .2, -.3, 3}, {.4, -3, .2, -1}, {0, .1, 4, 2}}
		tp := make([]Vec3, len(poles))
		for i, p := range poles {
			tp[i] = transform.Apply(p)
		}
		transformed, e := NewBezier(tp, weights)
		if e != nil {
			t.Fatal(e)
		}
		for sample := 0; sample <= 20; sample++ {
			u := float64(sample) / 20
			want := bernsteinPoint(poles, weights, u)
			p, e := c.Eval(u)
			if e != nil {
				t.Fatal(e)
			}
			near(t, p, want, 1e-12)
			p, e = refined.Eval(u)
			if e != nil {
				t.Fatal(e)
			}
			near(t, p, want, 1e-12)
			piece := left
			if u >= split {
				piece = right
			}
			p, e = piece.Eval(u)
			if e != nil {
				t.Fatal(e)
			}
			near(t, p, want, 1e-12)
			p, e = transformed.Eval(u)
			if e != nil {
				t.Fatal(e)
			}
			near(t, p, transform.Apply(want), 5e-12)
		}
	}
}
func TestRandomAffineRoundTrip(t *testing.T) {
	r := rand.New(rand.NewSource(22191))
	for i := 0; i < 300; i++ {
		axis := Vec3{r.Float64(), r.Float64(), r.Float64()}
		rotation, e := Rotation(axis, r.Float64()*2*math.Pi)
		if e != nil {
			t.Fatal(e)
		}
		scale := Scaling(Vec3{.1 + 10*r.Float64(), .1 + 10*r.Float64(), -.1 - 10*r.Float64()})
		m := Translation(Vec3{r.Float64() * 100, r.Float64() * 100, r.Float64() * 100}).Mul(rotation).Mul(scale)
		inv, e := m.Inverse()
		if e != nil {
			t.Fatal(e)
		}
		p := Vec3{r.Float64() * 100, r.Float64() * 100, r.Float64() * 100}
		near(t, inv.Apply(m.Apply(p)), p, 3e-12)
		normal := Vec3{0, 0, 1}
		n, e := m.ApplyNormal(normal)
		if e != nil {
			t.Fatal(e)
		}
		if math.Abs(n.Dot(m.ApplyVector(Vec3{1, 0, 0}))) > 1e-12 || math.Abs(n.Dot(m.ApplyVector(Vec3{0, 1, 0}))) > 1e-12 {
			t.Fatal("normal not perpendicular")
		}
	}
}
