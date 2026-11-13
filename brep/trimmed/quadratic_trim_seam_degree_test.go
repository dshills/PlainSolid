package trimmed

import (
	"context"
	"math"
	"math/big"
	"plainsolid/geom"
	"testing"
)

func TestExactDifferentDegreeSeamRestrictedReversedDomains(t *testing.T) {
	// A maps S(u,v)=(u,v,uv). B maps (u,v,u²-uv+v²). They agree
	// on u=v and differ elsewhere; both are genuine curved supports.
	k := []float64{0, 0, 3, 3}
	sa, e := geom.NewBSplineSurface(1, 1, 2, 2, []geom.Vec3{{}, {Y: 3}, {X: 3}, {X: 3, Y: 3, Z: 9}}, k, k, nil)
	if e != nil {
		t.Fatal(e)
	}
	a, e := geom.NewSurfacePatch(sa, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	var poles []geom.Vec3
	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			u, v := float64(i)*1.5, float64(j)*1.5
			z := -u * v
			if i == 2 {
				z += 9
			}
			if j == 2 {
				z += 9
			}
			poles = append(poles, geom.Vec3{X: u, Y: v, Z: z})
		}
	}
	knots := []float64{0, 0, 0, 3, 3, 3}
	sb, e := geom.NewBSplineSurface(2, 2, 3, 3, poles, knots, knots, nil)
	if e != nil {
		t.Fatal(e)
	}
	b, e := geom.NewSurfacePatch(sb, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	// Exact homogeneous elevation: (0,0,1),(3/2,3/2,1/2)
	// becomes (0,0,1),(3/4,3/4,3/4),(3/2,3/2,1/2).
	// Its Euclidean middle pole is exactly(1,1), so every value is binary
	// representable. Author the reversed elevated curve directly, without a
	// floating degree-elevation or approximate fitting routine.
	ca, e := geom.NewBSpline(1, []geom.Vec3{{}, {X: 3, Y: 3}}, []float64{-4, -4, 12, 12}, []float64{1, .5})
	if e != nil {
		t.Fatal(e)
	}
	elevated := []geom.Vec3{{X: 3, Y: 3}, {X: 1, Y: 1}, {}}
	cb, e := geom.NewBSpline(2, elevated, []float64{20, 20, 20, 52, 52, 52}, []float64{.5, .75, 1})
	if e != nil {
		t.Fatal(e)
	}
	ta := SplineUV{Curve: ca, Range: UVInterval{Min: 0, Max: 8}}
	tb := SplineUV{Curve: cb, Range: UVInterval{Min: 28, Max: 44}, Reversed: true}
	result, e := CertifyQuadraticTrimSeam(context.Background(), a, b, ta, tb, QuadraticTrimSeamOptions{Tessellation: QuadraticTrimTessellationOptions{Deflection: .03}})
	if e != nil {
		t.Fatal(e)
	}
	if result.ParameterMaps[0].Degree != 1 || result.ParameterMaps[1].Degree != 2 || result.ParameterMaps[0].Offset.Sign() != 0 || result.ParameterMaps[0].Scale.Cmp(big.NewRat(8, 1)) != 0 || result.ParameterMaps[1].Offset.Cmp(big.NewRat(44, 1)) != 0 || result.ParameterMaps[1].Scale.Cmp(big.NewRat(-16, 1)) != 0 {
		t.Fatal("unproved affine parameter map", result.ParameterMaps)
	}
	first, last := result.ExactPoints[0], result.ExactPoints[len(result.ExactPoints)-1]
	if first[0].Cmp(big.NewRat(3, 7)) != 0 || first[2].Cmp(big.NewRat(9, 49)) != 0 || last[0].Cmp(big.NewRat(9, 5)) != 0 || last[2].Cmp(big.NewRat(81, 25)) != 0 {
		t.Fatal("lost exact restricted endpoints", first, last)
	}
	for i, p := range result.ExactPoints {
		if p[0].Cmp(p[1]) != 0 || p[2].Cmp(new(big.Rat).Mul(p[0], p[0])) != 0 {
			t.Fatal("independent parabola model")
		}
		if result.SourceParameters[1][i] != 44-2*result.SourceParameters[0][i] {
			t.Fatal("affine/reverse correspondence")
		}
	}
	altered := append([]geom.Vec3(nil), elevated...)
	altered[1].X = math.Nextafter(1, 2)
	wrong, e := geom.NewBSpline(2, altered, []float64{20, 20, 20, 52, 52, 52}, []float64{.5, .75, 1})
	if e != nil {
		t.Fatal(e)
	}
	badTrim := tb
	badTrim.Curve = wrong
	bad, e := CertifyQuadraticTrimSeam(context.Background(), a, b, ta, badTrim, QuadraticTrimSeamOptions{Tessellation: QuadraticTrimTessellationOptions{Deflection: .03}})
	if e == nil || len(bad.Points) != 0 {
		t.Fatal("one-ULP approximate elevation treated as exact")
	}
	t.Logf("degree1/2 exact reversed-range seam: %d shared points", len(result.Points))
}
