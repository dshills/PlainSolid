package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
	"testing"
)

func placeAnnulusTestSolid(t *testing.T, s Solid, projection int, shift geom.Vec3) Solid {
	t.Helper()
	ctx := context.Background()
	transform := geom.Affine{{0, 0, -1, shift.X}, {-1, 0, 0, shift.Y}, {0, 1, 0, shift.Z}}
	if projection == 2 {
		transform = geom.Affine{{-1, 0, 0, shift.X}, {0, 0, 1, shift.Y}, {0, 1, 0, shift.Z}}
	}
	var faces []SewFace
	for _, f := range s.Faces() {
		if exact, ok := f.Surface.(*geom.ExactBezierSurface); ok {
			patch, e := exact.ExactPatch().Transformed(transform)
			if e != nil {
				t.Fatal(e)
			}
			support, e := geom.NewExactBezierSurface(patch)
			if e != nil {
				t.Fatal(e)
			}
			face, e := ExactSplineFaceRectangle(ctx, support, geom.ParameterRectangle{}, f.Reversed, geom.SurfacePatchOptions{})
			if e != nil {
				t.Fatal(e)
			}
			faces = append(faces, face)
		} else {
			d := f.Surface.(*geom.BSplineSurface).Data()
			for i, p := range d.Poles {
				if projection == 2 {
					d.Poles[i] = geom.Vec3{X: -p.X, Y: p.Z, Z: p.Y}.Add(shift)
				} else {
					d.Poles[i] = geom.Vec3{X: -p.Z, Y: -p.X, Z: p.Y}.Add(shift)
				}
			}
			support, e := geom.NewBSplineSurface(d.DegreeU, d.DegreeV, d.CountU, d.CountV, d.Poles, d.KnotsU, d.KnotsV, d.Weights)
			if e != nil {
				t.Fatal(e)
			}
			face, e := RectangularSplineFace(support, f.Reversed)
			if e != nil {
				t.Fatal(e)
			}
			faces = append(faces, face)
		}
	}
	out, _, e := Sew(ctx, faces, SewOptions{Tolerance: 1e-9})
	if e != nil {
		t.Fatal(e)
	}
	return out
}

func TestCertifiedExactPlacedXZBooleanAnnulusFill(t *testing.T) {
	checkCertifiedBooleanAnnulusOwnerFill(t, 2, true)
}

func TestExactAnnulusOwnerPlacementProof(t *testing.T) {
	s, _ := makeOpenSplineBox(t, [3]float64{-.125, -.125, 0}, [3]float64{1.125, 1.125, 1}, 0)
	s = exactPlaneOwnerWithControl(t, s, big.NewRat(7, 24))
	s = placeAnnulusTestSolid(t, s, 2, geom.Vec3{X: 2, Y: -3, Z: 4})
	exact, ok := s.faces[4].Surface.(*geom.ExactBezierSurface)
	if !ok {
		t.Fatal("rounded exact source")
	}
	recipe, e := exact.ExactPatch().ExportExactBezierRecipe()
	if e != nil {
		t.Fatal(e)
	}
	found := false
	for _, h := range recipe.Controls {
		x, _ := new(big.Rat).SetString(h[0])
		w, _ := new(big.Rat).SetString(h[3])
		x.Quo(x, w)
		if x.Cmp(big.NewRat(41, 24)) == 0 {
			_, binary := x.Float64()
			if binary {
				t.Fatal("expected genuinely nonbinary control")
			}
			found = true
		}
	}
	if !found {
		t.Fatal("lost exact placed41/24")
	}
	u, v := big.NewRat(2, 7), big.NewRat(3, 11)
	at, e := exact.ExactPatch().EvaluateRational(context.Background(), u, v, geom.SurfacePatchEvaluationOptions{})
	if e != nil {
		t.Fatal(e)
	}
	// This stock chart retains physical U/V ranges [-1/8,9/8], not a unit reparameterization.
	want := [3]*big.Rat{new(big.Rat).Sub(big.NewRat(2, 1), u), big.NewRat(-3, 1), new(big.Rat).Add(big.NewRat(4, 1), v)}
	for i := range want {
		if at.Point[i].Cmp(want[i]) != 0 {
			t.Fatal("independent exact world placement", i, at.Point[i], want[i])
		}
	}
}
