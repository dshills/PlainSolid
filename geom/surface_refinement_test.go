package geom

import (
	"errors"
	"math/rand"
	"testing"
)

func TestSurfaceKnotRefinementRandomInvariance(t *testing.T) {
	random := rand.New(rand.NewSource(36131))
	for trial := 0; trial < 30; trial++ {
		poles := make([]Vec3, 12)
		weights := make([]float64, len(poles))
		for i := range poles {
			poles[i] = Vec3{random.Float64() * 6, random.Float64() * 6, random.Float64() * 6}
			weights[i] = .05 + 4*random.Float64()
		}
		surface, e := NewBSplineSurface(2, 3, 3, 4, poles, []float64{0, 0, 0, 1, 1, 1}, []float64{0, 0, 0, 0, 1, 1, 1, 1}, weights)
		if e != nil {
			t.Fatal(e)
		}
		uRefined, e := surface.InsertUKnot(.37, 3)
		if e != nil {
			t.Fatal(e)
		}
		refined, e := uRefined.InsertVKnot(.64, 4)
		if e != nil {
			t.Fatal(e)
		}
		if refined.countU != 6 || refined.countV != 8 {
			t.Fatal(refined.countU, refined.countV)
		}
		for i := 0; i <= 10; i++ {
			for j := 0; j <= 10; j++ {
				u, v := float64(i)/10, float64(j)/10
				point, _ := surface.Eval(u, v)
				got, e := refined.Eval(u, v)
				if e != nil {
					t.Fatal(e)
				}
				near(t, got, point, 1e-12)
				du, dv, _ := surface.Partials(u, v)
				ru, rv, e := refined.Partials(u, v)
				if e != nil {
					t.Fatal(e)
				}
				near(t, ru, du, 2e-10)
				near(t, rv, dv, 2e-10)
			}
		}
	}
}
func TestSurfaceKnotRepeatedAndLimits(t *testing.T) {
	surface := quarterCylinder(t)
	refined, e := surface.InsertUKnot(.5, 1)
	if e != nil {
		t.Fatal(e)
	}
	refined, e = refined.InsertUKnot(.5, 2)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = refined.InsertUKnot(.5, 1); !errors.Is(e, ErrInvalid) {
		t.Fatal(e)
	}
	copy, e := surface.InsertVKnot(.5, 0)
	if e != nil {
		t.Fatal(e)
	}
	copy.poles[0] = Vec3{99, 99, 99}
	if surface.poles[0] == copy.poles[0] {
		t.Fatal("alias")
	}
	for _, parameter := range []float64{0, 1, -1, 2} {
		if _, e := surface.InsertUKnot(parameter, 1); !errors.Is(e, ErrDomain) {
			t.Fatal(parameter, e)
		}
		if _, e := surface.InsertVKnot(parameter, 1); !errors.Is(e, ErrDomain) {
			t.Fatal(parameter, e)
		}
	}
}
