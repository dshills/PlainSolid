package trimmed

import (
	"bytes"
	"context"
	"plainsolid/geom"
	"testing"
)

func anisotropicMassFixture(t *testing.T, asymmetric bool, translation geom.Vec3) Solid {
	t.Helper()
	ctx := context.Background()
	bands := []*geom.WrappedSurfacePatch{anisotropicStationBand(t, 0, 1, [2]float64{3, 1}, [2]float64{1, 2}, .75), anisotropicStationBand(t, 1, 3, [2]float64{1, 2}, [2]float64{2, 1}, .75)}
	for i, w := range bands {
		recipe, e := w.ExportRecipe()
		if e != nil {
			t.Fatal(e)
		}
		d := recipe.Surface.Data()
		if asymmetric {
			d.Weights[2], d.Weights[3] = .5, .5
		}
		for j := range d.Poles {
			d.Poles[j] = d.Poles[j].Add(translation)
		}
		s, e := geom.NewBSplineSurface(d.DegreeU, d.DegreeV, d.CountU, d.CountV, d.Poles, d.KnotsU, d.KnotsV, d.Weights)
		if e != nil {
			t.Fatal(e)
		}
		patch, e := geom.NewSurfacePatch(s, geom.SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
		bands[i], _, e = geom.NewWrappedSurfacePatch(ctx, patch, geom.WrappedSurfaceOptions{WrapU: true})
		if e != nil {
			t.Fatal(e)
		}
	}
	body, _, e := NewWrappedAnisotropicStationLoft(ctx, bands, 1e-8, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	var raw bytes.Buffer
	if e = WriteJSON(&raw, body); e != nil {
		t.Fatal(e)
	}
	body, e = ReadJSONContext(ctx, &raw, JSONLimits{})
	if e != nil {
		t.Fatal(e)
	}
	return body
}
