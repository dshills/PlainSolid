package trimmed

import (
	"bytes"
	"context"
	"math/big"
	"plainsolid/geom"
	"testing"
)

func TestNativeCanonicalToroidSignedProjection(t *testing.T) {
	ctx := context.Background()
	recipe, e := independentWrappedToroid(t).ExportRecipe()
	if e != nil {
		t.Fatal(e)
	}
	d := recipe.Surface.Data()
	weights := []float64{1, 1, 2, 2, 4, 2, 2, 1, 1}
	for i := 0; i < 9; i++ {
		for j := 0; j < 9; j++ {
			d.Weights[i*9+j] = weights[i] * weights[j]
		}
	}
	source, e := geom.NewBSplineSurface(d.DegreeU, d.DegreeV, d.CountU, d.CountV, d.Poles, d.KnotsU, d.KnotsV, d.Weights)
	if e != nil {
		t.Fatal(e)
	}
	patch, e := geom.NewSurfacePatch(source, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	w, _, e := geom.NewWrappedSurfacePatch(ctx, patch, recipe.Options)
	if e != nil {
		t.Fatal(e)
	}
	s, _, e := NewWrappedToroid(ctx, w, 1e-9, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}

	var buffer bytes.Buffer
	if e := WriteJSON(&buffer, s); e != nil {
		t.Fatal(e)
	}
	restored, e := ReadJSONContext(ctx, &buffer, JSONLimits{})
	if e != nil {
		t.Fatal(e)
	}
	for _, tc := range []struct {
		q        geom.Vec3
		location geom.ToroidMeridianLocation
		signed   *big.Rat
	}{
		{geom.Vec3{}, geom.ToroidMeridianOutside, big.NewRat(9, 4)},
		{geom.Vec3{X: 2}, geom.ToroidMeridianInside, big.NewRat(-1, 4)},
		{geom.Vec3{X: 1.5, Y: 2}, geom.ToroidMeridianBoundary, big.NewRat(0, 1)},
	} {
		for _, body := range []Solid{s, restored} {
			p, e := body.ProjectCanonicalCircularToroid(ctx, tc.q, geom.FrustumDistanceOptions{SquaredTolerance: 1e-12})
			if e != nil || p.Location != tc.location || p.SignedSquaredMin.Cmp(tc.signed) > 0 || p.SignedSquaredMax.Cmp(tc.signed) < 0 {
				t.Fatal(tc, p, e)
			}
			if tc.location == geom.ToroidMeridianInside && !p.Distance.CenterlineNonunique {
				t.Fatal("lost nonunique projection")
			}
		}
	}
	rejected := restored
	rejected.faces = restored.Faces()
	rejected.faces[0].Reversed = true
	p, e := rejected.ProjectCanonicalCircularToroid(ctx, geom.Vec3{}, geom.FrustumDistanceOptions{SquaredTolerance: 1e-12})
	if e == nil || p.SignedSquaredMin != nil {
		t.Fatal("unsupported altered solid", e)
	}
}
