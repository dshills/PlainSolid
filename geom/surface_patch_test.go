package geom

import (
	"errors"
	"math"
	"math/big"
	"math/rand"
	"sync"
	"testing"
)

func exactBernstein(degree int, t float64) []*big.Rat {
	x := new(big.Rat).SetFloat64(t)
	oneMinus := new(big.Rat).Sub(big.NewRat(1, 1), x)
	basis := make([]*big.Rat, degree+1)
	for i := range basis {
		v := new(big.Rat).SetInt(new(big.Int).Binomial(int64(degree), int64(i)))
		for j := 0; j < i; j++ {
			v.Mul(v, x)
		}
		for j := i; j < degree; j++ {
			v.Mul(v, oneMinus)
		}
		basis[i] = v
	}
	return basis
}
func exactBezierSurfacePoint(s *BSplineSurface, u, v float64) [3]*big.Rat {
	bu, bv := exactBernstein(s.degreeU, u), exactBernstein(s.degreeV, v)
	numerator := [3]*big.Rat{new(big.Rat), new(big.Rat), new(big.Rat)}
	denominator := new(big.Rat)
	for i := 0; i < s.countU; i++ {
		for j := 0; j < s.countV; j++ {
			index := i*s.countV + j
			weight := new(big.Rat).Mul(bu[i], bv[j])
			weight.Mul(weight, new(big.Rat).SetFloat64(s.weights[index]))
			denominator.Add(denominator, weight)
			for axis := 0; axis < 3; axis++ {
				numerator[axis].Add(numerator[axis], new(big.Rat).Mul(weight, new(big.Rat).SetFloat64(coordinate(s.poles[index], axis))))
			}
		}
	}
	for axis := range numerator {
		numerator[axis].Quo(numerator[axis], denominator)
	}
	return numerator
}
func assertExactPointInBox(t *testing.T, point [3]*big.Rat, box Box) {
	t.Helper()
	for axis := 0; axis < 3; axis++ {
		minimum, maximum := new(big.Rat).SetFloat64(coordinate(box.Min, axis)), new(big.Rat).SetFloat64(coordinate(box.Max, axis))
		if point[axis].Cmp(minimum) < 0 || point[axis].Cmp(maximum) > 0 {
			t.Fatal("exact point outside outward bounds", axis, box)
		}
	}
}

