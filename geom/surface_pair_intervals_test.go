package geom

import (
	"errors"
	"math/rand"
	"testing"
)

func TestSurfacePairIntervalsAffineSection(t *testing.T) {
	a := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, v, 0} })
	b := arcTestPatch(t, func(s, t float64) Vec3 { return Vec3{s, .5, t - .5} })
	r, e := BoundSurfacePairDifferentials(a, b, SurfacePairIntervalOptions{UseFixedParameter: true, FixedAxis: 0, FixedParameter: .25})
	if e != nil || r.ExcludesContact {
		t.Fatal(r, e)
	}
	expected := [3][4]float64{{1, 0, -1, 0}, {0, 1, 0, 0}, {0, 0, 0, -1}}
	for i := 0; i < 3; i++ {
		for j := 0; j < 4; j++ {
			v := r.Jacobian[i][j]
			if v.Min != expected[i][j] || v.Max != expected[i][j] {
				t.Fatal(i, j, v)
			}
		}
	}
	if r.Residual[0].Min != -.75 || r.Residual[0].Max != .25 {
		t.Fatal(r)
	}
}
func TestSurfacePairIntervalsRationalSamples(t *testing.T) {
	s := quarterCylinder(t)
	a, _ := NewSurfacePatch(s, SurfacePatchOptions{})
	c := quadraticContactSurface(t, -.1)
	b, _ := NewSurfacePatch(c, SurfacePatchOptions{})
	for fixed := -1; fixed < 4; fixed++ {
		o := SurfacePairIntervalOptions{}
		if fixed >= 0 {
			o.UseFixedParameter = true
			o.FixedAxis = fixed
			o.FixedParameter = .37
		}
		r, e := BoundSurfacePairDifferentials(a, b, o)
		if e != nil {
			t.Fatal(e)
		}
		rng := rand.New(rand.NewSource(821))
		for k := 0; k < 50; k++ {
			uv := [4]float64{rng.Float64(), rng.Float64(), rng.Float64(), rng.Float64()}
			if fixed >= 0 {
				uv[fixed] = .37
			}
			p, _ := s.Eval(uv[0], uv[1])
			q, _ := c.Eval(uv[2], uv[3])
			su, sv, _ := s.Partials(uv[0], uv[1])
			tu, tv, _ := c.Partials(uv[2], uv[3])
			derivatives := [4]Vec3{su, sv, tu.Scale(-1), tv.Scale(-1)}
			for axis := 0; axis < 3; axis++ {
				value := coordinate(p.Sub(q), axis)
				bound := r.Residual[axis]
				if value < bound.Min-1e-12 || value > bound.Max+1e-12 {
					t.Fatal(value, bound)
				}
				for j, d := range derivatives {
					value := coordinate(d, axis)
					bound := r.Jacobian[axis][j]
					if value < bound.Min-1e-12 || value > bound.Max+1e-12 {
						t.Fatal(value, bound)
					}
				}
			}
		}
	}
}
func TestSurfacePairIntervalsExclusionAndLimits(t *testing.T) {
	a := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, v, 0} })
	b := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, v, 2} })
	r, e := BoundSurfacePairDifferentials(a, b, SurfacePairIntervalOptions{})
	if e != nil || !r.ExcludesContact {
		t.Fatal(r, e)
	}
	if _, e = BoundSurfacePairDifferentials(a, b, SurfacePairIntervalOptions{MaxOperations: 1}); !errors.Is(e, ErrConvergence) {
		t.Fatal(e)
	}
	if _, e = BoundSurfacePairDifferentials(a, b, SurfacePairIntervalOptions{UseFixedParameter: true, FixedAxis: 2, FixedParameter: 2}); !errors.Is(e, ErrDomain) {
		t.Fatal(e)
	}
}
