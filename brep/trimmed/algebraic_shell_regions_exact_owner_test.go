package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
	"testing"
)

func exactPlaneOwnerForRegions(t *testing.T, s Solid) Solid {
	return exactPlaneOwnerWithControl(t, s, big.NewRat(1, 3))
}
func exactPlaneOwnerWithControl(t *testing.T, s Solid, expected *big.Rat) Solid {
	t.Helper()
	ctx := context.Background()
	var faces []SewFace
	for i, f := range s.Faces() {
		surface := f.Surface.(*geom.BSplineSurface)
		if i != 4 {
			face, e := RectangularSplineFace(surface, f.Reversed)
			if e != nil {
				t.Fatal(e)
			}
			faces = append(faces, face)
			continue
		}
		patch, e := geom.NewSurfacePatch(surface, geom.SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
		data, e := patch.ExportExactBezierRecipe()
		if e != nil {
			t.Fatal(e)
		}
		nv := data.DegreeV + 1
		for degree := data.DegreeU; degree < 3; degree++ {
			controls := make([][4]string, (degree+2)*nv)
			for u := 0; u < degree+2; u++ {
				for v := 0; v < nv; v++ {
					for axis := 0; axis < 4; axis++ {
						q := new(big.Rat)
						if u > 0 {
							z, _ := new(big.Rat).SetString(data.Controls[(u-1)*nv+v][axis])
							q.Mul(z, big.NewRat(int64(u), int64(degree+1)))
						}
						if u < degree+1 {
							z, _ := new(big.Rat).SetString(data.Controls[u*nv+v][axis])
							q.Add(q, z.Mul(z, big.NewRat(int64(degree+1-u), int64(degree+1))))
						}
						controls[u*nv+v][axis] = q.RatString()
					}
				}
			}
			data.DegreeU = degree + 1
			data.Controls = controls
		}
		nonbinary := false
		foundControl := false
		for _, h := range data.Controls {
			w, _ := new(big.Rat).SetString(h[3])
			for _, value := range h[:3] {
				q, _ := new(big.Rat).SetString(value)
				q.Quo(q, w)
				_, binary := q.Float64()
				nonbinary = nonbinary || !binary
				foundControl = foundControl || q.Cmp(expected) == 0
			}
		}
		if !nonbinary || !foundControl {
			t.Fatal("plane does not contain expected nonbinary control", expected)
		}
		patch, e = geom.ImportExactBezierSurfaceRecipe(ctx, data, geom.SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
		exact, e := geom.NewExactBezierSurface(patch)
		if e != nil {
			t.Fatal(e)
		}
		face, e := ExactSplineFaceRectangle(ctx, exact, geom.ParameterRectangle{}, f.Reversed, geom.SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
		faces = append(faces, face)
	}
	out, _, e := Sew(ctx, faces, SewOptions{Tolerance: 1e-9})
	if e != nil {
		t.Fatal(e)
	}
	if _, ok := out.faces[4].Surface.(*geom.ExactBezierSurface); !ok {
		t.Fatal("source adapter lost")
	}
	return out
}