func BenchmarkExactSurfacePatchTrim(b *testing.B) {
	knots := []float64{0, 0, 0, 1, 1, 1}
	poles := []Vec3{{0, 0, 0}, {0, .5, 0}, {0, 1, 0}, {.5, 0, 0}, {.5, .5, .125}, {.5, 1, .25}, {1, 0, 0}, {1, .5, .25}, {1, 1, .5}}
	s, err := NewBSplineSurface(2, 2, 3, 3, poles, knots, knots, nil)
	if err != nil {
		b.Fatal(err)
	}
	p, err := NewSurfacePatch(s, SurfacePatchOptions{})
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := p.Trim(ParameterRectangle{.23, .72, .11, .64}); err != nil {
			b.Fatal(err)
		}
	}
}
func TestExactSurfacePatchTrimBounds(t *testing.T) {
	surface, _ := NewBSplineSurface(1, 1, 2, 2, []Vec3{{0, 0, 0}, {0, 2, 1}, {1, 0, 1}, {1, 2, 2}}, []float64{0, 0, 1, 1}, []float64{0, 0, 1, 1}, nil)
	patch, e := NewSurfacePatch(surface, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	rectangle := ParameterRectangle{.2, .7, .3, .8}
	trimmed, e := patch.Trim(rectangle)
	if e != nil {
		t.Fatal(e)
	}
	if trimmed.Domain() != rectangle {
		t.Fatal(trimmed.Domain())
	}
	box, e := trimmed.Bounds()
	if e != nil {
		t.Fatal(e)
	}
	near(t, box.Min, Vec3{.2, .6, .5}, 1e-15)
	near(t, box.Max, Vec3{.7, 1.6, 1.5}, 1e-15)
	for i := 0; i <= 8; i++ {
		for j := 0; j <= 8; j++ {
			u := rectangle.UMin + (rectangle.UMax-rectangle.UMin)*float64(i)/8
			v := rectangle.VMin + (rectangle.VMax-rectangle.VMin)*float64(j)/8
			assertExactPointInBox(t, exactBezierSurfacePoint(surface, u, v), box)
		}
	}
	left, right, e := trimmed.SplitU(.4)
	if e != nil {
		t.Fatal(e)
	}
	if left.Domain().UMax != .4 || right.Domain().UMin != .4 {
		t.Fatal(left.Domain(), right.Domain())
	}
	plane, _ := NewPlane(Vec3{0, 0, 2}, Vec3{0, 0, 1})
	rangeResult, e := trimmed.PlaneBounds(plane)
	if e != nil || rangeResult.Relation != PlanePatchBelow || rangeResult.Max > -.5 {
		t.Fatal(rangeResult, e)
	}
}
func TestRandomExactSurfacePatchBounds(t *testing.T) {
	random := rand.New(rand.NewSource(8861))
	for trial := 0; trial < 30; trial++ {
		poles := make([]Vec3, 9)
		weights := make([]float64, 9)
		for i := range poles {
			poles[i] = Vec3{random.Float64()*6 - 3, random.Float64()*6 - 3, random.Float64()*6 - 3}
			weights[i] = .1 + 4*random.Float64()
		}
		surface, _ := NewBSplineSurface(2, 2, 3, 3, poles, []float64{0, 0, 0, 1, 1, 1}, []float64{0, 0, 0, 1, 1, 1}, weights)
		patch, e := NewSurfacePatch(surface, SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
		rectangle := ParameterRectangle{.17, .83, .23, .77}
		trimmed, e := patch.Trim(rectangle)
		if e != nil {
			t.Fatal(e)
		}
		box, _ := trimmed.Bounds()
		for i := 0; i <= 4; i++ {
			for j := 0; j <= 4; j++ {
				u := rectangle.UMin + (rectangle.UMax-rectangle.UMin)*float64(i)/4
				v := rectangle.VMin + (rectangle.VMax-rectangle.VMin)*float64(j)/4
				assertExactPointInBox(t, exactBezierSurfacePoint(surface, u, v), box)
			}
		}
		plane, _ := NewPlane(Vec3{.1, -.2, .3}, Vec3{1, 2, 3})
		rangeResult, e := trimmed.PlaneBounds(plane)
		if e != nil {
			t.Fatal(e)
		}
		for _, uv := range [][2]float64{{.2, .3}, {.4, .6}, {.7, .7}} {
			point := exactBezierSurfacePoint(surface, uv[0], uv[1])
			distance := new(big.Rat)
			for axis := 0; axis < 3; axis++ {
				delta := new(big.Rat).Sub(point[axis], new(big.Rat).SetFloat64(coordinate(plane.origin, axis)))
				distance.Add(distance, new(big.Rat).Mul(delta, new(big.Rat).SetFloat64(coordinate(plane.normal, axis))))
			}
			if distance.Cmp(new(big.Rat).SetFloat64(rangeResult.Min)) < 0 || distance.Cmp(new(big.Rat).SetFloat64(rangeResult.Max)) > 0 {
				t.Fatal("plane range excludes exact point", rangeResult)
			}
		}
	}
}
func TestSurfacePatchExactTinySign(t *testing.T) {
	tiny := math.SmallestNonzeroFloat64
	surface, _ := NewBSplineSurface(1, 1, 2, 2, []Vec3{{0, 0, 0}, {0, 1, 0}, {1, 0, tiny}, {1, 1, tiny}}, []float64{0, 0, 1, 1}, []float64{0, 0, 1, 1}, nil)
	patch, e := NewSurfacePatch(surface, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	patch, e = patch.Trim(ParameterRectangle{.25, .5, 0, 1})
	if e != nil {
		t.Fatal(e)
	}
	plane, _ := NewPlane(Vec3{}, Vec3{0, 0, 1})
	bound, e := patch.PlaneBounds(plane)
	if e != nil || bound.Min != 0 || bound.Relation != PlanePatchAbove {
		t.Fatal(bound, e)
	}
}
func TestSurfacePatchLimitsAndDomain(t *testing.T) {
	surface := quarterCylinder(t)
	patch, e := NewSurfacePatch(surface, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	for _, rectangle := range []ParameterRectangle{{0, 0, 0, 1}, {-1, 1, 0, 1}, {0, 1, 0, 2}, {math.NaN(), 1, 0, 1}} {
		if _, e := patch.Trim(rectangle); e == nil {
			t.Fatal(rectangle)
		}
	}
	if _, e := NewSurfacePatch(surface, SurfacePatchOptions{MaxPoles: 4}); !errors.Is(e, ErrConvergence) {
		t.Fatal(e)
	}
	if _, e := NewSurfacePatch(surface, SurfacePatchOptions{MaxOperations: 1}); !errors.Is(e, ErrConvergence) {
		t.Fatal(e)
	}
	poles := append([]Vec3(nil), surface.poles...)
	poles[2].X = 1.1
	wideCoefficients, e := NewBSplineSurface(surface.degreeU, surface.degreeV, surface.countU, surface.countV, poles, surface.knotsU, surface.knotsV, surface.weights)
	if e != nil {
		t.Fatal(e)
	}
	if _, e := NewSurfacePatch(wideCoefficients, SurfacePatchOptions{MaxCoefficientBits: 64}); !errors.Is(e, ErrConvergence) {
		t.Fatal(e)
	}
	if _, _, e := patch.SplitU(0); !errors.Is(e, ErrDomain) {
		t.Fatal(e)
	}
	var zero SurfacePatch
	if _, e := zero.Bounds(); !errors.Is(e, ErrInvalid) {
		t.Fatal(e)
	}
}

func TestSurfacePatchImmutableConcurrentTrim(t *testing.T) {
	source := quarterCylinder(t)
	patch, e := NewSurfacePatch(source, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	original, _ := patch.Bounds()
	source.poles[0] = Vec3{999, 999, 999}
	source.weights[0] = .1
	plane, _ := NewPlane(Vec3{}, Vec3{1, 0, 0})
	var wait sync.WaitGroup
	for i := 0; i < 6; i++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			for j := 0; j < 4; j++ {
				trimmed, e := patch.Trim(ParameterRectangle{.2, .8, .1, .9})
				if e != nil {
					t.Error(e)
					return
				}
				if _, e = trimmed.PlaneBounds(plane); e != nil {
					t.Error(e)
				}
			}
		}()
	}
	wait.Wait()
	box, e := patch.Bounds()
	if e != nil || box != original {
		t.Fatal(box, e)
	}
}
